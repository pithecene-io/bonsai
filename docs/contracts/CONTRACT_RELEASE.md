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
3. Entries MUST include PR references as markdown links: `([#N](url))`
4. Entries MUST use bold lead text for the component or area, followed by a colon and description
5. Section ordering: Added → Changed → Fixed → Upgrade Notes → Known Limitations → References
6. Omit empty sections
7. Date format: ISO 8601 (`YYYY-MM-DD`)

---

## GitHub Release Notes

Every tagged release MUST have a GitHub release with hand-curated notes.
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

The `.goreleaser.yml` MUST disable auto-generated changelogs:

```yaml
changelog:
  disable: true
```

Release notes are provided via `gh release edit` or the `--release-notes`
flag after GoReleaser creates the release with artifacts.

---

## Release Workflow

1. Update `CHANGELOG.md`: move `[Unreleased]` entries to `[X.Y.Z] - YYYY-MM-DD`
2. Commit: `chore(release): 🔖 prepare vX.Y.Z`
3. Tag: `git tag vX.Y.Z`
4. Push: `git push origin main vX.Y.Z`
5. GoReleaser builds and publishes artifacts
6. Edit the GitHub release: replace empty body with hand-curated notes
7. Verify: artifacts downloadable, release notes render correctly

---

## Conformance

A release is conformant when:

- [ ] `CHANGELOG.md` has an entry for the version
- [ ] `[Unreleased]` section is empty (all entries moved to version)
- [ ] GitHub release title is `vX.Y.Z` only
- [ ] GitHub release body follows the template above
- [ ] GoReleaser auto-changelog is disabled
- [ ] `**Full Changelog**` compare link is present and correct
