package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/prompt"
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

// patchSession encapsulates the state for a three-phase patch surgery.
type patchSession struct {
	env     cmdEnv
	builder *prompt.Builder
	agent   agent.Agent
	task    string
}

func runPatch(c *cli.Context) error {
	task := c.Args().First()
	if task == "" {
		return fmt.Errorf("usage: bonsai patch \"<task description>\"")
	}

	repoRoot := detectRepoRoot()
	wt, err := ensureFeatureBranch(repoRoot, "patch")
	if err != nil {
		return err
	}
	defer printWorktreeReminder(wt)

	env, err := bootstrapFrom(wt.RepoRoot)
	if err != nil {
		return err
	}

	ps := &patchSession{
		env:     env,
		builder: prompt.NewBuilder(env.Resolver, env.RepoRoot),
		agent:   newAgentRouter(env.Config),
		task:    task,
	}

	plan, err := ps.architect(c.Context)
	if err != nil {
		return err
	}
	if plan == "" {
		return nil // user aborted
	}

	if err := ps.emit(c.Context, plan); err != nil {
		return err
	}

	return ps.validate(c.Context)
}

// architect plans the patch and returns the plan text.
// Returns empty string if the user declines to proceed.
func (ps *patchSession) architect(ctx context.Context) (string, error) {
	fmt.Println("═══ Phase 1: Patch Architecture ═══")
	fmt.Printf("Task: %s\n\n", ps.task)

	systemPrompt, err := ps.builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeArchitect,
		Role: "architect",
	})
	if err != nil {
		return "", fmt.Errorf("build architect prompt: %w", err)
	}

	userPrompt := "Plan a patch for the following task. Output the files to modify, exact regions, and assertions for correctness:\n\n" + ps.task
	plan, err := ps.agent.Evaluate(
		ctx, systemPrompt, userPrompt, agent.Model(ps.env.Config.Models.ModelForRole("patcher")))
	if err != nil {
		return "", fmt.Errorf("patch architecture phase failed: %w", err)
	}

	planPath, err := ps.savePlan(plan)
	if err != nil {
		return "", err
	}

	fmt.Println(plan)
	fmt.Println()
	fmt.Println("─── Review the plan above ───")
	fmt.Printf("(Plan saved to %s)\n", planPath)

	if !confirmPrompt("Proceed to patch emission? [y/N] ", false) {
		fmt.Println("Aborted.")
		return "", nil
	}
	return plan, nil
}

func (ps *patchSession) savePlan(plan string) (string, error) {
	outDir := filepath.Join(ps.env.RepoRoot, ps.env.Config.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	planData := map[string]string{
		"task": ps.task, "plan": plan,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	planJSON, _ := json.MarshalIndent(planData, "", "  ")
	planPath := filepath.Join(outDir, "patch-plan.json")
	if err := os.WriteFile(planPath, planJSON, 0o644); err != nil {
		return "", fmt.Errorf("write patch plan: %w", err)
	}
	return planPath, nil
}

// emit runs the patcher agent to produce diffs from the architect plan.
func (ps *patchSession) emit(ctx context.Context, plan string) error {
	fmt.Println()
	fmt.Println("═══ Phase 2: Patch Emission ═══")

	patcherPrompt, err := ps.builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePatcher,
		Role: "patcher",
	})
	if err != nil {
		return fmt.Errorf("build patcher prompt: %w", err)
	}

	combinedPrompt := patcherPrompt + "\n\nArchitect plan:\n" + plan + "\n\nTask: " + ps.task + "\n\nExecute the architect plan above. Emit only unified diffs for the listed files."

	var extraArgs []string
	if patcherModel := ps.env.Config.Models.ModelForRole("patcher"); patcherModel != "" {
		extraArgs = append(extraArgs, "--model", patcherModel)
	}

	_ = ps.agent.Session(ctx, combinedPrompt, extraArgs)
	return nil
}

// validate runs the governance gate against the emitted patch.
func (ps *patchSession) validate(ctx context.Context) error {
	fmt.Println()
	fmt.Println("═══ Phase 3: Validation ═══")

	patchBase := repo.DetectMergeBase(ps.env.RepoRoot, ps.env.Config.Routing.MergeBaseCandidates)

	skills, err := ps.env.Registry.SkillsForBundle("patch")
	if err != nil {
		skills, err = ps.env.Registry.SkillsForBundle("default")
		if err != nil {
			return fmt.Errorf("no patch or default bundle: %w", err)
		}
	}

	orch := orchestrator.New(newAgentRouter(ps.env.Config), ps.env.Resolver)
	report, err := orch.RunWithLogger(ctx, orchestrator.RunOpts{
		Skills:              skills,
		Source:              "bundle:patch",
		BaseRef:             patchBase,
		FailFast:            true,
		RepoRoot:            ps.env.RepoRoot,
		Config:              ps.env.Config,
		DefaultRequiresDiff: ps.env.Registry.Defaults.EffectiveRequiresDiff(),
		Concurrency:         1,
	}, nil)
	if err != nil {
		return err
	}

	if report.ShouldFail() {
		fmt.Fprintln(os.Stderr, "\n✖ Patch validation failed. Review violations above.")
		return cli.Exit("", 1)
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
