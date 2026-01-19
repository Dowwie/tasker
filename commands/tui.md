---
description: Launch the TUI status dashboard in a tmux split pane
---

Launch the TUI dashboard:

```bash
.claude/hooks/launch-tui.sh
```

This opens a horizontal split at the bottom of your tmux window showing:
- Current phase and progress
- Task status breakdown
- Health checks (DAG, steel thread, verification)
- Verifier calibration
- Cost metrics
- Active and recently completed tasks

The TUI auto-refreshes every 5 seconds.

**Keybindings in TUI:**
- `r` - Manual refresh
- `a` - Toggle auto-refresh
- `d` - Toggle dark/light mode
- `q` - Quit

To close the TUI pane:
```bash
.claude/hooks/close-tui.sh
```

Or just press `q` in the TUI, then Enter.
