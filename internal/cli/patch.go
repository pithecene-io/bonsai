package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
)

func patchCommand() *cli.Command {
	return &cli.Command{
		Name:      "patch",
		Usage:     "Three-phase patch surgery: plan, emit, validate",
		ArgsUsage: "<task-description>",
		Action:    runPatch,
	}
}

func runPatch(c *cli.Context) error {
	task := c.Args().First()
	if task == "" {
		return fmt.Errorf("usage: bonsai patch \"<task description>\"")
	}

	repoRoot := detectRepoRoot()
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs
	builder := prompt.NewBuilder(resolver, repoRoot)

	architectPlan, err := patchPhaseArchitect(c, builder, cfg, repoRoot, task)
	if err != nil {
		return err
	}
	if architectPlan == "" {
		return nil // user aborted
	}

	if err := patchPhaseEmit(c, builder, cfg, architectPlan, task); err != nil {
		return err
	}

	return patchPhaseValidate(c, resolver, cfg, repoRoot)
}

func patchPhaseArchitect(c *cli.Context, builder *prompt.Builder, cfg *config.Config, repoRoot, task string) (string, error) {
	fmt.Println("═══ Phase 1: Patch Architecture ═══")
	fmt.Printf("Task: %s\n\n", task)

	architectPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePatchArchitect,
		Role: "patch-architect",
	})
	if err != nil {
		return "", fmt.Errorf("build architect prompt: %w", err)
	}

	userPrompt := fmt.Sprintf("Plan a patch for the following task. Output the files to modify, exact regions, and assertions for correctness:\n\n%s", task)
	architectPlan, err := agent.NewClaude(cfg.Agents.Claude.Bin).NonInteractive(
		c.Context, architectPrompt, userPrompt, cfg.Models.ModelForRole("patch"))
	if err != nil {
		return "", fmt.Errorf("patch architecture phase failed: %w", err)
	}

	planPath, err := savePatchPlan(repoRoot, cfg, task, architectPlan)
	if err != nil {
		return "", err
	}

	fmt.Println(architectPlan)
	fmt.Println()
	fmt.Println("─── Review the plan above ───")
	fmt.Printf("(Plan saved to %s)\n", planPath)

	if !confirmPrompt("Proceed to patch emission? [y/N] ", false) {
		fmt.Println("Aborted.")
		return "", nil
	}
	return architectPlan, nil
}

func savePatchPlan(repoRoot string, cfg *config.Config, task, plan string) (string, error) {
	outDir := filepath.Join(repoRoot, cfg.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	planData := map[string]string{
		"task": task, "plan": plan,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	planJSON, _ := json.MarshalIndent(planData, "", "  ")
	planPath := filepath.Join(outDir, "patch-plan.json")
	if err := os.WriteFile(planPath, planJSON, 0o644); err != nil {
		return "", fmt.Errorf("write patch plan: %w", err)
	}
	return planPath, nil
}

func patchPhaseEmit(c *cli.Context, builder *prompt.Builder, cfg *config.Config, architectPlan, task string) error {
	fmt.Println()
	fmt.Println("═══ Phase 2: Patch Emission ═══")

	patcherPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePatcher,
		Role: "patcher",
	})
	if err != nil {
		return fmt.Errorf("build patcher prompt: %w", err)
	}

	combinedPrompt := fmt.Sprintf("%s\n\nArchitect plan:\n%s\n\nTask: %s\n\nExecute the architect plan above. Emit only unified diffs for the listed files.",
		patcherPrompt, architectPlan, task)

	_ = agent.NewCodex(cfg.Agents.Codex.Bin).Interactive(c.Context, combinedPrompt, nil)
	return nil
}

func patchPhaseValidate(c *cli.Context, resolver *assets.Resolver, cfg *config.Config, repoRoot string) error {
	fmt.Println()
	fmt.Println("═══ Phase 3: Validation ═══")

	patchBase := repo.DetectMergeBase(repoRoot, cfg.Routing.MergeBaseCandidates)

	reg, err := registry.Load(resolver)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	skills, err := reg.SkillsForBundle("patch")
	if err != nil {
		skills, err = reg.SkillsForBundle("default")
		if err != nil {
			return fmt.Errorf("no patch or default bundle: %w", err)
		}
	}

	checkRouter := agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin)
	orch := orchestrator.New(checkRouter, resolver)
	sink, sinkDone := orchestrator.LoggerSink(func(msg string) { fmt.Println(msg) })

	report, err := orch.Run(c.Context, orchestrator.RunOpts{
		Skills:              skills,
		Source:              "bundle:patch",
		BaseRef:             patchBase,
		FailFast:            true,
		RepoRoot:            repoRoot,
		Config:              cfg,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
		Concurrency:         1,
	}, sink)
	close(sink)
	<-sinkDone
	if err != nil {
		return err
	}

	if report.ShouldFail() {
		fmt.Fprintln(os.Stderr, "\n✖ Patch validation failed. Review violations above.")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✔ Patch surgery complete.")
	return nil
}

// confirmPrompt asks a yes/no question and returns the answer.
// defaultYes controls the default behavior when the user presses Enter.
func confirmPrompt(msg string, defaultYes bool) bool {
	fmt.Print(msg)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}
