# Bonsai

AI governance toolkit for software repositories.

Bonsai is a Go CLI that provides structured AI-assisted workflows —
planning, implementation with governance gating, code review, skill-based
validation, and repository migration scaffolding.

## Install

```bash
go install github.com/justapithecus/bonsai/cmd/bonsai@latest
```

## Usage

```bash
bonsai version          # Print version
bonsai help             # Show help
```

## Development

Requires Go 1.25+.

```bash
# Build
go build ./cmd/bonsai

# Test
go test ./...

# Using task runner
task build
task test
task lint
```

## License

Apache-2.0 — see [LICENSE](LICENSE).
