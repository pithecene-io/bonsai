# Bonsai

AI governance toolkit for software repositories.

Bonsai is a Go CLI that provides structured AI-assisted workflows:
planning, implementation with governance gating, code review, skill-based
validation, and repository migration scaffolding. It replaces a collection
of shell scripts with a single distributable binary.

## Install

**From source:**

```bash
go install github.com/justapithecus/bonsai/cmd/bonsai@latest
```

**From release:**

```bash
# Download from GitHub releases
gh release download --repo justapithecus/bonsai --pattern '*_linux_amd64.tar.gz'
tar xzf bonsai_*_linux_amd64.tar.gz
mv bonsai ~/.local/bin/
```

## Commands

```
bonsai chat [role]           Interactive AI chat (default role: architect)
bonsai plan                  Planning session (produces plan.json)
bonsai implement             Implementation with governance gating loop
bonsai review                Code review session (uses codex)
bonsai patch "<task>"        Three-phase patch surgery: plan → emit → validate
bonsai skill <name>          Run a single governance skill
bonsai check                 Run governance skills (bundle or mode-based)
bonsai list                  List available skills, bundles, or roles
bonsai migrate [path]        Scaffold AI governance into a repository
bonsai hooks install         Install pre-push governance hook
bonsai hooks remove          Remove governance pre-push hook
bonsai completion {bash|zsh|fish}  Generate shell completions
```

### Governance Workflow

The core workflow is: **plan → implement → review**.

`bonsai implement` runs a 3-iteration gating loop:
1. Starts a Claude session for code changes
2. Computes diff profile when the session ends
3. Determines governance mode (PATCH/NORMAL/API/STRUCTURAL/HEAVY)
4. Runs skill-based validation via `bonsai check`
5. If validation fails, re-enters with findings injected into the prompt
6. Saves artifacts (patch + report) on pass

### Modes

| Mode | Trigger |
|------|---------|
| HEAVY | >500 diff lines OR >15 files OR (structural AND API) |
| STRUCTURAL | >1 top-level dirs OR renames |
| API | Public surface paths touched |
| PATCH | ≤3 files, no new files, no renames |
| NORMAL | Default |

## Configuration

Bonsai uses a layered config merge chain:

1. Embedded defaults (compiled into binary)
2. User config: `~/.config/bonsai/config.yaml`
3. Repo config: `<repo-root>/.bonsai.yaml`
4. Environment variables: `BONSAI_*` prefix
5. CLI flags

See `internal/config/config.go` for the full config schema and defaults.

## Development

Requires Go 1.25+.

```bash
# Build (with version injection)
task build

# Test
task test

# All checks (vet, lint, test)
task check

# Release snapshot (local)
task snapshot
```

## Architecture

```
cmd/bonsai/          Entrypoint
internal/cli/        Command definitions
internal/gate/       Gating loop state machine
internal/orchestrator/ Multi-skill execution
internal/skill/      Skill loading and running
internal/registry/   skills.yaml parsing
internal/prompt/     System prompt assembly
internal/diff/       Diff profiling and mode routing
internal/agent/      AI agent backends (claude, codex)
internal/config/     Configuration resolution
internal/assets/     Embedded asset resolution
internal/repo/       Repository detection
internal/gitutil/    Git command helpers
```

## License

Apache-2.0 — see [LICENSE](LICENSE).
