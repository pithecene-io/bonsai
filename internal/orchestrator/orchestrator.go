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

// indexedSkill pairs a registry skill with its original position index.
type indexedSkill struct {
	index int
	skill registry.Skill
}

// workerCtx holds immutable inputs for a single skill worker.
type workerCtx struct {
	is          indexedSkill
	runner      *skill.Runner
	repoTree    string
	diffPayload string
	opts        RunOpts
	total       int
}

// Run executes the skill set and returns an aggregate report.
// events may be nil; when non-nil, lifecycle events are sent for each skill.
// The caller must not close the events channel; Run does not close it either.
func (o *Orchestrator) Run(ctx context.Context, opts RunOpts, events chan<- Event) (*Report, error) {
	builder := prompt.NewBuilder(o.resolver, opts.RepoRoot)
	runner := skill.NewRunner(o.agent, builder)

	repoTree, err := repo.TreeWithScope(opts.RepoRoot, opts.Scope)
	if err != nil {
		return nil, fmt.Errorf("repo tree: %w", err)
	}
	repoTreeStr := joinLines(repoTree)

	var diffPayload string
	if opts.BaseRef != "" {
		diffPayload = buildDiffPayload(opts.RepoRoot, opts.BaseRef)
	}

	total := len(opts.Skills)
	results := make([]Result, total)
	runnable := o.partitionSkills(opts, events, results, total)
	o.dispatchWorkers(ctx, opts, events, results, runnable, runner, repoTreeStr, diffPayload, total)
	report := aggregateReport(opts, results)

	emit(events, Event{
		Kind:   EventComplete,
		Total:  total,
		Report: report,
	})

	return report, nil
}

// partitionSkills separates skippable skills from runnable ones,
// emitting skip/queue events and populating pre-filled results.
func (o *Orchestrator) partitionSkills(
	opts RunOpts,
	events chan<- Event,
	results []Result,
	total int,
) []indexedSkill {
	var runnable []indexedSkill
	for i := range opts.Skills {
		s := &opts.Skills[i]
		if s.EffectiveRequiresDiff(opts.DefaultRequiresDiff) && opts.BaseRef == "" {
			results[i] = Result{
				Name:          s.Name,
				Status:        "skipped",
				SkippedReason: "requires_diff without --base",
				Mandatory:     s.Mandatory,
			}
			emit(events, Event{
				Kind: EventSkipped, Index: i, Total: total,
				SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
				Reason: "requires --base for diff context",
			})
		} else {
			runnable = append(runnable, indexedSkill{index: i, skill: *s})
			emit(events, Event{
				Kind: EventQueued, Index: i, Total: total,
				SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
			})
		}
	}
	return runnable
}

// workerState holds shared mutable state for worker goroutines.
type workerState struct {
	mu        sync.Mutex
	triggered bool
	once      sync.Once
	cancel    context.CancelFunc
}

func (ws *workerState) isStopped() bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.triggered
}

func (ws *workerState) triggerFailFast(events chan<- Event, idx, total int, s registry.Skill) {
	ws.once.Do(func() {
		ws.mu.Lock()
		ws.triggered = true
		ws.mu.Unlock()
		emit(events, Event{
			Kind: EventFailFast, Index: idx, Total: total,
			SkillName: s.Name, Mandatory: s.Mandatory,
			Reason: "mandatory failure with --fail-fast",
		})
		ws.cancel()
	})
}

// dispatchWorkers launches concurrent skill workers with semaphore and fail-fast.
func (o *Orchestrator) dispatchWorkers(
	ctx context.Context,
	opts RunOpts,
	events chan<- Event,
	results []Result,
	runnable []indexedSkill,
	runner *skill.Runner,
	repoTreeStr, diffPayload string,
	total int,
) {
	concurrency := effectiveConcurrency(opts.Concurrency, len(runnable))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ws := &workerState{cancel: cancel}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for idx := range runnable {
		if ws.isStopped() {
			break
		}

		wc := workerCtx{
			is:          runnable[idx],
			runner:      runner,
			repoTree:    repoTreeStr,
			diffPayload: diffPayload,
			opts:        opts,
			total:       total,
		}

		select {
		case sem <- struct{}{}:
		case <-runCtx.Done():
			continue
		}

		wg.Add(1)
		go o.runWorker(runCtx, wc, events, results, sem, &wg, ws)
	}

	wg.Wait()
}

func effectiveConcurrency(requested, runnableCount int) int {
	if requested <= 0 {
		requested = runnableCount
	}
	if requested == 0 {
		return 1
	}
	return requested
}

func (o *Orchestrator) runWorker(
	ctx context.Context,
	wc workerCtx,
	events chan<- Event,
	results []Result,
	sem chan struct{},
	wg *sync.WaitGroup,
	ws *workerState,
) {
	defer wg.Done()
	defer func() { <-sem }()

	if ctx.Err() != nil {
		return
	}

	idx, s := wc.is.index, wc.is.skill

	emit(events, Event{
		Kind: EventStart, Index: idx, Total: wc.total,
		SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
	})

	result := o.runSingleSkill(ctx, s, wc.runner, wc.repoTree, wc.diffPayload, wc.opts)
	results[idx] = result

	emit(events, Event{
		Kind: EventDone, Index: idx, Total: wc.total,
		SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
		Result:  &results[idx],
		Elapsed: time.Duration(result.Elapsed * float64(time.Millisecond)),
	})

	if wc.opts.FailFast && result.ExitCode != 0 && s.Mandatory {
		ws.triggerFailFast(events, idx, wc.total, s)
	}
}

// aggregateReport tallies results into a Report in original skill order.
func aggregateReport(opts RunOpts, results []Result) *Report {
	report := &Report{
		Source:    opts.Source,
		Timestamp: time.Now().Format("20060102-150405"),
	}
	for i := range opts.Skills {
		r := results[i]
		if r.Name == "" {
			continue
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
	return report
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
		model = agent.Model(opts.Config.Models.ModelForSkill(string(s.Cost)))
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
	// Diff payload is best-effort; an error returns empty string,
	// which causes skills to run without diff context.
	diff, _ := skill.BuildDiffPayload(repoRoot, baseRef)
	return diff
}
