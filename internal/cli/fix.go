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
	"github.com/pithecene-io/bonsai/internal/gitutil"
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

	// Detect repo
	repoRoot := "."
	if gitutil.IsInsideWorkTree(".") {
		if r, err := gitutil.ShowToplevel("."); err == nil {
			repoRoot = r
		}
	}

	// Load config
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Resolve max-iterations: flag > config > default
	maxIter := cfg.Fix.MaxIterations
	if c.IsSet("max-iterations") {
		maxIter = c.Int("max-iterations")
	}
	if maxIter <= 0 {
		maxIter = 3
	}

	// Create resolver
	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs

	// Load registry
	reg, err := registry.Load(resolver)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Resolve skill set — bundle-based, then filter to cheap-only
	allSkills, err := reg.SkillsForBundle(bundle)
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
		baseRef = repo.DetectMergeBase(repoRoot, cfg.Routing.MergeBaseCandidates)
	}

	// Create agents
	var apiOpts []agent.AnthropicOption
	if cfg.Agents.Anthropic.APIKey != "" {
		apiOpts = append(apiOpts, agent.WithAPIKey(cfg.Agents.Anthropic.APIKey))
	}
	agentRouter := agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin, apiOpts...)

	// TTY detection: use TUI if stdout is a terminal and --no-progress is not set
	useTUI := term.IsTerminal(int(os.Stdout.Fd())) && !noProgress

	return fixLoop(c.Context, fixOpts{
		checkAgent:    agentRouter,
		sessionAgent:  agentRouter,
		resolver:      resolver,
		registry:      reg,
		config:        cfg,
		skills:        skills,
		source:        "bundle:" + bundle + " (cheap-only)",
		baseRef:       baseRef,
		repoRoot:      repoRoot,
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
	Cost  string
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

	// ═══ Initial check ═══
	fmt.Println("═══ bonsai fix: initial check ═══")
	report, err := doCheck(ctx, opts)
	if err != nil {
		return fmt.Errorf("initial check: %w", err)
	}

	if report == nil {
		// TUI interrupted — clean exit
		return nil
	}

	if !report.ShouldFail() {
		fmt.Println("\n✔ No findings — nothing to fix")
		return nil
	}

	// ═══ Fix loop ═══
	for iteration := 1; iteration <= opts.maxIterations; iteration++ {
		failedSkills := extractPerSkillFindings(report, opts.skills)
		if len(failedSkills) == 0 {
			fmt.Println("\n✔ No findings — nothing to fix")
			return nil
		}

		fmt.Printf("\n═══ Fix iteration %d/%d — %d skill(s) to fix ═══\n", iteration, opts.maxIterations, len(failedSkills))

		// Build system prompt once per iteration (same for all skills)
		builder := prompt.NewBuilder(opts.resolver, opts.repoRoot)
		systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
			Mode: prompt.ModeImplementer,
			Role: "implementer",
		})
		if err != nil {
			return fmt.Errorf("build prompt: %w", err)
		}

		// Per-skill autonomous fix
		for i, sf := range failedSkills {
			fmt.Printf("\n═══ Fixing: %s (%d/%d) ═══\n\n", sf.Name, i+1, len(failedSkills))

			// Model from cost tier config — same routing as check
			model := ""
			if opts.config != nil {
				model = opts.config.Agents.Models.ModelForCheck(sf.Cost)
			}

			if sessErr := opts.sessionAgent.Autonomous(ctx, systemPrompt, sf.UserPrompt(), model); sessErr != nil {
				if ctx.Err() != nil {
					return nil //nolint:nilerr // cancellation is intentional clean exit
				}
				fmt.Fprintf(os.Stderr, "warning: fix session for %s exited with error: %v\n", sf.Name, sessErr)
			}
		}

		// Re-check
		fmt.Printf("\n═══ Re-check after fix iteration %d/%d ═══\n", iteration, opts.maxIterations)
		report, err = doCheck(ctx, opts)
		if err != nil {
			return fmt.Errorf("re-check: %w", err)
		}

		if report == nil {
			// TUI interrupted — clean exit
			return nil
		}

		if !report.ShouldFail() {
			fmt.Printf("\n✔ All findings resolved (%d/%d skills passed)\n", report.Passed, report.Total)
			saveFixArtifacts(opts.repoRoot, opts.config, report)
			return nil
		}

		if iteration == opts.maxIterations {
			fmt.Fprintf(os.Stderr, "\n✖ Findings remain after %d fix iterations\n", opts.maxIterations)
			printDetailedFindings(report)
			return fmt.Errorf("findings remain after %d fix iterations", opts.maxIterations)
		}

		fmt.Printf("\n%d finding(s) remain — continuing to next iteration\n", report.BlockingFailed)
	}

	return fmt.Errorf("fix loop exited unexpectedly")
}

// filterCheapSkills returns only skills with cost == "cheap".
func filterCheapSkills(skills []registry.Skill) []registry.Skill {
	var cheap []registry.Skill
	for i := range skills {
		if skills[i].Cost == "cheap" {
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

// extractDetailedFindings builds a findings context string with full detail
// strings, not just counts. This gives the AI enough context to know WHAT to fix.
func extractDetailedFindings(report *orchestrator.Report) string {
	var sections []string
	for i := range report.Results {
		r := &report.Results[i]
		if r.ExitCode == 0 {
			continue
		}
		var lines []string
		lines = append(lines, fmt.Sprintf("SKILL: %s", r.Name))
		for _, d := range r.BlockingDetails {
			lines = append(lines, fmt.Sprintf("  blocking: %s", d))
		}
		for _, d := range r.MajorDetails {
			lines = append(lines, fmt.Sprintf("  major: %s", d))
		}
		for _, d := range r.WarningDetails {
			lines = append(lines, fmt.Sprintf("  warning: %s", d))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

// extractPerSkillFindings groups failed results into per-skill findings
// for targeted autonomous fix passes. The skills slice provides cost
// information not stored in orchestrator.Result.
func extractPerSkillFindings(report *orchestrator.Report, skills []registry.Skill) []skillFindings {
	// Build name→cost lookup
	costByName := make(map[string]string, len(skills))
	for i := range skills {
		costByName[skills[i].Name] = skills[i].Cost
	}

	var results []skillFindings
	for i := range report.Results {
		r := &report.Results[i]
		if r.ExitCode == 0 {
			continue
		}
		sf := skillFindings{
			Name: r.Name,
			Cost: costByName[r.Name],
		}
		for _, d := range r.BlockingDetails {
			sf.Lines = append(sf.Lines, fmt.Sprintf("blocking: %s", d))
		}
		for _, d := range r.MajorDetails {
			sf.Lines = append(sf.Lines, fmt.Sprintf("major: %s", d))
		}
		for _, d := range r.WarningDetails {
			sf.Lines = append(sf.Lines, fmt.Sprintf("warning: %s", d))
		}
		results = append(results, sf)
	}
	return results
}

// printDetailedFindings prints failed findings to stderr.
func printDetailedFindings(report *orchestrator.Report) {
	for i := range report.Results {
		r := &report.Results[i]
		if r.ExitCode == 0 {
			continue
		}
		fmt.Fprintf(os.Stderr, "  SKILL: %s | blocking:%d major:%d warning:%d\n",
			r.Name, r.Blocking, r.Major, r.Warning)
		for _, d := range r.BlockingDetails {
			fmt.Fprintf(os.Stderr, "    blocking: %s\n", d)
		}
		for _, d := range r.MajorDetails {
			fmt.Fprintf(os.Stderr, "    major: %s\n", d)
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
