package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/pithecene-io/bonsai/internal/orchestrator"
)

// skillState tracks the display state of a single skill.
type skillState int

const (
	statePending skillState = iota
	stateRunning
	statePassed
	stateFailed
	stateWarning
	stateSkipped
)

// skillEntry holds the display data for one skill.
type skillEntry struct {
	name      string
	cost      string
	mandatory bool
	state     skillState
	reason    string // skip/error reason
	elapsed   time.Duration
	result    *orchestrator.Result
	startTime time.Time
}

// Model is the bubbletea model for the check TUI.
type Model struct {
	source      string
	skills      []skillEntry
	total       int
	completed   int
	startTime   time.Time
	done        bool
	interrupted bool // user pressed q/ctrl+c before completion
	report      *orchestrator.Report
	err         error
	events      <-chan orchestrator.Event

	// Layout
	width int // terminal width; updated by tea.WindowSizeMsg

	// Spinner state
	spinnerFrames []string
	spinnerIdx    int
}

var defaultSpinnerFrames = []string{"◐", "◓", "◑", "◒"}

// NewModel creates a new TUI model.
func NewModel(source string, events <-chan orchestrator.Event) Model {
	return Model{
		source:        source,
		startTime:     time.Now(),
		events:        events,
		width:         80,
		spinnerFrames: defaultSpinnerFrames,
	}
}

// Init starts the event polling and tick timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.pollEvents(),
		m.tickCmd(),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.interrupted = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case TickMsg:
		if m.done {
			return m, nil
		}
		m.spinnerIdx = (m.spinnerIdx + 1) % len(m.spinnerFrames)
		return m, m.tickCmd()

	case SkillEventMsg:
		m = m.handleEvent(msg.Event)
		if m.done {
			return m, tea.Quit
		}
		return m, m.pollEvents()

	case DoneMsg:
		m.done = true
		m.report = msg.Report
		m.err = msg.Err
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleEvent(ev orchestrator.Event) Model {
	switch ev.Kind {
	case orchestrator.EventQueued:
		// Grow the skills slice if needed
		for len(m.skills) <= ev.Index {
			m.skills = append(m.skills, skillEntry{})
		}
		m.skills[ev.Index] = skillEntry{
			name:      ev.SkillName,
			cost:      ev.Cost,
			mandatory: ev.Mandatory,
			state:     statePending,
		}
		m.total = ev.Total

	case orchestrator.EventSkipped:
		for len(m.skills) <= ev.Index {
			m.skills = append(m.skills, skillEntry{})
		}
		m.skills[ev.Index] = skillEntry{
			name:      ev.SkillName,
			cost:      ev.Cost,
			mandatory: ev.Mandatory,
			state:     stateSkipped,
			reason:    ev.Reason,
		}
		m.total = ev.Total
		m.completed++

	case orchestrator.EventStart:
		if ev.Index < len(m.skills) {
			m.skills[ev.Index].state = stateRunning
			m.skills[ev.Index].startTime = time.Now()
		}

	case orchestrator.EventDone:
		if ev.Index < len(m.skills) {
			entry := &m.skills[ev.Index]
			entry.elapsed = ev.Elapsed
			entry.result = ev.Result
			switch {
			case ev.Result != nil && ev.Result.ExitCode == 0:
				entry.state = statePassed
			case ev.Result != nil && ev.Result.Mandatory:
				entry.state = stateFailed
			default:
				entry.state = stateWarning
			}
			m.completed++
		}

	case orchestrator.EventError:
		if ev.Index < len(m.skills) {
			m.skills[ev.Index].state = stateFailed
			m.skills[ev.Index].reason = fmt.Sprintf("error: %v", ev.Err)
			m.completed++
		}

	case orchestrator.EventFailFast:
		// Mark remaining pending skills as skipped
		for i := range m.skills {
			if m.skills[i].state == statePending {
				m.skills[i].state = stateSkipped
				m.skills[i].reason = "cancelled (fail-fast)"
				m.completed++
			}
		}

	case orchestrator.EventComplete:
		m.done = true
		m.report = ev.Report
	}

	return m
}

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	elapsed := time.Since(m.startTime)
	header := fmt.Sprintf("bonsai check — %s", m.source)
	b.WriteString(styleHeader.Render(header))
	b.WriteString(styleDim.Render(fmt.Sprintf("  %5.1fs", elapsed.Seconds())))
	b.WriteString("\n\n")

	for _, s := range m.skills {
		b.WriteString(m.renderSkill(s))
		b.WriteString("\n")

		// Show finding details for completed skills
		if s.result != nil {
			m.renderDetails(&b, "blocking", s.result.BlockingDetails)
			m.renderDetails(&b, "major", s.result.MajorDetails)
			m.renderDetails(&b, "warning", s.result.WarningDetails)
		}
	}

	if m.total > 0 {
		b.WriteString("\n")
		b.WriteString(m.renderProgress())
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderSkill(s skillEntry) string {
	var icon, name, meta, timing string

	switch s.state {
	case statePending:
		icon = stylePending.Render("○")
		name = styleDim.Render(s.name)
		meta = styleDim.Render(fmt.Sprintf("[%s]", s.cost))
		timing = styleDim.Render("—")
	case stateRunning:
		frame := m.spinnerFrames[m.spinnerIdx]
		icon = styleRunning.Render(frame)
		name = s.name
		meta = styleDim.Render(fmt.Sprintf("[%s]", s.cost))
		elapsed := time.Since(s.startTime)
		timing = styleDim.Render(fmt.Sprintf("%.1fs…", elapsed.Seconds()))
	case statePassed:
		icon = stylePassed.Render("✔")
		name = s.name
		meta = styleDim.Render(fmt.Sprintf("[%s]", s.cost))
		timing = styleDim.Render(fmt.Sprintf("%.1fs", s.elapsed.Seconds()))
	case stateFailed:
		icon = styleFailed.Render("✖")
		name = styleBold.Render(s.name)
		if s.mandatory {
			meta = styleFailed.Render("[mandatory]")
		} else {
			meta = styleFailed.Render("[error]")
		}
		timing = styleDim.Render(fmt.Sprintf("%.1fs", s.elapsed.Seconds()))
	case stateWarning:
		icon = styleWarning.Render("⚠")
		name = s.name
		meta = styleWarning.Render("[non-mandatory]")
		timing = styleDim.Render(fmt.Sprintf("%.1fs", s.elapsed.Seconds()))
	case stateSkipped:
		icon = styleSkipped.Render("⊘")
		name = styleDim.Render(s.name)
		meta = styleDim.Render("[skipped]")
		timing = styleDim.Render("—")
	}

	// Layout: "  {icon} {name…} {meta} {timing}"
	// Fixed columns: indent(2) + icon(1) + space(1) + space(1) + meta + space(1) + timing
	metaW := lipgloss.Width(meta)
	timingW := lipgloss.Width(timing)
	nameCol := m.width - 2 - 1 - 1 - 1 - metaW - 1 - timingW
	const minNameCol = 20
	if nameCol < minNameCol {
		nameCol = minNameCol
	}

	// Pad or truncate the name to exactly nameCol visual width.
	nameW := lipgloss.Width(name)
	switch {
	case nameW > nameCol:
		name = ansi.Truncate(name, nameCol-1, "…")
		nameW = lipgloss.Width(name)
		// Pad any remaining space (truncation may undershoot by one)
		if pad := nameCol - nameW; pad > 0 {
			name += strings.Repeat(" ", pad)
		}
	case nameW < nameCol:
		name += strings.Repeat(" ", nameCol-nameW)
	}

	return fmt.Sprintf("  %s %s %s %s", icon, name, meta, timing)
}

func (m Model) renderDetails(b *strings.Builder, severity string, details []string) {
	for _, d := range details {
		line := fmt.Sprintf("      %s: %s", severity, d)
		b.WriteString(styleDetail.Render(line))
		b.WriteString("\n")
	}
}

func (m Model) renderProgress() string {
	if m.total == 0 {
		return ""
	}

	width := 30
	filled := width * m.completed / m.total
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("  %s  %d/%d completed",
		styleProgress.Render(bar), m.completed, m.total)
}

// pollEvents returns a tea.Cmd that reads the next event from the channel.
func (m Model) pollEvents() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.events
		if !ok {
			return DoneMsg{}
		}
		return SkillEventMsg{Event: ev}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
