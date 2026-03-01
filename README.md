# Bonsai

**AI governance toolkit for software repositories.**

Bonsai is a CLI that validates, enforces, and fixes governance rules
across codebases using AI-powered skill invocations. It replaces a
collection of shell scripts with a single binary that ships 43
governance skills, six execution bundles, and a gating loop that blocks
non-conforming code from merging.

Bonsai is the enforcement layer, not the rules. Rules live in each
repository's `CLAUDE.md`, `AGENTS.md`, and `docs/ARCH_INDEX.md`. Bonsai
reads them, builds focused prompts, invokes skills against the repo, and
reports findings in a structured JSON format.

---

## What Bonsai Is (and Is Not)

**Bonsai is:**
- A governance validation and enforcement CLI
- A skill orchestrator that runs 43 checks in parallel
- A prompt assembler that layers repo context into focused AI invocations
- A gating loop that blocks non-conforming merges
- Language-agnostic — works on Go, TypeScript, Python, or anything with governance docs

**Bonsai is not:**
- A linter or static analysis tool (it delegates to AI models)
- A code formatter or style enforcer
- An AI agent framework or SDK
- A replacement for tests, CI, or code review
- A way to generate governance rules (those are authored by humans)

Bonsai owns **validation and enforcement**, not rule authoring.

---

## Why Bonsai Exists

Most AI coding tools operate without constraints. They generate code
that may violate architectural boundaries, introduce forbidden patterns,
drift from documented contracts, or ignore repository conventions.

What's usually missing is discipline:
- No structural validation before merge
- No enforcement of documented architectural rules
- No feedback loop between governance docs and actual code
- Ad-hoc review that misses systemic violations

Bonsai formalizes the parts that should be automated and provides the
feedback loop that keeps repositories honest.

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

## Quick Start

```bash
# Install
go install github.com/pithecene-io/bonsai/cmd/bonsai@latest

# Set your Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-...

# Run governance checks
cd your-repo
bonsai check

# Autonomously fix findings
bonsai fix
```

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

### Prerequisites

Bonsai requires an Anthropic API key for skill invocations.
Interactive commands (`plan`, `implement`, `chat`) require the
[Claude CLI](https://docs.anthropic.com/en/docs/claude-cli).
The `review` command requires the
[Codex CLI](https://github.com/openai/codex).

---

## Workflows

### Check → Fix → Check

The primary governance loop. Run validation, fix findings
autonomously, confirm the fixes.

```bash
bonsai check
bonsai fix
bonsai check
```

`check` runs skills in parallel, renders TUI progress, and writes a
JSON report to `ai/out/ai-check.json`. `fix` runs a check, then
launches Claude to resolve findings, repeating up to 3 iterations.

### Plan → Implement

Interactive AI sessions with governance gating:

```bash
bonsai plan        # Claude session: produce an architecture plan
bonsai implement   # Claude session + 3-iteration gating loop
```

`implement` is the full loop: Claude codes → diff profile → mode
routing → skill validation → re-inject findings if needed → up to 3
iterations.

### Patch Surgery

Three-phase autonomous patching for targeted tasks:

```bash
bonsai patch "fix the off-by-one error in pagination"
```

Phase 1 (architecture) → Phase 2 (code emission) → Phase 3
(validation). No human interaction required.

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

Creates `CLAUDE.md`, `docs/ARCH_INDEX.md`, `ai/skills/` scaffold, and
runs initial validation.

---

## Skills System

Bonsai ships **43 governance skills** organized by cost tier and domain.

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

## Governance Modes

Modes determine which skills run based on diff characteristics.

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

---

## Configuration

Layered merge chain (highest precedence last):

1. **Embedded defaults** — compiled into binary
2. **User config** — `~/.config/bonsai/config.yaml`
3. **Repo config** — `<repo>/.bonsai.yaml`
4. **Environment variables** — `BONSAI_*`
5. **CLI flags**

### Example `.bonsai.yaml`

```yaml
models:
  skills:
    cheap: haiku
    moderate: sonnet
    heavy: sonnet
  roles:
    implement: opus
    plan: opus
    review: codex
    patch: sonnet
    chat: sonnet

diff:
  heavy_diff_lines: 500
  heavy_files_changed: 15
  patch_max_files: 3

output:
  dir: ai/out
```

See `internal/config/config.go` for the full schema and defaults.

---

## Agent Backends

Bonsai routes to three AI backends based on model and use case:

| Backend | Models | Use |
|---------|--------|-----|
| Anthropic API (Go SDK) | haiku, sonnet, opus | Skill invocations (non-interactive) |
| Claude CLI | sonnet, opus | Interactive sessions (plan, implement, chat) |
| Codex CLI | codex | Code review |

See [docs/agent_backends.md](docs/agent_backends.md) for provider
details, fallback behavior, and OAuth configuration.

---

## Commands

| Command | Description |
|---------|-------------|
| `bonsai check` | Run governance skills (bundle or mode-based) |
| `bonsai fix` | Autonomously fix governance findings |
| `bonsai plan` | Start a planning session |
| `bonsai implement` | Implementation with governance gating loop |
| `bonsai review` | Code review session (codex backend) |
| `bonsai patch "<task>"` | Three-phase patch surgery |
| `bonsai skill <name>` | Run a single governance skill |
| `bonsai list` | List skills, bundles, or roles |
| `bonsai migrate [path]` | Scaffold governance into a repository |
| `bonsai chat [role]` | Interactive AI chat session |
| `bonsai hooks install\|remove` | Manage pre-push governance hook |
| `bonsai completion` | Generate shell completions |
| `bonsai version` | Print version |

---

## Gotchas

- **API key required** — skills invoke AI models. Without
  `ANTHROPIC_API_KEY`, skill invocations will fail.
- **Governance docs drive findings** — bonsai validates against what's
  in `CLAUDE.md` and `AGENTS.md`. If those docs are incomplete, findings
  will be incomplete.
- **Repo-local skills override globals** — a repo-local
  `ai/skills/<name>/v1/` completely replaces the embedded skill, including
  the output schema. Keep schemas in sync with the unified format.
- **Diff context requires `--base`** — many skills need diff context.
  Without `--base`, they skip. Use `--base main` for branch-based checks.
- **`fix` only runs cheap skills** — `bonsai fix` targets deterministic,
  cheap skills that Claude can resolve autonomously.

---

## Status

Bonsai is at **v0.1.0** and under active development. It has full
feature parity with the shell-based governance framework it replaces.

If you are evaluating Bonsai, focus on:
- `bonsai check` against a repo with `CLAUDE.md` and `AGENTS.md`
- `bonsai list --skills` to see available governance skills
- `bonsai migrate` to scaffold governance into a new repo

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

Architecture: [docs/ARCH_INDEX.md](docs/ARCH_INDEX.md)

---

## License

Apache-2.0. See [LICENSE](LICENSE).
