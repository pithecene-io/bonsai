package tui

import (
	"strings"
	"testing"
	"time"

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
