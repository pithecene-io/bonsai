package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func planCommand() *cli.Command {
	return &cli.Command{
		Name:      "plan",
		Usage:     "Start a planning session",
		ArgsUsage: "[-- extra-args...]",
		Action:    runPlan,
	}
}

func runPlan(c *cli.Context) error {
	repoRoot := detectRepoRoot()
	env, err := bootstrapLight(repoRoot)
	if err != nil {
		return err
	}

	builder := prompt.NewBuilder(env.Resolver, repoRoot)
	systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePlanner,
		Role: "planner",
	})
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	outDir := filepath.Join(repoRoot, env.Config.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	claudeAgent := agent.NewClaude(env.Config.Agents.Claude.Bin)
	planModel := env.Config.Models.ModelForRole("planner")

	var extraArgs []string
	if planModel != "" {
		extraArgs = append(extraArgs, "--model", planModel)
	}
	extraArgs = append(extraArgs, c.Args().Slice()...)

	// Match shell: `claude ... || true` — ignore ctrl-C / session end.
	_ = claudeAgent.Session(c.Context, systemPrompt, extraArgs)

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
