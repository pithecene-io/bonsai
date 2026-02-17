package skill_test

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/skill"
)

func TestParseOutput_Valid(t *testing.T) {
	raw := `{
		"skill": "repo-convention-enforcer",
		"version": "v1",
		"status": "pass",
		"blocking": [],
		"major": [],
		"warning": ["minor style issue"],
		"info": ["looks good"]
	}`

	out, err := skill.ParseOutput(raw)
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}

	if out.Skill != "repo-convention-enforcer" {
		t.Errorf("Skill = %q", out.Skill)
	}
	if out.Status != "pass" {
		t.Errorf("Status = %q", out.Status)
	}
	if out.ShouldFail() {
		t.Error("expected ShouldFail to be false")
	}
}

func TestParseOutput_Fail(t *testing.T) {
	raw := `{
		"skill": "test-skill",
		"version": "v1",
		"status": "fail",
		"blocking": ["critical issue"],
		"major": [],
		"warning": [],
		"info": []
	}`

	out, err := skill.ParseOutput(raw)
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}

	if !out.ShouldFail() {
		t.Error("expected ShouldFail to be true")
	}
}

func TestParseOutput_FailWithEmptyBlocking(t *testing.T) {
	raw := `{
		"skill": "test-skill",
		"version": "v1",
		"status": "fail",
		"blocking": [],
		"major": ["non-blocking issue"],
		"warning": [],
		"info": []
	}`

	out, err := skill.ParseOutput(raw)
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}

	// status=fail but blocking is empty -> should NOT cause exit 1
	if out.ShouldFail() {
		t.Error("expected ShouldFail to be false when blocking is empty")
	}
}

func TestParseOutput_WithCodeFences(t *testing.T) {
	raw := "```json\n" + `{
		"skill": "test",
		"version": "v1",
		"status": "pass",
		"blocking": [],
		"major": [],
		"warning": [],
		"info": []
	}` + "\n```"

	out, err := skill.ParseOutput(raw)
	if err != nil {
		t.Fatalf("ParseOutput: %v", err)
	}

	if out.Skill != "test" {
		t.Errorf("Skill = %q", out.Skill)
	}
}

func TestParseOutput_MissingField(t *testing.T) {
	raw := `{"skill": "test", "version": "v1", "status": "pass"}`

	_, err := skill.ParseOutput(raw)
	if err == nil {
		t.Error("expected error for missing array fields")
	}
}

func TestParseOutput_InvalidStatus(t *testing.T) {
	raw := `{
		"skill": "test",
		"version": "v1",
		"status": "maybe",
		"blocking": [],
		"major": [],
		"warning": [],
		"info": []
	}`

	_, err := skill.ParseOutput(raw)
	if err == nil {
		t.Error("expected error for invalid status enum")
	}
}

func TestParseOutput_NonStringInArray(t *testing.T) {
	raw := `{
		"skill": "test",
		"version": "v1",
		"status": "pass",
		"blocking": [123],
		"major": [],
		"warning": [],
		"info": []
	}`

	_, err := skill.ParseOutput(raw)
	if err == nil {
		t.Error("expected error for non-string element in array")
	}
}

func TestParseOutput_InvalidJSON(t *testing.T) {
	_, err := skill.ParseOutput("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseOutput_Empty(t *testing.T) {
	_, err := skill.ParseOutput("")
	if err == nil {
		t.Error("expected error for empty response")
	}
}
