# Versioning Strategy

## Status

Accepted (2026-08-07)

## Canonical Version Source

The canonical version is the `VERSION` file at repository root. The versioning policy is defined in this document.

The `VERSION` file is read at build time and injected via ldflags:

```bash
go build -ldflags="-X main.Version=$(cat VERSION)" -o mcp-server.exe ./cmd/mcp-server
```

This is what MCP clients see via `server.info`. All other references (CHANGELOG, git tags, release artifacts) must match `VERSION`. See [`../ci-cd-pipeline.md`](../ci-cd-pipeline.md) for the automated workflow.

## Versioning Scheme

```
v<major>.<minor>.<patch>
```

| Bump | When | Examples |
|------|------|----------|
| `+0.0.1` (patch) | Bug fixes, tool tweaks, doc updates, minor refactors | Fixing a recycle-bin edge case, adjusting a cache path, renaming a tool parameter |
| `+0.1.0` (minor) | New tools, new capabilities, architecture changes, dependency adds | Adding a new tool category, introducing the GUI |
| `+1.0.0` (major) | Stable release with proven architecture, all planned slices complete, field-tested | GUI complete, cleanup pipeline battle-tested |

**Current trajectory:** v0.1.x (initial tool set) → v0.2.x (GUI) → v1.0.0 (stable release)

Breaking changes at 0.x require a minor bump (not major), per SemVer spec §4.

## Git Tagging

Every release must have an annotated or lightweight tag matching the version:

```
v0.1.0  ← tagged on the release commit
v0.1.1  ← tagged on the release commit
```

Tags are immutable once pushed. If a release is faulty, bump the patch and re-tag. Never delete and recreate a pushed tag.

## Changelog Convention

`../meta/CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com) with sections:

- `### Added` — new tools, new capabilities
- `### Changed` — modifications to existing tools or behavior
- `### Fixed` — bug fixes
- `### Removed` — removed tools or features
- `### Security` — security-related changes
- `### Performance Improvements` — perf changes

A changelog entry is required for every release. Entries are written in present-tense imperative mood.

## Release Process

```
[1] Code complete — all changes for the release are merged
[2] Bump version in VERSION file
[3] Update ../meta/CHANGELOG.md with the new version heading
[4] Run pre-release gates:
      - go build ./cmd/mcp-server/     (compiles)
      - go vet ./...                   (static analysis)
[5] Commit: "release: vX.Y.Z"
[6] Tag:   git tag vX.Y.Z
[7] Push:  git push && git push origin vX.Y.Z
[8] Release workflow auto-builds and creates a GitHub Release
```

Steps 5–8 can be done in one shot with `../scripts/push-and-release.ps1`.

## Commit Strategy

Squash-merge into `main` — each release is a single commit on the default branch. Feature branches with incremental commits are collapsed into one commit on merge.

## Pre-Release Gates (mandatory)

| Gate | Command | Fail action |
|------|---------|-------------|
| Lint | `go vet ./...` | Fix warnings |
| Build | `go build ./cmd/mcp-server/` | Fix compilation |
| Version consistency | `git grep "0\.1\.10" -- ':!VERSION' ':!.git'` | Fix stale references |

## Example: Patch Release

```bash
# Edit VERSION: "0.1.0" → "0.1.1"
# Edit ../meta/CHANGELOG.md: add ## [0.1.1] section
$ver = (Get-Content VERSION -Raw).Trim()
go build -ldflags="-X main.Version=$ver" ./cmd/mcp-server/ && go vet ./...
git add -A && git commit -m "release: v0.1.1"
git tag v0.1.1
git push && git push origin v0.1.1  # triggers release workflow
```

## Cross-References

- `../meta/CHANGELOG.md` — release history
- `VERSION` — canonical version source
- `../ci-cd-pipeline.md` — CI/CD workflows for automated build + release
- `../scripts/push-and-release.ps1` — one-shot release script
