# Agent Backends

Bonsai dispatches AI work to three backends. Each has different
performance characteristics, billing models, and limitations. The
Router (`internal/agent/router.go`) selects a backend automatically
based on the model string, with automatic fallback when the preferred
backend fails.

## Overview

| Backend | Package | Transport | Session | Billing |
|---------|---------|-----------|---------|---------|
| Anthropic direct API | `internal/agent/anthropic.go` | HTTPS (Go SDK) | No | API credits or Max/Pro subscription |
| Claude CLI | `internal/agent/claude.go` | Subprocess (Node.js) | Yes | Via Claude CLI auth |
| Codex CLI | `internal/agent/codex.go` | Subprocess | Yes | Via Codex CLI auth |

## Anthropic Direct API

Source: `internal/agent/anthropic.go`

The fastest backend for non-interactive work. Calls the Anthropic
Messages API directly via the Go SDK, avoiding subprocess overhead.

### Credential resolution

Resolution order (first match wins):

1. **Explicit API key** — `WithAPIKey()` option (from config
   `providers.anthropic.api_key`)
2. **Claude CLI OAuth token** — read from
   `~/.claude/.credentials.json` → `claudeAiOauth.accessToken`
3. **`ANTHROPIC_API_KEY` environment variable**

OAuth is preferred over the env var because Max/Pro subscribers get
zero-overhead billing through their existing subscription, while
`ANTHROPIC_API_KEY` requires separate prepaid API credits.

When no credentials are found, `NewAnthropic()` returns nil and the
Router falls back to Claude CLI.

### OAuth billing (Max/Pro subscription)

When using an OAuth token, the request must match the Claude Code
request shape for the API to route billing correctly. This means:

- **System prompt prefix**: the first system block must be the literal
  string `"You are Claude Code, Anthropic's official CLI for Claude."`
- **Headers**:
  - `anthropic-beta: oauth-2025-04-20,interleaved-thinking-2025-05-14`
  - `User-Agent: claude-cli/2.1.52 (external, cli)`
  - `x-app: cli`
  - `anthropic-dangerous-direct-browser-access: true`
- **Metadata**: `user_id: "bonsai"`
- **Query param**: `?beta=true`

### X-Api-Key suppression workaround

The Go SDK reads `ANTHROPIC_API_KEY` from the environment and sends it
as an `X-Api-Key` header alongside the Bearer token. The API sees the
`X-Api-Key` first, checks that account's credit balance, and rejects
with "credit balance is too low" — even when the Bearer token is valid
for Max/Pro billing. The workaround is `option.WithAPIKey("")` to
suppress the header.

See: https://github.com/anthropics/claude-code/issues/18340

### Model aliases and token limits

Short tier names are resolved to full model identifiers:

| Alias | Resolved identifier | Max tokens |
|-------|---------------------|------------|
| `haiku` | `claude-haiku-4-5-20251001` | 4096 |
| `sonnet` | `claude-sonnet-4-6` | 8192 |
| `opus` | `claude-opus-4-6` | 8192 |

Unknown model names are passed through unchanged. Unknown tiers
fall back to the sonnet token profile (8192).

### Limitations

- **No Session or Execute mode** — `Session()` and `Execute()` return
  errors. The direct API is request/response only (Evaluate); terminal
  attachment and tool use require a CLI subprocess.

## Claude CLI

Source: `internal/agent/claude.go`

Subprocess-based backend that shells out to the `claude` Node.js CLI.
The only backend that supports interactive terminal sessions.

### Startup overhead

Every invocation spawns a Node.js process. Expect ~40-60s latency for
any model, dominated by CLI startup rather than inference.

### Flag ordering

The `--model` flag **must precede** other flags to ensure the CLI
parses it before processing the system prompt. This is a CLI quirk,
not a general convention.

### Evaluate flags

Full flag set for Evaluate (`-p`) mode:

```
claude --model <model> \
  -p \
  --system-prompt <system> \
  --tools "" \
  --disable-slash-commands \
  --no-session-persistence \
  --output-format text
```

The user prompt is passed via stdin.

### Effort tuning

When the model is haiku, `--effort low` is appended to reduce latency
on cheap evaluation passes.

### Environment filtering

`CLAUDECODE` is stripped from the subprocess environment to prevent
nested Claude Code invocations from interfering.

### Session mode

Session mode connects stdin/stdout/stderr directly to the subprocess.
Only `--system-prompt` and caller-provided extra args are passed.

### Execute mode

Execute mode uses `-p` with tools enabled (no `--tools ""` flag).
Output streams to stdout/stderr. The model can autonomously edit files
and run commands.

## Codex CLI

Source: `internal/agent/codex.go`

Subprocess-based backend that shells out to the `codex` CLI.

### No separate system prompt

Codex has no `--system-prompt` flag. System and user prompts are
concatenated into a single combined prompt, separated by two newlines.

### Evaluate invocation

```
codex exec --ephemeral --sandbox read-only [-m <model>] -
```

- The `-m` flag is only added when the model is not the default
  `"codex"`.
- The combined prompt is passed via stdin (the `-` argument).
- `--sandbox read-only` ensures evaluation is side-effect-free.

### Execute invocation

```
codex exec --ephemeral [-m <model>] -
```

No `--sandbox` flag — codex exec defaults to writable, allowing the
model to modify files.

### Session mode

Session mode passes the system prompt as a positional argument
(`codex "$PROMPT"`). This is not equivalent to Claude CLI's interactive
session with separate system/user prompt handling.

## Router Dispatch

Source: `internal/agent/router.go`

The Router implements the `Agent` interface and dispatches to backends
based on the model string.

### Dispatch precedence (Evaluate)

```
Model.IsCodex()                       → Codex CLI
Model.IsClaude() && Anthropic != nil  → Anthropic direct API
default                               → Claude CLI (universal fallback)
```

### Execute routing

```
Model.IsCodex()  → Codex CLI
default           → Claude CLI
```

The Anthropic direct API does not support Execute.

### Session routing

Session always routes to Claude CLI, regardless of model.

### Automatic fallback

When the Anthropic direct API fails during Evaluate (auth error,
outage, network issue), the Router silently retries the same request
via Claude CLI. The original error is logged to stderr when
`BONSAI_DEBUG=1` is set, but only the Claude CLI result (or error) is
returned to the caller.

The fallback **excludes** `context.Canceled` and
`context.DeadlineExceeded` — these are caller-initiated and retrying
would add noise. Both are checked via `ctx.Err()` and `errors.Is()`
on the error chain.

### Mock injection

`Router.Anthropic` is typed as `Agent` (interface), not `*Anthropic`.
This allows tests to inject a mock without constructing a real
Anthropic client.

## Model Routing

Source: `internal/config/config.go`

Model selection is a top-level config concern (`models:`), separate
from both providers and agents. The `ModelsConfig` struct maps skill
cost tiers and interactive roles to model names.

### Default routing table

| Context | Model |
|---------|-------|
| **Skill — cheap** | `haiku` |
| **Skill — moderate** | `sonnet` |
| **Skill — heavy** | `sonnet` |
| **Role — implementer** | `opus` |
| **Role — planner** | `opus` |
| **Role — reviewer** | `codex` |
| **Role — patcher** | `sonnet` |
| **Role — chat** | `sonnet` |

There is no fallback default. Every slot has a compiled-in default in
`Default()`. Unknown cost/role returns empty string (agent picks its
own default).

These defaults can be overridden at any layer of the config merge
chain: user config, repo config (`.bonsai.yaml`), or env vars.

### Resolution methods

- `ModelForSkill(cost)` — resolves by cost tier (`cheap`, `moderate`,
  `heavy`); unknown cost returns empty
- `ModelForRole(role)` — resolves by role name (`implementer`, `planner`,
  `reviewer`, `patcher`, `chat`); unknown role returns empty

## Debugging

Set `BONSAI_DEBUG=1` to enable stderr logging. The agent layer emits:

- **Anthropic**: model resolution, resolved identifier, max tokens,
  OAuth mode
- **Claude CLI**: full argument list (system prompt truncated to char
  count)
- **Codex CLI**: full argument list
- **Router fallback**: original Anthropic error when falling back to
  Claude CLI
