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

	"github.com/pithecene-io/bonsai/internal/config"
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

// checkArgs holds parsed and validated CLI arguments for the check command.
type checkArgs struct {
	mode          string
	bundle        string
	scope         string
	baseRef       string
	failFast      bool
	noProgress    bool
	modelOverride string
}

func parseCheckArgs(c *cli.Context) (checkArgs, error) {
	a := checkArgs{
		mode:          c.String("mode"),
		bundle:        c.String("bundle"),
		scope:         c.String("scope"),
		baseRef:       c.String("base"),
		failFast:      c.Bool("fail-fast"),
		noProgress:    c.Bool("no-progress"),
		modelOverride: c.String("model"),
	}

	if a.mode != "" && c.IsSet("bundle") {
		return a, fmt.Errorf("--mode and --bundle are mutually exclusive")
	}
	if a.mode != "" {
		if _, err := registry.ParseGovMode(a.mode); err != nil {
			return a, err
		}
	}
	return a, nil
}

func runCheck(c *cli.Context) error {
	args, err := parseCheckArgs(c)
	if err != nil {
		return err
	}

	env, err := bootstrap()
	if err != nil {
		return err
	}

	ss, err := resolveSkillSet(env.Registry, args.mode, args.bundle)
	if err != nil {
		return err
	}

	concurrency := resolveConcurrency(env.Config, c)

	orch := orchestrator.New(newAgentRouter(env.Config), env.Resolver)
	opts := orchestrator.RunOpts{
		Skills:              ss.Skills,
		Source:              ss.Source,
		BaseRef:             args.baseRef,
		Scope:               args.scope,
		FailFast:            args.failFast,
		RepoRoot:            env.RepoRoot,
		Config:              env.Config,
		DefaultRequiresDiff: env.Registry.Defaults.EffectiveRequiresDiff(),
		Concurrency:         concurrency,
		ModelOverride:       args.modelOverride,
	}

	useTUI := term.IsTerminal(int(os.Stdout.Fd())) && !args.noProgress

	var report *orchestrator.Report
	if useTUI {
		report, err = runCheckTUI(c.Context, orch, opts, ss.Source, ss.Skills)
	} else {
		report, err = runCheckPlain(c.Context, orch, opts)
	}
	if err != nil {
		return err
	}
	if report == nil {
		return nil // TUI interrupted
	}

	reportPath, err := writeCheckReport(env.RepoRoot, env.Config, report)
	if err != nil {
		return err
	}

	printCheckSummary(ss.Source, reportPath, report, args.baseRef)

	if report.ShouldFail() {
		return cli.Exit("", 1)
	}
	return nil
}

func runCheckTUI(
	ctx context.Context,
	orch *orchestrator.Orchestrator,
	opts orchestrator.RunOpts,
	source string,
	skills []registry.Skill,
) (*orchestrator.Report, error) {
	orchCtx, orchCancel := context.WithCancel(ctx)
	defer orchCancel()

	events := make(chan orchestrator.Event, len(skills)*4)
	var report *orchestrator.Report
	var runErr error
	orchDone := make(chan struct{})
	go func() {
		report, runErr = orch.Run(orchCtx, opts, events)
		close(events)
		close(orchDone)
	}()

	tuiReport, tuiErr := tui.RunWithTUI(events, source)
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

func runCheckPlain(
	ctx context.Context,
	orch *orchestrator.Orchestrator,
	opts orchestrator.RunOpts,
) (*orchestrator.Report, error) {
	sink, sinkDone := orchestrator.LoggerSink(func(msg string) { fmt.Println(msg) })
	report, err := orch.Run(ctx, opts, sink)
	close(sink)
	<-sinkDone
	return report, err
}

func writeCheckReport(repoRoot string, cfg *config.Config, report *orchestrator.Report) (string, error) {
	outDir := filepath.Join(repoRoot, cfg.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}

	reportPath := filepath.Join(outDir, "ai-check.json")
	if err := os.WriteFile(reportPath, reportJSON, 0o644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}
	return reportPath, nil
}

func printCheckSummary(source, reportPath string, report *orchestrator.Report, baseRef string) {
	fmt.Println()
	fmt.Println("═══ bonsai check summary ═══")
	fmt.Printf("Source: %s\n", source)
	fmt.Printf("Results: %d/%d passed (%d failed, %d skipped, %d blocking)\n",
		report.Passed, report.Total, report.Failed, report.Skipped, report.BlockingFailed)
	fmt.Printf("Output: %s\n", reportPath)

	if !report.ShouldFail() {
		return
	}

	if report.Total > 0 && report.Skipped == report.Total {
		fmt.Fprintf(os.Stderr, "\n✖ All %d skill(s) were skipped — no validation occurred\n", report.Total)
		if baseRef == "" {
			fmt.Fprintln(os.Stderr, "  hint: pass --base <ref> to provide diff context for requires_diff skills")
		}
	} else {
		fmt.Fprintf(os.Stderr, "\n✖ %d skill(s) had blocking findings\n", report.BlockingFailed)
	}
}
