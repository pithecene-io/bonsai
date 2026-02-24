package orchestrator

import "time"

// EventKind describes the type of orchestrator event.
type EventKind int

const (
	// EventQueued indicates a skill has been accepted for execution.
	EventQueued EventKind = iota
	// EventStart indicates a skill has begun executing.
	EventStart
	// EventDone indicates a skill has finished executing.
	EventDone
	// EventSkipped indicates a skill was skipped (e.g. requires_diff).
	EventSkipped
	// EventError indicates a skill encountered an error.
	EventError
	// EventFailFast indicates execution was halted due to a mandatory failure.
	EventFailFast
	// EventComplete is the final event, carrying the aggregate report.
	EventComplete
)

// Event represents a lifecycle event during orchestrator execution.
type Event struct {
	Kind      EventKind
	Index     int           // position in skill list (0-based)
	Total     int           // total number of skills
	SkillName string        // skill name
	Cost      string        // skill cost tier
	Mandatory bool          // whether the skill is mandatory
	Result    *Result       // for EventDone/EventError
	Report    *Report       // for EventComplete only
	Reason    string        // skip/error/fail-fast reason
	Elapsed   time.Duration // elapsed time (EventDone/EventError)
	Err       error         // underlying error (EventError)
}
