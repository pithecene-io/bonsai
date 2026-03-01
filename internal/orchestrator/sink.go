package orchestrator

import "fmt"

// LoggerSink returns an event channel and a done channel. The event channel
// is backed by a goroutine that formats events as human-readable strings and
// calls the provided logger function. The done channel is closed when the
// goroutine exits (after the event channel is closed).
func LoggerSink(logger func(string)) (events chan<- Event, done <-chan struct{}) {
	ch := make(chan Event, 64)
	d := make(chan struct{})
	go func() {
		defer close(d)
		for ev := range ch {
			logEvent(logger, ev)
		}
	}()
	return ch, d
}

func logEvent(logger func(string), ev Event) {
	switch ev.Kind {
	case EventSkipped:
		logger(fmt.Sprintf("  ⊘ %s [skipped: %s]", ev.SkillName, ev.Reason))
	case EventStart:
		logger(fmt.Sprintf("▶ Running: %s [%s]", ev.SkillName, ev.Cost))
	case EventDone:
		logResultLine(logger, ev.Result)
		if ev.Result != nil {
			for _, line := range ev.Result.Details("    ") {
				logger(line)
			}
		}
	case EventError:
		logger(fmt.Sprintf("  ✖ %s [error: %v]", ev.SkillName, ev.Err))
	case EventFailFast:
		logger(fmt.Sprintf("✖ Mandatory failure (--fail-fast): %s", ev.SkillName))
	case EventComplete, EventQueued:
		// No-op for logger sink; caller handles report.
	}
}

func logResultLine(logger func(string), r *Result) {
	if r == nil {
		return
	}
	summary := r.SummaryLine()
	switch {
	case r.Status == "error":
		logger(fmt.Sprintf("  ✖ %s [error]", r.Name))
	case !r.Failed():
		logger(fmt.Sprintf("  ✔ %s (%s)", r.Name, summary))
	case r.Mandatory:
		logger(fmt.Sprintf("  ✖ %s [mandatory] (%s)", r.Name, summary))
	default:
		logger(fmt.Sprintf("  ⚠ %s [non-mandatory] (%s)", r.Name, summary))
	}
}
