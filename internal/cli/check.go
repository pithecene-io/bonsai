package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/justapithecus/bonsai/internal/agent"
	"github.com/justapithecus/bonsai/internal/assets"
	"github.com/justapithecus/bonsai/internal/config"
	"github.com/justapithecus/bonsai/internal/gitutil"
	"github.com/justapithecus/bonsai/internal/orchestrator"
	"github.com/justapithecus/bonsai/internal/registry"
	"github.com/urfave/cli/v2"
)

func checkCommand() *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "Run governance skills (bundle or mode-based)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "bundle", Value: "default", Usage: "Bundle name"},
			&cli.StringFlag{Name: "mode", Usage: "Governance mode (PATCH, NORMAL, STRUCTURAL, API, HEAVY, AUDIT)"},
			&cli.StringFlag{Name: "scope", Usage: "Comma-separated path prefixes"},
			&cli.StringFlag{Name: "base", Usage: "Git ref for diff context"},
			&cli.BoolFlag{Name: "fail-fast", Usage: "Stop on first mandatory failure"},
			&cli.StringFlag{Name: "diff-profile", Usage: "JSON diff profile (reserved)"},
		},
		Action: runCheck,
	}
}

func runCheck(c *cli.Context) error {
	mode := c.String("mode")
	bundle := c.String("bundle")
	scope := c.String("scope")
	baseRef := c.String("base")
	failFast := c.Bool("fail-fast")
	bundleExplicit := c.IsSet("bundle")

	// Mutual exclusion
	if mode != "" && bundleExplicit {
		return fmt.Errorf("--mode and --bundle are mutually exclusive")
	}

	// Validate mode
	if mode != "" && !registry.IsValidMode(mode) {
		return fmt.Errorf("invalid mode %q (valid: %v)", mode, registry.ValidModes())
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

	// Resolve skill set
	var skills []registry.Skill
	var source string

	if mode != "" {
		skills, err = reg.SkillsForMode(mode)
		if err != nil {
			return err
		}
		source = "mode:" + mode
	} else {
		skills, err = reg.SkillsForBundle(bundle)
		if err != nil {
			return err
		}
		source = "bundle:" + bundle
	}

	// Create orchestrator
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	orch := orchestrator.New(claudeAgent, resolver)

	// Run
	logger := func(msg string) { fmt.Println(msg) }
	report, err := orch.Run(c.Context, orchestrator.RunOpts{
		Skills:              skills,
		Source:              source,
		BaseRef:             baseRef,
		Scope:               scope,
		FailFast:            failFast,
		RepoRoot:            repoRoot,
		Config:              cfg,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
	}, logger)
	if err != nil {
		return err
	}

	// Write report
	outDir := filepath.Join(repoRoot, cfg.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	reportPath := filepath.Join(outDir, "ai-check.json")
	if err := os.WriteFile(reportPath, reportJSON, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	// Summary
	fmt.Println()
	fmt.Println("═══ bonsai check summary ═══")
	fmt.Printf("Source: %s\n", source)
	fmt.Printf("Results: %d/%d passed (%d failed, %d skipped, %d blocking)\n",
		report.Passed, report.Total, report.Failed, report.Skipped, report.BlockingFailed)
	fmt.Printf("Output: %s\n", reportPath)

	if report.ShouldFail() {
		if report.Total > 0 && report.Skipped == report.Total {
			fmt.Fprintf(os.Stderr, "\n✖ All %d skill(s) were skipped — no validation occurred\n", report.Total)
			if baseRef == "" {
				fmt.Fprintln(os.Stderr, "  hint: pass --base <ref> to provide diff context for requires_diff skills")
			}
		} else {
			fmt.Fprintf(os.Stderr, "\n✖ %d skill(s) had blocking findings\n", report.BlockingFailed)
		}
		os.Exit(1)
	}

	return nil
}
