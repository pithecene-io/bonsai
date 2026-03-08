// Package orchestrator provides multi-skill execution, fail-fast logic,
// all-skipped detection, and aggregate JSON report generation.
// Faithful port of ai-check.sh.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
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
	ErrorDetail     string   `json:"error_detail,omitempty"`
	BlockingDetails []string `json:"blocking_details,omitempty"`
	MajorDetails    []string `json:"major_details,omitempty"`
	WarningDetails  []string `json:"warning_details,omitempty"`
	InfoDetails     []string `json:"info_details,omitempty"`
}

// severityPairs maps severity labels to detail slices for table-driven iteration.
var severityPairs = []struct {
	label   string
	details func(*Result) []string
}{
	{"blocking", func(r *Result) []string { return r.BlockingDetails }},
	{"major", func(r *Result) []string { return r.MajorDetails }},
	{"warning", func(r *Result) []string { return r.WarningDetails }},
}

// Failed returns true when this result represents a non-passing skill.
func (r *Result) Failed() bool { return r.ExitCode != 0 }

// Details returns severity-prefixed detail lines for this result's findings.
func (r *Result) Details(prefix string) []string {
	var lines []string
	for _, sp := range severityPairs {
		for _, d := range sp.details(r) {
			lines = append(lines, prefix+sp.label+": "+d)
		}
	}
	return lines
}

// SummaryLine returns a one-line status string for display.
func (r *Result) SummaryLine() string {
	return fmt.Sprintf("blocking:%d major:%d warning:%d", r.Blocking, r.Major, r.Warning)
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
	SkipWarning    string   `json:"skip_warning,omitempty"`
	Results        []Result `json:"results"`
}

// FailedResults returns pointers to all results with non-zero exit codes.
func (r *Report) FailedResults() []*Result {
	var failed []*Result
	for i := range r.Results {
		if r.Results[i].Failed() {
			failed = append(failed, &r.Results[i])
		}
	}
	return failed
}

// FormatFindings builds a multi-section findings string with full detail lines.
func (r *Report) FormatFindings() string {
	var sections []string
	for _, res := range r.FailedResults() {
		lines := append([]string{"SKILL: " + res.Name}, res.Details("  ")...)
		sections = append(sections, strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

// FindingSummary returns a compact summary for prompt re-injection.
// Format: "SKILL: <name> | blocking: <n> | major: <n> | warning: <n>"
func (r *Report) FindingSummary() string {
	var lines []string
	for _, res := range r.FailedResults() {
		lines = append(lines, fmt.Sprintf("SKILL: %s | blocking: %d | major: %d | warning: %d",
			res.Name, res.Blocking, res.Major, res.Warning))
	}
	return strings.Join(lines, "\n")
}

// PrintFindings writes failed findings to w.
func (r *Report) PrintFindings(w io.Writer) {
	for _, res := range r.FailedResults() {
		_, _ = fmt.Fprintf(w, "  SKILL: %s | %s\n", res.Name, res.SummaryLine())
		for _, line := range res.Details("    ") {
			_, _ = fmt.Fprintln(w, line)
		}
	}
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

// indexedSkill pairs a registry skill with its original position index.
type indexedSkill struct {
	index int
	skill registry.Skill
}

// runScope holds immutable per-run state shared across all workers.
// Created once in Run(), then passed by pointer to avoid parameter threading.
type runScope struct {
	opts        RunOpts
	runner      *skill.Runner
	resolver    *assets.Resolver
	repoTree    string
	diffPayload string
	events      chan<- Event
	results     []Result
	total       int
}

// emit sends an event if the channel is non-nil.
func (rs *runScope) emit(ev Event) {
	if rs.events != nil {
		rs.events <- ev
	}
}

// Run executes the skill set and returns an aggregate report.
// events may be nil; when non-nil, lifecycle events are sent for each skill.
// The caller must not close the events channel; Run does not close it either.
func (o *Orchestrator) Run(ctx context.Context, opts RunOpts, events chan<- Event) (*Report, error) {
	repoTree, err := repo.TreeWithScope(opts.RepoRoot, opts.Scope)
	if err != nil {
		return nil, fmt.Errorf("repo tree: %w", err)
	}

	var diffPayload string
	if opts.BaseRef != "" {
		diffPayload, _ = skill.BuildDiffPayload(opts.RepoRoot, opts.BaseRef)
	}

	rs := &runScope{
		opts:        opts,
		runner:      skill.NewRunner(o.agent, prompt.NewBuilder(o.resolver, opts.RepoRoot)),
		resolver:    o.resolver,
		repoTree:    strings.Join(repoTree, "\n"),
		diffPayload: diffPayload,
		events:      events,
		results:     make([]Result, len(opts.Skills)),
		total:       len(opts.Skills),
	}

	runnable := rs.partition()
	rs.dispatch(ctx, runnable)
	report := rs.aggregate()

	rs.emit(Event{Kind: EventComplete, Total: rs.total, Report: report})
	return report, nil
}

// RunWithLogger executes the skill set, logging events via logger.
// A nil logger defaults to fmt.Println.
// It manages the event channel lifecycle internally, eliminating the
// create-close-wait boilerplate that callers of Run would otherwise repeat.
func (o *Orchestrator) RunWithLogger(ctx context.Context, opts RunOpts, logger func(string)) (*Report, error) {
	if logger == nil {
		logger = func(msg string) { fmt.Println(msg) }
	}
	sink, done := LoggerSink(logger)
	report, err := o.Run(ctx, opts, sink)
	close(sink)
	<-done
	return report, err
}

// partition separates skippable skills from runnable ones,
// emitting skip/queue events and populating pre-filled results.
func (rs *runScope) partition() []indexedSkill {
	var runnable []indexedSkill
	for i := range rs.opts.Skills {
		s := &rs.opts.Skills[i]
		if s.EffectiveRequiresDiff(rs.opts.DefaultRequiresDiff) && rs.opts.BaseRef == "" {
			rs.results[i] = Result{
				Name:          s.Name,
				Status:        "skipped",
				SkippedReason: "requires_diff without --base",
				Mandatory:     s.Mandatory,
			}
			rs.emit(Event{
				Kind: EventSkipped, Index: i, Total: rs.total,
				SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
				Reason: "requires --base for diff context",
			})
		} else {
			runnable = append(runnable, indexedSkill{index: i, skill: *s})
			rs.emit(Event{
				Kind: EventQueued, Index: i, Total: rs.total,
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

func (ws *workerState) triggerFailFast(rs *runScope, idx int, s registry.Skill) {
	ws.once.Do(func() {
		ws.mu.Lock()
		ws.triggered = true
		ws.mu.Unlock()
		rs.emit(Event{
			Kind: EventFailFast, Index: idx, Total: rs.total,
			SkillName: s.Name, Mandatory: s.Mandatory,
			Reason: "mandatory failure with --fail-fast",
		})
		ws.cancel()
	})
}

// dispatch launches concurrent skill workers with semaphore and fail-fast.
func (rs *runScope) dispatch(ctx context.Context, runnable []indexedSkill) {
	concurrency := rs.opts.Concurrency
	if concurrency <= 0 {
		concurrency = len(runnable)
	}
	if concurrency == 0 {
		concurrency = 1
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ws := &workerState{cancel: cancel}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for idx := range runnable {
		if ws.isStopped() {
			break
		}

		select {
		case sem <- struct{}{}:
		case <-runCtx.Done():
			continue
		}

		wg.Add(1)
		go rs.runWorker(runCtx, runnable[idx], sem, &wg, ws)
	}

	wg.Wait()
}

func (rs *runScope) runWorker(
	ctx context.Context,
	is indexedSkill,
	sem chan struct{},
	wg *sync.WaitGroup,
	ws *workerState,
) {
	defer wg.Done()
	defer func() { <-sem }()

	if ctx.Err() != nil {
		return
	}

	idx, s := is.index, is.skill

	rs.emit(Event{
		Kind: EventStart, Index: idx, Total: rs.total,
		SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
	})

	result := rs.runSkill(ctx, s)
	rs.results[idx] = result

	rs.emit(Event{
		Kind: EventDone, Index: idx, Total: rs.total,
		SkillName: s.Name, Cost: s.Cost, Mandatory: s.Mandatory,
		Result:  &rs.results[idx],
		Elapsed: time.Duration(result.Elapsed * float64(time.Millisecond)),
	})

	if result.Status == "error" && result.ErrorDetail != "" {
		rs.emit(Event{
			Kind: EventError, Index: idx, Total: rs.total,
			SkillName: s.Name, Mandatory: s.Mandatory,
			Err: errors.New(result.ErrorDetail),
		})
	}

	if rs.opts.FailFast && result.Failed() && s.Mandatory {
		ws.triggerFailFast(rs, idx, s)
	}
}

// aggregate tallies results into a Report in original skill order.
func (rs *runScope) aggregate() *Report {
	report := &Report{
		Source:    rs.opts.Source,
		Timestamp: time.Now().Format("20060102-150405"),
	}
	for i := range rs.opts.Skills {
		r := rs.results[i]
		if r.Name == "" {
			continue
		}
		report.Total++
		switch {
		case r.Status == "skipped":
			report.Skipped++
		case r.Status == "error" || r.Failed():
			report.Failed++
			if r.Mandatory && r.Status != "skipped" {
				report.BlockingFailed++
			}
		default:
			report.Passed++
		}
		report.Results = append(report.Results, r)
	}

	if report.Skipped > 0 && report.Total > 0 && report.Skipped > report.Total/2 {
		report.SkipWarning = fmt.Sprintf(
			"%d/%d checks skipped (requires_diff without --base) — report is structurally incomplete",
			report.Skipped, report.Total,
		)
	}

	return report
}

// runSkill executes one skill and returns its Result.
// It does not mutate any shared state and is safe for concurrent use.
func (rs *runScope) runSkill(ctx context.Context, s registry.Skill) Result {
	start := time.Now()

	version := s.Version
	if version == "" {
		version = "v1"
	}
	def, err := skill.Load(rs.resolver, s.Name, version)
	if err != nil {
		return errorResult(s, start, err)
	}

	model := rs.resolveModel(s)
	output, err := rs.runner.Run(ctx, def, skill.RunOpts{
		RepoTree:    rs.repoTree,
		DiffPayload: rs.diffPayload,
		BaseRef:     rs.opts.BaseRef,
		Model:       model,
	})
	if err != nil {
		return errorResult(s, start, err)
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
		Elapsed:         float64(time.Since(start).Milliseconds()),
		BlockingDetails: output.Blocking,
		MajorDetails:    output.Major,
		WarningDetails:  output.Warning,
		InfoDetails:     output.Info,
	}
}

// resolveModel picks the model: explicit override > config routing by cost tier.
func (rs *runScope) resolveModel(s registry.Skill) agent.Model {
	if rs.opts.ModelOverride != "" {
		return agent.Model(rs.opts.ModelOverride)
	}
	if rs.opts.Config != nil {
		return agent.Model(rs.opts.Config.Models.ModelForSkill(string(s.Cost)))
	}
	return ""
}

// errorResult builds a Result for a skill that failed to load or execute.
func errorResult(s registry.Skill, start time.Time, err error) Result {
	return Result{
		Name:        s.Name,
		Status:      "error",
		ExitCode:    1,
		Mandatory:   s.Mandatory,
		Elapsed:     float64(time.Since(start).Milliseconds()),
		ErrorDetail: err.Error(),
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
