#!/bin/bash

set -euo pipefail

REPO="Dowwie/tasker"
BINARY_NAME="tasker"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/bin}/tasker"
VERSION_FILE="$DATA_DIR/.version"

# Get latest tag via git
get_latest_version() {
    git ls-remote --tags --sort=-v:refname "https://github.com/$REPO.git" \
        | head -1 \
        | sed 's/.*refs\/tags\/v//'
}

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
[[ "$ARCH" == "aarch64" ]] && ARCH="arm64"

BINARY_PATH="$DATA_DIR/$BINARY_NAME"
LATEST=$(get_latest_version)

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
mkdir -p "$DATA_DIR"
URL="https://github.com/$REPO/releases/download/v${LATEST}/${BINARY_NAME}-${OS}-${ARCH}"

echo "Downloading $BINARY_NAME v$LATEST for $OS/$ARCH..." >&2

curl -fsSL "$URL" -o "$BINARY_PATH"
chmod +x "$BINARY_PATH"
echo "$LATEST" > "$VERSION_FILE"

echo "$BINARY_PATH"