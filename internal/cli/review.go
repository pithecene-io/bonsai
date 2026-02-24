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

func reviewCommand() *cli.Command {
	return &cli.Command{
		Name:   "review",
		Usage:  "Start a code review session (uses codex)",
		Action: runReview,
	}
}

func runReview(c *cli.Context) error {
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

	// Build system prompt (review uses its own builder method)
	builder := prompt.NewBuilder(resolver, repoRoot)
	systemPrompt, err := builder.BuildReview()
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	// Route agent based on agents.models.review config.
	// Default is "codex"; any other value routes to claude with that model.
	reviewModel := cfg.Agents.Models.ModelForRole("review")
	if reviewModel == "codex" {
		codexAgent := agent.NewCodex(cfg.Agents.Codex.Bin)
		return codexAgent.Interactive(c.Context, systemPrompt, nil)
	}
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	var extraArgs []string
	if reviewModel != "" {
		extraArgs = append(extraArgs, "--model", reviewModel)
	}
	return claudeAgent.Interactive(c.Context, systemPrompt, extraArgs)
}
