# Architecture Index — Bonsai

Navigation-only lookup table for agents. Source of truth for package
relationships: `CLAUDE.md` §4 (dependency DAG).

## Root files

| File | Purpose |
|------|---------|
| `CLAUDE.md` | Repo-local constitution; structural invariants and dependency rules |
| `AGENTS.md` | Behavioral expectations for AI agents and Codex |
| `go.mod` | Module `github.com/pithecene-io/bonsai`, Go 1.25.6 |
| `Taskfile.yaml` | Developer task runner (build, test, lint) |
| `.bonsai.yaml` | Repo-local bonsai config overrides |

## `docs/`

Explanatory documentation and navigation indexes.

- **Key files:** `ARCH_INDEX.md` (this file)
- **Depends on:** *(nothing)*

## `cmd/bonsai/`

Binary entrypoint. Calls `cli.NewApp()` and runs it. No logic beyond
arg dispatch.

- **Key file:** `main.go`
- **Depends on:** `internal/cli`

## `internal/cli`

Command definitions for every subcommand (`chat`, `plan`, `implement`,
`review`, `patch`, `skill`, `check`, `list`, `migrate`, `hooks`,
`completion`). The only package allowed to import all other internal
packages.

- **Key files:** `app.go` (app + version), one file per subcommand
- **Depends on:** all other `internal/*` packages

## `internal/gitutil`

Exec-based git command helpers. No internal dependencies. Shells out to
`git` directly.

- **Key file:** `git.go`
- **Depends on:** *(nothing internal)*

## `internal/assets`

Embedded asset filesystem (`go:embed all:data`) with filesystem-first
override resolution. Provides `Resolver` for skills, roles, and config.

- **Key files:** `embed.go` (embed directive), `resolve.go` (Resolver)
- **Depends on:** *(nothing internal)*

### `internal/assets/data/`

Embedded governance assets consumed at runtime:

- `claude.md` — sovereign global CLAUDE.md (interactive modes)
- `claude_validator.md` — trimmed governance preamble for validator mode
- `governance.md` — governance framework reference
- `review_architecture.md` — review-mode architecture doc
- `roles/` — role definitions (architect, implementer, planner, reviewer, patcher)
- `skills/` — 44 governance skill definitions (SKILL.md + schemas)
- `skills.yaml` — skill registry (bundles, modes, costs)
- `templates/` — migration templates

## `internal/config`

Configuration with multi-source merge chain: embedded defaults → user
config → repo config (`.bonsai.yaml`) → env vars (`BONSAI_*`) → flags.

- **Key files:** `config.go` (types + defaults), `load.go` (merge chain)
- **Depends on:** `internal/assets`

## `internal/repo`

Repository detection, metadata, merge-base resolution, and tree listing.

- **Key files:** `detect.go` (repo info + merge base), `tree.go` (file listing)
- **Depends on:** `internal/gitutil`

## `internal/prompt`

System prompt assembly for all modes. Builds layered prompts from:
preamble → mode → CLAUDE.md → context layers → role → AGENTS.md →
ARCH_INDEX.md.

- **Key file:** `builder.go`
- **Depends on:** `internal/assets`, `internal/repo`

## `internal/agent`

AI agent backend interface with three implementations: direct Anthropic
API (Go SDK), Claude CLI (subprocess), and Codex CLI (subprocess).
Supports interactive and non-interactive invocation.

- **Key files:** `agent.go` (interface + Model + ToolPolicy types), `anthropic.go` (direct API), `claude.go`, `codex.go`, `router.go` (model-based dispatch + fallback), `mock.go`
- **Depends on:** *(nothing internal)*
- **See also:** [`docs/agent_backends.md`](agent_backends.md) for provider-specific behavior and quirks

## `internal/registry`

Skills registry parser (`skills.yaml`), bundle-based and mode-based
skill selection with cost/mode sorting.

- **Key files:** `registry.go` (load + lookup), `mode.go` (mode routing), `bundle.go` (bundle routing)
- **Depends on:** `internal/assets`

## `internal/skill`

Skill loading (SKILL.md + frontmatter parsing), invocation via agent
backend, diff payload construction, and output validation against the
unified JSON schema.

- **Key files:** `loader.go` (load + parse), `runner.go` (invoke), `diff.go` (diff payload), `output.go` (validate)
- **Depends on:** `internal/agent`, `internal/prompt`, `internal/registry`

## `internal/diff`

Diff profiling and governance mode determination. Ports of
`compute_diff_profile()` and `determine_mode()` from the shell scripts.

- **Key files:** `profile.go` (diff profile), `mode.go` (mode cascade)
- **Depends on:** `internal/gitutil`, `internal/repo`

## `internal/orchestrator`

Multi-skill execution engine with parallel worker pool, fail-fast logic,
skip detection, structured event emission, and aggregate JSON report
generation.

- **Key files:** `orchestrator.go` (run + worker pool), `event.go` (event types), `sink.go` (LoggerSink adapter)
- **Depends on:** `internal/skill`, `internal/registry`

## `internal/tui`

Bubbletea-based terminal UI for `bonsai check`. Renders per-skill
progress with spinner, timing, finding details, and a progress bar.
Falls back to plain text (LoggerSink) when stdout is not a TTY or
`--no-progress` is set.

- **Key files:** `model.go` (bubbletea Model), `events.go` (message types), `styles.go` (lipgloss styles), `tui.go` (entry point)
- **Depends on:** `internal/orchestrator`

## `internal/xio`

Small I/O helper functions (e.g. `DiscardClose` for lint-clean defers).

- **Key file:** `close.go`
- **Depends on:** *(nothing internal)*

## `internal/gate`

The 3-iteration gating state machine:
`preflight → [session → diff → profile → mode → gate → pass/fail/re-inject] × max_iterations`

Highest-level internal package; drives the full implement loop.

- **Key files:** `loop.go`
- **Depends on:** `internal/orchestrator`, `internal/diff`, `internal/agent`, `internal/prompt`

## `.github/workflows/`

| File | Purpose |
|------|---------|
| `ci.yml` | CI pipeline: test, vet, lint on push/PR |
| `release.yml` | GoReleaser-based release on tag push |
