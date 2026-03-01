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

	env, err := bootstrap()
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

	return fixLoop(c.Context, fixOpts{
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
	})
}

// checkFunc is the signature for running a governance check pass.
// Returns a report and error; (nil, nil) signals user interrupt.
type checkFunc func(ctx context.Context, opts fixOpts) (*orchestrator.Report, error)

// fixOpts holds dependencies for the fix loop, enabling testability.
type fixOpts struct {
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

	// runCheck overrides the default runFixCheck implementation.
	// Used by tests to inject mock check results.
	runCheck checkFunc
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

// fixLoop implements the check-fix-recheck loop with injected dependencies.
func fixLoop(ctx context.Context, opts fixOpts) error {
	doCheck := opts.runCheck
	if doCheck == nil {
		doCheck = runFixCheck
	}

	fmt.Println("═══ bonsai fix: initial check ═══")
	report, err := doCheck(ctx, opts)
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

	for iteration := 1; iteration <= opts.maxIterations; iteration++ {
		report, err = fixIteration(ctx, opts, doCheck, report, iteration)
		if err != nil {
			return err
		}
		if report == nil {
			return nil // resolved or interrupted
		}
	}

	return fmt.Errorf("fix loop exited unexpectedly")
}

// fixIteration runs one fix-then-recheck cycle. Returns nil report on
// success or TUI interrupt. Returns non-nil report with findings on failure.
func fixIteration(
	ctx context.Context,
	opts fixOpts,
	doCheck checkFunc,
	report *orchestrator.Report,
	iteration int,
) (*orchestrator.Report, error) {
	failedSkills := extractPerSkillFindings(report, opts.skills)
	if len(failedSkills) == 0 {
		fmt.Println("\n✔ No findings — nothing to fix")
		return nil, nil //nolint:nilnil // nil report signals "resolved" to caller
	}

	fmt.Printf("\n═══ Fix iteration %d/%d — %d skill(s) to fix ═══\n", iteration, opts.maxIterations, len(failedSkills))

	if err := runFixSessions(ctx, opts, failedSkills); err != nil {
		return nil, err
	}

	report, err := recheckAfterFix(ctx, opts, doCheck, iteration)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, nil //nolint:nilnil // nil report signals TUI interrupt to caller
	}

	if !report.ShouldFail() {
		fmt.Printf("\n✔ All findings resolved (%d/%d skills passed)\n", report.Passed, report.Total)
		saveFixArtifacts(opts.repoRoot, opts.config, report)
		return nil, nil //nolint:nilnil // nil report signals "resolved" to caller
	}

	if iteration == opts.maxIterations {
		fmt.Fprintf(os.Stderr, "\n✖ Findings remain after %d fix iterations\n", opts.maxIterations)
		printDetailedFindings(report)
		return nil, fmt.Errorf("findings remain after %d fix iterations", opts.maxIterations)
	}

	fmt.Printf("\n%d finding(s) remain — continuing to next iteration\n", report.BlockingFailed)
	return report, nil
}

// runFixSessions builds a system prompt and invokes autonomous fix sessions
// for each failed skill.
func runFixSessions(ctx context.Context, opts fixOpts, failedSkills []skillFindings) error {
	builder := prompt.NewBuilder(opts.resolver, opts.repoRoot)
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
		if opts.config != nil {
			model = opts.config.Models.ModelForSkill(string(sf.Cost))
		}

		if sessErr := opts.sessionAgent.Autonomous(ctx, systemPrompt, sf.UserPrompt(), agent.Model(model)); sessErr != nil {
			if ctx.Err() != nil {
				return nil //nolint:nilerr // cancellation is intentional clean exit
			}
			fmt.Fprintf(os.Stderr, "warning: fix session for %s exited with error: %v\n", sf.Name, sessErr)
		}
	}
	return nil
}

// recheckAfterFix runs a governance check after a fix iteration.
func recheckAfterFix(
	ctx context.Context,
	opts fixOpts,
	doCheck checkFunc,
	iteration int,
) (*orchestrator.Report, error) {
	fmt.Printf("\n═══ Re-check after fix iteration %d/%d ═══\n", iteration, opts.maxIterations)
	report, err := doCheck(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("re-check: %w", err)
	}
	return report, nil
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

// runFixCheck runs the orchestrator check with the given skill set.
// Uses TUI when opts.useTUI is true; falls back to LoggerSink otherwise.
func runFixCheck(ctx context.Context, opts fixOpts) (*orchestrator.Report, error) {
	orch := orchestrator.New(opts.checkAgent, opts.resolver)
	runOpts := orchestrator.RunOpts{
		Skills:              opts.skills,
		Source:              opts.source,
		BaseRef:             opts.baseRef,
		FailFast:            false,
		RepoRoot:            opts.repoRoot,
		Config:              opts.config,
		DefaultRequiresDiff: opts.registry.Defaults.EffectiveRequiresDiff(),
		Concurrency:         0, // unlimited
	}

	if opts.useTUI {
		orchCtx, orchCancel := context.WithCancel(ctx)
		defer orchCancel()

		events := make(chan orchestrator.Event, len(opts.skills)*4)
		var report *orchestrator.Report
		var runErr error
		orchDone := make(chan struct{})
		go func() {
			report, runErr = orch.Run(orchCtx, runOpts, events)
			close(events)
			close(orchDone)
		}()

		tuiReport, tuiErr := tui.RunWithTUI(events, opts.source)

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

	// Plain text output via LoggerSink
	sink, sinkDone := orchestrator.LoggerSink(func(msg string) { fmt.Println(msg) })
	report, err := orch.Run(ctx, runOpts, sink)
	close(sink)
	<-sinkDone

	return report, err
}

// severityPairs maps severity labels to Result detail accessors.
var severityPairs = []struct {
	label string
	get   func(*orchestrator.Result) []string
}{
	{"blocking", func(r *orchestrator.Result) []string { return r.BlockingDetails }},
	{"major", func(r *orchestrator.Result) []string { return r.MajorDetails }},
	{"warning", func(r *orchestrator.Result) []string { return r.WarningDetails }},
}

// formatDetails returns severity-prefixed lines for a result's findings.
func formatDetails(r *orchestrator.Result, prefix string) []string {
	var lines []string
	for _, sp := range severityPairs {
		for _, d := range sp.get(r) {
			lines = append(lines, prefix+sp.label+": "+d)
		}
	}
	return lines
}

// failedResults returns pointers to all results with non-zero exit codes.
func failedResults(report *orchestrator.Report) []*orchestrator.Result {
	var failed []*orchestrator.Result
	for i := range report.Results {
		if report.Results[i].ExitCode != 0 {
			failed = append(failed, &report.Results[i])
		}
	}
	return failed
}

// extractDetailedFindings builds a findings context string with full detail
// strings, not just counts.
func extractDetailedFindings(report *orchestrator.Report) string {
	var sections []string
	for _, r := range failedResults(report) {
		lines := append([]string{"SKILL: " + r.Name}, formatDetails(r, "  ")...)
		sections = append(sections, strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

// extractPerSkillFindings groups failed results into per-skill findings
// for targeted autonomous fix passes.
func extractPerSkillFindings(report *orchestrator.Report, skills []registry.Skill) []skillFindings {
	costByName := make(map[string]registry.Cost, len(skills))
	for i := range skills {
		costByName[skills[i].Name] = skills[i].Cost
	}

	var results []skillFindings
	for _, r := range failedResults(report) {
		results = append(results, skillFindings{
			Name:  r.Name,
			Cost:  costByName[r.Name],
			Lines: formatDetails(r, ""),
		})
	}
	return results
}

// printDetailedFindings prints failed findings to stderr.
func printDetailedFindings(report *orchestrator.Report) {
	for _, r := range failedResults(report) {
		fmt.Fprintf(os.Stderr, "  SKILL: %s | blocking:%d major:%d warning:%d\n",
			r.Name, r.Blocking, r.Major, r.Warning)
		for _, line := range formatDetails(r, "    ") {
			fmt.Fprintln(os.Stderr, line)
		}
	}
}

// saveFixArtifacts saves the fix report.
func saveFixArtifacts(repoRoot string, cfg *config.Config, report *orchestrator.Report) {
	outDir := filepath.Join(repoRoot, cfg.Output.Dir)
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
