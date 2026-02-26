// Package orchestrator provides multi-skill execution, fail-fast logic,
// all-skipped detection, and aggregate JSON report generation.
// Faithful port of ai-check.sh.
package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
	"github.com/pithecene-io/bonsai/internal/skill"
)

// RunOpts configures an orchestrator run.
type RunOpts struct {
	Skills              []registry.Skill // Ordered list of skills to run
	Source              string           // "mode:NORMAL" or "bundle:default"
	BaseRef             string           // Git ref for diff context
	Scope               string           // Comma-separated path prefixes
	FailFast            bool             // Stop on first mandatory failure
	RepoRoot            string           // Repository root
	Config              *config.Config
	DefaultRequiresDiff bool   // Registry defaults.requires_diff value
	Concurrency         int    // Max parallel skills; <= 0 means unlimited (sized to skill count)
	ModelOverride       string // When non-empty, overrides config-based model routing for all skills
}

// Result holds the outcome of a single skill invocation.
type Result struct {
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	SkippedReason   string   `json:"skipped_reason,omitempty"`
	Blocking        int      `json:"blocking"`
	Major           int      `json:"major"`
	Warning         int      `json:"warning"`
	ExitCode        int      `json:"exit_code"`
	Mandatory       bool     `json:"mandatory"`
	Elapsed         float64  `json:"elapsed_ms"`
	BlockingDetails []string `json:"blocking_details,omitempty"`
	MajorDetails    []string `json:"major_details,omitempty"`
	WarningDetails  []string `json:"warning_details,omitempty"`
	InfoDetails     []string `json:"info_details,omitempty"`
}

// Report holds the aggregate orchestrator output.
type Report struct {
	Source         string   `json:"source"`
	Timestamp      string   `json:"timestamp"`
	Total          int      `json:"total"`
	Passed         int      `json:"passed"`
	Failed         int      `json:"failed"`
	Skipped        int      `json:"skipped"`
	BlockingFailed int      `json:"blocking_failed"`
	Results        []Result `json:"results"`
}

// Orchestrator runs a set of skills and aggregates results.
type Orchestrator struct {
	agent    agent.Agent
	resolver *assets.Resolver
}

// New creates an orchestrator.
func New(a agent.Agent, resolver *assets.Resolver) *Orchestrator {
	return &Orchestrator{agent: a, resolver: resolver}
}

// emit sends an event if the channel is non-nil.
func emit(events chan<- Event, ev Event) {
	if events != nil {
		events <- ev
	}
}

// Run executes the skill set and returns an aggregate report.
// events may be nil; when non-nil, lifecycle events are sent for each skill.
// The caller must not close the events channel; Run does not close it either.
func (o *Orchestrator) Run(ctx context.Context, opts RunOpts, events chan<- Event) (*Report, error) {
	timestamp := time.Now().Format("20060102-150405")

	report := &Report{
		Source:    opts.Source,
		Timestamp: timestamp,
	}

	builder := prompt.NewBuilder(o.resolver, opts.RepoRoot)
	runner := skill.NewRunner(o.agent, builder)

	// Build repo tree
	repoTree, err := repo.TreeWithScope(opts.RepoRoot, opts.Scope)
	if err != nil {
		return nil, fmt.Errorf("repo tree: %w", err)
	}
	repoTreeStr := joinLines(repoTree)

	// Build diff payload
	var diffPayload string
	if opts.BaseRef != "" {
		diffPayload = buildDiffPayload(opts.RepoRoot, opts.BaseRef)
	}

	total := len(opts.Skills)

	// Pre-filter: separate skippable from runnable
	type indexedSkill struct {
		index int
		skill registry.Skill
	}
	var runnable []indexedSkill
	results := make([]Result, total)

	for i, s := range opts.Skills {
		requiresDiff := s.EffectiveRequiresDiff(opts.DefaultRequiresDiff)
		if requiresDiff && opts.BaseRef == "" {
			results[i] = Result{
				Name:          s.Name,
				Status:        "skipped",
				SkippedReason: "requires_diff without --base",
				Mandatory:     s.Mandatory,
			}
			emit(events, Event{
				Kind:      EventSkipped,
				Index:     i,
				Total:     total,
				SkillName: s.Name,
				Cost:      s.Cost,
				Mandatory: s.Mandatory,
				Reason:    "requires --base for diff context",
			})
		} else {
			runnable = append(runnable, indexedSkill{index: i, skill: s})
			emit(events, Event{
				Kind:      EventQueued,
				Index:     i,
				Total:     total,
				SkillName: s.Name,
				Cost:      s.Cost,
				Mandatory: s.Mandatory,
			})
		}
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = len(runnable) // unlimited: all skills at once
	}
	if concurrency == 0 {
		concurrency = 1 // edge case: no runnable skills
	}

	// Context with cancellation for fail-fast
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var failFastOnce sync.Once
	failFastTriggered := false
	var ffMu sync.Mutex

	for _, is := range runnable {
		// Check for fail-fast before launching
		ffMu.Lock()
		stopped := failFastTriggered
		ffMu.Unlock()
		if stopped {
			break
		}

		// Acquire semaphore in the dispatch goroutine to guarantee
		// ordered startup — skills acquire the sem in list order.
		select {
		case sem <- struct{}{}:
		case <-runCtx.Done():
			continue
		}

		wg.Add(1)
		go func(idx int, s registry.Skill) {
			defer wg.Done()
			defer func() { <-sem }()

			// Check context before running
			if runCtx.Err() != nil {
				return
			}

			emit(events, Event{
				Kind:      EventStart,
				Index:     idx,
				Total:     total,
				SkillName: s.Name,
				Cost:      s.Cost,
				Mandatory: s.Mandatory,
			})

			result := o.runSingleSkill(runCtx, s, runner, repoTreeStr, diffPayload, opts)
			results[idx] = result

			elapsed := time.Duration(result.Elapsed * float64(time.Millisecond))
			emit(events, Event{
				Kind:      EventDone,
				Index:     idx,
				Total:     total,
				SkillName: s.Name,
				Cost:      s.Cost,
				Mandatory: s.Mandatory,
				Result:    &results[idx],
				Elapsed:   elapsed,
			})

			// Fail-fast: cancel context on mandatory failure
			if opts.FailFast && result.ExitCode != 0 && s.Mandatory {
				failFastOnce.Do(func() {
					ffMu.Lock()
					failFastTriggered = true
					ffMu.Unlock()
					emit(events, Event{
						Kind:      EventFailFast,
						Index:     idx,
						Total:     total,
						SkillName: s.Name,
						Mandatory: s.Mandatory,
						Reason:    "mandatory failure with --fail-fast",
					})
					cancel()
				})
			}
		}(is.index, is.skill)
	}

	wg.Wait()

	// Aggregate results in original order
	for i := range opts.Skills {
		r := results[i]
		if r.Name == "" {
			continue // was not scheduled or cancelled before starting
		}
		report.Total++
		switch {
		case r.Status == "skipped":
			report.Skipped++
		case r.Status == "error" || r.ExitCode != 0:
			report.Failed++
			if r.Mandatory && r.Status != "skipped" {
				report.BlockingFailed++
			}
		default:
			report.Passed++
		}
		report.Results = append(report.Results, r)
	}

	emit(events, Event{
		Kind:   EventComplete,
		Total:  total,
		Report: report,
	})

	return report, nil
}

// runSingleSkill executes one skill and returns its Result.
// It does not mutate any shared state and is safe for concurrent use.
func (o *Orchestrator) runSingleSkill(
	ctx context.Context,
	s registry.Skill,
	runner *skill.Runner,
	repoTreeStr, diffPayload string,
	opts RunOpts,
) Result {
	start := time.Now()

	version := s.Version
	if version == "" {
		version = "v1"
	}
	def, err := skill.Load(o.resolver, s.Name, version)
	if err != nil {
		return Result{
			Name:      s.Name,
			Status:    "error",
			ExitCode:  1,
			Mandatory: s.Mandatory,
			Elapsed:   float64(time.Since(start).Milliseconds()),
		}
	}

	// Resolve model: explicit override > config routing by cost tier
	var model agent.Model
	if opts.ModelOverride != "" {
		model = agent.Model(opts.ModelOverride)
	} else if opts.Config != nil {
		model = agent.Model(opts.Config.Models.ModelForSkill(s.Cost))
	}

	output, err := runner.Run(ctx, def, skill.RunOpts{
		RepoTree:    repoTreeStr,
		DiffPayload: diffPayload,
		BaseRef:     opts.BaseRef,
		Model:       model,
	})
	elapsed := float64(time.Since(start).Milliseconds())

	if err != nil {
		return Result{
			Name:      s.Name,
			Status:    "error",
			ExitCode:  1,
			Mandatory: s.Mandatory,
			Elapsed:   elapsed,
		}
	}

	exitCode := 0
	if output.ShouldFail() {
		exitCode = 1
	}

	return Result{
		Name:            s.Name,
		Status:          output.Status,
		Blocking:        len(output.Blocking),
		Major:           len(output.Major),
		Warning:         len(output.Warning),
		ExitCode:        exitCode,
		Mandatory:       s.Mandatory,
		Elapsed:         elapsed,
		BlockingDetails: output.Blocking,
		MajorDetails:    output.Major,
		WarningDetails:  output.Warning,
		InfoDetails:     output.Info,
	}
}

// ShouldFail returns true if the report indicates a blocking failure.
// Matches ai-check.sh exit logic:
//   - exit 1 if all skills were skipped (no validation occurred)
//   - exit 1 if blocking_failed > 0
func (r *Report) ShouldFail() bool {
	if r.Total > 0 && r.Skipped == r.Total {
		return true // All skipped = false pass
	}
	return r.BlockingFailed > 0
}

func logFindingDetails(logger func(string), severity string, details []string) {
	for _, d := range details {
		logger(fmt.Sprintf("    %s: %s", severity, d))
	}
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

func buildDiffPayload(repoRoot, baseRef string) string {
	// This reuses the same logic as the skill command
	diff, _ := skill.BuildDiffPayload(repoRoot, baseRef)
	return diff
}
