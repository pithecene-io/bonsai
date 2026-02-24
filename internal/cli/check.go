package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/tui"
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
			&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}, Usage: "Max parallel skill invocations"},
			&cli.BoolFlag{Name: "no-progress", Usage: "Disable TUI progress display"},
			&cli.StringFlag{Name: "model", Usage: "Override model for all skills (e.g. haiku, sonnet, opus)"},
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
	noProgress := c.Bool("no-progress")
	modelOverride := c.String("model")

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

	// Resolve concurrency: flag > config > default(4)
	concurrency := cfg.Check.Concurrency
	if c.IsSet("jobs") {
		concurrency = c.Int("jobs")
	}
	if concurrency <= 0 {
		concurrency = 4
	}

	// Create orchestrator with agent router (claude + codex + anthropic direct)
	var apiOpts []agent.AnthropicOption
	if cfg.Agents.Anthropic.APIKey != "" {
		apiOpts = append(apiOpts, agent.WithAPIKey(cfg.Agents.Anthropic.APIKey))
	}
	agentRouter := agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin, apiOpts...)
	orch := orchestrator.New(agentRouter, resolver)

	opts := orchestrator.RunOpts{
		Skills:              skills,
		Source:              source,
		BaseRef:             baseRef,
		Scope:               scope,
		FailFast:            failFast,
		RepoRoot:            repoRoot,
		Config:              cfg,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
		Concurrency:         concurrency,
		ModelOverride:       modelOverride,
	}

	// TTY detection: use TUI if stdout is a terminal and --no-progress is not set
	useTUI := term.IsTerminal(int(os.Stdout.Fd())) && !noProgress

	var report *orchestrator.Report

	if useTUI {
		// Derive a cancellable context so an early TUI quit (q / ctrl+c)
		// signals the orchestrator to abort in-flight skills promptly.
		orchCtx, orchCancel := context.WithCancel(c.Context)
		defer orchCancel()

		events := make(chan orchestrator.Event, len(skills)*4)
		var runErr error
		orchDone := make(chan struct{})
		go func() {
			report, runErr = orch.Run(orchCtx, opts, events)
			close(events)
			close(orchDone)
		}()

		tuiReport, tuiErr := tui.RunWithTUI(events, source)

		// Cancel the orchestrator context — in the normal path this is a
		// no-op (orchestrator already finished); on early quit it kills
		// in-flight agent subprocesses via exec.CommandContext.
		orchCancel()

		// Wait for the orchestrator goroutine to finish so we don't leak
		// it and so report/runErr are safe to read.
		<-orchDone

		// User-initiated quit: exit cleanly without treating cancelled
		// skills as governance failures.
		if errors.Is(tuiErr, tui.ErrInterrupted) {
			fmt.Fprintln(os.Stderr, "\n⚠ check interrupted by user")
			return nil
		}
		if tuiErr != nil {
			return tuiErr
		}
		if runErr != nil {
			return runErr
		}
		// Prefer the TUI's report (same object) but fall back
		if tuiReport != nil {
			report = tuiReport
		}
	} else {
		// Plain text output via LoggerSink
		sink, sinkDone := orchestrator.LoggerSink(func(msg string) { fmt.Println(msg) })
		report, err = orch.Run(c.Context, opts, sink)
		close(sink)
		<-sinkDone
		if err != nil {
			return err
		}
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
		return cli.Exit("", 1)
	}

	return nil
}
