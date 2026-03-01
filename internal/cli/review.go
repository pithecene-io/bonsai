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
		Usage:  "Start a code review session (uses codex)",
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

	// Route agent based on models.roles.review config.
	// Default is "codex"; any other value routes to claude with that model.
	reviewModel := env.Config.Models.ModelForRole("review")
	if reviewModel == "codex" {
		return agent.NewCodex(env.Config.Agents.Codex.Bin).Interactive(c.Context, systemPrompt, nil)
	}
	claudeAgent := agent.NewClaude(env.Config.Agents.Claude.Bin)
	var extraArgs []string
	if reviewModel != "" {
		extraArgs = append(extraArgs, "--model", reviewModel)
	}
	return claudeAgent.Interactive(c.Context, systemPrompt, extraArgs)
}
