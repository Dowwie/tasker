---
description: Generate an evaluation report showing how well tasker performed the current job
---

Run the evaluate script to generate a comprehensive performance report:

```bash
python3 scripts/evaluate.py
```

If the user requests JSON output, use `--format json`. If they request just metrics without the full report, use `--metrics-only`.

The report includes:
- Planning quality (verdict, issues at planning time)
- Execution summary (completed, failed, success rate)
- First-attempt success rate and average attempts
- Verification breakdown (functional criteria, code quality, test quality)
- Cost analysis (tokens, cost per task)
- Failure analysis (which tasks failed and why)
- Improvement patterns (common issues across tasks)
