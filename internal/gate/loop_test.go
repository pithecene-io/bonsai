package gate

import (
	"strings"
	"testing"

	"github.com/justapithecus/bonsai/internal/orchestrator"
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

func TestPrintFailedFindings(t *testing.T) {
	// This test just verifies it doesn't panic; output goes to stderr
	l := &Loop{}
	report := &orchestrator.Report{
		Results: []orchestrator.Result{
			{Name: "skill-a", ExitCode: 1, Blocking: 2, Major: 0, Warning: 0},
		},
	}
	l.printFailedFindings(report)
}
