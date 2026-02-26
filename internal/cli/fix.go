package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	urfave "github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
)

func fixCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "fix",
		Usage:     "Fix governance findings interactively",
		ArgsUsage: "[-- claude-args...]",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "bundle", Value: "default", Usage: "Bundle name"},
			&urfave.StringFlag{Name: "base", Usage: "Git ref for diff context"},
			&urfave.IntFlag{Name: "max-iterations", Value: 3, Usage: "Max fix iterations"},
			&urfave.StringFlag{Name: "model", Usage: "Override model for fix session"},
		},
		Action: runFix,
	}
}

func runFix(c *urfave.Context) error {
	bundle := c.String("bundle")
	baseRef := c.String("base")
	maxIter := c.Int("max-iterations")
	modelOverride := c.String("model")

	if maxIter <= 0 {
		maxIter = 3
	}

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
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)

	return fixLoop(c.Context, fixOpts{
		checkAgent:    agentRouter,
		sessionAgent:  claudeAgent,
		resolver:      resolver,
		registry:      reg,
		config:        cfg,
		skills:        skills,
		source:        "bundle:" + bundle + " (cheap-only)",
		baseRef:       baseRef,
		repoRoot:      repoRoot,
		maxIterations: maxIter,
		modelOverride: modelOverride,
		extraArgs:     c.Args().Slice(),
	}, runFixCheck)
}

// fixOpts holds dependencies for the fix loop, enabling testability.
type fixOpts struct {
	checkAgent    agent.Agent // non-interactive agent for running checks
	sessionAgent  agent.Agent // interactive agent for fix sessions
	resolver      *assets.Resolver
	registry      *registry.Registry
	config        *config.Config
	skills        []registry.Skill
	source        string
	baseRef       string
	repoRoot      string
	maxIterations int
	modelOverride string
	extraArgs     []string
}

// checkFunc is the signature for running a governance check pass.
type checkFunc func(
	ctx context.Context, a agent.Agent, resolver *assets.Resolver,
	skills []registry.Skill, source, baseRef, repoRoot string,
	cfg *config.Config, reg *registry.Registry,
) (*orchestrator.Report, error)

// fixLoop implements the check-fix-recheck loop with injected dependencies.
// doCheck is the function used to run each check pass; production callers
// pass runFixCheck, tests may substitute a stub.
func fixLoop(ctx context.Context, opts fixOpts, doCheck checkFunc) error {
	// ═══ Initial check ═══
	fmt.Println("═══ bonsai fix: initial check ═══")
	report, err := doCheck(ctx, opts.checkAgent, opts.resolver, opts.skills, opts.source, opts.baseRef, opts.repoRoot, opts.config, opts.registry)
	if err != nil {
		return fmt.Errorf("initial check: %w", err)
	}
	if report == nil {
		return nil // interrupted before results — treat as clean exit
	}

	if !report.ShouldFail() {
		fmt.Println("\n✔ No findings — nothing to fix")
		return nil
	}

	findings := extractDetailedFindings(report)

	// ═══ Fix loop ═══
	for iteration := 1; iteration <= opts.maxIterations; iteration++ {
		fmt.Printf("\n═══ Fix session %d/%d ═══\n\n", iteration, opts.maxIterations)

		// Build prompt with findings
		builder := prompt.NewBuilder(opts.resolver, opts.repoRoot)
		systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
			Mode:         prompt.ModeImplementer,
			Role:         "implementer",
			ExtraContext: findings,
		})
		if err != nil {
			return fmt.Errorf("build prompt: %w", err)
		}

		extraArgs := fixSessionArgs(opts)
		// Interactive session — ctrl-C and normal exit are expected.
		// Log unexpected errors (binary not found, auth failure) but
		// continue to re-check so the user sees current state.
		if sessErr := opts.sessionAgent.Interactive(ctx, systemPrompt, extraArgs); sessErr != nil {
			if ctx.Err() != nil {
				// Parent context cancelled — user wants out entirely.
				// Return nil: cancellation is a clean exit, not an error.
				return nil //nolint:nilerr // cancellation is intentional clean exit
			}
			fmt.Fprintf(os.Stderr, "warning: fix session exited with error: %v\n", sessErr)
		}

		// Re-check
		fmt.Printf("\n═══ Re-check after fix session %d/%d ═══\n", iteration, opts.maxIterations)
		report, err = doCheck(ctx, opts.checkAgent, opts.resolver, opts.skills, opts.source, opts.baseRef, opts.repoRoot, opts.config, opts.registry)
		if err != nil {
			return fmt.Errorf("re-check: %w", err)
		}
		if report == nil {
			return nil // interrupted before results — treat as clean exit
		}

		if !report.ShouldFail() {
			fmt.Printf("\n✔ All findings resolved (%d/%d skills passed)\n", report.Passed, report.Total)
			saveFixArtifacts(opts.repoRoot, opts.config, report)
			return nil
		}

		// Still failing
		findings = extractDetailedFindings(report)

		if iteration == opts.maxIterations {
			fmt.Fprintf(os.Stderr, "\n✖ Findings remain after %d fix iterations\n", opts.maxIterations)
			printDetailedFindings(report)
			return fmt.Errorf("findings remain after %d fix iterations", opts.maxIterations)
		}

		fmt.Printf("\n%d finding(s) remain — re-entering fix session\n", report.BlockingFailed)
	}

	return fmt.Errorf("fix loop exited unexpectedly")
}

// fixSessionArgs resolves extra CLI args for the interactive fix session.
// Precedence: --model flag > config agents.models > none.
func fixSessionArgs(opts fixOpts) []string {
	args := append([]string{}, opts.extraArgs...)
	if opts.modelOverride != "" {
		return append([]string{"--model", opts.modelOverride}, args...)
	}
	if opts.config != nil {
		if m := opts.config.Agents.Models.ModelForRole("implement"); m != "" {
			return append([]string{"--model", m}, args...)
		}
	}
	return args
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
func runFixCheck(
	ctx context.Context,
	a agent.Agent,
	resolver *assets.Resolver,
	skills []registry.Skill,
	source, baseRef, repoRoot string,
	cfg *config.Config,
	reg *registry.Registry,
) (*orchestrator.Report, error) {
	orch := orchestrator.New(a, resolver)
	sink, sinkDone := orchestrator.LoggerSink(func(msg string) { fmt.Println(msg) })

	report, err := orch.Run(ctx, orchestrator.RunOpts{
		Skills:              skills,
		Source:              source,
		BaseRef:             baseRef,
		FailFast:            false,
		RepoRoot:            repoRoot,
		Config:              cfg,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
		Concurrency:         0, // unlimited
	}, sink)
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
