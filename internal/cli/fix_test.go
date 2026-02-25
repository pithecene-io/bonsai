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

func TestExtractPerSkillFindings(t *testing.T) {
	skills := []registry.Skill{
		{Name: "skill-a", Cost: "cheap"},
		{Name: "skill-b", Cost: "moderate"},
		{Name: "skill-c", Cost: "heavy"},
	}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{
				Name:            "skill-a",
				ExitCode:        1,
				BlockingDetails: []string{"blocking A"},
				MajorDetails:    []string{"major A"},
			},
			{
				Name:     "skill-b",
				ExitCode: 0, // passed — excluded
			},
			{
				Name:            "skill-c",
				ExitCode:        1,
				BlockingDetails: []string{"blocking C"},
			},
		},
	}

	got := extractPerSkillFindings(report, skills)
	if len(got) != 2 {
		t.Fatalf("extractPerSkillFindings: got %d, want 2", len(got))
	}

	// First failed skill
	if got[0].Name != "skill-a" {
		t.Errorf("got[0].Name = %q, want skill-a", got[0].Name)
	}
	if got[0].Cost != "cheap" {
		t.Errorf("got[0].Cost = %q, want cheap", got[0].Cost)
	}
	prompt := got[0].UserPrompt()
	if !strings.Contains(prompt, "blocking A") {
		t.Error("skill-a prompt missing blocking detail")
	}
	if !strings.Contains(prompt, "major A") {
		t.Error("skill-a prompt missing major detail")
	}

	// Second failed skill
	if got[1].Name != "skill-c" {
		t.Errorf("got[1].Name = %q, want skill-c", got[1].Name)
	}
	if got[1].Cost != "heavy" {
		t.Errorf("got[1].Cost = %q, want heavy", got[1].Cost)
	}
}

func TestExtractPerSkillFindings_AllPassed(t *testing.T) {
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0},
		},
	}
	got := extractPerSkillFindings(report, nil)
	if len(got) != 0 {
		t.Errorf("expected 0 findings, got %d", len(got))
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
	err := fixLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	// Session agent should never have been called
	if len(sessionMock.AutonomousCalls) != 0 {
		t.Errorf("autonomous calls = %d, want 0", len(sessionMock.AutonomousCalls))
	}
}

func TestFixLoop_FixResolvesOnFirstIteration(t *testing.T) {
	// Initial check: fail → autonomous fix → re-check: pass
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
	err := fixLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	// Session agent should have been called once (one failed skill)
	if len(sessionMock.AutonomousCalls) != 1 {
		t.Errorf("autonomous calls = %d, want 1", len(sessionMock.AutonomousCalls))
	}

	// The autonomous prompt should contain the findings
	if len(sessionMock.AutonomousCalls) > 0 {
		userPrompt := sessionMock.AutonomousCalls[0].UserPrompt
		if !strings.Contains(userPrompt, "critical issue") {
			t.Error("autonomous user prompt should contain findings from initial check")
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

	err := fixLoop(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error after max iterations exhausted")
	}
	if !strings.Contains(err.Error(), "findings remain") {
		t.Errorf("unexpected error: %v", err)
	}

	// Session agent should have been called maxIterations times
	// (one failed skill per iteration)
	if len(sessionMock.AutonomousCalls) != 2 {
		t.Errorf("autonomous calls = %d, want 2", len(sessionMock.AutonomousCalls))
	}
}

func TestFixLoop_ContextCancellation(t *testing.T) {
	// Initial check fails, then context is cancelled during autonomous session.
	ctx, cancel := context.WithCancel(context.Background())

	checkMock := &agent.MockAgent{
		NameVal:                "check",
		NonInteractiveResponse: skillJSON("fail", []string{"issue"}),
	}

	sessionMock := &cancellingMockAgent{cancel: cancel}

	opts := testFixOpts(t, checkMock, sessionMock)
	err := fixLoop(ctx, opts)
	// Should return nil (clean exit on cancellation)
	if err != nil {
		t.Errorf("expected nil on context cancellation, got: %v", err)
	}

	// Verify autonomous was actually called
	if sessionMock.calls == 0 {
		t.Error("expected autonomous session to be called")
	}
}

// cancellingMockAgent cancels a context when Autonomous is called,
// simulating the user pressing ctrl-C during a fix session.
type cancellingMockAgent struct {
	cancel func()
	calls  int
}

func (m *cancellingMockAgent) Name() string { return "cancel-mock" }

func (m *cancellingMockAgent) Interactive(_ context.Context, _ string, _ []string) error {
	return nil
}

func (m *cancellingMockAgent) NonInteractive(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (m *cancellingMockAgent) Autonomous(_ context.Context, _, _, _ string) error {
	m.calls++
	m.cancel()
	return context.Canceled
}

func TestFixLoop_FindingsPassedPerSkill(t *testing.T) {
	// Verify that per-skill findings appear in the autonomous user prompt
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
	err := fixLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	if len(sessionMock.AutonomousCalls) != 1 {
		t.Fatalf("autonomous calls = %d, want 1", len(sessionMock.AutonomousCalls))
	}

	userPrompt := sessionMock.AutonomousCalls[0].UserPrompt
	if !strings.Contains(userPrompt, "missing CLAUDE.md §4 entry") {
		t.Error("autonomous user prompt missing detailed finding text")
	}
	if !strings.Contains(userPrompt, "repo-convention-enforcer") {
		t.Error("autonomous user prompt missing skill name")
	}
}

func TestFixLoop_MultipleSkillsFailing(t *testing.T) {
	// Two skills fail — verify per-skill autonomous calls in order.
	skills := []registry.Skill{
		{
			Name:         "repo-convention-enforcer",
			Version:      "v1",
			Cost:         "cheap",
			Mode:         "deterministic",
			Mandatory:    true,
			RequiresDiff: boolPtr(false),
		},
		{
			Name:         "arch-index-alignment",
			Version:      "v1",
			Cost:         "cheap",
			Mode:         "deterministic",
			Mandatory:    true,
			RequiresDiff: boolPtr(false),
		},
	}

	// Build separate JSON for each skill (keyed by name)
	makeJSON := func(skillName, status string, blocking []string) string {
		out := struct {
			Skill    string   `json:"skill"`
			Version  string   `json:"version"`
			Status   string   `json:"status"`
			Blocking []string `json:"blocking"`
			Major    []string `json:"major"`
			Warning  []string `json:"warning"`
			Info     []string `json:"info"`
		}{
			Skill:    skillName,
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

	// Both skills get the same blocking message since the mock can't
	// distinguish parallel skill invocations by user prompt.
	var checkCallCount atomic.Int32
	checkMock := &agent.MockAgent{
		NameVal: "check",
		NonInteractiveFunc: func(_ context.Context, _, _, _ string) (string, error) {
			n := checkCallCount.Add(1)
			// First check run (2 skills): both fail
			if n <= 2 {
				return makeJSON("test", "fail", []string{"finding"}), nil
			}
			// Re-check: pass
			return makeJSON("test", "pass", []string{}), nil
		},
	}
	sessionMock := &agent.MockAgent{NameVal: "session"}

	resolver := assets.NewResolver("")
	reg := &registry.Registry{Defaults: registry.Defaults{}}
	opts := fixOpts{
		checkAgent:    checkMock,
		sessionAgent:  sessionMock,
		resolver:      resolver,
		registry:      reg,
		config:        config.Default(),
		skills:        skills,
		source:        "test:multi",
		repoRoot:      t.TempDir(),
		maxIterations: 3,
	}

	err := fixLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("fixLoop: %v", err)
	}

	// Should have 2 autonomous calls (one per failed skill)
	if len(sessionMock.AutonomousCalls) != 2 {
		t.Fatalf("autonomous calls = %d, want 2", len(sessionMock.AutonomousCalls))
	}

	// Each call should reference a different skill name in the user prompt
	firstPrompt := sessionMock.AutonomousCalls[0].UserPrompt
	secondPrompt := sessionMock.AutonomousCalls[1].UserPrompt
	hasRCE := strings.Contains(firstPrompt, "repo-convention-enforcer") || strings.Contains(secondPrompt, "repo-convention-enforcer")
	hasAIA := strings.Contains(firstPrompt, "arch-index-alignment") || strings.Contains(secondPrompt, "arch-index-alignment")
	if !hasRCE {
		t.Error("expected 'repo-convention-enforcer' in one of the autonomous prompts")
	}
	if !hasAIA {
		t.Error("expected 'arch-index-alignment' in one of the autonomous prompts")
	}
}

// --- interrupt regression tests (injected runCheck) ---

// TestFixLoop_InitialCheckInterrupt_NilReport verifies that fixLoop
// returns nil (clean exit) when the initial check returns (nil, nil),
// which signals a TUI interrupt. Before the nil guard was added this
// was a nil-pointer dereference on report.ShouldFail().
func TestFixLoop_InitialCheckInterrupt_NilReport(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "mock"}
	err := fixLoop(context.Background(), fixOpts{
		checkAgent:    mock,
		sessionAgent:  mock,
		skills:        []registry.Skill{testSkill()},
		maxIterations: 3,
		runCheck: func(_ context.Context, _ fixOpts) (*orchestrator.Report, error) {
			return nil, nil // simulate TUI interrupt
		},
	})
	if err != nil {
		t.Fatalf("fixLoop returned error on initial interrupt: %v", err)
	}
	if mock.CallCount() != 0 {
		t.Errorf("expected 0 agent calls on interrupt, got %d", mock.CallCount())
	}
}

// TestFixLoop_RecheckInterrupt_NilReport verifies that fixLoop returns
// nil (clean exit) when the re-check after a fix iteration returns
// (nil, nil). The initial check reports a failure so the loop enters
// the fix phase; the re-check then signals interrupt.
func TestFixLoop_RecheckInterrupt_NilReport(t *testing.T) {
	var checkCalls atomic.Int32
	mock := &agent.MockAgent{NameVal: "mock"}

	tmpDir := t.TempDir()
	err := fixLoop(context.Background(), fixOpts{
		checkAgent:   mock,
		sessionAgent: mock,
		resolver:     assets.NewResolver(tmpDir),
		skills:       []registry.Skill{testSkill()},
		maxIterations: 3,
		repoRoot:      tmpDir,
		runCheck: func(_ context.Context, _ fixOpts) (*orchestrator.Report, error) {
			n := checkCalls.Add(1)
			if n == 1 {
				// Initial check: report a blocking failure so the
				// loop enters the fix phase.
				return &orchestrator.Report{
					Total:          1,
					BlockingFailed: 1,
					Results: []orchestrator.Result{{
						Name:            testSkill().Name,
						ExitCode:        1,
						Blocking:        1,
						Mandatory:       true,
						BlockingDetails: []string{"finding-1"},
					}},
				}, nil
			}
			// Re-check: simulate TUI interrupt.
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("fixLoop returned error on re-check interrupt: %v", err)
	}
	if checkCalls.Load() != 2 {
		t.Errorf("expected 2 check calls (initial + re-check), got %d", checkCalls.Load())
	}
}
