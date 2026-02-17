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
- Asset files (Markdown, YAML, JSON) under `assets/`
- Test fixtures under `testdata/`

Guidelines for agents:
- Follow standard Go conventions (gofmt, govet, golangci-lint)
- Prefer table-driven tests
- Keep packages small and focused
- Do not introduce global state
- Do not use `init()` outside `cmd/`
- Never create symlinks (copies only — symlinks break on branch switch)
- Prefer `internal/` over `pkg/` — nothing is public API

Git workflow:
- Always use git worktrees for implementation work
- Worktree naming: `${repo-name}-${suffix}` as a sibling directory
- Never commit directly to `main`

When unsure:
- Ask for clarification rather than guessing
