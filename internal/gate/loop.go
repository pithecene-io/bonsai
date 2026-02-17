// Package gate provides the 3-iteration gating state machine,
// a faithful port of the implement loop from ai-implement.sh.
//
// The loop structure:
//
//	preflight → [session → diff → profile → mode → gate → pass/fail/re-inject] × max_iterations
package gate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justapithecus/bonsai/internal/agent"
	"github.com/justapithecus/bonsai/internal/assets"
	"github.com/justapithecus/bonsai/internal/config"
	"github.com/justapithecus/bonsai/internal/diff"
	"github.com/justapithecus/bonsai/internal/gitutil"
	"github.com/justapithecus/bonsai/internal/orchestrator"
	"github.com/justapithecus/bonsai/internal/prompt"
	"github.com/justapithecus/bonsai/internal/registry"
	"github.com/justapithecus/bonsai/internal/repo"
)

// Opts configures the gating loop.
type Opts struct {
	RepoRoot  string
	Config    *config.Config
	Agent     agent.Agent
	Resolver  *assets.Resolver
	ExtraArgs []string // Passthrough args to claude
}

// PlanInfo holds consumed plan metadata.
// Constraints is json.RawMessage because the planner emits an object
// (e.g. {"max_files":3}), not a string. Shell reference:
// ai-implement.sh:87 — jq -c '.constraints // {}'
type PlanInfo struct {
	Intent      string          `json:"intent"`
	Constraints json.RawMessage `json:"constraints"`
}

// Loop implements the 3-iteration gating state machine from ai-implement.sh.
type Loop struct {
	opts      Opts
	mergeBase string
	planInfo  *PlanInfo
}

// New creates a gating loop.
func New(opts Opts) *Loop {
	return &Loop{opts: opts}
}

// Preflight runs the pre-loop checks:
//   - Verify not on main/master branch
//   - Warn if not in a worktree
//   - Detect merge base
//   - Consume plan.json if present
func (l *Loop) Preflight() error {
	repoRoot := l.opts.RepoRoot

	// Check branch — hard-fail on main/master
	branch, err := gitutil.CurrentBranch(repoRoot)
	if err != nil {
		return fmt.Errorf("detect branch: %w", err)
	}
	if branch == "main" || branch == "master" {
		return fmt.Errorf("refusing to implement on %s — create a feature branch or use: git worktree add", branch)
	}

	// Warn if not in a worktree (not fatal, just advisory)
	info, err := repo.Detect(repoRoot)
	if err == nil && !info.IsWorktree {
		fmt.Fprintln(os.Stderr, "warning: running in main worktree (not a git worktree)")
	}

	// Detect merge base
	l.mergeBase = repo.DetectMergeBase(repoRoot, l.opts.Config.Routing.MergeBaseCandidates)
	if l.mergeBase != "" {
		short := l.mergeBase
		if len(short) > 12 {
			short = short[:12]
		}
		fmt.Printf("Merge base: %s\n", short)
	} else {
		fmt.Fprintln(os.Stderr, "warning: could not detect merge base — gating may be limited")
	}

	// Consume plan.json if present
	l.consumePlan()

	return nil
}

// Run executes the gating loop. Returns nil on success (governance passed),
// or an error on failure (max iterations exceeded or user declined).
func (l *Loop) Run(ctx context.Context) error {
	maxIter := l.opts.Config.Gate.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}

	var findingsContext string

	for iteration := 1; iteration <= maxIter; iteration++ {
		fmt.Printf("\n═══ Implementation session %d/%d ═══\n\n", iteration, maxIter)

		// 1. SESSION: Build prompt and invoke claude interactively
		builder := prompt.NewBuilder(l.opts.Resolver, l.opts.RepoRoot)
		systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
			Mode:         prompt.ModeImplementer,
			Role:         "implementer",
			ExtraContext: findingsContext,
		})
		if err != nil {
			return fmt.Errorf("build prompt: %w", err)
		}

		// Invoke claude interactively.
		// Match shell: `claude ... || true` — ignore exit from ctrl-C or session end.
		_ = l.opts.Agent.Interactive(ctx, systemPrompt, l.opts.ExtraArgs)

		// 2. CAPTURE DIFF: Check for changes
		if l.mergeBase == "" {
			fmt.Println("\nNo merge base — skipping governance gate")
			return nil
		}

		hasChanges, err := l.hasChanges()
		if err != nil || !hasChanges {
			fmt.Println("\nNo changes detected — skipping governance gate")
			return nil
		}

		// 3. PROFILE: Compute diff profile
		profile, err := diff.ComputeProfile(l.opts.RepoRoot, l.mergeBase, l.opts.Config)
		if err != nil {
			return fmt.Errorf("compute diff profile: %w", err)
		}

		// 4. MODE: Determine governance mode
		planIntent := ""
		if l.planInfo != nil {
			planIntent = l.planInfo.Intent
		}
		mode := diff.DetermineMode(profile, l.opts.Config, planIntent)
		fmt.Printf("\nGovernance mode: %s (files:%d lines:%d dirs:%d)\n",
			mode, profile.FilesChanged, profile.DiffLines, len(profile.TopLevelDirs))

		// 5. GATE: Run governance check
		report, err := l.runGate(ctx, mode)
		if err != nil {
			return fmt.Errorf("governance gate: %w", err)
		}

		// 6. PASS: If gate passed, save artifacts and return
		if !report.ShouldFail() {
			fmt.Printf("\n\u2714 Governance gate passed (%d/%d skills passed)\n",
				report.Passed, report.Total)
			l.saveArtifacts(report)
			return nil
		}

		// 7. FAIL: If max iterations reached, dump findings and fail
		if iteration == maxIter {
			fmt.Fprintf(os.Stderr, "\n\u2716 Governance gate failed after %d iterations\n", maxIter)
			l.printFailedFindings(report)
			return fmt.Errorf("governance gate failed after %d iterations", maxIter)
		}

		// 8. Ask user if they want to re-enter
		l.printFailedFindings(report)
		fmt.Printf("\nGovernance gate failed — %d blocking finding(s)\n", report.BlockingFailed)

		if !l.promptReenter() {
			return fmt.Errorf("user declined to re-enter")
		}

		// 9. RE-INJECT: Extract findings for next iteration's prompt
		findingsContext = l.extractFindings(report)
	}

	return fmt.Errorf("governance gate failed")
}

// consumePlan looks for plan.json in the output directory and consumes it.
// Matches ai-implement.sh: extract intent + constraints, rename to .consumed.json.
func (l *Loop) consumePlan() {
	outDir := filepath.Join(l.opts.RepoRoot, l.opts.Config.Output.Dir)
	planPath := filepath.Join(outDir, "plan.json")

	data, err := os.ReadFile(planPath)
	if err != nil {
		return // No plan.json — that's fine
	}

	var plan PlanInfo
	if err := json.Unmarshal(data, &plan); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to parse plan.json: %v\n", err)
		return
	}

	l.planInfo = &plan
	if plan.Intent != "" {
		fmt.Printf("Consuming plan.json (intent: %s)\n", plan.Intent)
	}

	// Rename to .consumed to prevent re-consumption
	consumedPath := filepath.Join(outDir, "plan.consumed.json")
	if err := os.Rename(planPath, consumedPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to rename plan.json: %v\n", err)
	}
}

// hasChanges checks for tracked or untracked changes relative to merge base.
func (l *Loop) hasChanges() (bool, error) {
	repoRoot := l.opts.RepoRoot

	// Check tracked changes
	names, _ := gitutil.DiffNameOnly(repoRoot, l.mergeBase)
	if len(names) > 0 {
		return true, nil
	}

	// Check untracked
	untracked, _ := gitutil.UntrackedFiles(repoRoot)
	return len(untracked) > 0, nil
}

// runGate invokes the orchestrator with mode-based skill selection.
// Matches ai-implement.sh: ai-check --mode $MODE --fail-fast --base $MERGE_BASE
func (l *Loop) runGate(ctx context.Context, mode string) (*orchestrator.Report, error) {
	reg, err := registry.Load(l.opts.Resolver)
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	skills, err := reg.SkillsForMode(mode)
	if err != nil {
		return nil, err
	}

	// Use a separate claude agent for non-interactive skill runs
	claudeAgent := agent.NewClaude(l.opts.Config.Agents.Claude.Bin)
	orch := orchestrator.New(claudeAgent, l.opts.Resolver)

	logger := func(msg string) { fmt.Println(msg) }

	return orch.Run(ctx, orchestrator.RunOpts{
		Skills:              skills,
		Source:              "mode:" + mode,
		BaseRef:             l.mergeBase,
		FailFast:            true,
		RepoRoot:            l.opts.RepoRoot,
		Config:              l.opts.Config,
		DefaultRequiresDiff: reg.Defaults.RequiresDiff,
	}, logger)
}

// saveArtifacts saves the patch and report on governance pass.
// Matches ai-implement.sh: save last.patch and last.report.json.
func (l *Loop) saveArtifacts(report *orchestrator.Report) {
	outDir := filepath.Join(l.opts.RepoRoot, l.opts.Config.Output.Dir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create output dir: %v\n", err)
		return
	}

	// Save last.patch
	patchData, err := gitutil.Diff(l.opts.RepoRoot, l.mergeBase)
	if err == nil && patchData != "" {
		patchPath := filepath.Join(outDir, "last.patch")
		if err := os.WriteFile(patchPath, []byte(patchData), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save patch: %v\n", err)
		} else {
			fmt.Printf("Saved: %s\n", patchPath)
		}
	}

	// Save last.report.json
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		reportPath := filepath.Join(outDir, "last.report.json")
		if err := os.WriteFile(reportPath, reportJSON, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save report: %v\n", err)
		} else {
			fmt.Printf("Saved: %s\n", reportPath)
		}
	}
}

// printFailedFindings prints a summary of failed skill findings to stderr.
func (l *Loop) printFailedFindings(report *orchestrator.Report) {
	for _, r := range report.Results {
		if r.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "  SKILL: %s | blocking:%d major:%d warning:%d\n",
				r.Name, r.Blocking, r.Major, r.Warning)
		}
	}
}

// extractFindings extracts failed findings into a string for prompt re-injection.
// Format matches ai-implement.sh:
//
//	"SKILL: <name> | blocking: <n> | major: <n> | warning: <n>"
func (l *Loop) extractFindings(report *orchestrator.Report) string {
	var lines []string
	for _, r := range report.Results {
		if r.ExitCode != 0 {
			lines = append(lines, fmt.Sprintf("SKILL: %s | blocking: %d | major: %d | warning: %d",
				r.Name, r.Blocking, r.Major, r.Warning))
		}
	}
	return strings.Join(lines, "\n")
}

// promptReenter asks the user if they want to re-enter the session.
func (l *Loop) promptReenter() bool {
	fmt.Print("\nRe-enter session to fix findings? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}
