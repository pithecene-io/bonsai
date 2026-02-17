# Repository Constitution — Bonsai

## 1. Constitutional Order of Authority

1. This file (repo-local constitution)
2. `AGENTS.md` (behavioral expectations — also read by Codex)

## 2. Identity

Bonsai is a Go CLI that provides an AI governance toolkit for software
repositories. It is the binary successor to the shell-based governance
framework in [dotfiles/ai/](https://github.com/justapithecus/dotfiles/tree/main/ai).

## 3. Structural Invariants

Required directories:
- `cmd/bonsai/` — single entrypoint
- `internal/` — all application packages (no `pkg/`)
- `assets/` — source files for `go:embed`

Forbidden:
- `pkg/` directory (everything is internal)
- Global state or `init()` functions outside of `cmd/`
- Circular package dependencies
- Symlinks in any git-tracked content
- Working directly in the main worktree (use `git worktree add` for all work)

## 4. Package Dependency Rules

Strictly acyclic. Dependency flow:

```
cmd/bonsai → internal/cli → (all internal packages)
internal/gitutil → (nothing internal)
internal/assets  → (nothing internal)
internal/config  → internal/assets
internal/repo    → internal/gitutil
internal/prompt  → internal/assets, internal/repo
internal/agent   → (nothing internal)
internal/skill   → internal/agent, internal/prompt, internal/registry
internal/registry → internal/assets
internal/diff    → internal/gitutil, internal/repo
internal/orchestrator → internal/skill, internal/registry
internal/gate    → internal/orchestrator, internal/diff, internal/agent, internal/prompt
```

`internal/cli` is the only package that may import from all other
internal packages. No other package may import `internal/cli`.

## 5. Build & Test

```
go build ./cmd/bonsai
go test ./...
```

Version injection: `go build -ldflags "-X github.com/pithecene-io/bonsai/internal/cli.Version=vX.Y.Z" ./cmd/bonsai`

## 6. Output Requirements

Same as dotfiles `ai/CLAUDE.md` §6 — diff-only output, no complete
file rewrites, conventional commits with gitmoji.
