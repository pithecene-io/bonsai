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

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/diff"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
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
//   - Warn if not in a worktree
//   - Detect merge base
//   - Consume plan.json if present
//
// Branch validation (main/master guard) is handled by the CLI layer
// via ensureFeatureBranch, which auto-creates a worktree before the
// gate loop is constructed.
func (l *Loop) Preflight() error {
	repoRoot := l.opts.RepoRoot

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

// iterState holds immutable state passed between gating iterations.
type iterState struct {
	findings string // empty on first iteration or when resolved
}

// Run executes the gating loop. Returns nil on success (governance passed),
// or an error on failure (max iterations exceeded or user declined).
func (l *Loop) Run(ctx context.Context) error {
	maxIter := l.opts.Config.Gate.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}

	state := iterState{}

	for iteration := 1; iteration <= maxIter; iteration++ {
		fmt.Printf("\n═══ Implementation session %d/%d ═══\n\n", iteration, maxIter)

		if err := l.runSession(ctx, state.findings); err != nil {
			return err
		}

		next, err := l.runGateIteration(ctx, iteration, maxIter)
		if err != nil {
			return err
		}
		if next == nil {
			return nil // passed or skipped
		}
		state = *next
	}

	return fmt.Errorf("governance gate failed")
}

// runGateIteration runs one capture-profile-gate cycle. Returns nil on
// pass/skip, or a non-nil iterState for re-injection on failure.
func (l *Loop) runGateIteration(ctx context.Context, iteration, maxIter int) (*iterState, error) {
	outcome, err := l.captureAndGate(ctx)
	if err != nil {
		return nil, err
	}
	if outcome == nil {
		return nil, nil //nolint:nilnil // nil signals "no merge base or no changes" to caller
	}

	report := outcome.report
	fmt.Printf("\nGovernance mode: %s\n", outcome.mode)

	if !report.ShouldFail() {
		fmt.Printf("\n✔ Governance gate passed (%d/%d skills passed)\n",
			report.Passed, report.Total)
		l.saveArtifacts(report)
		return nil, nil //nolint:nilnil // nil signals "gate passed" to caller
	}

	if iteration == maxIter {
		fmt.Fprintf(os.Stderr, "\n✖ Governance gate failed after %d iterations\n", maxIter)
		report.PrintFindings(os.Stderr)
		return nil, fmt.Errorf("governance gate failed after %d iterations", maxIter)
	}

	report.PrintFindings(os.Stderr)
	fmt.Printf("\nGovernance gate failed — %d blocking finding(s)\n", report.BlockingFailed)

	if !l.promptReenter() {
		return nil, fmt.Errorf("user declined to re-enter")
	}

	return &iterState{findings: report.FindingSummary()}, nil
}

// runSession builds the system prompt and invokes claude interactively.
func (l *Loop) runSession(ctx context.Context, findingsContext string) error {
	builder := prompt.NewBuilder(l.opts.Resolver, l.opts.RepoRoot)
	systemPrompt, err := builder.BuildInteractive(prompt.InteractiveOpts{
		Mode:         prompt.ModeImplementer,
		Role:         "implementer",
		ExtraContext: findingsContext,
	})
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	extraArgs := l.opts.ExtraArgs
	if l.opts.Config != nil {
		if implModel := l.opts.Config.Models.ModelForRole("implementer"); implModel != "" {
			extraArgs = append([]string{"--model", implModel}, extraArgs...)
		}
	}

	// Match shell: `claude ... || true` — ignore exit from ctrl-C or session end.
	_ = l.opts.Agent.Session(ctx, systemPrompt, extraArgs)
	return nil
}

// gateOutcome holds the result of a capture-and-gate cycle.
// A nil value signals that gating was skipped (no merge base or no changes).
type gateOutcome struct {
	report *orchestrator.Report
	mode   string
}

// captureAndGate checks for changes, profiles them, and runs the governance gate.
// Returns nil when gating should be skipped (no merge base or no changes).
func (l *Loop) captureAndGate(ctx context.Context) (*gateOutcome, error) {
	if l.mergeBase == "" {
		fmt.Println("\nNo merge base — skipping governance gate")
		return nil, nil //nolint:nilnil // nil outcome signals "skip gating" to caller
	}

	if !l.hasChanges() {
		fmt.Println("\nNo changes detected — skipping governance gate")
		return nil, nil //nolint:nilnil // nil outcome signals "skip gating" to caller
	}

	profile, err := diff.ComputeProfile(l.opts.RepoRoot, l.mergeBase, l.opts.Config)
	if err != nil {
		return nil, fmt.Errorf("compute diff profile: %w", err)
	}

	planIntent := ""
	if l.planInfo != nil {
		planIntent = l.planInfo.Intent
	}
	mode := diff.DetermineMode(profile, l.opts.Config, planIntent)

	report, err := l.runGate(ctx, mode)
	if err != nil {
		return nil, fmt.Errorf("governance gate: %w", err)
	}

	return &gateOutcome{report: report, mode: mode}, nil
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
func (l *Loop) hasChanges() bool {
	repoRoot := l.opts.RepoRoot

	// Git commands may fail (e.g. corrupted index); treat as "no changes"
	// rather than blocking the loop.
	names, _ := gitutil.DiffNameOnly(repoRoot, l.mergeBase)
	if len(names) > 0 {
		return true
	}

	untracked, _ := gitutil.UntrackedFiles(repoRoot)
	return len(untracked) > 0
}

// runGate invokes the orchestrator with mode-based skill selection.
// Matches ai-implement.sh: ai-check --mode $MODE --fail-fast --base $MERGE_BASE
func (l *Loop) runGate(ctx context.Context, mode string) (*orchestrator.Report, error) {
	reg, err := registry.Load(l.opts.Resolver)
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	skills, err := reg.SkillsForMode(registry.GovMode(mode))
	if err != nil {
		return nil, err
	}

	// Use agent router for non-interactive skill runs (supports both claude and codex)
	agentRouter := agent.NewRouter(l.opts.Config.Agents.Claude.Bin, l.opts.Config.Agents.Codex.Bin)
	orch := orchestrator.New(agentRouter, l.opts.Resolver)

	return orch.RunWithLogger(ctx, orchestrator.RunOpts{
		Skills:              skills,
		Source:              "mode:" + mode,
		BaseRef:             l.mergeBase,
		FailFast:            true,
		RepoRoot:            l.opts.RepoRoot,
		Config:              l.opts.Config,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
		Concurrency:         1,
	}, nil)
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
