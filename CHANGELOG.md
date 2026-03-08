# Changelog

All notable changes to Bonsai will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [0.1.2] - 2026-03-08

### Added

- **Agent tool policy**: new `ToolPolicy` type on `Evaluate` controls tool availability — `ToolsDisabled` (zero value, no tools) and `ToolsReadOnly` (read-only tools: Read, Glob, Grep, WebFetch, WebSearch); callers must explicitly select a policy ([#41](https://github.com/pithecene-io/bonsai/pull/41))
- **Patch plan detection**: `looksLikePlan()` heuristic detects when the architect phase produces a clarification request instead of an actionable plan, skipping the y/N confirmation gate and printing guidance ([#42](https://github.com/pithecene-io/bonsai/pull/42))

### Changed

- **Agent interface**: `Evaluate` signature gains a `ToolPolicy` parameter; all callers updated — governance skills pass `ToolsDisabled`, patch architect passes `ToolsReadOnly` ([#41](https://github.com/pithecene-io/bonsai/pull/41))
- **Agent routing contract**: `CONTRACT_AGENT_ROUTING.md` updated with Tool Policy section and per-backend behavior table ([#41](https://github.com/pithecene-io/bonsai/pull/41))

### Fixed

- **Patch URL access**: `bonsai patch` architect phase can now access URLs (GitHub issues, Linear tickets, etc.) via read-only tools — previously `--tools ""` blocked all tool use ([#41](https://github.com/pithecene-io/bonsai/pull/41))
- **Plan regex URL false positive**: file path regex no longer matches URLs like `github.com/foo/bar.go` — requires `/` directory separator and excludes `://` prefixes ([#42](https://github.com/pithecene-io/bonsai/pull/42))
- **Plan regex root-file false negative**: added `rootFilePattern` to detect plans targeting repo-root files (`CHANGELOG.md`, `.goreleaser.yaml`, `AGENTS.md`, etc.) that lack directory separators ([#42](https://github.com/pithecene-io/bonsai/pull/42))
- **YAML extension inconsistency**: renamed all `.yml` files to `.yaml` for consistency (`.golangci.yaml`, `.goreleaser.yaml`, CI workflows); updated all references in docs, contracts, code, and tests
- **YAML linting**: added yamllint 1.38.0 via mise with `.yamllint.yaml` config; integrated into `task lint` and CI pipeline

---

## [0.1.1] - 2026-03-08

### Fixed

- **Release pipeline**: hardened with strict semver tag validation, `release` environment approval gate, and draft GitHub releases via `softprops/action-gh-release@v2`; GoReleaser now runs with `--skip=publish` as a pure build tool ([#39](https://github.com/pithecene-io/bonsai/pull/39))
- **Release contract**: added `CONTRACT_RELEASE.md` defining the full release lifecycle, conformance checklist, and artifact format requirements; explicitly exempts draft releases from body format requirements ([#39](https://github.com/pithecene-io/bonsai/pull/39))
- **GoReleaser config**: disabled auto-changelog (`changelog.disable: true`), removed unused `release:` block ([#39](https://github.com/pithecene-io/bonsai/pull/39))

---

## [0.1.0] - 2026-03-07

Initial release. Bonsai is a Go CLI that provides AI-powered governance
checks for software repositories — 44 built-in skills, structured JSON
findings, AI-assisted fixing, and interactive sessions with governance
gating.

### Added

- **Full CLI surface**: `check`, `fix`, `plan`, `implement`, `review`, `patch`, `chat`, `skill`, `list`, `migrate`, `hooks`, `completion`, `version` (c206d54, [#2](https://github.com/pithecene-io/bonsai/pull/2), [#3](https://github.com/pithecene-io/bonsai/pull/3))
- **44 governance skills** across 7 domains (structural, architecture, contract, discipline, entropy, depgraph, hygiene) with 3 cost tiers (cheap, moderate, heavy) ([#22](https://github.com/pithecene-io/bonsai/pull/22))
- **Parallel orchestrator**: worker-pool execution with fail-fast, skip detection, structured events, and aggregate JSON reports ([#6](https://github.com/pithecene-io/bonsai/pull/6))
- **Bubbletea TUI**: real-time per-skill progress with spinner, timing, and finding details; falls back to plain text when stdout is not a TTY ([#6](https://github.com/pithecene-io/bonsai/pull/6), [#16](https://github.com/pithecene-io/bonsai/pull/16), [#17](https://github.com/pithecene-io/bonsai/pull/17))
- **Direct Anthropic API backend**: Go SDK integration with cost-model routing and OAuth credential discovery from Claude CLI ([0b1d440](https://github.com/pithecene-io/bonsai/commit/0b1d440))
- **Three agent backends**: Anthropic API (Go SDK), Claude CLI (subprocess), and Codex CLI (subprocess) — model-based dispatch with automatic fallback ([#29](https://github.com/pithecene-io/bonsai/pull/29))
- **`bonsai fix`**: check-then-fix loop that resolves governance findings with AI-assisted automation ([#11](https://github.com/pithecene-io/bonsai/pull/11), [#13](https://github.com/pithecene-io/bonsai/pull/13))
- **Governance gating loop**: 3-iteration state machine — session → diff → profile → mode → gate → pass/fail/re-inject — for `implement` and `patch` commands ([c206d54](https://github.com/pithecene-io/bonsai/commit/c206d54))
- **Diff profiling and mode routing**: automatic governance mode selection (PATCH → NORMAL → STRUCTURAL → API → HEAVY → AUDIT) based on diff characteristics ([c206d54](https://github.com/pithecene-io/bonsai/commit/c206d54))
- **Layered prompt assembly**: preamble → mode → CLAUDE.md → context → role → AGENTS.md → ARCH_INDEX.md ([c206d54](https://github.com/pithecene-io/bonsai/commit/c206d54))
- **Configuration merge chain**: embedded defaults → user config → repo config (`.bonsai.yaml`) → env vars (`BONSAI_*`) → CLI flags ([#12](https://github.com/pithecene-io/bonsai/pull/12))
- **6 skill bundles**: patch (7), default (16), structural-change (17), api-change (13), heavy (36), audit-full (44) ([#30](https://github.com/pithecene-io/bonsai/pull/30))
- **6-phase repository migration**: `bonsai migrate` scaffolds CLAUDE.md, AGENTS.md, ARCH_INDEX.md, and repo-local skill overrides ([c206d54](https://github.com/pithecene-io/bonsai/commit/c206d54))
- **Auto-worktree creation**: code-modifying commands (`implement`, `patch`, `fix`) automatically create git worktrees when run on main/master ([#30](https://github.com/pithecene-io/bonsai/pull/30))
- **Contract existence check**: `config-contract-drift` skill validates that referenced contracts exist on disk ([#25](https://github.com/pithecene-io/bonsai/pull/25))
- **Pre-push governance hook**: `bonsai hooks install` adds a git pre-push hook that gates on governance findings ([c206d54](https://github.com/pithecene-io/bonsai/commit/c206d54))
- **Contract suite**: 8 prescriptive contract documents covering config, output, prompt assembly, roles, skills, agent routing, CLI, and gating ([#26](https://github.com/pithecene-io/bonsai/pull/26), [#28](https://github.com/pithecene-io/bonsai/pull/28))
- **Test coverage**: audit-hardening, boundary-case, and injection-order tests across cli, orchestrator, agent, skill, gate, assets, and xio packages ([#5](https://github.com/pithecene-io/bonsai/pull/5), [#20](https://github.com/pithecene-io/bonsai/pull/20))
- **CI pipeline**: build, test (with race detector), vet, and golangci-lint on every push/PR ([#4](https://github.com/pithecene-io/bonsai/pull/4))
- **GoReleaser**: cross-platform builds (linux/darwin × amd64/arm64) with version injection ([c206d54](https://github.com/pithecene-io/bonsai/commit/c206d54))
- **Task runner**: `Taskfile.yaml` for build, test, lint, check, and snapshot tasks ([#7](https://github.com/pithecene-io/bonsai/pull/7))

### Changed

- **Config schema flattened**: YAML reorganized into `providers`/`agents`/`models` top-level keys ([#12](https://github.com/pithecene-io/bonsai/pull/12))
- **Role naming unified**: architect, implementer, planner, patcher, reviewer — patch-architect removed ([#27](https://github.com/pithecene-io/bonsai/pull/27))
- **Agent interface renamed**: methods aligned with contract-prescribed names ([#29](https://github.com/pithecene-io/bonsai/pull/29))
- **Heavy mode differentiated from audit**: heavy bundle reduced from 44 to 36 skills; 8 whole-codebase analysis skills moved to audit-only ([#30](https://github.com/pithecene-io/bonsai/pull/30))
- **Orchestrator parallelism**: defaults to unlimited for I/O-bound skills ([#10](https://github.com/pithecene-io/bonsai/pull/10))
- **Sink lifecycle encapsulated**: run-scoped state with single-pass JSON output ([#22](https://github.com/pithecene-io/bonsai/pull/22), [#23](https://github.com/pithecene-io/bonsai/pull/23))

### Fixed

- **Stderr swallowing**: agent subprocess stderr now propagated correctly ([#4](https://github.com/pithecene-io/bonsai/pull/4))
- **Module path**: corrected to `github.com/pithecene-io/bonsai` ([#3](https://github.com/pithecene-io/bonsai/pull/3))
- **TUI width clipping**: skill names and output wrapped to terminal width with ANSI-safe layout ([#16](https://github.com/pithecene-io/bonsai/pull/16), [#17](https://github.com/pithecene-io/bonsai/pull/17))
- **List resolver**: fixed skill, bundle, and role listing ([#3](https://github.com/pithecene-io/bonsai/pull/3))
- **Migrate timeout**: migration phases no longer time out prematurely ([#3](https://github.com/pithecene-io/bonsai/pull/3))
- **golangci-lint v2**: migrated linter config to v2 schema ([e873fe0](https://github.com/pithecene-io/bonsai/commit/e873fe0))
- **gofumpt alignment**: struct field alignment in worktreeResult ([#32](https://github.com/pithecene-io/bonsai/pull/32))
- **Implement plan consumption**: plan.json now consumed and fed to agent via Execute mode; Session used when no plan present ([#34](https://github.com/pithecene-io/bonsai/pull/34))
- **Signal handling**: app-level SIGINT/SIGTERM propagation via `RunContext` — all commands now respond to CTRL-C ([#34](https://github.com/pithecene-io/bonsai/pull/34), [#36](https://github.com/pithecene-io/bonsai/pull/36))
- **Agent routing**: bare `NewClaude`/`NewRouter` calls replaced with `newAgentRouter` across review, patch, and migrate commands — Anthropic direct API now reachable from all commands ([#36](https://github.com/pithecene-io/bonsai/pull/36))
- **Session dispatch**: `Router.Session` now dispatches based on `--model` in extraArgs, matching Execute behavior — backends fully hotswappable ([#36](https://github.com/pithecene-io/bonsai/pull/36))
- **CLAUDECODE env filtering**: added to `Codex.Session` for consistency with `Claude.Session` ([#36](https://github.com/pithecene-io/bonsai/pull/36))
- **Release pipeline**: GoReleaser now creates draft releases; hand-curated notes added before publishing ([#37](https://github.com/pithecene-io/bonsai/pull/37))

### Known Limitations

- The generate-then-validate gating loop is probabilistic; it may exhaust its 3-iteration budget without resolving all findings
- At least one agent backend (Claude CLI, Codex CLI, or Anthropic API key) is required; backends are hotswappable via model configuration
- Repo-local skills override the embedded version entirely, including the output schema
