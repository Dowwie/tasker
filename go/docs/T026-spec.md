# T026: Release Pipeline and Homebrew Formula

## Summary

Implements a GitHub Actions release pipeline that triggers on version tags, builds cross-platform binaries, creates GitHub releases with artifacts, and updates the Homebrew formula for easy installation.

## Components

- `.github/workflows/release.yml` - Release pipeline configuration

## Workflow Jobs

| Job | Purpose |
|-----|---------|
| `build` | Build cross-platform binaries (darwin/linux/windows, amd64/arm64) |
| `release` | Create GitHub release with binaries and checksums |
| `homebrew` | Update Homebrew tap formula with new version |

## Triggers

- Push of tags matching `v*.*.*` pattern (e.g., `v1.0.0`, `v2.1.3`)

## Build Matrix

Builds binaries for the following platforms:
- macOS (darwin) amd64 and arm64
- Linux amd64 and arm64
- Windows amd64

## Pipeline Steps

### Build Job
1. Checkout code
2. Set up Go 1.22
3. Extract version from tag
4. Build binary with version embedded via ldflags
5. Upload artifact for release job

### Release Job
1. Depends on build job completion
2. Download all build artifacts
3. Generate SHA256 checksums
4. Create GitHub release using softprops/action-gh-release
5. Attach all binaries and checksums to release
6. Mark as prerelease if tag contains `-` (e.g., `v1.0.0-beta`)

### Homebrew Job
1. Depends on release job completion
2. Skipped for prerelease versions
3. Calculate SHA256 for darwin-arm64 binary
4. Clone homebrew-tap repository
5. Update Formula/tasker.rb with new version and checksum
6. Commit and push changes

## Configuration

### Required Secrets

| Secret | Purpose |
|--------|---------|
| `GITHUB_TOKEN` | Automatically provided for release creation |
| `HOMEBREW_TAP_TOKEN` | PAT for pushing to homebrew-tap repository (optional) |

## Dependencies

- T025: CI pipeline (builds must pass before release)
- T024: Makefile (provides build infrastructure)

## Testing

```bash
# Verify workflow syntax
cat .github/workflows/release.yml | head -20

# Verify tag trigger
grep -A2 "tags:" .github/workflows/release.yml

# Verify release action
grep "softprops/action-gh-release" .github/workflows/release.yml

# Verify Homebrew integration
grep "homebrew" .github/workflows/release.yml
```

## Usage

```bash
# Create a release
git tag v1.0.0
git push origin v1.0.0
```
