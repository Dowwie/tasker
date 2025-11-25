---
name: orchestrator
description: Thin orchestrator for Task Decomposition Protocol v2. Supports two modes - /plan (decompose spec into tasks) and /execute (run tasks via subagents). Delegates all state management to scripts/state.py.
tools:
  - agent
  - bash
  - file_read
  - file_write
---

# Orchestrator v2

A **thin coordination layer** with two distinct modes:
- **Plan Mode** (`/plan`) - Decompose spec into task DAG
- **Execute Mode** (`/execute`) - Run tasks via isolated subagents

## Philosophy

The orchestrator does NOT:
- Track state itself (state.py does this)
- Validate artifacts (state.py does this)
- Compute ready tasks (state.py does this)

The orchestrator ONLY:
- Queries state via `scripts/state.py`
- Dispatches agents based on mode/phase
- Handles user interaction

---

# Plan Mode

Triggered by `/plan`. Runs phases 0-5.

## Plan Inputs

Ask user for:
1. **Specification** - Requirements document
2. **Target Directory** - Where code will be written
3. **Constraints** (optional) - Tech stack, architecture rules

## Plan Phase Dispatch

```bash
# Initialize if no state exists
if [ ! -f project-planning/state.json ]; then
    python3 scripts/state.py init "$TARGET_DIR"
fi

# Check current phase
python3 scripts/state.py status
```

| Phase | Agent | Output | Validation |
|-------|-------|--------|------------|
| `ingestion` | (none) | `inputs/spec.md` | File exists |
| `logical` | **logic-architect** | `artifacts/capability-map.json` | `state.py validate capability_map` |
| `physical` | **physical-architect** | `artifacts/physical-map.json` | `state.py validate physical_map` |
| `definition` | **task-author** | `tasks/*.json` | `state.py load-tasks` |
| `sequencing` | **plan-auditor** | Updated task waves | DAG is valid |
| `ready` | (done) | Planning complete | — |

## Plan Loop

```python
while phase not in ["ready", "executing", "complete"]:
    1. Query current phase
    2. Spawn appropriate agent
    3. Wait for agent to complete
    4. Validate output: state.py validate <artifact>
    5. If valid: state.py advance
    6. If invalid: Tell agent to fix, re-validate
```

## Plan Completion

When phase reaches "ready":
```markdown
## Planning Complete ✓

**Tasks:** 24
**Waves:** 4  
**Steel Thread:** T001 → T003 → T007 → T012

Run `/execute` to begin implementation.
```

---

# Execute Mode

Triggered by `/execute`. Runs phase 6.

## Execute Inputs

Ask user for (or detect from current directory):
1. **Planning Directory** - Where `project-planning/state.json` lives
2. **Target Directory** - Where code will be written

## Execute Prerequisites

```bash
# Verify planning is complete
python3 scripts/state.py status
# Phase must be: ready, executing, or have tasks

# Verify target directory
[ -d "$TARGET_DIR" ] || echo "Target directory not found"
```

## Execute Loop

```bash
while true; do
    # 1. Get ready tasks
    READY=$(python3 scripts/state.py ready-tasks)

    if [ -z "$READY" ]; then
        # Try to advance (might complete)
        python3 scripts/state.py advance
        if [ $? -eq 0 ]; then
            echo "All tasks complete!"
            break
        else
            echo "No ready tasks. Check for blockers."
            python3 scripts/state.py status
            break
        fi
    fi

    # 2. Select task (first ready, or user choice)
    TASK_ID=$(echo "$READY" | head -1 | cut -d: -f1)

    # 3. Generate execution bundle (BEFORE starting task)
    python3 scripts/bundle.py generate $TASK_ID

    # 4. Mark started
    python3 scripts/state.py start-task $TASK_ID

    # 5. Spawn isolated task-executor subagent with bundle
    # (See "Subagent Spawn" section below)

    # 6. Handle result
    if [ $SUCCESS ]; then
        python3 scripts/state.py complete-task $TASK_ID
    else
        python3 scripts/state.py fail-task $TASK_ID "$ERROR"
    fi

    # 7. Ask to continue (unless --batch mode)
    read -p "Continue? (y/n): " CONTINUE
done
```

## Subagent Spawn

Spawn task-executor with self-contained bundle:

```
Execute task [TASK_ID]

Bundle: project-planning/bundles/[TASK_ID]-bundle.json

The bundle contains everything you need:
- Task definition and acceptance criteria
- Expanded atom details (what to implement)
- File paths and purposes
- Target directory
- Constraints and patterns to follow
- Dependencies (files from prior tasks)

Instructions:
1. Read the bundle file - it has ALL context
2. Implement atoms in specified files
3. Run acceptance criteria verification
4. Report: files created, tests passed, any issues
```

The subagent:
- Has NO memory of previous tasks
- Gets full context budget
- Reads ONE file (the bundle) for complete context
- Tracks files for rollback
- Returns structured completion report

## Bundle Contents

The bundle (`project-planning/bundles/T001-bundle.json`) includes:

| Field | Purpose |
|-------|---------|
| `task_id`, `name` | Task identification |
| `target_dir` | Where to write code |
| `atoms` | Expanded atom details (not just IDs) |
| `files` | Paths, actions, purposes, layers |
| `acceptance_criteria` | Verification commands |
| `constraints` | Tech stack, patterns, testing |
| `dependencies.files` | Files from prior tasks to read |
| `context` | Domain, capability, spec reference |

Generate bundles with:
```bash
python3 scripts/bundle.py generate T001       # Single task
python3 scripts/bundle.py generate-ready      # All ready tasks
```

## Execute Options

| Command | Behavior |
|---------|----------|
| `/execute` | Interactive, one task at a time |
| `/execute T005` | Execute specific task only |
| `/execute --batch` | All ready tasks, no prompts |
| `/execute --parallel 3` | Up to 3 tasks simultaneously |

---

# State Commands Reference

```bash
# General
python3 scripts/state.py status          # Current phase, task counts
python3 scripts/state.py advance         # Try to advance phase

# Planning
python3 scripts/state.py init <dir>      # Initialize new plan
python3 scripts/state.py validate <art>  # Validate artifact

# Execution
python3 scripts/state.py ready-tasks     # List ready tasks
python3 scripts/state.py start-task <id> # Mark running
python3 scripts/state.py complete-task <id>  # Mark done
python3 scripts/state.py fail-task <id> <e>  # Mark failed
python3 scripts/state.py load-tasks      # Reload from files

# Bundles
python3 scripts/bundle.py generate <id>   # Generate bundle for task
python3 scripts/bundle.py generate-ready  # Generate all ready bundles
python3 scripts/bundle.py validate <id>   # Validate existing bundle
python3 scripts/bundle.py list            # List existing bundles
python3 scripts/bundle.py clean           # Remove all bundles

# Observability
python3 scripts/state.py log-tokens <s> <i> <o> <c>  # Log usage
```

---

# Error Recovery

## Planning Errors

If agent produces invalid output:
1. Validation fails (`state.py validate` returns non-zero)
2. Report errors to agent
3. Agent fixes and re-outputs
4. Re-validate
5. Only advance on success

## Execution Errors

If task fails:
1. `state.py fail-task` marks it failed
2. Rollback triggered (created files deleted, modified files restored)
3. Dependent tasks auto-blocked
4. Other ready tasks can continue
5. User can retry later: fix issue, then `state.py start-task` again
