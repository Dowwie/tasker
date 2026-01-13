#!/bin/bash
# Ensures git is initialized in the target directory
# Usage: ./scripts/ensure-git.sh <TARGET_DIR>

set -e

TARGET_DIR="$1"

if [ -z "$TARGET_DIR" ]; then
    echo "ERROR: Target directory required"
    echo "Usage: $0 <TARGET_DIR>"
    exit 1
fi

if [ ! -d "$TARGET_DIR" ]; then
    echo "ERROR: Target directory does not exist: $TARGET_DIR"
    exit 1
fi

if [ -d "$TARGET_DIR/.git" ]; then
    echo "Git already initialized at $TARGET_DIR"
    exit 0
fi

echo "Git not initialized in target directory. Initializing..."
cd "$TARGET_DIR" && git init
echo "Git repository initialized at $TARGET_DIR"
