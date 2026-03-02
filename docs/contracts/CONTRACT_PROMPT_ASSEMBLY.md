# CONTRACT_PROMPT_ASSEMBLY — Prompt Assembly

Defines the prompt assembly API, layer ordering, and resolver override
chain. This is a contract document. Implementations must conform.

## Scope

System prompt construction for all modes in `internal/prompt/`.

## Invariants

- Prompt assembly order is fixed and MUST NOT be reordered.
- The validator preamble (`claude_validator.md`) is a strict subset of
  the interactive preamble (`claude.md`).

## Builder API Surface

The `Builder` provides three public build methods:

| Method | Used by |
|--------|---------|
| `BuildInteractive(opts)` | `bonsai chat`, `bonsai plan`, `bonsai implement`, `bonsai patch`, `bonsai fix` |
| `BuildValidator(opts)` | Skill evaluation (orchestrator) |
| `BuildReview()` | `bonsai review` |

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

Lite mode is triggered for cheap-tier models (`IsLite()` returns true
for haiku and codex models). It skips all governance layers for fast
evaluation under tight token and latency budgets.

## Prompt Assembly — Review

Layer order for review sessions (`BuildReview`):

1. **Preamble** + **Mode declaration** (REVIEWER)
2. **Global CLAUDE.md**
3. **Context layers**
4. **Role definition** (reviewer)
5. **Review architecture** — `review_architecture.md`
6. **AGENTS.md**
7. **ARCH_INDEX.md**

## Resolver Override Chain

Assets resolve filesystem-first (first match wins):

1. `<repo>/ai/skills/<name>/` — repo-local override
2. `~/.config/bonsai/skills/<name>/` — user config
3. Embedded in binary — global default

A repo-local override completely replaces the embedded asset.
