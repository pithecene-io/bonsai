package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

// RunWithTUI starts the bubbletea program that displays progress
// while the orchestrator runs in a separate goroutine.
// It blocks until the TUI exits. The report is obtained from the
// EventComplete message.
func RunWithTUI(events <-chan orchestrator.Event, source string) (*orchestrator.Report, error) {
	m := NewModel(source, events)

	// No alt-screen — output stays in scrollback
	p := tea.NewProgram(m, tea.WithoutSignalHandler())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	fm := finalModel.(Model)
	if fm.err != nil {
		return fm.report, fm.err
	}
	return fm.report, nil
}
