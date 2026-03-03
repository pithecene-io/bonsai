# CONTRACT_GATING — Gating Loop

Defines the gating loop state machine, iteration budget, failure
behavior, and output artifacts. This is a contract document.
Implementations must conform.

## Scope

The `bonsai implement` gating state machine (`internal/gate/`),
the fix loop (`internal/cli/fix.go`), and artifact persistence.

## Invariants

- The gating loop MUST NOT exceed `gate.max_iterations` iterations.
- The loop MUST NOT silently accept non-compliant output — it reports
  failure with diagnostics when the budget is exhausted.

## Gating Loop State Machine

```
preflight → [session → diff → profile → mode → gate → pass/fail/re-inject] × max_iterations
```

### Branch Safety (CLI Layer)

Before the gating loop is constructed, code-modifying CLI commands
(`implement`, `patch`, `fix`) MUST ensure the process is not on
`main` or `master`. If on a protected branch, the CLI layer
auto-creates a git worktree with a timestamped branch and changes
process CWD to the new worktree. This guarantees agent subprocesses
execute in the correct directory.

If the working tree has uncommitted changes, the CLI layer MUST warn
that those edits will not be present in the new worktree.

### Preflight

1. Warn if not in a git worktree (advisory)
2. Detect merge base from configured candidates
3. Consume `plan.json` if present (rename to `.consumed.json`)

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

## Fix Loop

The fix loop (`bonsai fix`) is a simplified version of the gating
loop:

1. Run governance check (cheap-cost skills only)
2. If blocking findings exist, launch autonomous fix sessions
   (one per failed skill)
3. Re-check after fixes
4. Repeat up to `fix.max_iterations` times
5. Exit 0 on pass, exit 1 on budget exhaustion

## Artifacts

### On Governance Pass

The gating loop saves:

- `{output_dir}/last.patch` — unified diff of all changes
- `{output_dir}/last.report.json` — full JSON report

### On Plan Consumption

During preflight:

- `{output_dir}/plan.json` → `{output_dir}/plan.consumed.json`

See `CONTRACT_OUTPUT.md` for the complete artifact inventory and
schema definitions.
