package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func reviewCommand() *cli.Command {
	return &cli.Command{
		Name:   "review",
		Usage:  "Autonomous code review",
		Action: runReview,
	}
}

func runReview(c *cli.Context) error {
	repoRoot := detectRepoRoot()
	env, err := bootstrapLight(repoRoot)
	if err != nil {
		return err
	}

	builder := prompt.NewBuilder(env.Resolver, repoRoot)
	systemPrompt, err := builder.BuildReview()
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	rs := &reviewRunner{
		agent: newAgentRouter(env.Config),
		model: agent.Model(env.Config.Models.ModelForRole("reviewer")),
	}
	return rs.run(c.Context, systemPrompt)
}

// reviewRunner encapsulates the review dispatch so the agent is injectable
// for testing.
type reviewRunner struct {
	agent agent.Agent
	model agent.Model
}

// run invokes an autonomous review session via Execute.
func (rr *reviewRunner) run(ctx context.Context, systemPrompt string) error {
	userPrompt := "Review the code changes on this branch."
	return rr.agent.Execute(ctx, systemPrompt, userPrompt, rr.model)
}
