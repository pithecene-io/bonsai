# CONTRACT_AGENT_ROUTING — Agent Dispatch Rules

Defines the agent interface, invocation modes, dispatch rules,
fallback behavior, and billing distinctions. This is a contract
document. Implementations must conform.

## Scope

Agent backend abstraction, invocation mode semantics, model-to-backend
dispatch, credential resolution, and fallback behavior in
`internal/agent/`.

## Invariants

- The `Agent` interface is provider-agnostic with a capability model.
- Backend selection is determined by model classification and backend
  capability, not by role or command.
- Fallback from Anthropic direct API to Claude CLI MUST be automatic
  and silent (logged only at debug level).
- Fallback MUST exclude context cancellation and deadline exceeded —
  these are caller-initiated and retrying would add noise.
- Credential resolution MUST NOT require configuration — environment
  variables and CLI OAuth tokens are auto-discovered.

## Agent Interface

The `Agent` interface defines four methods — three invocation modes
and one identity method:

```go
type Agent interface {
    Evaluate(ctx, systemPrompt, userPrompt, model) (string, error)
    Execute(ctx, systemPrompt, userPrompt, model) error
    Session(ctx, systemPrompt, extraArgs) error
    Name() string
}
```

## Invocation Modes

### Evaluate

Read-only, tools disabled, output captured and returned as a string.

- **Use case**: Skill evaluation, planning queries, any invocation
  where the caller needs the model's text response.
- **I/O**: stdin not attached; stdout captured; stderr captured or
  discarded.
- **Tools**: Disabled (claude: `--tools ""`).
- **Side effects**: None.

### Execute

Read-write, tools enabled, output streamed to terminal.

- **Use case**: Autonomous code modification (`bonsai fix`,
  `bonsai review`, `bonsai implement` with plan).
- **I/O**: stdin is a prompt string; stdout/stderr stream to terminal.
- **Tools**: Enabled — model can edit files, run commands.
- **Side effects**: Yes — the model modifies the working tree.

### Session

Interactive terminal session, stdin/stdout/stderr attached.

- **Use case**: `bonsai chat`, `bonsai plan`, `bonsai implement`
  (without plan — interactive implementation loop).
- **I/O**: All three streams attached to the terminal.
- **Tools**: Controlled by the backend CLI (typically enabled).
- **Side effects**: User-driven.

### Name

Returns the backend identity string (e.g., `"claude"`, `"codex"`,
`"anthropic"`, `"router"`).

## Capability Model

Shipped backends and their supported invocation modes:

| Backend | Evaluate | Execute | Session |
|---------|----------|---------|---------|
| Anthropic API | ✓ | ✗ | ✗ |
| Claude CLI | ✓ | ✓ | ✓ |
| Codex CLI | ✓ | ✓ | ✓ |

Backends that do not support a mode MUST return an error when that
mode is called.

## Model Classification

Model strings determine backend routing via classification methods
on the `Model` type:

- `IsCodex()` — true when the model string starts with `"codex"`
  (case-insensitive).
- `IsClaude()` — true when the model string contains `"haiku"`,
  `"sonnet"`, `"opus"`, or starts with `"claude"`.
- `IsHaiku()` — true when the model string contains `"haiku"`.
- `IsLite()` — true for haiku or codex models (uses lite validator
  prompt).

Unknown models fall through to the default backend (Claude CLI).

## Dispatch Precedence

### Evaluate

```
Model.IsCodex()                       → Codex CLI
Model.IsClaude() && Anthropic != nil  → Anthropic direct API
default                               → Claude CLI (universal fallback)
```

### Execute

```
Model.IsCodex()  → Codex CLI
default           → Claude CLI
```

The Anthropic direct API does not support Execute.

### Session

```
Model extracted from extraArgs (--model / -m)
Model.IsCodex()  → Codex CLI
default           → Claude CLI
```

The model is passed via `extraArgs` (e.g., `--model codex`) and
forwarded to the backend CLI. The Router extracts the model flag
from extraArgs to determine dispatch, matching Execute behavior.

## Fallback Behavior

When the Anthropic direct API fails during Evaluate, the Router
retries the same request via Claude CLI. The original error is logged
to stderr when `BONSAI_DEBUG=1` is set.

The fallback **excludes**:
- `context.Canceled` — the caller cancelled the operation.
- `context.DeadlineExceeded` — the caller's timeout expired.

Both are checked via `ctx.Err()` and `errors.Is()` on the error
chain. When either is detected, the error is returned immediately
without fallback.

All other errors (auth failures, network issues, rate limits) trigger
the fallback.

## Model Aliases

Short tier names are resolved to full Anthropic model identifiers:

| Alias | Resolved identifier |
|-------|---------------------|
| `haiku` | `claude-haiku-4-5-20251001` |
| `sonnet` | `claude-sonnet-4-6` |
| `opus` | `claude-opus-4-6` |

Unknown model names are passed through unchanged. Alias resolution
is Anthropic-backend-specific — other backends receive the raw model
string.

## Credential Resolution (Anthropic)

Resolution order (first match wins):

1. Explicit API key via `WithAPIKey()` option
2. Claude CLI OAuth token from `~/.claude/.credentials.json`
3. `ANTHROPIC_API_KEY` environment variable

OAuth is preferred over the env var because Max/Pro subscribers get
billing through their existing subscription.

When no credentials are found, `NewAnthropic()` returns nil and the
Router skips the direct API backend entirely.

This section is Anthropic-backend-specific. Other backends handle
their own credential resolution.

## Billing Distinction

- **`ANTHROPIC_API_KEY`** — prepaid API credits (Anthropic console)
- **Claude CLI OAuth** — Claude Pro/Max subscription billing
- **Codex CLI** — Codex CLI's own auth and billing

These are independent billing systems. The Router does not conflate
them. This is a per-backend concern.
