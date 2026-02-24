# AGENTS.md — Bonsai

This repository is a Go CLI application.

Characteristics:
- Single binary (`bonsai`)
- Strictly internal packages (no `pkg/`)
- Embedded assets via `go:embed`
- Shell-outs to `claude` and `codex` CLIs for AI agent backends
- No CGo, no libgit2 — exec-based git wrappers only

Expect:
- Go source code under `cmd/` and `internal/`
- Embedded assets (Markdown, YAML, JSON) under `internal/assets/data/`
- Test fixtures under `testdata/`

Guidelines for agents:
- Follow standard Go conventions (gofmt, govet, golangci-lint)
- Prefer table-driven tests
- Keep packages small and focused
- Do not introduce global state
- Do not use `init()` outside `cmd/`
- Never create symlinks (copies only — symlinks break on branch switch)
- Prefer `internal/` over `pkg/` — nothing is public API

Core principles:
- Clarity over cleverness
- Explicit over implicit
- Shallow, inspectable abstractions
- Optimize for correctness and debuggability first

Abstraction discipline:
- Composition over flags
- Do not expand abstractions for backend quirks or performance optimizations
- Extract a shared abstraction only when ≥3 call sites repeat the same pattern

Code style and composition:
- Data-first shapes: define the data, then the behavior
- Table-driven logic for mappings and dispatch
- Named helpers for multi-step transforms
- One level of nesting per function; extract deeper logic into helpers
- File-local helpers unless shared by another package

Error handling:
- Sentinel errors (type-based) for cross-package boundaries
- Use `errors.Is` / `errors.As` for matching
- Context-wrap with `%w`; use `errors.New` when no format verbs needed
- Never discard errors silently — return, log, or explicitly ignore with comment

Go rules:
- `any` over `interface{}`
- `t.Context()` in tests (Go 1.24+)
- Iterators over intermediate slices where practical
- `//nolint:` directives require an explanation comment

Control flow:
- Early returns over nesting
- No implicit mutation — make state changes visible at the call site
- Pure functions where practical

Formatting and comments:
- `gofmt` only — no custom formatting tools
- Comments explain *why*, not *what*
- No commented-out code in committed files
- Doc comments on all exported symbols

Git workflow:
- Always use git worktrees for implementation work
- Worktree naming: `${repo-name}-${suffix}` as a sibling directory
- Never commit directly to `main`

When unsure:
- Ask for clarification rather than guessing
