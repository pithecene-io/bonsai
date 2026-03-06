package gate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

func TestReportFindingSummary(t *testing.T) {
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0, Blocking: 0, Major: 1, Warning: 2},
			{Name: "skill-b", ExitCode: 1, Blocking: 3, Major: 1, Warning: 0},
			{Name: "skill-c", ExitCode: 1, Blocking: 1, Major: 0, Warning: 5},
		},
	}

	findings := report.FindingSummary()

	if findings == "" {
		t.Fatal("expected non-empty findings")
	}

	// Only failed skills should appear
	if strings.Contains(findings, "skill-a") {
		t.Error("expected skill-a NOT in findings (it passed)")
	}
	if !strings.Contains(findings, "skill-b") {
		t.Error("expected skill-b in findings")
	}
	if !strings.Contains(findings, "skill-c") {
		t.Error("expected skill-c in findings")
	}

	// Check exact format matches ai-implement.sh
	if !strings.Contains(findings, "SKILL: skill-b | blocking: 3 | major: 1 | warning: 0") {
		t.Errorf("unexpected format for skill-b: %s", findings)
	}
	if !strings.Contains(findings, "SKILL: skill-c | blocking: 1 | major: 0 | warning: 5") {
		t.Errorf("unexpected format for skill-c: %s", findings)
	}
}

func TestReportFindingSummary_Empty(t *testing.T) {
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0},
			{Name: "skill-b", ExitCode: 0},
		},
	}

	findings := report.FindingSummary()
	if findings != "" {
		t.Errorf("expected empty findings for all-passing report, got: %q", findings)
	}
}

func TestReportPrintFindings(_ *testing.T) {
	// This test just verifies it doesn't panic; output goes to stderr
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 1, Blocking: 2, Major: 0, Warning: 0},
		},
	}
	report.PrintFindings(os.Stderr)
}

func TestReportPrintFindings_NoneFailedNoPanic(_ *testing.T) {
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0},
		},
	}
	report.PrintFindings(os.Stderr)
}

func TestConsumePlan_Valid(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "ai", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	plan := PlanInfo{
		Intent:      "refactor auth module",
		Constraints: json.RawMessage(`{"max_files":3}`),
	}
	data, _ := json.Marshal(plan)
	planPath := filepath.Join(outDir, "plan.json")
	if err := os.WriteFile(planPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	l := &Loop{
		opts: Opts{
			RepoRoot: dir,
			Config:   &config.Config{Output: config.OutputConfig{Dir: "ai/out"}},
		},
	}

	l.consumePlan()

	if l.planInfo == nil {
		t.Fatal("expected planInfo to be set")
	}
	if l.planInfo.Intent != "refactor auth module" {
		t.Errorf("Intent = %q, want %q", l.planInfo.Intent, "refactor auth module")
	}

	// Verify plan.json was renamed to plan.consumed.json
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Error("plan.json should have been renamed")
	}
	consumedPath := filepath.Join(outDir, "plan.consumed.json")
	if _, err := os.Stat(consumedPath); err != nil {
		t.Errorf("plan.consumed.json should exist: %v", err)
	}
}

func TestConsumePlan_NoPlanFile(t *testing.T) {
	dir := t.TempDir()

	l := &Loop{
		opts: Opts{
			RepoRoot: dir,
			Config:   &config.Config{Output: config.OutputConfig{Dir: "ai/out"}},
		},
	}

	// Should not panic when plan.json doesn't exist
	l.consumePlan()

	if l.planInfo != nil {
		t.Error("expected planInfo to be nil when no plan.json exists")
	}
}

func TestConsumePlan_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "ai", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(outDir, "plan.json")
	if err := os.WriteFile(planPath, []byte("not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := &Loop{
		opts: Opts{
			RepoRoot: dir,
			Config:   &config.Config{Output: config.OutputConfig{Dir: "ai/out"}},
		},
	}

	// Should not panic; logs warning and returns
	l.consumePlan()

	if l.planInfo != nil {
		t.Error("expected planInfo to be nil for invalid JSON")
	}

	// Original file should still exist (not renamed)
	if _, err := os.Stat(planPath); err != nil {
		t.Error("plan.json should still exist after parse failure")
	}
}

func TestConsumePlan_EmptyIntent(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "ai", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	plan := PlanInfo{
		Intent:      "",
		Constraints: json.RawMessage(`{}`),
	}
	data, _ := json.Marshal(plan)
	planPath := filepath.Join(outDir, "plan.json")
	if err := os.WriteFile(planPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	l := &Loop{
		opts: Opts{
			RepoRoot: dir,
			Config:   &config.Config{Output: config.OutputConfig{Dir: "ai/out"}},
		},
	}

	l.consumePlan()

	if l.planInfo == nil {
		t.Fatal("expected planInfo to be set even with empty intent")
	}
	if l.planInfo.Intent != "" {
		t.Errorf("Intent = %q, want empty", l.planInfo.Intent)
	}

	// Should still rename the file
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Error("plan.json should have been renamed even with empty intent")
	}
}

func TestPlanInfo_JSONRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		intent string
	}{
		{
			name:   "full plan",
			input:  `{"intent":"add auth","constraints":{"max_files":3}}`,
			intent: "add auth",
		},
		{
			name:   "empty constraints",
			input:  `{"intent":"cleanup","constraints":{}}`,
			intent: "cleanup",
		},
		{
			name:   "null constraints",
			input:  `{"intent":"fix bug","constraints":null}`,
			intent: "fix bug",
		},
		{
			name:   "missing constraints",
			input:  `{"intent":"quick fix"}`,
			intent: "quick fix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var plan PlanInfo
			if err := json.Unmarshal([]byte(tt.input), &plan); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if plan.Intent != tt.intent {
				t.Errorf("Intent = %q, want %q", plan.Intent, tt.intent)
			}
		})
	}
}

func TestBuildPlanPrompt_WithPlan(t *testing.T) {
	l := &Loop{
		planInfo: &PlanInfo{
			Intent:      "refactor auth module",
			Constraints: json.RawMessage(`{"max_files":3}`),
		},
	}

	got := l.buildPlanPrompt("")
	if got == "" {
		t.Fatal("expected non-empty prompt when plan is present")
	}
	if !strings.Contains(got, "refactor auth module") {
		t.Error("expected intent in prompt")
	}
	if !strings.Contains(got, `"max_files":3`) {
		t.Error("expected constraints in prompt")
	}
}

func TestBuildPlanPrompt_WithFindings(t *testing.T) {
	l := &Loop{
		planInfo: &PlanInfo{
			Intent:      "add feature X",
			Constraints: json.RawMessage(`{}`),
		},
	}

	got := l.buildPlanPrompt("SKILL: lint | blocking: 2 | major: 0 | warning: 0")
	if !strings.Contains(got, "add feature X") {
		t.Error("expected intent in prompt")
	}
	if !strings.Contains(got, "SKILL: lint") {
		t.Error("expected findings in prompt")
	}
	// Empty constraints ({}) should not appear
	if strings.Contains(got, "Constraints:") {
		t.Error("empty constraints should be omitted")
	}
}

func TestBuildPlanPrompt_NoPlan(t *testing.T) {
	l := &Loop{planInfo: nil}
	if got := l.buildPlanPrompt(""); got != "" {
		t.Errorf("expected empty prompt without plan, got %q", got)
	}
}

func TestBuildPlanPrompt_EmptyIntent(t *testing.T) {
	l := &Loop{
		planInfo: &PlanInfo{
			Intent:      "",
			Constraints: json.RawMessage(`{"max_files":3}`),
		},
	}
	if got := l.buildPlanPrompt(""); got != "" {
		t.Errorf("expected empty prompt for empty intent, got %q", got)
	}
}

func TestNew(t *testing.T) {
	cfg := config.Default()
	opts := Opts{
		RepoRoot: "/some/path",
		Config:   cfg,
	}

	l := New(opts)
	if l == nil {
		t.Fatal("New returned nil")
	}
	if l.opts.RepoRoot != "/some/path" {
		t.Errorf("RepoRoot = %q, want /some/path", l.opts.RepoRoot)
	}
	if l.planInfo != nil {
		t.Error("expected planInfo to be nil on new loop")
	}
	if l.mergeBase != "" {
		t.Error("expected mergeBase to be empty on new loop")
	}
}

func TestSaveArtifacts_CreatesOutputDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "ai", "out")

	l := &Loop{
		opts: Opts{
			RepoRoot: dir,
			Config:   &config.Config{Output: config.OutputConfig{Dir: "ai/out"}},
		},
		mergeBase: "abc123",
	}

	// saveArtifacts with no git repo will fail on the diff call,
	// but should still create the output directory and write the report
	report := &orchestrator.Report{
		Source:  "test",
		Total:   2,
		Passed:  2,
		Failed:  0,
		Skipped: 0,
		Results: []orchestrator.Result{{Name: "skill-a", ExitCode: 0}},
	}

	l.saveArtifacts(report)

	// Output directory should have been created
	info, err := os.Stat(outDir)
	if err != nil {
		t.Fatalf("output dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected output path to be a directory")
	}

	// Report file should exist (even if patch fails due to no git)
	reportPath := filepath.Join(outDir, "last.report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	var saved orchestrator.Report
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("failed to parse saved report: %v", err)
	}
	if saved.Total != 2 {
		t.Errorf("saved report Total = %d, want 2", saved.Total)
	}
	if saved.Source != "test" {
		t.Errorf("saved report Source = %q, want test", saved.Source)
	}
}

// --- runSession integration tests (mock agent) ---

// newTestLoop creates a Loop with a mock agent and embedded resolver,
// suitable for testing runSession dispatch without a real git repo.
func newTestLoop(t *testing.T, mock *agent.MockAgent, plan *PlanInfo) *Loop {
	t.Helper()
	return &Loop{
		opts: Opts{
			RepoRoot:  t.TempDir(),
			Config:    config.Default(),
			Agent:     mock,
			Resolver:  assets.NewResolver(""),
			ExtraArgs: []string{"--extra"},
		},
		planInfo: plan,
	}
}

func TestRunSession_WithPlan_CallsExecute(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	plan := &PlanInfo{
		Intent:      "add feature X",
		Constraints: json.RawMessage(`{"max_files":5}`),
	}
	l := newTestLoop(t, mock, plan)

	err := l.runSession(t.Context(), "")
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	// Execute should be called (one-shot mode per CONTRACT_GATING)
	if len(mock.ExecuteCalls) != 1 {
		t.Fatalf("ExecuteCalls = %d, want 1", len(mock.ExecuteCalls))
	}
	// Session should NOT be called when plan is present
	if len(mock.SessionCalls) != 0 {
		t.Errorf("SessionCalls = %d, want 0 (plan present → Execute)", len(mock.SessionCalls))
	}

	// The user prompt must contain the plan intent
	userPrompt := mock.ExecuteCalls[0].UserPrompt
	if !strings.Contains(userPrompt, "add feature X") {
		t.Errorf("Execute user prompt missing intent: %q", userPrompt)
	}
	if !strings.Contains(userPrompt, `"max_files":5`) {
		t.Errorf("Execute user prompt missing constraints: %q", userPrompt)
	}
}

func TestRunSession_WithPlan_IncludesFindings(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	plan := &PlanInfo{
		Intent:      "fix auth",
		Constraints: json.RawMessage(`{}`),
	}
	l := newTestLoop(t, mock, plan)

	findings := "SKILL: lint | blocking: 1 | major: 0 | warning: 0"
	err := l.runSession(t.Context(), findings)
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	if len(mock.ExecuteCalls) != 1 {
		t.Fatalf("ExecuteCalls = %d, want 1", len(mock.ExecuteCalls))
	}

	userPrompt := mock.ExecuteCalls[0].UserPrompt
	if !strings.Contains(userPrompt, "SKILL: lint") {
		t.Errorf("Execute user prompt missing findings context: %q", userPrompt)
	}
}

func TestRunSession_NoPlan_CallsSession(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	l := newTestLoop(t, mock, nil) // no plan

	err := l.runSession(t.Context(), "")
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	// Session should have been called (interactive mode)
	if len(mock.SessionCalls) != 1 {
		t.Fatalf("SessionCalls = %d, want 1", len(mock.SessionCalls))
	}
	// Execute should NOT have been called
	if len(mock.ExecuteCalls) != 0 {
		t.Errorf("ExecuteCalls = %d, want 0 (no plan → Session, not Execute)", len(mock.ExecuteCalls))
	}

	// ExtraArgs should be passed through to Session
	args := mock.SessionCalls[0].ExtraArgs
	found := false
	for _, a := range args {
		if a == "--extra" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Session extra args missing --extra: %v", args)
	}
}

func TestRunSession_EmptyIntent_CallsSession(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	plan := &PlanInfo{
		Intent:      "",
		Constraints: json.RawMessage(`{"max_files":3}`),
	}
	l := newTestLoop(t, mock, plan)

	err := l.runSession(t.Context(), "")
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	// Empty intent → interactive session, not execute
	if len(mock.SessionCalls) != 1 {
		t.Errorf("SessionCalls = %d, want 1 (empty intent → interactive)", len(mock.SessionCalls))
	}
	if len(mock.ExecuteCalls) != 0 {
		t.Errorf("ExecuteCalls = %d, want 0 (empty intent → interactive)", len(mock.ExecuteCalls))
	}
}

func TestRunSession_WithPlan_ExecuteDoesNotUseExtraArgs(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}
	plan := &PlanInfo{Intent: "do something"}
	l := newTestLoop(t, mock, plan)

	err := l.runSession(t.Context(), "")
	if err != nil {
		t.Fatalf("runSession: %v", err)
	}

	// Execute mode uses model from config, not extra CLI args.
	// Extra args (-- extra-args...) apply to Session mode only.
	// This is by design: Execute takes (systemPrompt, userPrompt, model).
	if len(mock.ExecuteCalls) != 1 {
		t.Fatalf("ExecuteCalls = %d, want 1", len(mock.ExecuteCalls))
	}
	if len(mock.SessionCalls) != 0 {
		t.Errorf("SessionCalls = %d, want 0 (plan present → Execute)", len(mock.SessionCalls))
	}
}

func TestRunSession_ExecuteError_NotFatal(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:    "test",
		ExecuteErr: errors.New("agent crashed"),
	}
	plan := &PlanInfo{Intent: "do something"}
	l := newTestLoop(t, mock, plan)

	// Execute errors should be swallowed (match shell `claude ... || true`)
	err := l.runSession(t.Context(), "")
	if err != nil {
		t.Errorf("runSession should not propagate Execute errors, got: %v", err)
	}
}

func TestRunSession_SessionError_NotFatal(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:    "test",
		SessionErr: errors.New("session crashed"),
	}
	l := newTestLoop(t, mock, nil) // no plan → session

	// Session errors should be swallowed (match shell `claude ... || true`)
	err := l.runSession(t.Context(), "")
	if err != nil {
		t.Errorf("runSession should not propagate Session errors, got: %v", err)
	}
}
