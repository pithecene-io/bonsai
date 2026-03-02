# CONTRACT_SKILLS — Skill System

Defines the skill system: cost tiers, domains, bundles, governance
modes, mode cascade logic, and output schema. This is a contract
document. Implementations must conform.

## Scope

Skill definitions, the skill registry (`skills.yaml`), bundle and
mode routing, cost-tier model assignment, and structured output
validation.

## Invariants

- Every skill MUST have a `SKILL.md` file with YAML frontmatter.
- Every skill MUST produce output conforming to the unified output
  schema.
- Skill names are lowercase, hyphen-separated identifiers.
- Skills are versioned by directory: `<name>/v1/`, `<name>/v2/`, etc.
- Skills MUST NOT have side effects — they read input and emit JSON.
- The registry (`skills.yaml`) is the authoritative source for skill
  metadata (cost, mode, domain, bundle membership).

## Skill Resolution

Skills resolve filesystem-first (first match wins):

1. `<repo>/ai/skills/<name>/<version>/` — repo-local override
2. `~/.config/bonsai/skills/<name>/<version>/` — user config
3. Embedded in binary — global default

A repo-local override completely replaces the embedded skill.

## Cost Tiers

| Tier | Model default | Intent |
|------|---------------|--------|
| `cheap` | `haiku` | Fast, deterministic checks |
| `moderate` | `sonnet` | Heuristic analysis |
| `heavy` | `sonnet` | Semantic, expensive checks |

Cost tiers control model selection via `ModelForSkill(cost)`. The
mapping from tier to model is configurable (see `CONTRACT_CONFIG.md`).

## Domains

Skills are categorized into domains for organizational purposes:

`structural` · `architecture` · `contract` · `discipline` · `entropy`
· `depgraph` · `hygiene`

Domains are metadata only — they do not affect execution.

## Bundles

Bundles group skills for execution by `bonsai check --bundle <name>`:

| Bundle | Purpose |
|--------|---------|
| `patch` | Small changes (≤3 files) |
| `default` | Standard governance |
| `structural-change` | Top-level directory changes |
| `api-change` | Public surface modifications |
| `heavy` | Full validation |
| `audit-full` | Complete audit |

Bundle membership is defined in `skills.yaml`. A skill MAY belong to
multiple bundles.

## Governance Modes

Modes determine which skills run based on diff characteristics:

| Mode | Trigger |
|------|---------|
| `PATCH` | ≤3 files, no new files, no renames |
| `NORMAL` | Default |
| `STRUCTURAL` | Top-level dirs changed or renames |
| `API` | Public surface paths touched |
| `HEAVY` | >500 lines OR >15 files OR structural+API |
| `AUDIT` | Explicit (`--mode AUDIT`) |

## Mode Cascade Logic

Mode determination follows this cascade (in `internal/diff/mode.go`):

1. If plan intent is set, use the plan's declared mode
2. If diff is heavy (lines > threshold OR files > threshold), use
   `HEAVY`
3. If both structural and API changes detected, use `HEAVY`
4. If API changes detected, use `API`
5. If structural changes detected, use `STRUCTURAL`
6. If diff qualifies as patch (files ≤ threshold, no new files, no
   renames), use `PATCH`
7. Default: `NORMAL`

## Output Schema

All skills MUST produce JSON conforming to the unified output schema.
The schema requires:

- `status`: `"pass"` or `"fail"`
- `blocking`: array of blocking findings
- `major`: array of major findings
- `warning`: array of warning findings
- `info`: array of info findings

Each finding is an object with at minimum a `message` string field.

Status MUST be `"fail"` if and only if the `blocking` array is
non-empty.

## SKILL.md Frontmatter

Required frontmatter fields:

```yaml
---
name: skill-name
description: Brief description
requires_diff: true|false
---
```

- `name` MUST match the directory name.
- `requires_diff` controls whether the skill is skipped when no diff
  is available.

## Mandatory Skills

Skills marked `mandatory: true` in `skills.yaml` cause `bonsai check`
to fail if they produce blocking findings, even in fail-fast mode.
Non-mandatory skills may be skipped on fail-fast.
