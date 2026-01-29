#!/bin/bash
# Detect /plan or /execute commands and launch TUI
#
# This hook receives JSON on stdin with the user's prompt.
# If it matches /plan or /execute, launch the TUI dashboard.

set -e

DEBUG_LOG="/tmp/claude/tasker-hook-debug.log"
mkdir -p /tmp/claude

echo "=== $(date) ===" >> "$DEBUG_LOG"
echo "Hook invoked" >> "$DEBUG_LOG"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

echo "SCRIPT_DIR=$SCRIPT_DIR" >> "$DEBUG_LOG"
echo "PLUGIN_ROOT=$PLUGIN_ROOT" >> "$DEBUG_LOG"

# Get tasker binary path (fast if already installed)
if [[ -z "${TASKER_BINARY:-}" ]]; then
    TASKER_BIN=$("$PLUGIN_ROOT/scripts/ensure-tasker.sh" 2>/dev/null) || exit 0
else
    TASKER_BIN="$TASKER_BINARY"
fi

echo "TASKER_BIN=$TASKER_BIN" >> "$DEBUG_LOG"

# Read the input JSON from stdin
INPUT=$(cat)
echo "INPUT=$INPUT" >> "$DEBUG_LOG"

# Extract the prompt field using Go CLI
PROMPT=$(echo "$INPUT" | "$TASKER_BIN" hook get-prompt 2>/dev/null || echo "")
echo "PROMPT=$PROMPT" >> "$DEBUG_LOG"

# Check if this is a /plan or /execute command
if [[ "$PROMPT" =~ ^[[:space:]]*/plan([[:space:]]|$) ]] || [[ "$PROMPT" =~ ^[[:space:]]*/execute([[:space:]]|$) ]]; then
    echo "MATCH: launching TUI" >> "$DEBUG_LOG"
    # Launch TUI in background (don't block Claude)
    "$SCRIPT_DIR/launch-tui.sh" &
else
    echo "NO MATCH for pattern" >> "$DEBUG_LOG"
fi

# Always allow the prompt to continue
exit 0
