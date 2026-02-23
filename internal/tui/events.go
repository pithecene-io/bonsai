package tui

import (
	"time"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

// SkillEventMsg wraps an orchestrator event for the bubbletea message loop.
type SkillEventMsg struct {
	Event orchestrator.Event
}

// TickMsg is sent at regular intervals to update the spinner and elapsed time.
type TickMsg time.Time

// DoneMsg is sent when the orchestrator run has completed.
type DoneMsg struct {
	Report *orchestrator.Report
	Err    error
}
