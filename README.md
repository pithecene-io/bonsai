# Bonsai

**AI governance toolkit for software repositories.**

Bonsai is a CLI that runs AI-powered governance checks against your
codebase, reports structured findings, and can autonomously fix them.
Think of it as a programmable code review layer: you define rules in
governance documents checked into your repo, and bonsai enforces them
on every change — in CI, as a pre-push hook, or on demand.

## Table of Contents

- [Capabilities](#capabilities)
- [Quick Start](#quick-start)
  - [Prerequisites](#prerequisites)
  - [Install](#install)
- [Conceptual Model](#conceptual-model)
- [Commands](#commands)
- [Workflows](#workflows)
- [Skills System](#skills-system)
  - [Governance Modes](#governance-modes)
- [Agent Backends](#agent-backends)
- [Configuration](#configuration)
  - [Default Model Assignments](#default-model-assignments)
- [Gotchas](#gotchas)
- [Design](#design)
- [Development](#development)

---

## Capabilities

- **Skill-based validation** — 43 built-in governance skills run in
  parallel, each producing structured JSON findings with severity levels
- **Autonomous fixing** — iterative check-fix loops that resolve
  findings without manual intervention
- **Merge gating** — blocks merges when blocking findings are present
  (CI integration or pre-push git hook)
- **Interactive sessions** — AI-assisted planning, implementation with
  governance gating loops, code review, and patch surgery
- **Layered prompt assembly** — composes system prompts from your repo's
  governance documents, so the AI understands your project's rules
- **Language-agnostic** — works on any codebase with governance docs
- **Provider-agnostic** — routes to multiple AI backends via a unified
  agent interface; swap models per role or cost tier in config

---

## Quick Start

```bash
# Install
go install github.com/pithecene-io/bonsai/cmd/bonsai@latest

# Configure at least one AI backend:
export ANTHROPIC_API_KEY=sk-ant-...  # Anthropic API
# — or —
export OPENAI_API_KEY=sk-...         # Codex CLI
# — or —
claude login                         # Claude CLI (OAuth)

# Run governance checks
cd your-repo
bonsai check

# Autonomously fix findings
bonsai fix
```

`bonsai check` runs the default skill bundle (15 skills) against your
diff and prints a summary of findings. `bonsai fix` picks up any
blocking findings and launches AI sessions to resolve them.

### Prerequisites

- **Go 1.25+** to install from source (or download a
  [prebuilt binary](https://github.com/pithecene-io/bonsai/releases))
- **At least one AI backend** — Anthropic API key, Claude CLI, or Codex
  CLI (see [Agent Backends](#agent-backends) for details)
- **Claude CLI** required for interactive sessions (`plan`, `implement`,
  `chat`)
- **Codex CLI** required for code review (`review`) and autonomous fixes
  (`fix`)
- **Governance documents** in your repo — at minimum a `CLAUDE.md` at
  the repo root (use `bonsai migrate` to scaffold these; see
  [Repository Onboarding](#repository-onboarding) for what gets created)

### Install

**From source** (requires Go 1.25+):

```bash
go install github.com/pithecene-io/bonsai/cmd/bonsai@latest
```

**From GitHub release:**

```bash
gh release download --repo pithecene-io/bonsai --pattern '*_linux_amd64.tar.gz'
tar xzf bonsai_*_linux_amd64.tar.gz
mv bonsai ~/.local/bin/
```

---

## Conceptual Model

```
Repository Governance Docs
  (CLAUDE.md, AGENTS.md, ARCH_INDEX.md)
              ↓
    Bonsai Prompt Assembly
  (preamble → mode → docs → role)
              ↓
    Skill Invocations (parallel)
  (43 skills × cost tiers × AI backends)
              ↓
    Structured JSON Findings
  (blocking / major / warning / info)
              ↓
    Gate Decision (pass / fail / re-inject)
```

Skills are the atomic unit. Each skill is a focused prompt with an
input schema, output schema, and severity classification. Skills are
grouped into bundles and routed by governance mode.

---

## Commands

| Command | Description |
|---------|-------------|
| `bonsai check` | Run governance skills against your diff (bundle or mode-based) |
| `bonsai fix` | Autonomously fix governance findings |
| `bonsai plan` | Start an interactive planning session |
| `bonsai implement` | Interactive implementation with governance gating loop |
| `bonsai review` | Autonomous code review session |
| `bonsai patch "<task>"` | Three-phase patch surgery: plan → emit → validate |
| `bonsai chat [role]` | Interactive AI chat session with a given role (default: architect) |
| `bonsai skill <name>` | Run a single governance skill |
| `bonsai list` | List available skills, bundles, or roles |
| `bonsai migrate [path]` | Scaffold AI governance into a repository (6-phase) |
| `bonsai hooks install\|remove` | Manage pre-push governance hook |
| `bonsai completion {bash\|zsh\|fish}` | Generate shell completions |
| `bonsai version` | Print the bonsai version |

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

---

## Workflows

### Check → Fix → Check

The primary governance loop. `check` validates your diff, `fix` launches
AI sessions to resolve findings, then `check` confirms the fixes passed.

```bash
bonsai check
bonsai fix
bonsai check
```

`check` runs skills in parallel, renders TUI progress, and writes a
JSON report to `ai/out/ai-check.json`. `fix` runs a check, then
launches AI sessions to resolve findings, repeating up to 3 iterations.

### Plan → Implement

Interactive AI sessions with governance gating. `implement` runs a
3-iteration loop: AI session → diff profile → mode routing → skill
validation → re-inject findings if needed.

```bash
bonsai plan        # architect an approach interactively
bonsai implement   # code with AI + automatic governance checks
```

### Patch Surgery

Three-phase autonomous patching for targeted tasks:

```bash
bonsai patch "fix the off-by-one error in pagination"
```

Phase 1: architecture planning → Phase 2: code emission → Phase 3:
governance validation. No human interaction required.

### Single Skill

Run one governance skill directly for debugging or targeted checks:

```bash
bonsai skill repo-convention-enforcer
bonsai skill arch-index-alignment --base main
```

### Repository Onboarding

Scaffold governance into a new repository (6-phase migration):

```bash
bonsai migrate /path/to/repo
```

This creates the governance documents that bonsai reads at runtime:

- **`CLAUDE.md`** — the repo's constitution; defines structural
  invariants, dependency rules, and behavioral constraints
- **`AGENTS.md`** — behavioral expectations for AI agents working in
  the repo
- **`docs/ARCH_INDEX.md`** — architecture navigation index; maps
  packages, files, and subsystem boundaries so skills can validate
  structural alignment

It also scaffolds an `ai/skills/` directory for repo-local skill
overrides and runs initial validation.

---

## Skills System

Bonsai ships with **43 governance skills** organized by cost tier and
domain.

### Cost Tiers

| Tier | Speed | Examples |
|------|-------|---------|
| **cheap** | Fast, deterministic | `repo-convention-enforcer`, `forbidden-top-level-detector` |
| **moderate** | Heuristic analysis | `dependency-layer-violation`, `boundary-leak-detector` |
| **heavy** | Semantic, expensive | `god-module-detector`, `abstraction-leak-detector` |

### Domains

`structural` · `architecture` · `contract` · `discipline` · `entropy`
· `depgraph` · `hygiene`

### Bundles

| Bundle | Skills | Use case |
|--------|--------|----------|
| `patch` | 7 | Small changes (≤3 files) |
| `default` | 15 | Standard governance |
| `structural-change` | 17 | Top-level directory changes |
| `api-change` | 13 | Public surface modifications |
| `heavy` | 43 | Full validation |
| `audit-full` | 43 | Complete audit |

```bash
bonsai list --skills    # all skills with cost/domain
bonsai list --bundles   # bundles with skill counts
```

### Governance Modes

Modes determine which skills run based on the characteristics of your
diff. Bonsai profiles the diff automatically and selects the appropriate
mode — smaller changes run fewer skills, larger or structural changes
trigger more thorough validation.

| Mode | Trigger | Purpose |
|------|---------|---------|
| PATCH | ≤3 files, no new files, no renames | Lightweight |
| NORMAL | Default | Standard |
| STRUCTURAL | Top-level dirs changed or renames | Structural integrity |
| API | Public surface paths touched | API compatibility |
| HEAVY | >500 lines OR >15 files OR structural+API | Full validation |
| AUDIT | Explicit (`--mode AUDIT`) | Complete suite |

Mode routing is automatic in `bonsai implement` (derived from diff
profile). Use `--mode` to override in `bonsai check`.

### Repo-Local Skill Overrides

Skills resolve filesystem-first:

1. `<repo>/ai/skills/<name>/<version>/` — repo-local override
2. `~/.config/bonsai/skills/<name>/<version>/` — user config
3. Embedded in binary — global default

Repo-local overrides let you distill your governance docs into focused,
efficient prompts tailored to your repo's specific boundaries and rules.
This is the primary mechanism for making skill invocations cheaper and
more reliable.

---

## Agent Backends

Bonsai supports three AI backends. You configure which model to use for
each role and cost tier, and the router dispatches to the appropriate
backend automatically based on the model name.

| Backend | Available models | Capabilities |
|---------|------------------|--------------|
| Anthropic API (Go SDK) | haiku, sonnet, opus | Non-interactive invocations |
| Claude CLI | sonnet, opus | Non-interactive and interactive sessions |
| Codex CLI | codex | Non-interactive and interactive sessions |

The backend is determined by the model name, not the role — if you
assign `reviewer: sonnet` in your config, code review routes through the
Anthropic API (or Claude CLI) instead of Codex. See
[Default Model Assignments](#default-model-assignments) for the
out-of-the-box configuration.

### Anthropic API key vs. Claude CLI billing

The Anthropic API key and the Claude CLI use **separate billing
systems**:

- **`ANTHROPIC_API_KEY`** draws from prepaid API credits on your
  Anthropic console account
- **Claude CLI** (`claude login`) authenticates via OAuth and bills
  against your Claude Pro or Max subscription

If you have a Claude Max plan, you can skip the API key entirely —
bonsai's direct API backend will automatically discover the Claude CLI's
OAuth token from `~/.claude/.credentials.json` and bill through your
subscription.

For implementation details — credential resolution order, dispatch
precedence, fallback behavior, OAuth header shaping, and CLI quirks —
see [docs/agent_backends.md](docs/agent_backends.md).

---

## Configuration

Bonsai resolves configuration from multiple sources, merged in order
(highest precedence last):

1. **Embedded defaults** — compiled into the binary
2. **User config** — `~/.config/bonsai/config.yaml`
3. **Repo config** — `<repo>/.bonsai.yaml`
4. **Environment variables** — `BONSAI_*`
5. **CLI flags**

You only need to set values that differ from the defaults. An empty
`.bonsai.yaml` is valid.

### Default Model Assignments

Out of the box, bonsai assigns these models to each role and cost tier:

**Skill cost tiers** (used by `bonsai check` and `bonsai fix`):

| Cost tier | Default model | Typical use |
|-----------|---------------|-------------|
| cheap | haiku | Fast, deterministic governance checks |
| moderate | sonnet | Heuristic analysis |
| heavy | sonnet | Semantic, expensive checks |

**Interactive roles** (used by `bonsai plan`, `implement`, etc.):

| Role | Default model | Command |
|------|---------------|---------|
| implementer | opus | `bonsai implement` |
| planner | opus | `bonsai plan` |
| reviewer | codex | `bonsai review` |
| patcher | sonnet | `bonsai patch` |
| chat | sonnet | `bonsai chat` |

Override any of these in `.bonsai.yaml` under `models.skills.*` or
`models.roles.*`, or via environment variables.

### Example `.bonsai.yaml`

```yaml
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

diff:
  heavy_diff_lines: 500
  heavy_files_changed: 15
  patch_max_files: 3

output:
  dir: ai/out
```

### Environment Variables

| Variable | Config path |
|----------|-------------|
| `ANTHROPIC_API_KEY` | `providers.anthropic.api_key` |
| `BONSAI_MODEL_SKILL_CHEAP` | `models.skills.cheap` |
| `BONSAI_MODEL_SKILL_MODERATE` | `models.skills.moderate` |
| `BONSAI_MODEL_SKILL_HEAVY` | `models.skills.heavy` |
| `BONSAI_MODEL_ROLE_IMPLEMENTER` | `models.roles.implementer` |
| `BONSAI_CHECK_JOBS` | `check.concurrency` |
| `BONSAI_OUTPUT_DIR` | `output.dir` |

---

## Gotchas

- **Governance docs drive findings** — bonsai validates against what's
  in `CLAUDE.md` and `AGENTS.md`. If those docs are incomplete, findings
  will be incomplete.
- **AI backend required** — skills invoke AI models. Configure at least
  one backend: `ANTHROPIC_API_KEY` for the direct API, or install the
  Claude CLI or Codex CLI.
- **Repo-local skills override globals** — a repo-local
  `ai/skills/<name>/v1/` completely replaces the embedded skill, including
  the output schema. Keep schemas in sync with the unified format.
- **Diff context requires `--base`** — many skills need diff context.
  Without `--base`, they skip. Use `--base main` for branch-based checks.
- **`fix` only runs cheap skills** — `bonsai fix` targets deterministic,
  cheap skills that can be resolved autonomously.

---

## Design

Putting a rule in a system prompt doesn't guarantee the AI follows it —
compliance is probabilistic. Bonsai treats governance the same way
compilers treat type safety: as an external check that runs after
generation, not a suggestion embedded in the prompt.

**How it works.** Each governance concern lives in a SKILL.md file — a
focused validation prompt with structured input/output schemas. Bonsai
applies these skills in two ways:

- **Code-generating commands** (`implement`, `fix`, `patch`) use dual
  enforcement: compact criteria are injected into the role prompt at
  generation time (zero extra API calls), then skills run as a post-hoc
  gate. On failure, diagnostics feed back for a corrective iteration.
- **Validation commands** (`check`) run the post-hoc gate only.

**Economics.** Pre-injection adds ~500 tokens of extracted criteria, not
full SKILL.md prose. Post-hoc skills run cheapest-first with fail-fast.
The gate loop targets first-pass compliance to minimize iterations.
Bonsai should cost fewer total tokens than verbose prompting with blind
retries.

**Limitations.** The generate-then-validate loop is probabilistic. There
is no convergence guarantee within the iteration budget. When the loop
exhausts its budget, bonsai reports failure with diagnostics — it never
silently accepts non-compliant output.

---

## Development

Requires Go 1.25+ and [Task](https://taskfile.dev/).

```bash
task build      # go build with version injection
task test       # go test -race ./...
task lint       # golangci-lint
task check      # vet + lint + test
task snapshot   # goreleaser snapshot (local)
```

## License

Apache-2.0. See [LICENSE](LICENSE).
