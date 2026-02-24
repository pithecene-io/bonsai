package orchestrator_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
)

func passJSON() string {
	b, _ := json.Marshal(skillOutput{
		Skill: "test", Version: "v1", Status: "pass",
		Blocking: []string{}, Major: []string{}, Warning: []string{}, Info: []string{},
	})
	return string(b)
}

func failJSON() string {
	b, _ := json.Marshal(skillOutput{
		Skill: "test", Version: "v1", Status: "fail",
		Blocking: []string{"critical"}, Major: []string{}, Warning: []string{}, Info: []string{},
	})
	return string(b)
}

func TestRun_ParallelExecution(t *testing.T) {
	// 3 skills with concurrency 3 — wall time should be ~1 skill duration, not sum
	delay := 50 * time.Millisecond
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveFunc: func(_ context.Context, _, _, _ string) (string, error) {
			time.Sleep(delay)
			return passJSON(), nil
		},
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", false),
		passSkill("arch-index-alignment", false),
		passSkill("orphan-directory-detector", false),
	}

	opts := defaultOpts(skills, t.TempDir())
	opts.Concurrency = 3

	start := time.Now()
	report, err := orch.Run(context.Background(), opts, nil)
	wall := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Passed != 3 {
		t.Errorf("Passed = %d, want 3", report.Passed)
	}

	// With 3 parallel skills each taking ~50ms, wall time should be
	// well under 3*50ms = 150ms. Allow 120ms margin for overhead.
	if wall > 2*delay+70*time.Millisecond {
		t.Errorf("wall time %v too high for parallel execution (expected < %v)", wall, 2*delay+70*time.Millisecond)
	}
}

func TestRun_ParallelFailFast(t *testing.T) {
	// With concurrency=1: skills run in order.
	// Skill 0 (non-mandatory) passes, skill 1 (mandatory) fails → fail-fast,
	// skill 2 should never run.
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveFunc: func(_ context.Context, _, _, _ string) (string, error) {
			// All skills get the same fail response; mandatory matters for fail-fast
			return failJSON(), nil
		},
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", false), // non-mandatory — fails but continues
		passSkill("arch-index-alignment", true),      // mandatory — fails, triggers fail-fast
		passSkill("orphan-directory-detector", true),  // should be skipped by fail-fast
	}

	opts := defaultOpts(skills, t.TempDir())
	opts.FailFast = true
	opts.Concurrency = 1

	report, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !report.ShouldFail() {
		t.Error("expected ShouldFail() = true")
	}

	// Only 2 results: skill 0 and skill 1 (skill 2 cancelled by fail-fast)
	if len(report.Results) != 2 {
		t.Errorf("Results = %d, want 2", len(report.Results))
	}

	// Verify the mock wasn't called more than twice
	if mock.CallCount() > 2 {
		t.Errorf("mock calls = %d, want <= 2 (fail-fast should prevent third call)", mock.CallCount())
	}
}

func TestRun_EventChannel(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", false),
		passSkill("arch-index-alignment", false),
	}

	opts := defaultOpts(skills, t.TempDir())
	opts.Concurrency = 1

	events := make(chan orchestrator.Event, 100)
	report, err := orch.Run(context.Background(), opts, events)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Passed != 2 {
		t.Errorf("Passed = %d, want 2", report.Passed)
	}

	// Drain events and verify ordering invariants
	close(events)
	var evs []orchestrator.Event
	for ev := range events {
		evs = append(evs, ev)
	}

	// Expected: queued(0), queued(1), start(0), done(0), start(1), done(1), complete
	// At minimum: 2 queued + 2 start + 2 done + 1 complete = 7
	if len(evs) < 7 {
		t.Fatalf("expected at least 7 events, got %d", len(evs))
	}

	// First events should be queued
	if evs[0].Kind != orchestrator.EventQueued {
		t.Errorf("first event = %d, want EventQueued", evs[0].Kind)
	}

	// Last event should be complete
	last := evs[len(evs)-1]
	if last.Kind != orchestrator.EventComplete {
		t.Errorf("last event = %d, want EventComplete", last.Kind)
	}
	if last.Report == nil {
		t.Error("EventComplete.Report should not be nil")
	}

	// For each skill, Start must come before Done
	for _, name := range []string{"repo-convention-enforcer", "arch-index-alignment"} {
		startIdx := -1
		doneIdx := -1
		for i, ev := range evs {
			if ev.SkillName == name && ev.Kind == orchestrator.EventStart {
				startIdx = i
			}
			if ev.SkillName == name && ev.Kind == orchestrator.EventDone {
				doneIdx = i
			}
		}
		if startIdx == -1 || doneIdx == -1 {
			t.Errorf("skill %s: missing start (%d) or done (%d) event", name, startIdx, doneIdx)
			continue
		}
		if startIdx >= doneIdx {
			t.Errorf("skill %s: start (%d) should be before done (%d)", name, startIdx, doneIdx)
		}
	}
}

func TestRun_NilEventChannel(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{passSkill("repo-convention-enforcer", false)}

	// Should not panic with nil events
	report, err := orch.Run(context.Background(), defaultOpts(skills, t.TempDir()), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Passed != 1 {
		t.Errorf("Passed = %d, want 1", report.Passed)
	}
}

func TestRun_ConcurrencyOne(t *testing.T) {
	// Verify sequential behavior matches expectations
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", false),
		passSkill("arch-index-alignment", false),
		passSkill("orphan-directory-detector", false),
	}

	opts := orchestrator.RunOpts{
		Skills:              skills,
		Source:              "test",
		RepoRoot:            t.TempDir(),
		Config:              config.Default(),
		DefaultRequiresDiff: true,
		Concurrency:         1,
	}

	report, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Passed != 3 {
		t.Errorf("Passed = %d, want 3", report.Passed)
	}
	// Results should be in order
	for i, r := range report.Results {
		if r.Name != skills[i].Name {
			t.Errorf("Results[%d].Name = %s, want %s", i, r.Name, skills[i].Name)
		}
	}
}

func TestRun_FindingDetails(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveResponse: mustJSON(t, skillOutput{
			Skill:    "repo-convention-enforcer",
			Version:  "v1",
			Status:   "fail",
			Blocking: []string{"missing file X"},
			Major:    []string{"major issue Y"},
			Warning:  []string{"warning Z"},
			Info:     []string{"info W"},
		}),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{passSkill("repo-convention-enforcer", false)}

	report, err := orch.Run(context.Background(), defaultOpts(skills, t.TempDir()), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(report.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(report.Results))
	}
	r := report.Results[0]
	if len(r.BlockingDetails) != 1 || r.BlockingDetails[0] != "missing file X" {
		t.Errorf("BlockingDetails = %v, want [missing file X]", r.BlockingDetails)
	}
	if len(r.MajorDetails) != 1 || r.MajorDetails[0] != "major issue Y" {
		t.Errorf("MajorDetails = %v, want [major issue Y]", r.MajorDetails)
	}
	if len(r.WarningDetails) != 1 || r.WarningDetails[0] != "warning Z" {
		t.Errorf("WarningDetails = %v, want [warning Z]", r.WarningDetails)
	}
	if len(r.InfoDetails) != 1 || r.InfoDetails[0] != "info W" {
		t.Errorf("InfoDetails = %v, want [info W]", r.InfoDetails)
	}
	if r.Elapsed < 0 {
		t.Errorf("Elapsed = %f, want >= 0", r.Elapsed)
	}
}

func TestRun_ModelRouting(t *testing.T) {
	// Verify that the model from config.ModelRouting reaches the agent.
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)

	cheapSkill := registry.Skill{
		Name: "repo-convention-enforcer", Version: "v1", Cost: "cheap",
		RequiresDiff: boolPtr(false),
	}
	heavySkill := registry.Skill{
		Name: "arch-index-alignment", Version: "v1", Cost: "heavy",
		RequiresDiff: boolPtr(false),
	}

	cfg := config.Default()
	cfg.Agents.Models.Check.Cheap = "haiku"
	cfg.Agents.Models.Check.Heavy = "opus"

	opts := orchestrator.RunOpts{
		Skills:              []registry.Skill{cheapSkill, heavySkill},
		Source:              "test",
		RepoRoot:            t.TempDir(),
		Config:              cfg,
		DefaultRequiresDiff: true,
		Concurrency:         1,
	}

	_, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if mock.CallCount() != 2 {
		t.Fatalf("calls = %d, want 2", mock.CallCount())
	}
	if got := mock.NonInteractiveCalls[0].Model; got != "haiku" {
		t.Errorf("call[0].Model = %q, want haiku", got)
	}
	if got := mock.NonInteractiveCalls[1].Model; got != "opus" {
		t.Errorf("call[1].Model = %q, want opus", got)
	}
}

func TestRun_ModelOverride(t *testing.T) {
	// Verify that ModelOverride in RunOpts takes precedence over config routing.
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)

	cheapSkill := registry.Skill{
		Name: "repo-convention-enforcer", Version: "v1", Cost: "cheap",
		RequiresDiff: boolPtr(false),
	}
	heavySkill := registry.Skill{
		Name: "arch-index-alignment", Version: "v1", Cost: "heavy",
		RequiresDiff: boolPtr(false),
	}

	cfg := config.Default()
	// Config says cheap=haiku, heavy=sonnet, but override should win
	cfg.Agents.Models.Check.Cheap = "haiku"
	cfg.Agents.Models.Check.Heavy = "sonnet"

	opts := orchestrator.RunOpts{
		Skills:              []registry.Skill{cheapSkill, heavySkill},
		Source:              "test",
		RepoRoot:            t.TempDir(),
		Config:              cfg,
		DefaultRequiresDiff: true,
		Concurrency:         1,
		ModelOverride:       "opus",
	}

	_, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if mock.CallCount() != 2 {
		t.Fatalf("calls = %d, want 2", mock.CallCount())
	}
	// Both should be "opus" regardless of cost tier
	if got := mock.NonInteractiveCalls[0].Model; got != "opus" {
		t.Errorf("call[0].Model = %q, want opus (override)", got)
	}
	if got := mock.NonInteractiveCalls[1].Model; got != "opus" {
		t.Errorf("call[1].Model = %q, want opus (override)", got)
	}
}

func TestRun_ModelRouting_DefaultFallback(t *testing.T) {
	// Verify that an empty cost field falls back to ModelRouting.Default.
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)

	// Skill with empty cost
	noCostSkill := registry.Skill{
		Name: "repo-convention-enforcer", Version: "v1", Cost: "",
		RequiresDiff: boolPtr(false),
	}

	cfg := config.Default()
	cfg.Agents.Models.Default = "custom-model"

	opts := orchestrator.RunOpts{
		Skills:              []registry.Skill{noCostSkill},
		Source:              "test",
		RepoRoot:            t.TempDir(),
		Config:              cfg,
		DefaultRequiresDiff: true,
		Concurrency:         1,
	}

	_, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if mock.CallCount() != 1 {
		t.Fatalf("calls = %d, want 1", mock.CallCount())
	}
	if got := mock.NonInteractiveCalls[0].Model; got != "custom-model" {
		t.Errorf("call[0].Model = %q, want custom-model (default fallback)", got)
	}
}

func TestRun_ModelRouting_NilConfig(t *testing.T) {
	// Verify no panic when Config is nil (model should be empty).
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: passJSON(),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{passSkill("repo-convention-enforcer", false)}

	opts := orchestrator.RunOpts{
		Skills:              skills,
		Source:              "test",
		RepoRoot:            t.TempDir(),
		Config:              nil,
		DefaultRequiresDiff: true,
		Concurrency:         1,
	}

	_, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if mock.CallCount() != 1 {
		t.Fatalf("calls = %d, want 1", mock.CallCount())
	}
	if got := mock.NonInteractiveCalls[0].Model; got != "" {
		t.Errorf("call[0].Model = %q, want empty (nil config)", got)
	}
}
