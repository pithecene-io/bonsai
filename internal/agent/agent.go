// Package agent provides AI agent backend interfaces and exec-based
// implementations for the claude and codex CLIs.
package agent

import "context"

// Agent is the interface for AI agent backends.
type Agent interface {
	// Interactive starts an interactive session with the given system prompt.
	// It connects stdin/stdout/stderr to the terminal.
	// extraArgs are passed through to the agent CLI.
	Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error

	// NonInteractive runs a non-interactive query with the given system
	// prompt and user prompt. Returns the agent's text response.
	NonInteractive(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// Name returns the agent name (e.g., "claude", "codex").
	Name() string
}
