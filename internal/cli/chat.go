package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func chatCommand() *cli.Command {
	return &cli.Command{
		Name:      "chat",
		Usage:     "Start an interactive AI chat session",
		ArgsUsage: "[role] [-- claude-args...]",
		Action:    runChat,
	}
}

func runChat(c *cli.Context) error {
	// First arg is role, default to architect
	role := c.Args().First()
	if role == "" {
		role = "architect"
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

	// Verify role exists
	if _, err := resolver.ResolveRoleFile(role); err != nil {
		return fmt.Errorf("role %q not found (available: architect, implementer, planner, reviewer, patch-architect, patcher)", role)
	}

	// Determine mode from role
	mode := roleToMode(role)

	// Build system prompt
	builder := prompt.NewBuilder(resolver, repoRoot)
	systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: mode,
		Role: role,
	})
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	// Collect extra args (everything after the role argument)
	var extraArgs []string
	if c.Args().Len() > 1 {
		extraArgs = c.Args().Slice()[1:]
	}

	// Start interactive session
	// Matches: exec claude --system-prompt "$SYSTEM_PROMPT" "$@"
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	return claudeAgent.Interactive(c.Context, systemPrompt, extraArgs)
}

// roleToMode maps role names to prompt modes.
func roleToMode(role string) prompt.Mode {
	switch role {
	case "architect":
		return prompt.ModeArchitect
	case "planner":
		return prompt.ModePlanner
	case "implementer":
		return prompt.ModeImplementer
	case "reviewer":
		return prompt.ModeReviewer
	default:
		return prompt.ModeArchitect
	}
}
