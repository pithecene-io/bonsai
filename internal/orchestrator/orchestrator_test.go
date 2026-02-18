package orchestrator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
)

// skillOutput is a helper to build a valid skill JSON response.
type skillOutput struct {
	Skill    string   `json:"skill"`
	Version  string   `json:"version"`
	Status   string   `json:"status"`
	Blocking []string `json:"blocking"`
	Major    []string `json:"major"`
	Warning  []string `json:"warning"`
	Info     []string `json:"info"`
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func boolPtr(v bool) *bool { return &v }

// passSkill returns a registry.Skill with requires_diff=false and mandatory.
func passSkill(name string, mandatory bool) registry.Skill {
	return registry.Skill{
		Name:         name,
		Version:      "v1",
		Cost:         "cheap",
		Mode:         "deterministic",
		Mandatory:    mandatory,
		RequiresDiff: boolPtr(false),
	}
}

// newTestOrch creates an orchestrator with a mock agent and embedded-only resolver.
func newTestOrch(t *testing.T, mock *agent.MockAgent) *orchestrator.Orchestrator {
	t.Helper()
	resolver := assets.NewResolver("")
	return orchestrator.New(mock, resolver)
}

func defaultOpts(skills []registry.Skill, repoRoot string) orchestrator.RunOpts {
	return orchestrator.RunOpts{
		Skills:              skills,
		Source:              "test",
		RepoRoot:            repoRoot,
		Config:              config.Default(),
		DefaultRequiresDiff: true,
	}
}

func TestRun_AllSkillsPass(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveResponse: mustJSON(t, skillOutput{
			Skill:    "repo-convention-enforcer",
			Version:  "v1",
			Status:   "pass",
			Blocking: []string{},
			Major:    []string{},
			Warning:  []string{},
			Info:     []string{},
		}),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", true),
		passSkill("arch-index-alignment", false),
	}

	report, err := orch.Run(context.Background(), defaultOpts(skills, t.TempDir()), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Passed != 2 {
		t.Errorf("Passed = %d, want 2", report.Passed)
	}
	if report.ShouldFail() {
		t.Error("expected ShouldFail() = false")
	}
}

func TestRun_MandatoryFailure(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveResponse: mustJSON(t, skillOutput{
			Skill:    "repo-convention-enforcer",
			Version:  "v1",
			Status:   "fail",
			Blocking: []string{"critical violation"},
			Major:    []string{},
			Warning:  []string{},
			Info:     []string{},
		}),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", true),
	}

	report, err := orch.Run(context.Background(), defaultOpts(skills, t.TempDir()), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.BlockingFailed != 1 {
		t.Errorf("BlockingFailed = %d, want 1", report.BlockingFailed)
	}
	if !report.ShouldFail() {
		t.Error("expected ShouldFail() = true")
	}
}

func TestRun_FailFast(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveResponse: mustJSON(t, skillOutput{
			Skill:    "repo-convention-enforcer",
			Version:  "v1",
			Status:   "fail",
			Blocking: []string{"critical"},
			Major:    []string{},
			Warning:  []string{},
			Info:     []string{},
		}),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", false), // non-mandatory — runs, fails, continues
		passSkill("arch-index-alignment", true),      // mandatory — fails, triggers fail-fast
		passSkill("orphan-directory-detector", true), // should be skipped by fail-fast
	}

	opts := defaultOpts(skills, t.TempDir())
	opts.FailFast = true

	report, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Should have exactly 2 results: first non-mandatory + second mandatory that triggers fail-fast
	if len(report.Results) != 2 {
		t.Errorf("Results = %d, want 2", len(report.Results))
	}
}

func TestRun_SkippedRequiresDiff(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	orch := newTestOrch(t, mock)

	// Skill with requires_diff=true (default) and no BaseRef → should be skipped
	s := registry.Skill{
		Name:      "repo-convention-enforcer",
		Version:   "v1",
		Mandatory: false,
	}
	// RequiresDiff is nil → uses DefaultRequiresDiff (true)

	opts := defaultOpts([]registry.Skill{s}, t.TempDir())
	opts.BaseRef = "" // no base ref

	report, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRun_AllSkipped_ShouldFail(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	orch := newTestOrch(t, mock)

	skills := []registry.Skill{
		{Name: "repo-convention-enforcer", Version: "v1"},
		{Name: "arch-index-alignment", Version: "v1"},
	}

	opts := defaultOpts(skills, t.TempDir())
	opts.BaseRef = "" // no base ref → both require diff → both skipped

	report, err := orch.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", report.Skipped)
	}
	if !report.ShouldFail() {
		t.Error("expected ShouldFail() = true when all skipped")
	}
}

func TestRun_SkillLoadError(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	orch := newTestOrch(t, mock)

	skills := []registry.Skill{
		passSkill("nonexistent-skill-that-does-not-exist", false),
	}

	report, err := orch.Run(context.Background(), defaultOpts(skills, t.TempDir()), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}
}

func TestRun_NonMandatoryFailure(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal: "test",
		NonInteractiveResponse: mustJSON(t, skillOutput{
			Skill:    "repo-convention-enforcer",
			Version:  "v1",
			Status:   "fail",
			Blocking: []string{"issue"},
			Major:    []string{},
			Warning:  []string{},
			Info:     []string{},
		}),
	}

	orch := newTestOrch(t, mock)
	skills := []registry.Skill{
		passSkill("repo-convention-enforcer", false), // non-mandatory
	}

	report, err := orch.Run(context.Background(), defaultOpts(skills, t.TempDir()), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.BlockingFailed != 0 {
		t.Errorf("BlockingFailed = %d, want 0", report.BlockingFailed)
	}
	if report.ShouldFail() {
		t.Error("expected ShouldFail() = false for non-mandatory failure")
	}
}

func TestReport_ShouldFail(t *testing.T) {
	tests := []struct {
		name   string
		report orchestrator.Report
		want   bool
	}{
		{
			name:   "all passed",
			report: orchestrator.Report{Total: 2, Passed: 2},
			want:   false,
		},
		{
			name:   "blocking failed",
			report: orchestrator.Report{Total: 2, Failed: 1, BlockingFailed: 1},
			want:   true,
		},
		{
			name:   "all skipped",
			report: orchestrator.Report{Total: 2, Skipped: 2},
			want:   true,
		},
		{
			name:   "empty report",
			report: orchestrator.Report{},
			want:   false,
		},
		{
			name:   "some skipped some passed",
			report: orchestrator.Report{Total: 3, Passed: 1, Skipped: 2},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.report.ShouldFail(); got != tt.want {
				t.Errorf("ShouldFail() = %v, want %v", got, tt.want)
			}
		})
	}
}
