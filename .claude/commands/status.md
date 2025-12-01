---
description: Display executor status dashboard with task progress and recent activity
---

Run the status script to display a workflow dashboard:

```bash
python3 scripts/status.py --once
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

For interactive TUI mode (requires textual), use:
```bash
python3 scripts/status.py
```

For JSON output:
```bash
python3 scripts/status.py --json
```
