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
	"github.com/pithecene-io/bonsai/internal/gitutil"
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

	builder := prompt.NewBuilder(resolver, repoRoot)

	// ═══ Phase 1: Patch Architecture (Claude) ═══
	fmt.Println("═══ Phase 1: Patch Architecture ═══")
	fmt.Printf("Task: %s\n\n", task)

	architectPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePatchArchitect,
		Role: "patch-architect",
	})
	if err != nil {
		return fmt.Errorf("build architect prompt: %w", err)
	}

	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)

	userPrompt := fmt.Sprintf("Plan a patch for the following task. Output the files to modify, exact regions, and assertions for correctness:\n\n%s", task)

	architectPlan, err := claudeAgent.NonInteractive(c.Context, architectPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("patch architecture phase failed: %w", err)
	}

	// Persist plan for audit trail
	outDir := filepath.Join(repoRoot, cfg.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	planData := map[string]string{
		"task":      task,
		"plan":      architectPlan,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	planJSON, _ := json.MarshalIndent(planData, "", "  ")
	planPath := filepath.Join(outDir, "patch-plan.json")
	if err := os.WriteFile(planPath, planJSON, 0o644); err != nil {
		return fmt.Errorf("write patch plan: %w", err)
	}

	fmt.Println(architectPlan)
	fmt.Println()
	fmt.Println("─── Review the plan above ───")
	fmt.Printf("(Plan saved to %s)\n", planPath)

	if !confirmPrompt("Proceed to patch emission? [y/N] ", false) {
		fmt.Println("Aborted.")
		return nil
	}

	// ═══ Phase 2: Patch Emission (Codex) ═══
	fmt.Println()
	fmt.Println("═══ Phase 2: Patch Emission ═══")

	patcherPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModePatcher,
		Role: "patcher",
	})
	if err != nil {
		return fmt.Errorf("build patcher prompt: %w", err)
	}

	// Build combined codex prompt (prompt + plan + task + instruction)
	// Matches: codex "$PATCHER_PROMPT\n\nArchitect plan:\n$ARCHITECT_PLAN\n\nTask: $TASK\n\nExecute..."
	combinedPrompt := fmt.Sprintf("%s\n\nArchitect plan:\n%s\n\nTask: %s\n\nExecute the architect plan above. Emit only unified diffs for the listed files.",
		patcherPrompt, architectPlan, task)

	codexAgent := agent.NewCodex(cfg.Agents.Codex.Bin)
	_ = codexAgent.Interactive(c.Context, combinedPrompt, nil)

	// ═══ Phase 3: Validation ═══
	fmt.Println()
	fmt.Println("═══ Phase 3: Validation ═══")

	// Auto-detect merge base for diff context
	patchBase := repo.DetectMergeBase(repoRoot, cfg.Routing.MergeBaseCandidates)

	reg, err := registry.Load(resolver)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	skills, err := reg.SkillsForBundle("patch")
	if err != nil {
		// Fallback to default bundle if patch bundle doesn't exist
		skills, err = reg.SkillsForBundle("default")
		if err != nil {
			return fmt.Errorf("no patch or default bundle: %w", err)
		}
	}

	checkAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	orch := orchestrator.New(checkAgent, resolver)
	logger := func(msg string) { fmt.Println(msg) }

	report, err := orch.Run(c.Context, orchestrator.RunOpts{
		Skills:              skills,
		Source:              "bundle:patch",
		BaseRef:             patchBase,
		FailFast:            true,
		RepoRoot:            repoRoot,
		Config:              cfg,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
	}, logger)
	if err != nil {
		return err
	}

	if report.ShouldFail() {
		fmt.Fprintln(os.Stderr, "\n\u2716 Patch validation failed. Review violations above.")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("\u2714 Patch surgery complete.")
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
