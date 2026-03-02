# CONTRACT_ROLES — Role Taxonomy

Defines the authoritative set of prompt roles, their naming convention,
and their relationship to CLI commands. This is a contract document.
Implementations must conform.

## Scope

All prompt roles used in interactive sessions, prompt assembly, shell
completions, and user-facing output.

## Invariants

- Roles are **actor nouns** (never command verbs).
- The role set is closed — no role may be added without updating this
  contract.
- Each role has exactly one role definition file at
  `internal/assets/data/roles/<role>.md`.
- Role names are lowercase, hyphen-free, single-word identifiers.

## Authoritative Role List

| Role | Purpose | CLI command |
|------|---------|-------------|
| `architect` | Architecture-focused reasoning; no code output | `bonsai chat architect` |
| `implementer` | Precision code implementation with governance | `bonsai implement` |
| `planner` | Planning and execution sequencing | `bonsai plan` |
| `reviewer` | Code review with structured observations | `bonsai review` |
| `patcher` | Minimal patch emission against an architect plan | `bonsai patch` (phase 2) |

Five roles. No others exist.

## Prompt Modes

Each role maps to exactly one prompt mode constant:

| Role | Mode constant | Mode string |
|------|---------------|-------------|
| `architect` | `ModeArchitect` | `ARCHITECT` |
| `implementer` | `ModeImplementer` | `IMPLEMENTER` |
| `planner` | `ModePlanner` | `PLANNER` |
| `reviewer` | `ModeReviewer` | `REVIEWER` |
| `patcher` | `ModePatcher` | `PATCHER` |

The `ModeValidator` constant exists for skill evaluation but is not a
user-facing role. No other mode constants may exist.

## Command-to-Role Mapping

| Command | Role used | Notes |
|---------|-----------|-------|
| `bonsai chat [role]` | Specified role (default: `architect`) | Any role is valid |
| `bonsai plan` | `planner` | |
| `bonsai implement` | `implementer` | |
| `bonsai review` | `reviewer` | |
| `bonsai patch` | `architect` (phase 1), `patcher` (phase 2) | Two-phase |

## Config Model Assignment

Config keys for role-based model assignment MUST use the role name
(actor noun), not the command name (verb):

```yaml
models:
  roles:
    implementer: opus
    planner: opus
    reviewer: codex
    patcher: sonnet
    chat: sonnet
```

`chat` is a config key but not a prompt role — it controls the model
for `bonsai chat` sessions regardless of which role is selected.

`patcher` is a single config key controlling the model for both
`architect` (phase 1) and `patcher` (phase 2) of `bonsai patch`.

## Role File Format

Each role file is a Markdown document with:

- Opening sentence declaring the persona
- `## Purpose` or behavioral guidance section
- `## Hard Constraints` (if applicable)
- `## Output Style` (if applicable)

Role files MUST NOT contain configuration, model selection, or
command dispatch logic.
