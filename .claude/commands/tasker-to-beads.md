# Tasker to Beads

Transform Tasker task definitions into rich, self-contained Beads issues.

Use the `tasker-to-beads` skill: `Skill("tasker-to-beads")`

## What This Does

1. Extracts structural data from Tasker task files
2. Uses LLM comprehension to enrich with spec context, architecture narrative, and human-readable descriptions
3. Creates Beads issues with proper labels, priorities, and dependencies

## Usage

- `/tasker-to-beads` — Interactive mode, processes all tasks
- `/tasker-to-beads T001` — Process single task by ID
- `/tasker-to-beads --batch` — Process all tasks and create manifest for batch import

## Helper Script

The skill includes a Python script for mechanical transformations:

```bash
python3 .claude/skills/tasker-to-beads/scripts/transform.py context --all
python3 .claude/skills/tasker-to-beads/scripts/transform.py status
```
