package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func planCommand() *cli.Command {
	return &cli.Command{
		Name:      "plan",
		Usage:     "Start a planning session",
		ArgsUsage: "[-- claude-args...]",
		Action:    runPlan,
	}
}

func runPlan(c *cli.Context) error {
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

	// Build system prompt
	builder := prompt.NewBuilder(resolver, repoRoot)
	systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePlanner,
		Role: "planner",
	})
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	// Create output dir
	outDir := filepath.Join(repoRoot, cfg.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Create agent and start interactive session
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	extraArgs := c.Args().Slice()

	// Run session. Match shell: `claude ... || true` — ignore ctrl-C / session end.
	_ = claudeAgent.Interactive(c.Context, systemPrompt, extraArgs)

	// Post-session: detect plan.json
	planPath := filepath.Join(outDir, "plan.json")
	if data, err := os.ReadFile(planPath); err == nil {
		var plan struct {
			Intent string `json:"intent"`
		}
		if json.Unmarshal(data, &plan) == nil && plan.Intent != "" {
			fmt.Printf("\nPlan saved: %s (intent: %s)\n", planPath, plan.Intent)
			fmt.Println("  Run 'bonsai implement' to execute with governance gating")
		} else {
			fmt.Printf("\nPlan saved: %s\n", planPath)
		}
	}

	return nil
}
