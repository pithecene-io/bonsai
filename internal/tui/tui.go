package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

// ErrInterrupted is returned by RunWithTUI when the user quits early
// via q or ctrl+c, before the orchestrator completes.
var ErrInterrupted = errors.New("interrupted by user")

// RunWithTUI starts the bubbletea program that displays progress
// while the orchestrator runs in a separate goroutine.
// It blocks until the TUI exits. The report is obtained from the
// EventComplete message.
func RunWithTUI(events <-chan orchestrator.Event, source string) (*orchestrator.Report, error) {
	return runTUI(events, source)
}

// runTUI is the internal implementation. Extra tea.ProgramOptions can be
// passed for testing (e.g. tea.WithInput).
func runTUI(events <-chan orchestrator.Event, source string, opts ...tea.ProgramOption) (*orchestrator.Report, error) {
	m := NewModel(source, events)

	// No alt-screen — output stays in scrollback
	baseOpts := []tea.ProgramOption{tea.WithoutSignalHandler()}
	baseOpts = append(baseOpts, opts...)
	p := tea.NewProgram(m, baseOpts...)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	fm := finalModel.(Model)
	if fm.interrupted {
		return fm.report, ErrInterrupted
	}
	if fm.err != nil {
		return fm.report, fm.err
	}
	return fm.report, nil
}
