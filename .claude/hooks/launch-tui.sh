#!/bin/bash
# Launch TUI dashboard in a tmux horizontal split (bottom pane)
#
# This hook is triggered when entering planning or execution phases.
# It checks if a TUI pane already exists to avoid duplicates.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TASKER_BIN="${TASKER_BINARY:-$PROJECT_ROOT/go/bin/tasker}"
TUI_PANE_MARKER="TASKER_TUI_PANE"

# Only run if we're in tmux
if [ -z "$TMUX" ]; then
    exit 0
fi

# Check if TUI pane already exists by looking for our marker in pane titles
existing_pane=$(tmux list-panes -F '#{pane_id} #{pane_title}' 2>/dev/null | grep "$TUI_PANE_MARKER" | head -1 | cut -d' ' -f1 || true)

if [ -n "$existing_pane" ]; then
    # TUI already running, just make sure it's visible
    exit 0
fi

# Create horizontal split (bottom pane, 30% height) and run TUI
tmux split-window -v -l 30% -c "$PROJECT_ROOT" \
    "printf '\\033]2;${TUI_PANE_MARKER}\\033\\\\'; '$TASKER_BIN' tui; read -p 'TUI exited. Press Enter to close...'"

# Return focus to the original pane (where Claude Code is running)
tmux select-pane -t '{previous}'

exit 0
