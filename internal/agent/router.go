package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// Router implements Agent by dispatching to Claude CLI, Codex CLI, or
// the direct Anthropic API backend based on the model string.
//
// Dispatch order for NonInteractive:
//
//	IsCodex()                      → Codex CLI
//	IsClaude() && Anthropic != nil → Anthropic direct API
//	default                        → Claude CLI (fallback)
//
// Interactive always routes to Claude CLI.
type Router struct {
	Claude    *Claude
	Codex     *Codex
	Anthropic Agent // nil when no API key is available
}

// NewRouter creates an agent router with all backends configured.
// AnthropicOption values configure the direct API backend; when no API
// key is available the Anthropic field is nil and Claude CLI is used
// as the fallback for claude-family models.
func NewRouter(claudeBin, codexBin string, opts ...AnthropicOption) *Router {
	r := &Router{
		Claude: NewClaude(claudeBin),
		Codex:  NewCodex(codexBin),
	}
	// Assign only when non-nil to avoid the nil-concrete-in-interface
	// trap: (*Anthropic)(nil) stored in an Agent interface is non-nil.
	if a := NewAnthropic(opts...); a != nil {
		r.Anthropic = a
	}
	return r
}

// Name returns "router".
func (r *Router) Name() string { return "router" }

// Interactive starts an interactive session. Always routes to Claude CLI.
func (r *Router) Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error {
	return r.Claude.Interactive(ctx, systemPrompt, extraArgs)
}

// NonInteractive dispatches based on the model string.
// When the Anthropic direct API is selected but fails (auth error,
// outage, network), it falls back to Claude CLI automatically.
func (r *Router) NonInteractive(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	m := Model(model)

	switch {
	case m.IsCodex():
		return r.Codex.NonInteractive(ctx, systemPrompt, userPrompt, model)
	case m.IsClaude() && r.Anthropic != nil:
		out, err := r.Anthropic.NonInteractive(ctx, systemPrompt, userPrompt, model)
		if err == nil {
			return out, nil
		}
		// Context cancellation or deadline exceeded means the caller is
		// done — falling back would just add noise and latency.  Check
		// both the context and the error chain: the context reflects the
		// caller's intent, while the error chain catches transport-level
		// timeouts where ctx.Err() may still be nil.
		if ctx.Err() != nil ||
			errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		// Anthropic failed — fall back to Claude CLI so a bad key or
		// transient outage doesn't hard-fail the entire check run.
		if os.Getenv("BONSAI_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[bonsai:debug] anthropic failed, falling back to claude CLI: %v\n", err)
		}
		out, fallbackErr := r.Claude.NonInteractive(ctx, systemPrompt, userPrompt, model)
		if fallbackErr != nil {
			return "", errors.Join(err, fallbackErr)
		}
		return out, nil
	default:
		return r.Claude.NonInteractive(ctx, systemPrompt, userPrompt, model)
	}
}

// Autonomous always routes to Claude CLI. Tool-enabled autonomous mode
// requires the Claude CLI — neither the Anthropic API nor Codex
// supports the required tool-use + streaming pattern.
func (r *Router) Autonomous(ctx context.Context, systemPrompt, userPrompt, model string) error {
	return r.Claude.Autonomous(ctx, systemPrompt, userPrompt, model)
}
