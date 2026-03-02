package cli

import (
	"errors"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
)

func TestReviewRunner_CallsExecute(t *testing.T) {
	mock := &agent.MockAgent{NameVal: "test"}

	rr := &reviewRunner{
		agent: mock,
		model: agent.Model("sonnet"),
	}

	err := rr.run(t.Context(), "system prompt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Must dispatch via Execute (autonomous), not Session (interactive).
	if len(mock.ExecuteCalls) != 1 {
		t.Fatalf("ExecuteCalls = %d, want 1", len(mock.ExecuteCalls))
	}
	if len(mock.SessionCalls) != 0 {
		t.Errorf("SessionCalls = %d, want 0 (review must be autonomous)", len(mock.SessionCalls))
	}

	call := mock.ExecuteCalls[0]
	if call.SystemPrompt != "system prompt" {
		t.Errorf("SystemPrompt = %q, want %q", call.SystemPrompt, "system prompt")
	}
	if call.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", call.Model)
	}
	if call.UserPrompt == "" {
		t.Error("UserPrompt should not be empty")
	}
}

func TestReviewRunner_PropagatesError(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:    "test",
		ExecuteErr: errors.New("agent failed"),
	}

	rr := &reviewRunner{agent: mock, model: "sonnet"}
	err := rr.run(t.Context(), "sys")
	if err == nil {
		t.Fatal("expected error to propagate from Execute")
	}
}
