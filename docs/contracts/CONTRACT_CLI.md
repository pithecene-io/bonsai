# CONTRACT_CLI — Command Surface

Defines the CLI command surface, flag contracts, exit codes, and
command-to-role mapping. This is a contract document. Implementations
must conform.

## Scope

All user-facing CLI commands, flags, arguments, and exit behavior
defined in `internal/cli/`.

## Invariants

- The binary name is `bonsai`.
- All commands are top-level subcommands (no nested command groups
  except `completion` and `hooks`).
- Command names are imperative verbs or nouns (`check`, `fix`, `plan`,
  `implement`, `review`, `patch`, `chat`, `skill`, `list`, `migrate`,
  `hooks`, `completion`, `version`).
- Exit code 0 means success; exit code 1 means governance failure or
  error.

## Command Surface

| Command | Arguments | Description |
|---------|-----------|-------------|
| `bonsai check` | *(none)* | Run governance skills against diff |
| `bonsai fix` | *(none)* | Autonomously fix governance findings |
| `bonsai plan` | `[-- extra-args...]` | Interactive planning session |
| `bonsai implement` | `[-- extra-args...]` | Implementation with governance gating (Execute with plan, Session without) |
| `bonsai review` | *(none)* | Autonomous code review |
| `bonsai patch` | `<task-description>` | Three-phase patch surgery |
| `bonsai chat` | `[role] [-- extra-args...]` | Interactive AI chat |
| `bonsai skill` | `<name>` | Run a single governance skill |
| `bonsai list` | *(none)* | List skills, bundles, or roles |
| `bonsai migrate` | `[path]` | Scaffold governance into a repo |
| `bonsai hooks` | `install\|remove` | Manage pre-push hook |
| `bonsai completion` | `bash\|zsh\|fish` | Generate shell completions |
| `bonsai version` | *(none)* | Print version |

## Command-to-Role Mapping

Each interactive command uses a specific prompt role (see
`CONTRACT_ROLES.md`):

| Command | Role | Config key for model |
|---------|------|---------------------|
| `bonsai plan` | `planner` | `models.roles.planner` |
| `bonsai implement` | `implementer` | `models.roles.implementer` |
| `bonsai review` | `reviewer` | `models.roles.reviewer` |
| `bonsai patch` (phase 1) | `architect` | `models.roles.patcher` |
| `bonsai patch` (phase 2) | `patcher` | `models.roles.patcher` |
| `bonsai chat [role]` | specified role | `models.roles.chat` |

## Key Flags

### `bonsai check`

| Flag | Type | Description |
|------|------|-------------|
| `--bundle` | string | Bundle name |
| `--mode` | string | Governance mode override |
| `--base` | string | Git ref for diff context |
| `--scope` | string | Comma-separated path prefixes |
| `--fail-fast` | bool | Stop on first mandatory failure |
| `--jobs` | int | Concurrency limit |
| `--no-progress` | bool | Disable TUI progress |
| `--model` | string | Override model for all skills |
| `--diff-profile` | string | Pre-computed JSON diff profile |

### `bonsai fix`

| Flag | Type | Description |
|------|------|-------------|
| `--bundle` | string | Bundle name |
| `--base` | string | Git ref for diff context |
| `--max-iterations` | int | Max fix iterations |
| `--no-progress` | bool | Disable TUI progress |

### `bonsai skill`

| Flag | Type | Description |
|------|------|-------------|
| `--version` | string | Skill version |
| `--scope` | string | Comma-separated path prefixes |
| `--base` | string | Git ref for diff context |
| `--model` | string | Override model |

### `bonsai list`

| Flag | Type | Description |
|------|------|-------------|
| `--skills` | bool | List skills |
| `--bundles` | bool | List bundles |
| `--roles` | bool | List roles |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (governance passed, command completed) |
| 1 | Failure (governance failed, error occurred) |

## Shell Completions

Shell completion scripts MUST list exactly the roles defined in
`CONTRACT_ROLES.md` for the `chat` subcommand. No other roles may
appear in completions.
