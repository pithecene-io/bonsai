# CONTRACT_OUTPUT — Output Artifacts

Defines the output directory, artifact inventory, and report JSON
schema. This is a contract document. Implementations must conform.

## Scope

All file artifacts produced by bonsai commands, their paths, formats,
and the report JSON schema.

## Invariants

- Output directory: `{repo_root}/{config.output.dir}` (default:
  `ai/out/`).
- The output directory is auto-created on write (`os.MkdirAll`).
- `.gitignore` SHOULD exclude the output directory.

## Artifact Inventory

| Artifact | Producer | Path | Format |
|----------|----------|------|--------|
| `ai-check.json` | `bonsai check` | `{output_dir}/ai-check.json` | Report JSON |
| `fix.report.json` | `bonsai fix` | `{output_dir}/fix.report.json` | Report JSON |
| `last.patch` | gating loop (on pass) | `{output_dir}/last.patch` | Unified diff |
| `last.report.json` | gating loop (on pass) | `{output_dir}/last.report.json` | Report JSON |
| `plan.json` | planner session | `{output_dir}/plan.json` | Plan JSON |
| `plan.consumed.json` | gating loop | `{output_dir}/plan.consumed.json` | Plan JSON (renamed) |
| `patch-plan.json` | `bonsai patch` phase 1 | `{output_dir}/patch-plan.json` | Plan JSON |

## Report JSON Schema

All report artifacts (`ai-check.json`, `fix.report.json`,
`last.report.json`) share this schema:

```json
{
  "source": "string",
  "timestamp": "string (20060102-150405)",
  "total": "int",
  "passed": "int",
  "failed": "int",
  "skipped": "int",
  "blocking_failed": "int",
  "results": [
    {
      "name": "string",
      "status": "passed|failed|skipped|error",
      "skipped_reason": "string",
      "blocking": "int",
      "major": "int",
      "warning": "int",
      "exit_code": "int",
      "mandatory": "bool",
      "elapsed_ms": "float",
      "blocking_details": ["string"],
      "major_details": ["string"],
      "warning_details": ["string"],
      "info_details": ["string"]
    }
  ]
}
```

### Field Semantics

- `source` — identifies how the skill set was selected (e.g.,
  `"bundle:default"`, `"mode:NORMAL"`).
- `timestamp` — Go `time.Now().Format("20060102-150405")`.
- `total` — number of skills that were run or skipped.
- `passed` — skills with exit code 0 and status not skipped.
- `failed` — skills with non-zero exit code or error status.
- `skipped` — skills skipped (e.g., `requires_diff` without
  `--base`).
- `blocking_failed` — mandatory skills that failed.
- `results[].blocking` — count of blocking findings (plain strings).
- `results[].blocking_details` — the blocking finding strings.
- `results[].status` — `"passed"`, `"failed"`, `"skipped"`, or
  `"error"`.

### Failure Semantics

A report `ShouldFail()` when:
- `blocking_failed > 0`, OR
- All skills were skipped (`total > 0 && skipped == total`).
