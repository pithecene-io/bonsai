package cli

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
)

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

	// Should contain skill-a and skill-c, not skill-b
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
