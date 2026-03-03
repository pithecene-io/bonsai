package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	urfave "github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
	"github.com/pithecene-io/bonsai/internal/tui"
)

func fixCommand() *urfave.Command {
	return &urfave.Command{
		Name:  "fix",
		Usage: "Fix governance findings autonomously",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "bundle", Value: "default", Usage: "Bundle name"},
			&urfave.StringFlag{Name: "base", Usage: "Git ref for diff context"},
			&urfave.IntFlag{Name: "max-iterations", Usage: "Max fix iterations (default: config or 3)"},
			&urfave.BoolFlag{Name: "no-progress", Usage: "Disable TUI progress display"},
		},
		Action: runFix,
	}
}

func runFix(c *urfave.Context) error {
	bundle := c.String("bundle")
	baseRef := c.String("base")
	noProgress := c.Bool("no-progress")

	repoRoot := detectRepoRoot()
	wt, err := ensureFeatureBranch(repoRoot, "fix")
	if err != nil {
		return err
	}
	defer printWorktreeReminder(wt)

	env, err := bootstrapFrom(wt.RepoRoot)
	if err != nil {
		return err
	}

	// Resolve max-iterations: flag > config > default
	maxIter := env.Config.Fix.MaxIterations
	if c.IsSet("max-iterations") {
		maxIter = c.Int("max-iterations")
	}
	if maxIter <= 0 {
		maxIter = 3
	}

	// Resolve skill set — bundle-based, then filter to cheap-only
	allSkills, err := env.Registry.SkillsForBundle(bundle)
	if err != nil {
		return err
	}

	skills := filterCheapSkills(allSkills)
	if len(skills) == 0 {
		fmt.Println("No cheap-cost skills in bundle — nothing to fix")
		return nil
	}

	// Detect merge base for diff context
	if baseRef == "" {
		baseRef = repo.DetectMergeBase(env.RepoRoot, env.Config.Routing.MergeBaseCandidates)
	}

	agentRouter := newAgentRouter(env.Config)

	// TTY detection: use TUI if stdout is a terminal and --no-progress is not set
	useTUI := term.IsTerminal(int(os.Stdout.Fd())) && !noProgress

	fl := &fixLoop{
		checkAgent:    agentRouter,
		sessionAgent:  agentRouter,
		resolver:      env.Resolver,
		registry:      env.Registry,
		config:        env.Config,
		skills:        skills,
		source:        "bundle:" + bundle + " (cheap-only)",
		baseRef:       baseRef,
		repoRoot:      env.RepoRoot,
		maxIterations: maxIter,
		useTUI:        useTUI,
	}
	return fl.run(c.Context)
}

// fixLoop encapsulates the check-fix-recheck loop and its dependencies.
type fixLoop struct {
	checkAgent    agent.Agent // non-interactive agent for running checks
	sessionAgent  agent.Agent // autonomous agent for fix sessions
	resolver      *assets.Resolver
	registry      *registry.Registry
	config        *config.Config
	skills        []registry.Skill
	source        string
	baseRef       string
	repoRoot      string
	maxIterations int
	useTUI        bool

	// checker overrides the default check implementation.
	// Used by tests to inject mock check results.
	checker func(ctx context.Context) (*orchestrator.Report, error)
}

// skillFindings groups findings for a single failed skill.
type skillFindings struct {
	Name  string
	Cost  registry.Cost
	Lines []string // detail lines for user prompt
}

// UserPrompt builds the user-facing prompt containing this skill's findings.
func (sf skillFindings) UserPrompt() string {
	var b strings.Builder
	b.WriteString("Fix the following governance findings for skill: " + sf.Name + "\n\n")
	for _, l := range sf.Lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	b.WriteString("\nFix all findings listed above. Do not introduce unrelated changes.")
	return b.String()
}

// run implements the check-fix-recheck loop.
func (fl *fixLoop) run(ctx context.Context) error {
	fmt.Println("═══ bonsai fix: initial check ═══")
	report, err := fl.check(ctx)
	if err != nil {
		return fmt.Errorf("initial check: %w", err)
	}
	if report == nil {
		return nil // TUI interrupted
	}
	if !report.ShouldFail() {
		fmt.Println("\n✔ No findings — nothing to fix")
		return nil
	}

	for iteration := 1; iteration <= fl.maxIterations; iteration++ {
		report, err = fl.iterate(ctx, report, iteration)
		if err != nil {
			return err
		}
		if report == nil {
			return nil // resolved or interrupted
		}
	}

	return fmt.Errorf("fix loop exited unexpectedly")
}

// iterate runs one fix-then-recheck cycle. Returns nil report on
// success or TUI interrupt. Returns non-nil report with findings on failure.
func (fl *fixLoop) iterate(ctx context.Context, report *orchestrator.Report, iteration int) (*orchestrator.Report, error) {
	failedSkills := fl.perSkillFindings(report)
	if len(failedSkills) == 0 {
		fmt.Println("\n✔ No findings — nothing to fix")
		return nil, nil //nolint:nilnil // nil report signals "resolved" to caller
	}

	fmt.Printf("\n═══ Fix iteration %d/%d — %d skill(s) to fix ═══\n", iteration, fl.maxIterations, len(failedSkills))

	if err := fl.fixSessions(ctx, failedSkills); err != nil {
		return nil, err
	}

	fmt.Printf("\n═══ Re-check after fix iteration %d/%d ═══\n", iteration, fl.maxIterations)
	report, err := fl.check(ctx)
	if err != nil {
		return nil, fmt.Errorf("re-check: %w", err)
	}
	if report == nil {
		return nil, nil //nolint:nilnil // nil report signals TUI interrupt to caller
	}

	if !report.ShouldFail() {
		fmt.Printf("\n✔ All findings resolved (%d/%d skills passed)\n", report.Passed, report.Total)
		fl.saveArtifacts(report)
		return nil, nil //nolint:nilnil // nil report signals "resolved" to caller
	}

	if iteration == fl.maxIterations {
		fmt.Fprintf(os.Stderr, "\n✖ Findings remain after %d fix iterations\n", fl.maxIterations)
		report.PrintFindings(os.Stderr)
		return nil, fmt.Errorf("findings remain after %d fix iterations", fl.maxIterations)
	}

	fmt.Printf("\n%d finding(s) remain — continuing to next iteration\n", report.BlockingFailed)
	return report, nil
}

// fixSessions builds a system prompt and invokes autonomous fix sessions
// for each failed skill.
func (fl *fixLoop) fixSessions(ctx context.Context, failedSkills []skillFindings) error {
	builder := prompt.NewBuilder(fl.resolver, fl.repoRoot)
	systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeImplementer,
		Role: "implementer",
	})
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	for i, sf := range failedSkills {
		fmt.Printf("\n═══ Fixing: %s (%d/%d) ═══\n\n", sf.Name, i+1, len(failedSkills))

		model := ""
		if fl.config != nil {
			model = fl.config.Models.ModelForSkill(string(sf.Cost))
		}

		if sessErr := fl.sessionAgent.Execute(ctx, systemPrompt, sf.UserPrompt(), agent.Model(model)); sessErr != nil {
			if ctx.Err() != nil {
				return nil //nolint:nilerr // cancellation is intentional clean exit
			}
			fmt.Fprintf(os.Stderr, "warning: fix session for %s exited with error: %v\n", sf.Name, sessErr)
		}
	}
	return nil
}

// check runs the orchestrator with the configured skill set.
func (fl *fixLoop) check(ctx context.Context) (*orchestrator.Report, error) {
	if fl.checker != nil {
		return fl.checker(ctx)
	}

	orch := orchestrator.New(fl.checkAgent, fl.resolver)
	runOpts := orchestrator.RunOpts{
		Skills:              fl.skills,
		Source:              fl.source,
		BaseRef:             fl.baseRef,
		FailFast:            false,
		RepoRoot:            fl.repoRoot,
		Config:              fl.config,
		DefaultRequiresDiff: fl.registry.Defaults.EffectiveRequiresDiff(),
		Concurrency:         0, // unlimited
	}

	if fl.useTUI {
		return fl.checkTUI(ctx, orch, runOpts)
	}

	return orch.RunWithLogger(ctx, runOpts, nil)
}

func (fl *fixLoop) checkTUI(ctx context.Context, orch *orchestrator.Orchestrator, runOpts orchestrator.RunOpts) (*orchestrator.Report, error) {
	orchCtx, orchCancel := context.WithCancel(ctx)
	defer orchCancel()

	events := make(chan orchestrator.Event, len(fl.skills)*4)
	var report *orchestrator.Report
	var runErr error
	orchDone := make(chan struct{})
	go func() {
		report, runErr = orch.Run(orchCtx, runOpts, events)
		close(events)
		close(orchDone)
	}()

	tuiReport, tuiErr := tui.RunWithTUI(events, fl.source)

	orchCancel()
	<-orchDone

	if errors.Is(tuiErr, tui.ErrInterrupted) {
		fmt.Fprintln(os.Stderr, "\n⚠ check interrupted by user")
		return nil, nil //nolint:nilnil // interrupt returns clean nil to signal early exit
	}
	if tuiErr != nil {
		return nil, tuiErr
	}
	if runErr != nil {
		return nil, runErr
	}
	if tuiReport != nil {
		report = tuiReport
	}
	return report, nil
}

// perSkillFindings groups failed results into per-skill findings
// for targeted autonomous fix passes.
func (fl *fixLoop) perSkillFindings(report *orchestrator.Report) []skillFindings {
	costByName := make(map[string]registry.Cost, len(fl.skills))
	for i := range fl.skills {
		costByName[fl.skills[i].Name] = fl.skills[i].Cost
	}

	var results []skillFindings
	for _, r := range report.FailedResults() {
		results = append(results, skillFindings{
			Name:  r.Name,
			Cost:  costByName[r.Name],
			Lines: r.Details(""),
		})
	}
	return results
}

// saveArtifacts saves the fix report.
func (fl *fixLoop) saveArtifacts(report *orchestrator.Report) {
	outDir := filepath.Join(fl.repoRoot, fl.config.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create output dir: %v\n", err)
		return
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to marshal report: %v\n", err)
		return
	}

	reportPath := filepath.Join(outDir, "fix.report.json")
	if err := os.WriteFile(reportPath, reportJSON, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save report: %v\n", err)
	} else {
		fmt.Printf("Saved: %s\n", reportPath)
	}
}

// filterCheapSkills returns only skills with cost == CostCheap.
func filterCheapSkills(skills []registry.Skill) []registry.Skill {
	var cheap []registry.Skill
	for i := range skills {
		if skills[i].Cost == registry.CostCheap {
			cheap = append(cheap, skills[i])
		}
	}
	return cheap
}
