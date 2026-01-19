#!/bin/bash
# Close the TUI dashboard pane when workflow completes
#
# This hook is triggered when execution completes.

set -e

TUI_PANE_MARKER="TASKER_TUI_PANE"

# Only run if we're in tmux
if [ -z "$TMUX" ]; then
    exit 0
fi

# Find and close the TUI pane
existing_pane=$(tmux list-panes -F '#{pane_id} #{pane_title}' 2>/dev/null | grep "$TUI_PANE_MARKER" | head -1 | cut -d' ' -f1 || true)

if [ -n "$existing_pane" ]; then
    tmux kill-pane -t "$existing_pane" 2>/dev/null || true
fi

exit 0
