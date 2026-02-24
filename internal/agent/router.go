package agent

import "context"

// Router implements Agent by dispatching to either the Claude or Codex
// backend based on the model string. When model is "codex", the Codex
// agent handles the request; otherwise Claude handles it with the given
// model alias.
type Router struct {
	Claude *Claude
	Codex  *Codex
}

// NewRouter creates an agent router with both backends configured.
func NewRouter(claudeBin, codexBin string) *Router {
	return &Router{
		Claude: NewClaude(claudeBin),
		Codex:  NewCodex(codexBin),
	}
}

// Name returns "router".
func (r *Router) Name() string { return "router" }

// Interactive starts an interactive session. Routes to codex if the
// system prompt hints at review mode, otherwise to claude.
// In practice, callers should use the specific agent directly for
// interactive sessions.
func (r *Router) Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error {
	return r.Claude.Interactive(ctx, systemPrompt, extraArgs)
}

// NonInteractive dispatches based on the model string.
// "codex" → Codex agent; anything else → Claude agent with that model.
func (r *Router) NonInteractive(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	if model == "codex" {
		return r.Codex.NonInteractive(ctx, systemPrompt, userPrompt, model)
	}
	return r.Claude.NonInteractive(ctx, systemPrompt, userPrompt, model)
}
