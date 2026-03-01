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
			switch ev.Kind {
			case EventSkipped:
				logger(fmt.Sprintf("  ⊘ %s [skipped: %s]", ev.SkillName, ev.Reason))
			case EventStart:
				logger(fmt.Sprintf("▶ Running: %s [%s]", ev.SkillName, ev.Cost))
			case EventDone:
				logResultLine(logger, ev.Result)
				if ev.Result != nil {
					logFindingDetails(logger, "blocking", ev.Result.BlockingDetails)
					logFindingDetails(logger, "major", ev.Result.MajorDetails)
					logFindingDetails(logger, "warning", ev.Result.WarningDetails)
				}
			case EventError:
				logger(fmt.Sprintf("  ✖ %s [error: %v]", ev.SkillName, ev.Err))
			case EventFailFast:
				logger(fmt.Sprintf("✖ Mandatory failure (--fail-fast): %s", ev.SkillName))
			case EventComplete:
				// No-op for logger sink; caller handles report.
			case EventQueued:
				// No-op for logger sink.
			}
		}
	}()
	return ch, d
}

func logResultLine(logger func(string), r *Result) {
	if r == nil {
		return
	}
	if r.Status == "error" {
		logger(fmt.Sprintf("  ✖ %s [error]", r.Name))
		return
	}
	if r.ExitCode == 0 {
		logger(fmt.Sprintf("  ✔ %s (blocking:%d major:%d warning:%d)",
			r.Name, r.Blocking, r.Major, r.Warning))
		return
	}
	if r.Mandatory {
		logger(fmt.Sprintf("  ✖ %s [mandatory] (blocking:%d major:%d warning:%d)",
			r.Name, r.Blocking, r.Major, r.Warning))
	} else {
		logger(fmt.Sprintf("  ⚠ %s [non-mandatory] (blocking:%d major:%d warning:%d)",
			r.Name, r.Blocking, r.Major, r.Warning))
	}
}
