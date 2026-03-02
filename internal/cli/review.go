package cli

import (
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

	// Review is autonomous: the agent receives the prompt, reviews the
	// code changes, and exits. Use the router for model-aware dispatch.
	reviewModel := env.Config.Models.ModelForRole("reviewer")
	router := agent.NewRouter(env.Config.Agents.Claude.Bin, env.Config.Agents.Codex.Bin)
	userPrompt := "Review the code changes on this branch."
	return router.Execute(c.Context, systemPrompt, userPrompt, agent.Model(reviewModel))
}
