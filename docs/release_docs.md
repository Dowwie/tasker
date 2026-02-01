# Release Process

## Prerequisites

- Code changes committed and tested
- GitHub Actions workflow configured (`.github/workflows/release.yaml`)

## Steps

```bash
# 1. Commit your changes
git add -A && git commit -m "Prepare for release"

# 2. Push to GitHub
git push origin main

# 3. Create and push a tag (triggers the release workflow)
./scripts/release.sh patch   # v0.0.1 -> v0.0.2
./scripts/release.sh minor   # v0.0.2 -> v0.1.0
./scripts/release.sh major   # v0.1.0 -> v1.0.0
```

## What Happens

The GitHub Actions workflow:
1. Builds binaries for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
2. Creates `.tar.gz` archives named `tasker_{version}_{os}_{arch}.tar.gz`
3. Generates checksums.txt
4. Publishes to GitHub releases with auto-generated release notes

## Verification

After the workflow completes:
```bash
# Check the release page
open https://github.com/Dowwie/tasker/releases/latest

# Test installation
curl -fsSL https://raw.githubusercontent.com/Dowwie/tasker/main/scripts/install.sh | bash -s -- --easy-mode
```
