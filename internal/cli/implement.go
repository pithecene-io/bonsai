package cli

import (
	"os"
	"os/signal"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/gate"
)

func implementCommand() *cli.Command {
	return &cli.Command{
		Name:      "implement",
		Usage:     "Start an implementation session with governance gating",
		ArgsUsage: "[-- extra-args...]",
		Action:    runImplement,
	}
}

func runImplement(c *cli.Context) error {
	repoRoot := detectRepoRoot()

	wt, err := ensureFeatureBranch(repoRoot, "implement")
	if err != nil {
		return err
	}
	defer printWorktreeReminder(wt)
	repoRoot = wt.RepoRoot

	env, err := bootstrapLight(repoRoot)
	if err != nil {
		return err
	}

	// Wire up OS signal handling so CTRL-C during post-implement
	// governance checks actually cancels the orchestrator.
	ctx, stop := signal.NotifyContext(c.Context, os.Interrupt)
	defer stop()

	loop := gate.New(gate.Opts{
		RepoRoot:  repoRoot,
		Config:    env.Config,
		Agent:     newAgentRouter(env.Config),
		Resolver:  env.Resolver,
		ExtraArgs: c.Args().Slice(),
	})

	if err := loop.Preflight(); err != nil {
		return err
	}

	return loop.Run(ctx)
}
