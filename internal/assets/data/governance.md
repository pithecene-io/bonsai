# GOVERNANCE.md — Agentic Governance Model

Normative specification for the AI governance system. Defines modes,
routing, gating, and skill execution mechanics.

---

## 1. Overview

The governance system validates repository changes through composable
skill checks before blessing a diff. The primary flow:

1. `ai-plan` — optional planning session; may produce `ai/out/plan.json`
2. `ai-implement` — interactive implementation session with auto-gating
3. On session exit: diff capture → diff profiling → mode determination →
   skill execution → bless or retry

A change is **blessed** when governance passes: all mandatory skills
complete with no blocking findings. Blessed changes produce `last.patch`
and `last.report.json` in `ai/out/`.

---

## 2. Governance Modes

Six modes control which skills execute during a governance check.

| Mode | Skills | Use |
|------|--------|-----|
| PATCH | 7 | Surgical edits (≤3 files, no new files) |
| NORMAL | 16 | Standard feature work |
| STRUCTURAL | 17 | Directory / module boundary changes |
| API | 13 | Public surface / contract changes |
| HEAVY | 36 | Large features / refactors |
| AUDIT | 44 | Full-spectrum audit |

Modes correspond to bundles in `skills.yaml`:

| Mode | Bundle |
|------|--------|
| PATCH | `patch` |
| NORMAL | `default` |
| STRUCTURAL | `structural-change` |
| API | `api-change` |
| HEAVY | `heavy` |
| AUDIT | `audit-full` |

---

## 3. Mode Routing

When `ai-implement` exits a session, it determines the governance mode
automatically from the diff profile. Precedence (first match wins):

1. **HEAVY** — `diff_lines > 500` OR `files_changed > 15` OR
   (`has_structural` AND `has_public_surface`)
2. **STRUCTURAL** — `top_level_dirs` changed (>1) OR renames/moves
   detected
3. **API** — public surface paths touched (`api/`, `sdk/`, `public/`,
   `cmd/`, `cli/`)
4. **PATCH** — plan intent is `patch` OR (≤3 files, no new files,
   no renames)
5. **NORMAL** — default fallback

An explicit `--mode` flag on `ai-check` overrides auto-routing.

---

## 4. Diff Profiling

`compute_diff_profile()` extracts structured metadata from the working
tree diff against merge base.

| Field | Type | Description |
|-------|------|-------------|
| `files_changed` | int | Total files with changes |
| `new_files` | int | Files not in merge base |
| `renames` | int | Detected renames / moves |
| `lines_added` | int | Lines added |
| `lines_removed` | int | Lines removed |
| `diff_lines` | int | `lines_added + lines_removed` |
| `top_level_dirs` | string[] | Distinct top-level directories touched |
| `public_surface_paths` | string[] | Paths matching public surface globs |
| `has_structural` | bool | Control-plane path detected |

Source: `git diff $MERGE_BASE` (merge base to working tree, includes
uncommitted changes).

Untracked files: merged via
`git -C $REPO_ROOT ls-files --others --exclude-standard`.

---

## 5. Gating State Machine

`ai-implement` wraps the Claude session in a gating loop:

```
SESSION → CAPTURE_DIFF → PROFILE → MODE → GATE
  PASS → blessed (last.patch + last.report.json), exit 0
  FAIL → "Re-enter to fix? [y/N]"
    y → inject findings → SESSION (max 3 iterations)
    n → exit 1 (changes in tree, not blessed)
```

Preflight checks (before first session):
- Auto-create git worktree if on `main` or `master` (CLI layer)
- Detect and record merge base

---

## 6. Plan Handoff

`ai-plan` may write `ai/out/plan.json` at the end of a planning session.
`ai-implement` detects and consumes it on startup.

Consumption:
- The file is renamed to `plan.consumed.json` after first read
- The `intent` field is used as the lowest-priority hint in mode
  determination (step 4 of §3)
- Consumption is optional — workflows without `ai-plan` are
  completely unaffected

Schema:
```json
{
  "task": "short description",
  "intent": "patch | normal | structural | api | heavy",
  "constraints": {"max_files": 3, "no_new_deps": true},
  "notes": ["note 1", "note 2"]
}
```

---

## 7. Skill Execution

`ai-check` orchestrates skill execution for a given mode or bundle.

Selection:
- `ai-check --mode MODE` selects the corresponding bundle (§2)
- `ai-check --bundle NAME` selects skills by explicit bundle name
- `--mode` and `--bundle` are mutually exclusive
- `--bundle` is preserved for backward compatibility and migration
  contexts (where no diff context exists)

Execution order:
- `--bundle`: listing order in `skills.yaml` is authoritative. Bundle
  authors control the sequence (e.g., cheap gate skills first).
- `--mode`: skills are sorted by cost (cheap → moderate → heavy), then
  by mode (deterministic → heuristic → semantic).

Behavior:
- `--fail-fast` stops on first mandatory failure
- All-skipped detection: exit 1 if every skill was skipped (prevents
  false pass)

---

## 8. Skill Output Schema

Every skill must return this unified JSON structure:

```json
{
  "skill": "name",
  "version": "v1",
  "status": "pass|fail",
  "blocking": [],
  "major": [],
  "warning": [],
  "info": []
}
```

Severity levels:
- **BLOCKING** — hard violations that prevent merge (exit code 1)
- **MAJOR** — significant issues that should be addressed
- **WARNING** — potential concerns worth reviewing
- **INFO** — observations and context

Exit code: `ai-skill` exits 1 if `status == "fail"` AND `blocking` is
non-empty.

---

## 9. Exit Code Semantics

| Script | Exit 0 | Exit 1 |
|--------|--------|--------|
| `ai-implement` | Governance passed (blessed) | Governance failed or max iterations |
| `ai-check` | All skills passed | Blocking failures or all-skipped |
| `ai-skill` | Skill passed | Fail with blocking findings |

---

## 10. Artifacts

| Path | Description |
|------|-------------|
| `ai/out/ai-check.json` | Aggregate results from last `ai-check` run |
| `ai/out/last.patch` | Diff at time of governance pass |
| `ai/out/last.report.json` | Report at time of governance pass |
| `ai/out/plan.json` | Plan handoff (produced by `ai-plan`) |
| `ai/out/plan.consumed.json` | Plan after consumption by `ai-implement` |
| `ai/out/<timestamp>/` | Per-run skill outputs |

---

## 11. Known Limitations (v1)

- `--diff-profile` flag accepted but not yet used for predicate routing
- Plan constraints parsed but not enforced
- Mode routing is tier-based; no per-skill path predicates yet
- One Claude call per skill (no batching)
