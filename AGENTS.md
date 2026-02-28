# AGENTS.md — Bonsai Guardrails

This file defines **non-negotiable guardrails** for working on Bonsai.
It encodes *discipline and constraints*, not architecture.

---

This repository is a Go CLI application.

Characteristics:
- Single binary (`bonsai`)
- Strictly internal packages (no `pkg/`)
- Embedded assets via `go:embed`
- Shell-outs to `claude` and `codex` CLIs, plus direct Anthropic API calls via SDK, for AI agent backends
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

---

## Core Principles

- Prefer **clarity over cleverness**
- Favor **explicit behavior** over implicit magic
- Keep abstractions **shallow and inspectable**
- Optimize for **correctness and debuggability**, not elegance

---

## Core Constraint

Bonsai orchestrates **AI governance workflows**, not AI behavior.

The system defines:
- skill invocation and output validation
- governance mode routing (diff → profile → mode)
- multi-skill orchestration with parallel execution
- configuration merge chains
- prompt assembly from layered context

The system does NOT define:
- AI model behavior or training
- repository-specific governance rules (those live in each repo's CLAUDE.md)
- git workflow enforcement beyond pre-push hooks

Do not introduce features that cross this boundary.

---

## Scope Discipline

Agents must not:
- Invent new features unless explicitly requested
- Redesign core abstractions unprompted
- Introduce DSLs, frameworks, or configuration layers
- Optimize for scale or performance without evidence
- Touch unrelated code while fixing bugs

If scope feels ambiguous or expanding, **pause and ask**.

---

## Change Discipline

- API changes are expensive; internal refactors are cheap
- Behavior changes must be observable
- Avoid silent fallbacks, hidden retries, or implicit recovery
- Separate concerns into separate commits (don't refactor while fixing bugs)
- Prefer minimal diffs — if a change isn't necessary for the task, don't make it

---

## Abstraction Discipline

Agents must not expand abstractions to accommodate:
- backend-specific quirks
- performance optimizations
- convenience behaviors
- conditional logic for edge cases

Differences between backends must be handled by composition, not flags.
Extract a shared abstraction only when ≥3 call sites repeat the same pattern.

---

## Structural Rules

- Small, single-purpose modules
- No premature generalization
- No "utility" dumping grounds
- Separate concerns explicitly:
  - agent backends vs orchestration
  - skill loading vs skill execution
  - prompt assembly vs configuration
  - diff profiling vs mode routing

---

## Allowed Additions

Agents may add:
- new governance skills (under `internal/assets/data/skills/`)
- new CLI subcommands (under `internal/cli/`)
- new agent backend implementations (under `internal/agent/`)
- tests for any package

Provided they respect the dependency DAG in CLAUDE.md §4.

---

## Disallowed Additions

Agents must not add:
- background workers or daemons
- hidden global state or singletons
- packages under `pkg/` (everything is internal)
- circular dependencies between packages
- `init()` functions outside `cmd/`
- symlinks in any git-tracked content

---

## Error Handling

- Sentinel errors (type-based) for cross-package boundaries
- Use `errors.Is` / `errors.As` for matching
- Context-wrap with `%w`; use `errors.New` when no format verbs needed
- Never discard errors silently — return, log, or explicitly ignore with comment

---

## Go Rules

- `any` over `interface{}`
- `t.Context()` in tests (Go 1.24+)
- Iterators over intermediate slices where practical
- `//nolint:` directives require an explanation comment
- Prefer `errors.New` over `fmt.Errorf` when no formatting verbs are needed

---

## Control Flow

- Early returns over nesting
- No implicit mutation — make state changes visible at the call site
- Pure functions where practical
- Errors must be handled or propagated explicitly

---

## Formatting & Comments

- `gofmt` only — no custom formatting tools
- Comments explain *why*, not *what*
- No commented-out code in committed files
- Doc comments on all exported symbols

---

## Git Workflow

- Always use git worktrees for implementation work
- Worktree naming: `${repo-name}-${suffix}` as a sibling directory
- Never commit directly to `main`

---

## Priority Order

1. Correctness
2. Clarity
3. Explicitness
4. Convenience
5. Extensibility

---

## Litmus Test

Before adding code, ask:

> Does this change follow the dependency DAG? Does it respect package
> boundaries? Is the diff minimal? Does it make the system easier to
> reason about for a future reader?

If any answer is no, reconsider.

---

## Agent Implementation Procedure

When given a task:

1. Read this file (`AGENTS.md`) in full.
2. Read `CLAUDE.md` for structural invariants and dependency rules.
3. Read only the files explicitly referenced by the task.
4. Do not infer architecture beyond what is visible in code.
5. Modify only files within the stated scope.
6. Do not introduce new dependencies unless explicitly requested.
7. Preserve existing interfaces unless the task explicitly permits changes.
8. Make all behavior changes observable.
9. Follow Go Rules strictly.
10. If an instruction is ambiguous, stop and ask before writing code.
11. If a change feels like scope expansion, stop and surface the concern.
12. Do not refactor unrelated code "for cleanliness."
13. Output only the requested artifacts (code, diffs, or explanations).

---

## When Uncertain

If a change increases:
- API surface area
- configuration options
- conditional behavior

Assume it is invalid and stop.
