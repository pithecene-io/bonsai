# CONTRACT_AGENT_ROUTING — Agent Dispatch Rules

Defines the dispatch rules for routing AI invocations to backends,
fallback behavior, and billing distinctions. This is a contract
document. Implementations must conform.

## Scope

Agent backend selection, model-to-backend dispatch, credential
resolution, and fallback behavior in `internal/agent/`.

## Invariants

- The `Agent` interface MUST have exactly four methods:
  `Interactive(ctx, system, extraArgs)`,
  `NonInteractive(ctx, system, user, model)`,
  `Autonomous(ctx, system, user, model)`, and `Name()`.
- Backend selection is determined by the model string, not the role
  or command.
- Fallback from Anthropic direct API to Claude CLI MUST be automatic
  and silent (logged only at debug level).
- Credential resolution MUST NOT require configuration — environment
  variables and Claude CLI OAuth tokens are auto-discovered.

## Backends

| Backend | Package | Transport | Capabilities |
|---------|---------|-----------|--------------|
| Anthropic direct API | `anthropic.go` | HTTPS (Go SDK) | Non-interactive only (no interactive or autonomous) |
| Claude CLI | `claude.go` | Subprocess | Non-interactive, interactive, and autonomous |
| Codex CLI | `codex.go` | Subprocess | Non-interactive, interactive, and autonomous |

## Dispatch Precedence (NonInteractive)

```
Model.IsCodex()                       → Codex CLI
Model.IsClaude() && Anthropic != nil  → Anthropic direct API
default                               → Claude CLI (universal fallback)
```

## Dispatch Precedence (Interactive)

Interactive mode always routes to Claude CLI, regardless of model.
The Anthropic direct API does not support interactive sessions.

## Dispatch Precedence (Autonomous)

```
Model.IsCodex()  → Codex CLI
default           → Claude CLI
```

The Anthropic direct API does not support autonomous mode.

## Credential Resolution (Anthropic)

Resolution order (first match wins):

1. Explicit API key via `WithAPIKey()` option
2. Claude CLI OAuth token from `~/.claude/.credentials.json`
3. `ANTHROPIC_API_KEY` environment variable

OAuth is preferred over the env var because Max/Pro subscribers get
billing through their existing subscription.

When no credentials are found, `NewAnthropic()` returns nil and the
Router skips the direct API backend entirely.

## Billing Distinction

- **`ANTHROPIC_API_KEY`** — prepaid API credits (Anthropic console)
- **Claude CLI OAuth** — Claude Pro/Max subscription billing
- **Codex CLI** — Codex CLI's own auth and billing

These are independent billing systems. The Router does not conflate
them.

## Model Aliases

Short tier names are resolved to full model identifiers:

| Alias | Resolved identifier |
|-------|---------------------|
| `haiku` | `claude-haiku-4-5-20251001` |
| `sonnet` | `claude-sonnet-4-6` |
| `opus` | `claude-opus-4-6` |

Unknown model names are passed through unchanged.

## Fallback Behavior

When the Anthropic direct API fails (any error), the Router retries
the same request via Claude CLI. The original error is logged to
stderr when `BONSAI_DEBUG=1` is set. Only the Claude CLI result is
returned to the caller.

The fallback excludes context cancellation and deadline exceeded
errors — when the caller is done, falling back would just add noise
and latency. All other errors (auth failures, outages, network
issues) trigger automatic fallback.
