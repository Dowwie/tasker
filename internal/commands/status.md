---
description: Display executor status dashboard with task progress and recent activity
---

Run the status command to display a workflow dashboard:

```bash
tasker state status --once
```

This shows:
- Current phase and target directory
- Progress (completed/total tasks, current phase)
- Status breakdown (completed, running, failed, blocked, pending)
- Health checks (DAG validation, steel thread, verification commands)
- Verifier calibration score
- Cost metrics (tokens, USD)
- Active tasks currently running
- Recent failures

For interactive TUI mode, use:
```bash
tasker tui
```

For JSON output:
```bash
tasker state status --json
```
