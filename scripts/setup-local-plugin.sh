#!/bin/bash
# Setup script for tasker local plugin development
# Run from the tasker project root directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MARKETPLACE_NAME="tasker-marketplace"
PLUGIN_NAME="tasker"

echo "Setting up tasker as a local Claude Code plugin..."
echo "Project root: $PROJECT_ROOT"

# 1. Symlink marketplace to marketplaces directory
MARKETPLACES_DIR="$HOME/.claude/plugins/marketplaces"
if [ ! -L "$MARKETPLACES_DIR/$MARKETPLACE_NAME" ]; then
    echo "Creating marketplace symlink..."
    ln -sf "$PROJECT_ROOT" "$MARKETPLACES_DIR/$MARKETPLACE_NAME"
    echo "  Created: $MARKETPLACES_DIR/$MARKETPLACE_NAME -> $PROJECT_ROOT"
else
    echo "  Marketplace symlink already exists"
fi

# 2. Create skill content symlinks
SKILLS_DIR="$HOME/.claude/skills/$PLUGIN_NAME"
mkdir -p "$SKILLS_DIR"

for skill_dir in "$PROJECT_ROOT/skills/"*/; do
    if [ -d "$skill_dir" ]; then
        skill_name=$(basename "$skill_dir")
        skill_file="$skill_dir/SKILL.md"

        if [ -f "$skill_file" ]; then
            symlink_path="$SKILLS_DIR/${skill_name}.md"
            if [ ! -L "$symlink_path" ]; then
                ln -sf "$skill_file" "$symlink_path"
                echo "  Created skill symlink: $symlink_path"
            else
                echo "  Skill symlink already exists: $symlink_path"
            fi
        fi
    fi
done

echo ""
echo "Setup complete. Start a new Claude Code session to use the plugin."
echo ""
echo "Available skills:"
for skill_dir in "$PROJECT_ROOT/skills/"*/; do
    if [ -d "$skill_dir" ]; then
        skill_name=$(basename "$skill_dir")
        echo "  /$PLUGIN_NAME:$skill_name"
    fi
done
