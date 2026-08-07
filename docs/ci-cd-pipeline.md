# CI/CD Pipeline

## Overview

Windows-only Go project. CI builds + vets on every push/PR. Release workflow cuts a GitHub Release with the binary + changelog when tagged. Modeled on `go-mcp-computer-use` (see its `docs/ci-cd-pipeline.md`).

## Version File

`VERSION` at repository root — single plain-text file containing the semver string (e.g. `0.1.0`). This is the canonical source:

- `go build -ldflags="-X main.Version=$(cat VERSION)"` injects it into the binary
- CI reads it for artifact naming
- Release workflow validates the git tag matches `VERSION` before building
- `docs/meta/CHANGELOG.md` headings must match

## Workflows

### CI (`.github/workflows/ci.yml`)

| Trigger | Action |
|---------|--------|
| Push to `main` | Build + vet + upload artifact |
| PR to `main` | Build + vet |

Artifact name: `mcp-server-windows-<sha>` (uses `${{ github.sha }}`).

### Release (`.github/workflows/release.yml`)

| Trigger | Action |
|---------|--------|
| Push tag `v*` | Build + SHA256 + GitHub Release |

Validates the tag matches `VERSION`. Builds with `scripts/build.ps1 -Release` (stripped, version-injected — pure Go, no CGO/Zig needed). Extracts the corresponding section from `docs/meta/CHANGELOG.md` as the release body. Uploads `mcp-server.exe` + `mcp-server.exe.sha256` + `launch.ps1`.

## Branching Strategy

Single long-lived branch:

```
main ────────────────────────────────────●─── (stable releases)
```

| Branch | Purpose |
|--------|---------|
| `main` | Stable — release-ready. CI runs. Tags cut here. |
| Feature branches | Short-lived forks from `main`. Squash-merged. |

## Release Process

Run `scripts/push-and-release.ps1` after bumping `VERSION` + `CHANGELOG.md`. It:

1. Reads `VERSION` and the matching changelog section
2. Commits (`release: vX.Y.Z` with changelog body), tags, pushes
3. Waits for the `Release` workflow to complete
4. Downloads the CI-built `mcp-server.exe` and schedules replacement of the local binary

Or release manually per [`reference/versioning-strategy.md`](reference/versioning-strategy.md).

## Running CI Locally

```powershell
# Full lint (vet + build)
.\scripts\lint.ps1

# Just vet
go vet ./...

# Build with version injection
.\scripts\build.ps1

# Release build (stripped)
.\scripts\build.ps1 -Release
```

## Cross-References

- `VERSION` — canonical version source
- `docs/meta/CHANGELOG.md` — release notes per version
- `scripts/lint.ps1` — local CI runner (vet + build)
- `scripts/build.ps1` — version-injected build
- `scripts/push-and-release.ps1` — one-shot commit/tag/push/release
- `.github/workflows/ci.yml` — CI workflow
- `.github/workflows/release.yml` — release workflow
- `reference/versioning-strategy.md` — version bump rules
