// Package tui provides a Bubbletea-based terminal UI for bonsai check.
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Status icon styles
	stylePending  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleRunning  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	stylePassed   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleFailed   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleWarning  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleSkipped  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleDetail   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleHeader   = lipgloss.NewStyle().Bold(true)
	styleProgress = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
)
