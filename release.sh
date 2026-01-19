#!/bin/bash
set -euo pipefail

usage() {
    echo "Usage: $0 <major|minor|patch>"
    echo "  major: v1.2.3 -> v2.0.0"
    echo "  minor: v1.2.3 -> v1.3.0"
    echo "  patch: v1.2.3 -> v1.2.4"
    exit 1
}

[[ $# -ne 1 ]] && usage

BUMP_TYPE="$1"
[[ ! "$BUMP_TYPE" =~ ^(major|minor|patch)$ ]] && usage

CURRENT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
VERSION="${CURRENT_TAG#v}"

IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

case "$BUMP_TYPE" in
    major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    patch) PATCH=$((PATCH + 1)) ;;
esac

NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"

echo "Current version: $CURRENT_TAG"
echo "New version:     $NEW_TAG"
echo ""

read -p "Continue? [y/N] " -n 1 -r
echo ""
[[ ! $REPLY =~ ^[Yy]$ ]] && exit 1

if [[ -n $(git status --porcelain) ]]; then
    echo "Error: Working directory not clean. Commit or stash changes first."
    exit 1
fi

git tag "$NEW_TAG"
git push origin main
git push origin "$NEW_TAG"

echo ""
echo "Released $NEW_TAG"
