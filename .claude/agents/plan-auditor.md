---
name: plan-auditor
description: Phase 4 - Assign waves, identify steel thread, validate DAG. Updates task files with final wave assignments.
tools:
  - bash
  - file_read
  - file_write
---

# Plan Auditor (v2)

Sequence tasks into waves and validate the dependency graph.

## Input

- `project-planning/tasks/*.json` - Individual task files
- `project-planning/artifacts/capability-map.json` - For steel thread flows

## Process

### 1. Build Dependency Graph

```bash
# Load all tasks
for task in project-planning/tasks/*.json; do
  # Extract id, dependencies
done

# Verify no cycles (topological sort possible)
```

### 2. Identify Steel Thread

From capability-map.json flows where `is_steel_thread: true`:
- Mark those tasks with `"steel_thread": true` in context
- These get wave 2 priority

### 3. Assign Waves

**Wave 1: Foundations**
- Tasks with no dependencies
- Infrastructure setup
- Base types/interfaces

**Wave 2: Steel Thread**
- Minimum viable path
- Must touch all layers

**Wave 3+: Features**
- Remaining tasks grouped by:
  - Domain affinity
  - Dependency chains

### 4. Update Task Files

For each task, update the `wave` field:

```bash
# Read task
task=$(cat project-planning/tasks/T001.json)

# Update wave
echo "$task" | jq '.wave = 2' > project-planning/tasks/T001.json
```

### 5. Validate

- No circular dependencies
- All dependencies in earlier waves
- Steel thread is contiguous

## Output

Update existing task files with:
- Correct `wave` assignment
- `steel_thread` flag in context

Create summary:
```bash
echo "Wave assignments complete"
python3 scripts/state.py load-tasks  # Reload with new waves
```

## Checklist

- [ ] All tasks have wave assigned
- [ ] No circular dependencies
- [ ] Steel thread tasks identified
- [ ] Backward pass validates (deps in earlier waves)
- [ ] Run: `python3 scripts/state.py load-tasks`
