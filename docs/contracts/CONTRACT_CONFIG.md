# CONTRACT_CONFIG — Configuration Schema

Defines the configuration schema, merge chain, YAML structure, and
environment variable naming. This is a contract document.
Implementations must conform.

## Scope

All configuration surfaces: Go struct definitions, YAML serialization,
environment variable bindings, and CLI flag overrides.

## Invariants

- Configuration is read-only after `Load()` returns.
- The merge chain order is fixed and MUST NOT be reordered.
- Environment variable names MUST use the `BONSAI_` prefix.
- YAML keys MUST use `snake_case`.
- All configuration fields MUST have compiled-in defaults in
  `Default()`.

## Merge Chain

Sources are merged in precedence order (highest precedence last):

1. **Embedded defaults** — `Default()` in `internal/config/config.go`
2. **User config** — `~/.config/bonsai/config.yaml`
3. **Repo config** — `<repoRoot>/.bonsai.yaml`
4. **Environment variables** — `BONSAI_*`
5. **CLI flags** — applied by the caller after `Load()` returns

A later source overwrites an earlier source for scalar values. Slice
values use full replacement (not append).

## Top-Level Schema

```yaml
diff:
  heavy_diff_lines: 500
  heavy_files_changed: 15
  patch_max_files: 3
routing:
  public_surface_globs: [...]
  structural_patterns: [...]
  merge_base_candidates: [...]
gate:
  max_iterations: 3
check:
  concurrency: 0
fix:
  max_iterations: 3
providers:
  anthropic:
    api_key: ""
agents:
  claude:
    bin: "claude"
  codex:
    bin: "codex"
models:
  skills:
    cheap: haiku
    moderate: sonnet
    heavy: sonnet
  roles:
    implementer: opus
    planner: opus
    reviewer: codex
    patcher: sonnet
    chat: sonnet
output:
  dir: "ai/out"
skills:
  extra_dirs: []
```

## Model Assignment Keys

Role model keys MUST use role names (actor nouns) as defined in
`CONTRACT_ROLES.md`:

| YAML key | Role |
|----------|------|
| `models.roles.implementer` | implementer |
| `models.roles.planner` | planner |
| `models.roles.reviewer` | reviewer |
| `models.roles.patcher` | patcher |
| `models.roles.chat` | chat (not a role — controls `bonsai chat` model) |

Skill cost tier keys are: `models.skills.cheap`, `models.skills.moderate`,
`models.skills.heavy`.

## Environment Variables

Primary environment variable bindings:

| Variable | Config path |
|----------|-------------|
| `BONSAI_MODEL_ROLE_IMPLEMENTER` | `models.roles.implementer` |
| `BONSAI_MODEL_ROLE_PLANNER` | `models.roles.planner` |
| `BONSAI_MODEL_ROLE_REVIEWER` | `models.roles.reviewer` |
| `BONSAI_MODEL_ROLE_PATCHER` | `models.roles.patcher` |
| `BONSAI_MODEL_ROLE_CHAT` | `models.roles.chat` |
| `BONSAI_MODEL_SKILL_CHEAP` | `models.skills.cheap` |
| `BONSAI_MODEL_SKILL_MODERATE` | `models.skills.moderate` |
| `BONSAI_MODEL_SKILL_HEAVY` | `models.skills.heavy` |
| `BONSAI_PROVIDER_ANTHROPIC_API_KEY` | `providers.anthropic.api_key` |
| `BONSAI_CLAUDE_BIN` | `agents.claude.bin` |
| `BONSAI_CODEX_BIN` | `agents.codex.bin` |
| `BONSAI_CHECK_JOBS` | `check.concurrency` |
| `BONSAI_OUTPUT_DIR` | `output.dir` |
| `BONSAI_DIFF_HEAVY_LINES` | `diff.heavy_diff_lines` |
| `BONSAI_DIFF_HEAVY_FILES` | `diff.heavy_files_changed` |
| `BONSAI_DIFF_PATCH_MAX_FILES` | `diff.patch_max_files` |
| `BONSAI_GATE_MAX_ITERATIONS` | `gate.max_iterations` |
| `BONSAI_FIX_MAX_ITERATIONS` | `fix.max_iterations` |
| `BONSAI_SKILLS_EXTRA_DIRS` | `skills.extra_dirs` (colon-separated) |

## Legacy Environment Variables

The following legacy names are accepted as fallbacks when the primary
name is not set. They MAY be removed in a future version:

| Legacy variable | Primary variable |
|----------------|------------------|
| `BONSAI_ANTHROPIC_API_KEY` | `BONSAI_PROVIDER_ANTHROPIC_API_KEY` |
| `BONSAI_MODEL_CHECK_CHEAP` | `BONSAI_MODEL_SKILL_CHEAP` |
| `BONSAI_MODEL_CHECK_MODERATE` | `BONSAI_MODEL_SKILL_MODERATE` |
| `BONSAI_MODEL_CHECK_HEAVY` | `BONSAI_MODEL_SKILL_HEAVY` |
| `BONSAI_MODEL_IMPLEMENT` | `BONSAI_MODEL_ROLE_IMPLEMENTER` |
| `BONSAI_MODEL_PLAN` | `BONSAI_MODEL_ROLE_PLANNER` |
| `BONSAI_MODEL_REVIEW` | `BONSAI_MODEL_ROLE_REVIEWER` |
| `BONSAI_MODEL_PATCH` | `BONSAI_MODEL_ROLE_PATCHER` |
| `BONSAI_MODEL_CHAT` | `BONSAI_MODEL_ROLE_CHAT` |
| `BONSAI_MODEL_DEFAULT` | *(blanket fallback for all slots)* |

## Default Values

All defaults are defined in `Default()` and compiled into the binary.
No external file or service is required at startup. See the top-level
schema section above for default values.

## ModelForRole Resolution

`ModelForRole(role string) string` resolves a role name to a model
string. It MUST accept exactly the role names defined in
`CONTRACT_ROLES.md` plus `"chat"`. Unknown role names return empty
string (agent picks its own default).

## ModelForSkill Resolution

`ModelForSkill(cost string) string` resolves a cost tier to a model
string. It MUST accept exactly `"cheap"`, `"moderate"`, `"heavy"`.
Unknown cost tiers return empty string.
