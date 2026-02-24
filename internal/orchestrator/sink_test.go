package orchestrator_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

func TestLoggerSink_FormatsEvents(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	logger := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, msg)
	}

	ch, done := orchestrator.LoggerSink(logger)

	ch <- orchestrator.Event{
		Kind:      orchestrator.EventSkipped,
		SkillName: "test-skill",
		Reason:    "requires --base for diff context",
	}
	ch <- orchestrator.Event{
		Kind:      orchestrator.EventStart,
		SkillName: "another-skill",
		Cost:      "cheap",
	}
	ch <- orchestrator.Event{
		Kind:      orchestrator.EventDone,
		SkillName: "another-skill",
		Result: &orchestrator.Result{
			Name:           "another-skill",
			Status:         "pass",
			ExitCode:       0,
			Blocking:       0,
			Major:          0,
			Warning:        1,
			WarningDetails: []string{"some warning"},
		},
	}
	ch <- orchestrator.Event{
		Kind:      orchestrator.EventError,
		SkillName: "broken-skill",
		Err:       fmt.Errorf("load failed"),
	}
	ch <- orchestrator.Event{
		Kind:      orchestrator.EventFailFast,
		SkillName: "broken-skill",
	}
	ch <- orchestrator.Event{Kind: orchestrator.EventComplete}
	close(ch)

	// Wait for goroutine to drain all events
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(lines) < 5 {
		t.Fatalf("expected at least 5 log lines, got %d: %v", len(lines), lines)
	}

	// Check key substrings
	if !strings.Contains(lines[0], "⊘") || !strings.Contains(lines[0], "test-skill") {
		t.Errorf("line 0 = %q, want skipped icon + skill name", lines[0])
	}
	if !strings.Contains(lines[1], "▶") {
		t.Errorf("line 1 = %q, want start marker", lines[1])
	}
	if !strings.Contains(lines[2], "✔") {
		t.Errorf("line 2 = %q, want pass icon", lines[2])
	}
	// lines[3] should be the warning detail
	if !strings.Contains(lines[3], "warning: some warning") {
		t.Errorf("line 3 = %q, want finding detail", lines[3])
	}
	if !strings.Contains(lines[4], "✖") || !strings.Contains(lines[4], "load failed") {
		t.Errorf("line 4 = %q, want error line", lines[4])
	}
}
