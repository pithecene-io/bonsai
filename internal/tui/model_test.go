package tui

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pithecene-io/bonsai/internal/xio"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

func TestModel_HandleEventQueued(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	m = m.handleEvent(orchestrator.Event{
		Kind:      orchestrator.EventQueued,
		Index:     0,
		Total:     3,
		SkillName: "test-skill",
		Cost:      "cheap",
		Mandatory: true,
	})

	if len(m.skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(m.skills))
	}
	if m.skills[0].state != statePending {
		t.Errorf("state = %d, want statePending", m.skills[0].state)
	}
	if m.total != 3 {
		t.Errorf("total = %d, want 3", m.total)
	}
}

func TestModel_Update_KeyQuit_SetsInterrupted(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	// Simulate q keypress
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	um := updated.(Model)

	if !um.interrupted {
		t.Error("expected interrupted = true after q keypress")
	}
}

func TestModel_HandleEventStart(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventQueued, Index: 0, Total: 1,
		SkillName: "test-skill", Cost: "cheap",
	})
	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventStart, Index: 0,
		SkillName: "test-skill",
	})

	if m.skills[0].state != stateRunning {
		t.Errorf("state = %d, want stateRunning", m.skills[0].state)
	}
}

func TestModel_HandleEventDone_Pass(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventQueued, Index: 0, Total: 1,
		SkillName: "test-skill", Cost: "cheap",
	})
	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventDone, Index: 0,
		SkillName: "test-skill",
		Elapsed:   100 * time.Millisecond,
		Result: &orchestrator.Result{
			Name: "test-skill", Status: "pass", ExitCode: 0,
		},
	})

	if m.skills[0].state != statePassed {
		t.Errorf("state = %d, want statePassed", m.skills[0].state)
	}
	if m.completed != 1 {
		t.Errorf("completed = %d, want 1", m.completed)
	}
}

func TestModel_HandleEventDone_MandatoryFail(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventQueued, Index: 0, Total: 1,
		SkillName: "test-skill", Cost: "cheap", Mandatory: true,
	})
	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventDone, Index: 0,
		SkillName: "test-skill",
		Elapsed:   200 * time.Millisecond,
		Result: &orchestrator.Result{
			Name: "test-skill", Status: "fail", ExitCode: 1, Mandatory: true,
			BlockingDetails: []string{"missing required file"},
		},
	})

	if m.skills[0].state != stateFailed {
		t.Errorf("state = %d, want stateFailed", m.skills[0].state)
	}
}

func TestModel_HandleEventSkipped(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventSkipped, Index: 0, Total: 1,
		SkillName: "test-skill", Reason: "requires --base",
	})

	if m.skills[0].state != stateSkipped {
		t.Errorf("state = %d, want stateSkipped", m.skills[0].state)
	}
	if m.completed != 1 {
		t.Errorf("completed = %d, want 1", m.completed)
	}
}

func TestModel_HandleEventFailFast_IncrementsCompleted(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	// Queue 3 skills, complete 1, leave 2 pending
	for i := range 3 {
		m = m.handleEvent(orchestrator.Event{
			Kind: orchestrator.EventQueued, Index: i, Total: 3,
			SkillName: "skill-" + string(rune('a'+i)), Cost: "cheap", Mandatory: true,
		})
	}
	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventDone, Index: 0,
		SkillName: "skill-a", Elapsed: 100 * time.Millisecond,
		Result: &orchestrator.Result{
			Name: "skill-a", Status: "fail", ExitCode: 1, Mandatory: true,
		},
	})

	if m.completed != 1 {
		t.Fatalf("completed before fail-fast = %d, want 1", m.completed)
	}

	// Fire fail-fast — remaining 2 pending skills should become skipped
	m = m.handleEvent(orchestrator.Event{Kind: orchestrator.EventFailFast})

	if m.completed != 3 {
		t.Errorf("completed after fail-fast = %d, want 3 (1 done + 2 skipped)", m.completed)
	}
	for i := 1; i <= 2; i++ {
		if m.skills[i].state != stateSkipped {
			t.Errorf("skills[%d].state = %d, want stateSkipped", i, m.skills[i].state)
		}
	}
}

func TestModel_HandleEventComplete(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	report := &orchestrator.Report{Total: 1, Passed: 1}
	m = m.handleEvent(orchestrator.Event{
		Kind:   orchestrator.EventComplete,
		Report: report,
	})

	if !m.done {
		t.Error("expected done = true")
	}
	if m.report != report {
		t.Error("report not set")
	}
}

func TestModel_View_ContainsHeader(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	view := m.View()
	if !strings.Contains(view, "bonsai check") {
		t.Errorf("view missing header, got: %s", view)
	}
	if !strings.Contains(view, "bundle:default") {
		t.Errorf("view missing source, got: %s", view)
	}
}

func TestModel_View_RendersFindingDetails(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)

	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventQueued, Index: 0, Total: 1,
		SkillName: "test-skill", Cost: "cheap",
	})
	m = m.handleEvent(orchestrator.Event{
		Kind: orchestrator.EventDone, Index: 0,
		SkillName: "test-skill",
		Result: &orchestrator.Result{
			Name: "test-skill", Status: "fail", ExitCode: 1,
			BlockingDetails: []string{"missing file X"},
			WarningDetails:  []string{"something iffy"},
		},
	})

	view := m.View()
	if !strings.Contains(view, "blocking: missing file X") {
		t.Errorf("view missing blocking detail, got: %s", view)
	}
	if !strings.Contains(view, "warning: something iffy") {
		t.Errorf("view missing warning detail, got: %s", view)
	}
}

func TestModel_RenderProgress(t *testing.T) {
	events := make(chan orchestrator.Event, 10)
	m := NewModel("bundle:default", events)
	m.total = 10
	m.completed = 5

	progress := m.renderProgress()
	if !strings.Contains(progress, "5/10") {
		t.Errorf("progress = %q, want to contain 5/10", progress)
	}
	if !strings.Contains(progress, "█") {
		t.Errorf("progress = %q, want filled blocks", progress)
	}
}

// --- integration tests for RunWithTUI interrupt contract ---

// TestRunTUI_UserQuit_ReturnsErrInterrupted exercises the full TUI
// interrupt path end-to-end: event channel with a queued skill, user
// sends "q" via input pipe, RunWithTUI returns ErrInterrupted.
// This is the exact contract that internal/cli/check.go relies on.
func TestRunTUI_UserQuit_ReturnsErrInterrupted(t *testing.T) {
	events := make(chan orchestrator.Event, 10)

	// Queue one skill so there's something in-flight
	events <- orchestrator.Event{
		Kind: orchestrator.EventQueued, Index: 0, Total: 1,
		SkillName: "slow-skill", Cost: "expensive", Mandatory: true,
	}

	// Pipe "q" as user input — bubbletea reads it and triggers KeyMsg
	pr, pw := io.Pipe()
	go func() {
		// Small delay so the TUI processes the queued event first
		time.Sleep(50 * time.Millisecond)
		_, _ = pw.Write([]byte("q"))
		_ = pw.Close()
	}()

	_, err := runTUI(events, "bundle:default",
		tea.WithInput(pr),
		tea.WithOutput(io.Discard),
	)

	// Verify the sentinel error
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("err = %v, want ErrInterrupted", err)
	}
}

// TestRunTUI_NormalCompletion_NoError verifies that a normal run
// (EventComplete received) returns no error.
func TestRunTUI_NormalCompletion_NoError(t *testing.T) {
	events := make(chan orchestrator.Event, 10)

	report := &orchestrator.Report{Total: 1, Passed: 1}
	events <- orchestrator.Event{
		Kind: orchestrator.EventQueued, Index: 0, Total: 1,
		SkillName: "fast-skill", Cost: "cheap",
	}
	events <- orchestrator.Event{
		Kind: orchestrator.EventDone, Index: 0,
		SkillName: "fast-skill", Elapsed: 50 * time.Millisecond,
		Result: &orchestrator.Result{
			Name: "fast-skill", Status: "pass", ExitCode: 0,
		},
	}
	events <- orchestrator.Event{
		Kind:   orchestrator.EventComplete,
		Total:  1,
		Report: report,
	}
	close(events)

	// Pipe that never sends anything — TUI exits on EventComplete, not
	// on user input.
	pr, pw := io.Pipe()
	defer xio.DiscardClose(pw)

	got, err := runTUI(events, "bundle:default",
		tea.WithInput(pr),
		tea.WithOutput(io.Discard),
	)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != report {
		t.Errorf("report = %v, want %v", got, report)
	}
}
