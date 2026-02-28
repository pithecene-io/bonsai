# Bonsai

AI governance toolkit for software repositories.

Bonsai is a Go CLI that provides structured AI-assisted workflows:
governance validation, autonomous fixing, planning, implementation with
gating loops, code review, and repository migration scaffolding. It
replaces a collection of shell scripts with a single distributable binary.

## Quick Start

```bash
# Install
go install github.com/pithecene-io/bonsai/cmd/bonsai@latest

# Set your Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-...

# Run governance checks against the current repo
bonsai check

# Autonomously fix governance findings
bonsai fix
```

## Install

**From source (requires Go 1.25+):**

```bash
go install github.com/pithecene-io/bonsai/cmd/bonsai@latest
```

**From GitHub release:**

```bash
gh release download --repo pithecene-io/bonsai --pattern '*_linux_amd64.tar.gz'
tar xzf bonsai_*_linux_amd64.tar.gz
mv bonsai ~/.local/bin/
```

## Commands

| Command | Description |
|---------|-------------|
| `bonsai version` | Print the bonsai version |
| `bonsai chat [role]` | Interactive AI chat session (default role: architect) |
| `bonsai plan` | Start a planning session |
| `bonsai implement` | Implementation with governance gating loop |
| `bonsai review` | Code review session (uses codex backend) |
| `bonsai patch "<task>"` | Three-phase patch surgery: plan → emit → validate |
| `bonsai skill <name>` | Run a single governance skill |
| `bonsai check` | Run governance skills (bundle or mode-based) |
| `bonsai fix` | Autonomously fix governance findings |
| `bonsai list` | List available skills, bundles, or roles |
| `bonsai migrate [path]` | Scaffold AI governance into a repository (6-phase) |
| `bonsai hooks install\|remove` | Manage pre-push governance hook |
| `bonsai completion {bash\|zsh\|fish}` | Generate shell completions |

### Key Flags

**`bonsai check`:**
`--bundle <name>`, `--mode <MODE>`, `--base <ref>`, `--scope <paths>`,
`--fail-fast`, `--jobs <n>`, `--no-progress`, `--model <name>`

**`bonsai fix`:**
`--bundle <name>`, `--base <ref>`, `--max-iterations <n>`, `--no-progress`

**`bonsai skill`:**
`--version <v>`, `--scope <paths>`, `--base <ref>`, `--model <name>`

**`bonsai list`:**
`--skills`, `--bundles`, `--roles`

## Workflows

### Check → Fix → Check

The primary governance loop. `check` validates, `fix` resolves findings
autonomously, then `check` confirms the fixes.

```bash
bonsai check --bundle default
bonsai fix --bundle default
bonsai check --bundle default
```

### Plan → Implement

Interactive AI sessions with governance gating. `implement` runs a
3-iteration loop: Claude session → diff profile → mode routing → skill
validation → re-inject findings if needed.

```bash
bonsai plan
bonsai implement
```

### Patch Surgery

Three-phase autonomous patching for targeted tasks:

```bash
bonsai patch "fix the off-by-one error in pagination"
```

Phase 1: architecture planning → Phase 2: code emission → Phase 3: validation.

### Single Skill

Run one governance skill directly:

```bash
bonsai skill repo-convention-enforcer
bonsai skill arch-index-alignment --model sonnet
```

### Repository Onboarding

Scaffold governance into a new repository:

```bash
bonsai migrate /path/to/repo
```

Creates CLAUDE.md, docs/ARCH_INDEX.md, ai/skills/ scaffold, and runs
initial validation.

## Governance Modes

Modes determine which skills run based on diff characteristics.

| Mode | Trigger | Purpose |
|------|---------|---------|
| PATCH | ≤3 files, no new files, no renames | Lightweight checks |
| NORMAL | Default | Standard governance |
| STRUCTURAL | Top-level dirs changed or renames | Structural integrity |
| API | Public surface paths touched | API compatibility |
| HEAVY | >500 diff lines OR >15 files OR structural+API | Full validation |
| AUDIT | Explicit (`--mode AUDIT`) | Complete skill suite |

Mode routing is automatic when using `bonsai implement` (derived from
diff profile). Use `--mode` to override in `bonsai check`.

## Skills System

Bonsai ships with **43 governance skills** organized by:

- **Cost tier:** cheap (fast, deterministic), moderate (heuristic),
  heavy (semantic, expensive)
- **Domain:** structural, architecture, contract, discipline, entropy,
  depgraph, hygiene

**6 built-in bundles:** `patch` (7), `default` (15),
`structural-change` (17), `api-change` (13), `heavy` (43),
`audit-full` (43)

```bash
bonsai list --skills    # all 43 skills with cost/domain
bonsai list --bundles   # bundle names and skill counts
```

### Repo-local Skill Overrides

Skills resolve filesystem-first:

1. `<repo>/ai/skills/<name>/<version>/` — repo-local override
2. `~/.config/bonsai/skills/<name>/<version>/` — user config
3. Embedded in binary — default

Repo-local overrides let you distill your CLAUDE.md rules into focused,
efficient skill prompts tailored to your repo's specific boundaries.

## Configuration

Layered merge chain (highest precedence last):

1. **Embedded defaults** — compiled into binary
2. **User config** — `~/.config/bonsai/config.yaml`
3. **Repo config** — `<repo-root>/.bonsai.yaml`
4. **Environment variables** — `BONSAI_*` prefix
5. **CLI flags** — per-command

### Example `.bonsai.yaml`

```yaml
providers:
  anthropic:
    api_key: sk-ant-...      # or use ANTHROPIC_API_KEY env

agents:
  claude:
    bin: claude              # path to claude CLI
  codex:
    bin: codex               # path to codex CLI

models:
  skills:
    cheap: haiku             # fast governance checks
    moderate: sonnet         # medium-complexity checks
    heavy: sonnet            # expensive checks
  roles:
    implement: opus          # feature work
    plan: opus               # planning sessions
    review: codex            # code review
    patch: sonnet            # patch surgery
    chat: sonnet             # interactive chat

diff:
  heavy_diff_lines: 500      # threshold for HEAVY mode
  heavy_files_changed: 15    # threshold for HEAVY mode
  patch_max_files: 3         # max files for PATCH mode

output:
  dir: ai/out                # report output directory
```

### Environment Variables

| Variable | Config path |
|----------|-------------|
| `ANTHROPIC_API_KEY` | `providers.anthropic.api_key` |
| `BONSAI_MODEL_SKILL_CHEAP` | `models.skills.cheap` |
| `BONSAI_MODEL_SKILL_MODERATE` | `models.skills.moderate` |
| `BONSAI_MODEL_SKILL_HEAVY` | `models.skills.heavy` |
| `BONSAI_MODEL_ROLE_IMPLEMENT` | `models.roles.implement` |
| `BONSAI_CHECK_JOBS` | `check.concurrency` |
| `BONSAI_OUTPUT_DIR` | `output.dir` |

## Agent Backends

Bonsai routes to three AI backends based on model name:

| Backend | Models | Use case |
|---------|--------|----------|
| Anthropic API (Go SDK) | haiku, sonnet, opus | Skill invocations (non-interactive) |
| Claude CLI | sonnet, opus | Interactive sessions (plan, implement, chat) |
| Codex CLI | codex | Code review |

See [docs/agent_backends.md](docs/agent_backends.md) for provider
details, fallback behavior, and OAuth configuration.

## Development

Requires Go 1.25+ and [Task](https://taskfile.dev/).

```bash
task build      # go build with version injection
task test       # go test -race ./...
task lint       # golangci-lint
task check      # vet + lint + test
task snapshot   # goreleaser snapshot (local)
```

## Architecture

See [docs/ARCH_INDEX.md](docs/ARCH_INDEX.md) for the full package
dependency graph and subsystem guide.

```
cmd/bonsai/           Binary entrypoint
internal/cli/         Command definitions (all 13 subcommands)
internal/gate/        3-iteration gating state machine
internal/orchestrator/ Multi-skill parallel execution
internal/skill/       Skill loading, invocation, and output validation
internal/registry/    skills.yaml parsing and bundle/mode routing
internal/prompt/      Layered system prompt assembly
internal/diff/        Diff profiling and governance mode determination
internal/agent/       AI agent backends (Anthropic API, Claude CLI, Codex CLI)
internal/config/      Multi-source configuration resolution
internal/assets/      Embedded asset filesystem with override resolution
internal/tui/         Bubbletea-based terminal UI for check/fix progress
internal/repo/        Repository detection and metadata
internal/gitutil/     Exec-based git command helpers
internal/xio/         I/O utility helpers
```

## License

Apache-2.0 — see [LICENSE](LICENSE).
