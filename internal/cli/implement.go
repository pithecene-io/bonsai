package cli

import (
	"fmt"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gate"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/urfave/cli/v2"
)

func implementCommand() *cli.Command {
	return &cli.Command{
		Name:      "implement",
		Usage:     "Start an implementation session with governance gating",
		ArgsUsage: "[-- claude-args...]",
		Action:    runImplement,
	}
}

func runImplement(c *cli.Context) error {
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

	// Create agent
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)

	// Create and run gating loop
	loop := gate.New(gate.Opts{
		RepoRoot:  repoRoot,
		Config:    cfg,
		Agent:     claudeAgent,
		Resolver:  resolver,
		ExtraArgs: c.Args().Slice(),
	})

	// Preflight checks (branch, worktree, merge base, plan.json)
	if err := loop.Preflight(); err != nil {
		return err
	}

	// Run the gating loop
	return loop.Run(c.Context)
}
