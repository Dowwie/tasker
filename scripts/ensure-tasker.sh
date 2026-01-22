#!/bin/bash

set -euo pipefail

REPO="Dowwie/tasker"
BINARY_NAME="tasker"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/bin}/tasker"
VERSION_FILE="$DATA_DIR/.version"
LAST_CHECK_FILE="$DATA_DIR/.last_update_check"
UPDATE_CHECK_INTERVAL=86400  # 24 hours in seconds

# If binary exists, return it immediately (fast path)
BINARY_PATH="$DATA_DIR/$BINARY_NAME"
if [[ -x "$BINARY_PATH" ]]; then
    # Check for updates only periodically, not every invocation
    if [[ -f "$LAST_CHECK_FILE" ]]; then
        LAST_CHECK=$(cat "$LAST_CHECK_FILE")
        NOW=$(date +%s)
        if (( NOW - LAST_CHECK < UPDATE_CHECK_INTERVAL )); then
            echo "$BINARY_PATH"
            exit 0
        fi
    fi
fi

# Get latest tag via git (only called if update check needed)
get_latest_version() {
    git ls-remote --tags --sort=-v:refname "https://github.com/$REPO.git" 2>/dev/null \
        | head -1 \
        | sed 's/.*refs\/tags\/v//' || echo ""
}

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
[[ "$ARCH" == "aarch64" ]] && ARCH="arm64"

LATEST=$(get_latest_version)

# Record that we checked (even if it failed)
mkdir -p "$DATA_DIR"
date +%s > "$LAST_CHECK_FILE"

# If no releases available, use existing binary or exit gracefully
if [[ -z "$LATEST" ]]; then
    if [[ -x "$BINARY_PATH" ]]; then
        echo "$BINARY_PATH"
        exit 0
    fi
    echo "No releases found and no binary installed" >&2
    exit 1
fi

# Already installed and current?
if [[ -x "$BINARY_PATH" && -f "$VERSION_FILE" ]]; then
    INSTALLED=$(cat "$VERSION_FILE")
    if [[ "$INSTALLED" == "$LATEST" ]]; then
        echo "$BINARY_PATH"
        exit 0
    fi
    echo "Upgrading $BINARY_NAME $INSTALLED → $LATEST..." >&2
fi

# Download
URL="https://github.com/$REPO/releases/download/v${LATEST}/${BINARY_NAME}-${OS}-${ARCH}"

echo "Downloading $BINARY_NAME v$LATEST for $OS/$ARCH..." >&2

if curl -fsSL "$URL" -o "$BINARY_PATH" 2>/dev/null; then
    chmod +x "$BINARY_PATH"
    echo "$LATEST" > "$VERSION_FILE"
else
    # Download failed - use existing binary if available
    if [[ -x "$BINARY_PATH" ]]; then
        echo "Download failed, using existing binary" >&2
        echo "$BINARY_PATH"
        exit 0
    fi
    echo "Download failed and no binary installed" >&2
    exit 1
fi

echo "$BINARY_PATH"