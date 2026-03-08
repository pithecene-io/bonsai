# CONTRACT_RELEASE — Release Artifacts

This contract defines the required format for release artifacts:
CHANGELOG.md and GitHub release notes.

---

## CHANGELOG.md

The repository MUST maintain a `CHANGELOG.md` at the repo root.

### Format

- Based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
- Project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
- Sections separated by horizontal rules (`---`)

### Required structure

```markdown
# Changelog

All notable changes to Bonsai will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [X.Y.Z] - YYYY-MM-DD

### Added
### Changed
### Fixed
### Upgrade Notes   (optional)
### Known Limitations   (optional)
### References   (optional)
```

### Rules

1. Every tagged release MUST have a corresponding CHANGELOG entry
2. The `[Unreleased]` section MUST exist and accumulate changes between releases
3. Entries MUST include provenance as markdown links: PR references `([#N](url))` or commit links `([sha](url))` when no PR exists
4. Entries MUST use bold lead text for the component or area, followed by a colon and description
5. Section ordering: Added → Changed → Fixed → Upgrade Notes → Known Limitations → References
6. Omit empty sections
7. Date format: ISO 8601 (`YYYY-MM-DD`)

---

## GitHub Release Notes

Every **published** GitHub release MUST have hand-curated notes following
the template below. Draft releases created by CI are pre-publication
artifacts and are exempt from body format requirements until published.
GoReleaser auto-generated changelogs MUST be disabled.

### Title

`vX.Y.Z` — version only, no tagline in the title.

### Body format

```markdown
**Bold tagline — a concise phrase describing the release theme**

## Summary

1–2 sentences explaining what and why.

## Highlights

- **Component**: description of change (3–6 bullets)

## Breaking Changes   (only if applicable)

## Upgrade Notes   (only if applicable)

## Known Limitations   (only if applicable)

## References

- [CHANGELOG.md](link-to-changelog-at-tag)
- [Other relevant docs](links)

**Full Changelog**: https://github.com/pithecene-io/bonsai/compare/PREV...vX.Y.Z
```

### Rules

1. Tagline MUST appear before `## Summary` as a bold phrase on its own line, no trailing period
2. Do not repeat the version in the body
3. Do not include auto-generated "What's Changed" commit lists
4. Keep the body concise and user-facing
5. Highlight bullets MUST use bold lead text for the feature or area
6. The `**Full Changelog**` compare link MUST be the last line
7. `## References` MUST link to `CHANGELOG.md` at the release tag

---

## GoReleaser Configuration

The `.goreleaser.yaml` MUST disable auto-generated changelogs:

```yaml
changelog:
  disable: true
```

GoReleaser runs with `--skip=publish` to produce archives and checksums
without creating a GitHub release. A separate workflow job creates the
release as a draft using `softprops/action-gh-release@v2`.

---

## Release Workflow

1. Update `CHANGELOG.md`: move `[Unreleased]` entries to `[X.Y.Z] - YYYY-MM-DD`
2. Commit: `chore(release): 🔖 prepare vX.Y.Z`
3. Open a PR, get review, and merge to `main`
4. Tag the merge commit on `main`: `git tag vX.Y.Z` and `git push origin vX.Y.Z`
5. CI validates the tag (strict semver, must be `vX.Y.Z`) and builds artifacts via GoReleaser
6. Approve the release in the `release` GitHub environment
7. A DRAFT GitHub release is created with the built artifacts
8. Edit the draft release: add hand-curated notes per the template above
9. Publish the release
10. Verify: artifacts downloadable, release notes render correctly

---

## Conformance

A release is conformant when:

- [ ] `CHANGELOG.md` has an entry for the version
- [ ] `[Unreleased]` section is empty (all entries moved to version)
- [ ] CI tag validation passed (strict semver)
- [ ] Release was approved via `release` GitHub environment
- [ ] Published GitHub release title is `vX.Y.Z` only
- [ ] Published GitHub release body follows the template above
- [ ] GoReleaser auto-changelog is disabled
- [ ] `**Full Changelog**` compare link is present and correct
