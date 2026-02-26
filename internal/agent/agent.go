// Package agent provides AI agent backend interfaces and exec-based
// implementations for the claude and codex CLIs.
package agent

import (
	"context"
	"strings"
)

// Model is a model name alias (e.g. "haiku", "sonnet", "codex").
type Model string

// IsHaiku returns true if the model name identifies a haiku variant.
func (m Model) IsHaiku() bool {
	return strings.Contains(strings.ToLower(string(m)), "haiku")
}

// IsCodex returns true if the model name identifies a codex variant.
func (m Model) IsCodex() bool {
	return strings.HasPrefix(strings.ToLower(string(m)), "codex")
}

// IsClaude returns true for models served by the Anthropic API
// (haiku, sonnet, opus, or full claude-* model names).
func (m Model) IsClaude() bool {
	low := strings.ToLower(string(m))
	switch {
	case strings.Contains(low, "haiku"),
		strings.Contains(low, "sonnet"),
		strings.Contains(low, "opus"),
		strings.HasPrefix(low, "claude"):
		return true
	}
	return false
}

// Tier returns the model family (haiku, sonnet, opus) extracted from
// the model name. Falls back to the full lowercase name.
func (m Model) Tier() string {
	low := strings.ToLower(string(m))
	for _, family := range []string{"haiku", "sonnet", "opus"} {
		if strings.Contains(low, family) {
			return family
		}
	}
	return low
}

// IsLite returns true for models that should use the lite (governance-free)
// validator prompt. Covers cheap-tier models where latency and token budgets
// are tight.
func (m Model) IsLite() bool {
	return m.IsHaiku() || m.IsCodex()
}

// Agent is the interface for AI agent backends.
type Agent interface {
	// Interactive starts an interactive session with the given system prompt.
	// It connects stdin/stdout/stderr to the terminal.
	// extraArgs are passed through to the agent CLI.
	Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error

	// NonInteractive runs a non-interactive query with the given system
	// prompt and user prompt. Returns the agent's text response.
	// model is optional; when non-empty it overrides the agent's default model.
	NonInteractive(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)

	// Autonomous runs a non-interactive session with tools enabled.
	// The model receives the prompt, autonomously makes changes (file
	// edits, commands), and exits. Output streams to stdout/stderr.
	Autonomous(ctx context.Context, systemPrompt, userPrompt, model string) error

	// Name returns the agent name (e.g., "claude", "codex").
	Name() string
}
