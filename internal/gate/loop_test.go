package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

func TestExtractFindings(t *testing.T) {
	l := &Loop{}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0, Blocking: 0, Major: 1, Warning: 2},
			{Name: "skill-b", ExitCode: 1, Blocking: 3, Major: 1, Warning: 0},
			{Name: "skill-c", ExitCode: 1, Blocking: 1, Major: 0, Warning: 5},
		},
	}

	findings := l.extractFindings(report)

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

func TestExtractFindingsEmpty(t *testing.T) {
	l := &Loop{}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0},
			{Name: "skill-b", ExitCode: 0},
		},
	}

	findings := l.extractFindings(report)
	if findings != "" {
		t.Errorf("expected empty findings for all-passing report, got: %q", findings)
	}
}

func TestExtractFindings_AllFailed(t *testing.T) {
	l := &Loop{}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 1, Blocking: 1, Major: 0, Warning: 0},
			{Name: "skill-b", ExitCode: 1, Blocking: 0, Major: 2, Warning: 3},
		},
	}

	findings := l.extractFindings(report)
	lines := strings.Split(findings, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), findings)
	}
}

func TestExtractFindings_EmptyResults(t *testing.T) {
	l := &Loop{}
	report := &orchestrator.Report{}

	findings := l.extractFindings(report)
	if findings != "" {
		t.Errorf("expected empty findings for empty results, got: %q", findings)
	}
}

func TestPrintFailedFindings(_ *testing.T) {
	// This test just verifies it doesn't panic; output goes to stderr
	l := &Loop{}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 1, Blocking: 2, Major: 0, Warning: 0},
		},
	}
	l.printFailedFindings(report)
}

func TestPrintFailedFindings_NoneFailedNoPanic(_ *testing.T) {
	l := &Loop{}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 0},
		},
	}
	l.printFailedFindings(report)
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
