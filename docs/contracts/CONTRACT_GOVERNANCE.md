# CONTRACT_GOVERNANCE — Gating Loop and Prompt Assembly

Defines the gating loop, iteration budget, failure behavior, and
prompt assembly layers. This is a contract document. Implementations
must conform.

## Scope

The `bonsai implement` gating state machine (`internal/gate/`), prompt
assembly order (`internal/prompt/`), and the fix loop
(`internal/cli/fix.go`).

## Invariants

- The gating loop MUST NOT exceed `gate.max_iterations` iterations.
- The loop MUST NOT silently accept non-compliant output — it reports
  failure with diagnostics when the budget is exhausted.
- Prompt assembly order is fixed and MUST NOT be reordered.
- The validator preamble (`claude_validator.md`) is a strict subset of
  the interactive preamble (`claude.md`).

## Gating Loop State Machine

```
preflight → [session → diff → profile → mode → gate → pass/fail/re-inject] × max_iterations
```

### Preflight

1. Verify not on `main` or `master` branch (hard fail)
2. Warn if not in a git worktree (advisory)
3. Detect merge base from configured candidates
4. Consume `plan.json` if present (rename to `.consumed.json`)

### Iteration

1. **Session** — build system prompt, invoke agent interactively
2. **Diff** — check for changes relative to merge base
3. **Profile** — compute diff profile (lines, files, new files,
   renames, scopes)
4. **Mode** — determine governance mode from profile
5. **Gate** — run orchestrator with mode-based skill selection
6. **Decision**:
   - Pass → save artifacts, exit success
   - Fail (not last iteration) → prompt user, re-inject findings
   - Fail (last iteration) → report failure, exit error

### Termination

The loop terminates when:

- Governance gate passes (exit 0)
- Max iterations exceeded (exit 1)
- User declines re-entry (exit 1)
- No merge base or no changes (skip gating, exit 0)

## Prompt Assembly — Interactive

Layer order for interactive sessions (`BuildInteractive`):

1. **Preamble** — static persona declaration
2. **Repository root** — absolute path
3. **Mode declaration** — e.g., "You are operating in IMPLEMENTER mode."
4. **Global CLAUDE.md** — sovereign governance (always from embedded)
5. **Context layers** — `ai/context/*.md` (sorted)
6. **Role definition** — from `internal/assets/data/roles/<role>.md`
7. **AGENTS.md** — repo-local behavioral constraints
8. **ARCH_INDEX.md** — repo-local architecture navigation
9. **Extra context** — findings from previous governance gate iteration
   (only on re-entry)

## Prompt Assembly — Validator

Layer order for skill evaluation (`BuildValidator`):

### Standard mode

1. **Mode declaration** — "You are operating in VALIDATOR mode."
2. **Validator preamble** — `claude_validator.md` (trimmed governance)
3. **Repo CLAUDE.md** — repo-local constitution
4. **AGENTS.md** — repo-local constraints
5. **ARCH_INDEX.md** — architecture index
6. **SKILL.md body** — skill prompt (frontmatter stripped)
7. **Output schema** — JSON schema
8. **JSON-only suffix** — "No markdown. No prose. JSON only."

### Lite mode

1. **Mode declaration** — "You are operating in VALIDATOR mode."
2. **Minimal preamble** — "You are a code-quality validator."
3. **SKILL.md body**
4. **Output schema**
5. **JSON-only suffix**

Lite mode skips all governance layers for fast evaluation on cheap
cost tiers.

## Prompt Assembly — Review

Layer order for review sessions (`BuildReview`):

1. **Preamble** + **Mode declaration** (REVIEWER)
2. **Global CLAUDE.md**
3. **Context layers**
4. **Role definition** (reviewer)
5. **Review architecture** — `review_architecture.md`
6. **AGENTS.md**
7. **ARCH_INDEX.md**

## Fix Loop

The fix loop (`bonsai fix`) is a simplified version of the gating
loop:

1. Run governance check
2. If blocking findings exist, launch AI session to fix
3. Repeat up to `fix.max_iterations` times
4. Exit 0 on pass, exit 1 on budget exhaustion

## Artifacts

On governance pass, the gating loop saves:

- `<output.dir>/last.patch` — unified diff of all changes
- `<output.dir>/last.report.json` — full JSON report

On plan consumption:

- `<output.dir>/plan.json` → `<output.dir>/plan.consumed.json`
