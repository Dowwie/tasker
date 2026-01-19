#!/bin/bash
# Detect /plan or /execute commands and launch TUI
#
# This hook receives JSON on stdin with the user's prompt.
# If it matches /plan or /execute, launch the TUI dashboard.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASKER_BIN="${TASKER_BINARY:-$SCRIPT_DIR/../../go/bin/tasker}"

# Read the input JSON from stdin
INPUT=$(cat)

# Extract the prompt field using Go CLI
PROMPT=$(echo "$INPUT" | "$TASKER_BIN" hook get-prompt 2>/dev/null || echo "")

# Check if this is a /plan or /execute command
if [[ "$PROMPT" =~ ^[[:space:]]*/plan([[:space:]]|$) ]] || [[ "$PROMPT" =~ ^[[:space:]]*/execute([[:space:]]|$) ]]; then
    # Launch TUI in background (don't block Claude)
    "$SCRIPT_DIR/launch-tui.sh" &
fi

# Always allow the prompt to continue
exit 0
