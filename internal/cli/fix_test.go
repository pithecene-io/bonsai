package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
)

// --- helper tests (pure functions) ---

func TestFilterCheapSkills(t *testing.T) {
	skills := []registry.Skill{
		{Name: "a", Cost: "cheap"},
		{Name: "b", Cost: "moderate"},
		{Name: "c", Cost: "heavy"},
		{Name: "d", Cost: "cheap"},
		{Name: "e", Cost: ""},
	}

	got := filterCheapSkills(skills)
	if len(got) != 2 {
		t.Fatalf("filterCheapSkills: got %d skills, want 2", len(got))
	}
	if got[0].Name != "a" || got[1].Name != "d" {
		t.Errorf("filterCheapSkills: got [%s, %s], want [a, d]", got[0].Name, got[1].Name)
	}
}

func TestFilterCheapSkills_NoCheap(t *testing.T) {
	skills := []registry.Skill{
		{Name: "a", Cost: "moderate"},
		{Name: "b", Cost: "heavy"},
	}

	got := filterCheapSkills(skills)
	if len(got) != 0 {
		t.Errorf("filterCheapSkills: got %d skills, want 0", len(got))
	}
}

func TestFilterCheapSkills_Empty(t *testing.T) {
	got := filterCheapSkills(nil)
	if len(got) != 0 {
		t.Errorf("filterCheapSkills(nil): got %d skills, want 0", len(got))
	}
}

func TestExtractDetailedFindings(t *testing.T) {
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{
				Name:            "skill-a",
				ExitCode:        1,
				BlockingDetails: []string{"missing file X"},
				MajorDetails:    []string{"major issue Y"},
				WarningDetails:  []string{"warning Z"},
			},
			{
				Name:     "skill-b",
				ExitCode: 0, // passed — should be excluded
			},
			{
				Name:            "skill-c",
				ExitCode:        1,
				BlockingDetails: []string{"another blocking"},
			},
		},
	}

	got := extractDetailedFindings(report)

	if !contains(got, "SKILL: skill-a") {
		t.Error("missing skill-a header")
	}
	if !contains(got, "blocking: missing file X") {
		t.Error("missing skill-a blocking detail")
	}
	if !contains(got, "major: major issue Y") {
		t.Error("missing skill-a major detail")
	}
	if !contains(got, "warning: warning Z") {
		t.Error("missing skill-a warning detail")
	}
	if contains(got, "skill-b") {
		t.Error("should not contain passing skill-b")
	}
	if !contains(got, "SKILL: skill-c") {
		t.Error("missing skill-c header")
	}
	if !contains(got, "blocking: another blocking") {
		t.Error("missing skill-c blocking detail")
	}
}

func TestExtractDetailedFindings_AllPassed(t *testing.T) {
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0},
		},
	}
	got := extractDetailedFindings(report)
	if got != "" {
		t.Errorf("expected empty string for all-passed report, got %q", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- fixLoop flow tests ---

// skillJSON builds a valid skill output JSON for the mock agent.
func skillJSON(status string, blocking []string) string {
	out := struct {
		Skill    string   `json:"skill"`
		Version  string   `json:"version"`
		Status   string   `json:"status"`
		Blocking []string `json:"blocking"`
		Major    []string `json:"major"`
		Warning  []string `json:"warning"`
		Info     []string `json:"info"`
	}{
		Skill:    "test",
		Version:  "v1",
		Status:   status,
		Blocking: blocking,
		Major:    []string{},
		Warning:  []string{},
		Info:     []string{},
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func boolPtr(v bool) *bool { return &v }

// testSkill returns a skill referencing a real embedded skill name so
// skill.Load succeeds, with requires_diff=false so it runs without --base.
func testSkill() registry.Skill {
	return registry.Skill{
		Name:         "repo-convention-enforcer",
		Version:      "v1",
		Cost:         "cheap",
		Mode:         "deterministic",
		Mandatory:    true,
		RequiresDiff: boolPtr(false),
	}
}

// testFixOpts returns fixOpts wired to mock agents for testing.
func testFixOpts(t *testing.T, checkMock, sessionMock agent.Agent) fixOpts {
	t.Helper()
	resolver := assets.NewResolver("")
	reg := &registry.Registry{
		Defaults: registry.Defaults{},
	}
	return fixOpts{
		checkAgent:    checkMock,
		sessionAgent:  sessionMock,
		resolver:      resolver,
		registry:      reg,
		config:        config.Default(),
		skills:        []registry.Skill{testSkill()},
		source:        "test:fix",
		repoRoot:      t.TempDir(),
		maxIterations: 3,
	}
}

func TestFixLoop_InitialCheckPasses(t *testing.T) {
	// All skills pass on initial check → "nothing to fix"
	checkMock := &agent.MockAgent{
		NameVal:                "check",
		NonInteractiveResponse: skillJSON("pass", []string{}),
	}
	sessionMock := &agent.MockAgent{NameVal: "session"}

	opts := testFixOpts(t, checkMock, sessionMock)
	err := fixLoop(context.Background(), opts, runFixCheck)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	// Session agent should never have been called
	if len(sessionMock.InteractiveCalls) != 0 {
		t.Errorf("interactive calls = %d, want 0", len(sessionMock.InteractiveCalls))
	}
}

func TestFixLoop_FixResolvesOnFirstIteration(t *testing.T) {
	// Initial check: fail → fix session → re-check: pass
	var callCount atomic.Int32
	checkMock := &agent.MockAgent{
		NameVal: "check",
		NonInteractiveFunc: func(_ context.Context, _, _, _ string) (string, error) {
			n := callCount.Add(1)
			if n == 1 {
				// Initial check: fail
				return skillJSON("fail", []string{"critical issue"}), nil
			}
			// Re-check after fix: pass
			return skillJSON("pass", []string{}), nil
		},
	}
	sessionMock := &agent.MockAgent{NameVal: "session"}

	opts := testFixOpts(t, checkMock, sessionMock)
	err := fixLoop(context.Background(), opts, runFixCheck)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	// Session agent should have been called once
	if len(sessionMock.InteractiveCalls) != 1 {
		t.Errorf("interactive calls = %d, want 1", len(sessionMock.InteractiveCalls))
	}

	// The interactive prompt should contain the findings
	if len(sessionMock.InteractiveCalls) > 0 {
		prompt := sessionMock.InteractiveCalls[0].SystemPrompt
		if !strings.Contains(prompt, "critical issue") {
			t.Error("interactive prompt should contain findings from initial check")
		}
	}

	// Artifacts should be saved
	reportPath := filepath.Join(opts.repoRoot, config.Default().Output.Dir, "fix.report.json")
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Error("fix.report.json not saved after successful fix")
	}
}

func TestFixLoop_MaxIterationsExhausted(t *testing.T) {
	// Every check fails → exhausts max iterations → error
	checkMock := &agent.MockAgent{
		NameVal:                "check",
		NonInteractiveResponse: skillJSON("fail", []string{"persistent issue"}),
	}
	sessionMock := &agent.MockAgent{NameVal: "session"}

	opts := testFixOpts(t, checkMock, sessionMock)
	opts.maxIterations = 2

	err := fixLoop(context.Background(), opts, runFixCheck)
	if err == nil {
		t.Fatal("expected error after max iterations exhausted")
	}
	if !strings.Contains(err.Error(), "findings remain") {
		t.Errorf("unexpected error: %v", err)
	}

	// Session agent should have been called maxIterations times
	if len(sessionMock.InteractiveCalls) != 2 {
		t.Errorf("interactive calls = %d, want 2", len(sessionMock.InteractiveCalls))
	}
}

func TestFixLoop_ContextCancellation(t *testing.T) {
	// Initial check fails, then context is cancelled during interactive session.
	// The session mock cancels the context on Interactive(), simulating ctrl-C.
	ctx, cancel := context.WithCancel(context.Background())

	checkMock := &agent.MockAgent{
		NameVal:                "check",
		NonInteractiveResponse: skillJSON("fail", []string{"issue"}),
	}

	sessionMock := &cancellingMockAgent{cancel: cancel}

	opts := testFixOpts(t, checkMock, sessionMock)
	err := fixLoop(ctx, opts, runFixCheck)
	// Should return nil (clean exit on cancellation)
	if err != nil {
		t.Errorf("expected nil on context cancellation, got: %v", err)
	}

	// Verify interactive was actually called
	if sessionMock.calls == 0 {
		t.Error("expected interactive session to be called")
	}
}

// cancellingMockAgent cancels a context when Interactive is called,
// simulating the user pressing ctrl-C during a fix session.
type cancellingMockAgent struct {
	cancel func()
	calls  int
}

func (m *cancellingMockAgent) Name() string { return "cancel-mock" }

func (m *cancellingMockAgent) Interactive(_ context.Context, _ string, _ []string) error {
	m.calls++
	m.cancel()
	return context.Canceled
}

func (m *cancellingMockAgent) NonInteractive(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func TestFixLoop_FindingsPassedToSession(t *testing.T) {
	// Verify that detailed findings from check appear in the session prompt
	var callCount atomic.Int32
	checkMock := &agent.MockAgent{
		NameVal: "check",
		NonInteractiveFunc: func(_ context.Context, _, _, _ string) (string, error) {
			n := callCount.Add(1)
			if n == 1 {
				return skillJSON("fail", []string{"missing CLAUDE.md §4 entry"}), nil
			}
			return skillJSON("pass", []string{}), nil
		},
	}
	sessionMock := &agent.MockAgent{NameVal: "session"}

	opts := testFixOpts(t, checkMock, sessionMock)
	err := fixLoop(context.Background(), opts, runFixCheck)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	if len(sessionMock.InteractiveCalls) != 1 {
		t.Fatalf("interactive calls = %d, want 1", len(sessionMock.InteractiveCalls))
	}

	prompt := sessionMock.InteractiveCalls[0].SystemPrompt
	if !strings.Contains(prompt, "missing CLAUDE.md §4 entry") {
		t.Error("session prompt missing detailed finding text")
	}
	if !strings.Contains(prompt, "Previous governance findings") {
		t.Error("session prompt missing findings header")
	}
}

func TestFixLoop_NilReportNilErr_InitialCheck(t *testing.T) {
	// Regression: if the check function returns (nil, nil) — e.g. due to
	// an interrupt racing with result construction — fixLoop must not panic.
	sessionMock := &agent.MockAgent{NameVal: "session"}
	opts := testFixOpts(t, nil, sessionMock)

	nilCheck := func(context.Context, agent.Agent, *assets.Resolver,
		[]registry.Skill, string, string, string,
		*config.Config, *registry.Registry,
	) (*orchestrator.Report, error) {
		return nil, nil //nolint:nilnil // testing the nil-report guard
	}

	err := fixLoop(context.Background(), opts, nilCheck)
	if err != nil {
		t.Fatalf("expected nil error for (nil, nil) check return, got: %v", err)
	}

	// Session agent must not have been called.
	if len(sessionMock.InteractiveCalls) != 0 {
		t.Errorf("interactive calls = %d, want 0", len(sessionMock.InteractiveCalls))
	}
}

func TestFixLoop_NilReportNilErr_Recheck(t *testing.T) {
	// Regression: (nil, nil) during re-check after a fix session must
	// also be handled safely — no panic, clean exit.
	var callCount atomic.Int32
	sessionMock := &agent.MockAgent{NameVal: "session"}
	opts := testFixOpts(t, nil, sessionMock)

	recheckNil := func(context.Context, agent.Agent, *assets.Resolver,
		[]registry.Skill, string, string, string,
		*config.Config, *registry.Registry,
	) (*orchestrator.Report, error) {
		n := callCount.Add(1)
		if n == 1 {
			// Initial check: findings exist.
			return &orchestrator.Report{
				Total: 1, Failed: 1, BlockingFailed: 1,
				Results: []orchestrator.Result{
					{
						Name: "s", ExitCode: 1, Mandatory: true,
						BlockingDetails: []string{"issue"},
					},
				},
			}, nil
		}
		// Re-check: interrupt → (nil, nil)
		return nil, nil //nolint:nilnil // testing the nil-report guard
	}

	err := fixLoop(context.Background(), opts, recheckNil)
	if err != nil {
		t.Fatalf("expected nil error for (nil, nil) re-check return, got: %v", err)
	}
}
