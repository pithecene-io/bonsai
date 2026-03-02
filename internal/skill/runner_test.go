package skill_test

import (
	"errors"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/skill"
)

func TestRunner_Run_ValidResponse(t *testing.T) {
	validJSON := `{
		"skill": "test-skill",
		"version": "v1",
		"status": "pass",
		"blocking": [],
		"major": [],
		"warning": [],
		"info": ["all good"]
	}`

	mock := &agent.MockAgent{
		NameVal:                "mock",
		EvaluateResponse: validJSON,
	}
	resolver := assets.NewResolver("")
	builder := prompt.NewBuilder(resolver, "")
	runner := skill.NewRunner(mock, builder)

	def := &skill.Definition{
		Name:         "test-skill",
		Body:         "You are a test skill.",
		OutputSchema: `{"type":"object"}`,
		InputSchema:  `{"type":"object"}`,
	}

	out, err := runner.Run(t.Context(), def, skill.RunOpts{
		RepoTree: "file1.go\nfile2.go\n",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Status != "pass" {
		t.Errorf("Status = %q, want pass", out.Status)
	}
	if out.ShouldFail() {
		t.Error("expected ShouldFail = false")
	}
}

func TestRunner_Run_InvalidResponse(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:                "mock",
		EvaluateResponse: "not valid json at all",
	}
	resolver := assets.NewResolver("")
	builder := prompt.NewBuilder(resolver, "")
	runner := skill.NewRunner(mock, builder)

	def := &skill.Definition{
		Name:         "test-skill",
		Body:         "You are a test skill.",
		OutputSchema: `{"type":"object"}`,
		InputSchema:  `{"type":"object"}`,
	}

	_, err := runner.Run(t.Context(), def, skill.RunOpts{
		RepoTree: "file1.go\n",
	})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestRunner_Run_AgentError(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:           "mock",
		EvaluateErr: errors.New("agent failed"),
	}
	resolver := assets.NewResolver("")
	builder := prompt.NewBuilder(resolver, "")
	runner := skill.NewRunner(mock, builder)

	def := &skill.Definition{
		Name:         "test-skill",
		Body:         "You are a test skill.",
		OutputSchema: `{"type":"object"}`,
		InputSchema:  `{"type":"object"}`,
	}

	_, err := runner.Run(t.Context(), def, skill.RunOpts{
		RepoTree: "file1.go\n",
	})
	if err == nil {
		t.Error("expected error when agent returns error")
	}
}

func TestRunner_Run_WithDiffPayload(t *testing.T) {
	validJSON := `{
		"skill": "test-skill",
		"version": "v1",
		"status": "fail",
		"blocking": ["issue found"],
		"major": [],
		"warning": [],
		"info": []
	}`

	mock := &agent.MockAgent{
		NameVal:                "mock",
		EvaluateResponse: validJSON,
	}
	resolver := assets.NewResolver("")
	builder := prompt.NewBuilder(resolver, "")
	runner := skill.NewRunner(mock, builder)

	def := &skill.Definition{
		Name:         "test-skill",
		Body:         "You are a test skill.",
		OutputSchema: `{"type":"object"}`,
		InputSchema:  `{"type":"object"}`,
	}

	out, err := runner.Run(t.Context(), def, skill.RunOpts{
		RepoTree:    "file1.go\n",
		DiffPayload: "diff --git a/file1.go b/file1.go\n",
		BaseRef:     "main",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.ShouldFail() {
		t.Error("expected ShouldFail = true for blocking findings")
	}
}
