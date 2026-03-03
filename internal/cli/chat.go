package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func chatCommand() *cli.Command {
	return &cli.Command{
		Name:      "chat",
		Usage:     "Start an interactive AI chat session",
		ArgsUsage: "[role] [-- extra-args...]",
		Action:    runChat,
	}
}

// roleModes maps role names to prompt modes for interactive sessions.
var roleModes = map[string]prompt.Mode{
	"architect":   prompt.ModeArchitect,
	"planner":     prompt.ModePlanner,
	"implementer": prompt.ModeImplementer,
	"reviewer":    prompt.ModeReviewer,
	"patcher":     prompt.ModePatcher,
}

func runChat(c *cli.Context) error {
	role := c.Args().First()
	if role == "" {
		role = "architect"
	}

	repoRoot := detectRepoRoot()
	env, err := bootstrapLight(repoRoot)
	if err != nil {
		return err
	}

	if _, err := env.Resolver.ResolveRoleFile(role); err != nil {
		return fmt.Errorf("role %q not found (available: architect, implementer, planner, reviewer, patcher)", role)
	}

	mode, ok := roleModes[role]
	if !ok {
		mode = prompt.ModeArchitect
	}

	builder := prompt.NewBuilder(env.Resolver, repoRoot)
	systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: mode,
		Role: role,
	})
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	var extraArgs []string
	if c.Args().Len() > 1 {
		extraArgs = c.Args().Slice()[1:]
	}

	chatModel := env.Config.Models.ModelForRole("chat")
	claudeAgent := agent.NewClaude(env.Config.Agents.Claude.Bin)
	var modelArgs []string
	if chatModel != "" {
		modelArgs = append(modelArgs, "--model", chatModel)
	}
	modelArgs = append(modelArgs, extraArgs...)
	return claudeAgent.Session(c.Context, systemPrompt, modelArgs)
}
