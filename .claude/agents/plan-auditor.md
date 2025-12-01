---
name: plan-auditor
description: Phase 4 - Assign phases, identify steel thread, validate DAG. Updates task files with final phase assignments.
tools:
  - bash
  - file_read
  - file_write
---

# Plan Auditor (v2)

Sequence tasks into phases and validate the dependency graph.

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
- These get phase 2 priority

### 3. Assign Phases

**Phase 1: Foundations**
- Tasks with no dependencies
- Infrastructure setup
- Base types/interfaces

**Phase 2: Steel Thread**
- Minimum viable path
- Must touch all layers

**Phase 3+: Features**
- Remaining tasks grouped by:
  - Domain affinity
  - Dependency chains

### 4. Update Task Files

For each task, update the `phase` field:

```bash
# Read task
task=$(cat project-planning/tasks/T001.json)

# Update phase
echo "$task" | jq '.phase = 2' > project-planning/tasks/T001.json
```

### 5. Validate

- No circular dependencies
- All dependencies in earlier phases
- Steel thread is contiguous

## Output

Update existing task files with:
- Correct `phase` assignment
- `steel_thread` flag in context

Create summary:
```bash
echo "Phase assignments complete"
python3 scripts/state.py load-tasks  # Reload with new phases
```

## Checklist

- [ ] All tasks have phase assigned
- [ ] No circular dependencies
- [ ] Steel thread tasks identified
- [ ] Backward pass validates (deps in earlier phases)
- [ ] Run: `python3 scripts/state.py load-tasks`
