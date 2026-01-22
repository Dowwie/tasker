---
name: execute
description: EXECUTION PHASE - Run tasks via isolated subagents. Requires completed task DAG from plan phase.
tools:
  - agent
  - bash
  - file_read
  - file_write
---

# Execute Workflow

**CRITICAL: This is the EXECUTE skill. Only use this after /plan has completed and task files exist in .tasker/tasks/. If no tasks exist, tell the user to run /tasker:plan first.**

Execute a task DAG via isolated subagents. This is Phase 3 of the tasker workflow:

```
/specify → /plan → /execute
```

## Input Requirements

- **Task DAG** from `/plan`: `.tasker/tasks/T001.json`, `T002.json`, etc.
- **State file**: `.tasker/state.json` with phase = "ready" or "executing"

## Output

- **Working implementation** in target directory
- **Task results**: `.tasker/bundles/T001-result.json`, etc.
- **Evaluation report**: `.tasker/reports/evaluation-report.txt`

---

## MANDATORY FIRST STEP: Ask for Target Project Directory

**ALWAYS ask for target_dir FIRST before anything else.** No guessing, no inference from CWD.

Use AskUserQuestion to ask:
```
What is the target project directory?
```
Free-form text input. User must provide an absolute or relative path.

After target_dir is confirmed:
```bash
TARGET_DIR="<user-provided-path>"
# Convert to absolute path
TARGET_DIR=$(cd "$TARGET_DIR" 2>/dev/null && pwd)

TASKER_DIR="$TARGET_DIR/.tasker"

# Verify .tasker/ exists and has state
if [ ! -f "$TASKER_DIR/state.json" ]; then
    echo "Error: No tasker session found at $TASKER_DIR"
    echo "Run /plan first to create a task plan."
    exit 1
fi
```

---

## Execute Prerequisites

```bash
# Verify planning is complete
tasker state status
# Phase must be: ready, executing, or have tasks
```

---

## Git Repository Initialization (MANDATORY)

**Before any implementation begins**, check if the target repository has git initialized. If not, initialize it:

```bash
./scripts/ensure-git.sh "$TARGET_DIR"
```

**Why this is required:**
- Enables automatic commit hooks to track changes per task
- Provides audit trail of all implementation changes
- Required for the post-task-commit hook to function

---

## Recovery on Start (CRITICAL)

**Before starting the execute loop**, always check for and recover from a previous crash:

```bash
# Check for existing checkpoint from previous run
tasker state checkpoint status

# If checkpoint exists and is active, recover
tasker state checkpoint recover

# This will:
# 1. Find tasks that completed (have result files) but weren't acknowledged
# 2. Identify orphaned tasks (still "running" with no result file)
# 3. Update checkpoint state accordingly
```

If orphaned tasks are found, ask user:
```markdown
Found 3 orphaned tasks from previous run:
- T019, T011, T006

Options:
1. Retry orphaned tasks (reset to pending)
2. Skip orphaned tasks (mark failed)
3. Abort and investigate
```

To retry orphaned tasks:
```bash
tasker state task retry T019
tasker state task retry T011
tasker state task retry T006
tasker state checkpoint clear
```

---

## Execute Loop

**CRITICAL CONSTRAINTS:**
- **Max 3 parallel executors** - More causes orchestrator context exhaustion
- **Task-executors are self-completing** - They call `tasker state task complete` directly
- **Checkpoint before spawning** - Track batch for crash recovery
- **Minimal returns** - Executors return only `T001: SUCCESS` or `T001: FAILED - reason`

```bash
PARALLEL_LIMIT=3

while true; do
    # 0. CHECK FOR HALT
    tasker state check-halt
    if [ $? -ne 0 ]; then
        echo "Halt requested. Stopping gracefully."
        tasker state checkpoint complete

        # Generate evaluation report even on halt
        tasker evaluate --output $TASKER_DIR/reports/evaluation-report.txt
        tasker evaluate --format json --output $TASKER_DIR/reports/evaluation-report.json

        tasker state confirm-halt
        break
    fi

    # 1. Get ready tasks
    READY=$(tasker state ready)

    if [ -z "$READY" ]; then
        tasker state advance
        if [ $? -eq 0 ]; then
            echo "All tasks complete!"
            tasker state checkpoint clear

            # Generate evaluation report (MANDATORY)
            tasker evaluate --output $TASKER_DIR/reports/evaluation-report.txt
            tasker evaluate --format json --output $TASKER_DIR/reports/evaluation-report.json

            break
        else
            echo "No ready tasks. Check for blockers."
            tasker state status
            break
        fi
    fi

    # 2. Select batch (up to PARALLEL_LIMIT tasks)
    BATCH=$(echo "$READY" | head -$PARALLEL_LIMIT | cut -d: -f1)
    BATCH_ARRAY=($BATCH)
    echo "Batch: ${BATCH_ARRAY[@]}"

    # 3. Generate and validate bundles for all tasks in batch
    VALID_TASKS=()
    for TASK_ID in ${BATCH_ARRAY[@]}; do
        tasker bundle generate $TASK_ID
        tasker bundle validate-integrity $TASK_ID
        INTEGRITY_CODE=$?

        if [ $INTEGRITY_CODE -eq 0 ]; then
            VALID_TASKS+=($TASK_ID)
        elif [ $INTEGRITY_CODE -eq 2 ]; then
            # WARNING: Artifacts changed since bundle creation - regenerate
            ./scripts/log-activity.sh WARN orchestrator validation "$TASK_ID: Bundle drift detected, regenerating"
            tasker bundle generate $TASK_ID
            VALID_TASKS+=($TASK_ID)
        else
            # CRITICAL: Missing dependencies or validation failure
            ./scripts/log-activity.sh ERROR orchestrator validation "$TASK_ID: Bundle integrity FAILED"
            tasker state task fail $TASK_ID "Bundle integrity validation failed" --category dependency
        fi
    done

    # Update batch to only include valid tasks
    if [ ${#VALID_TASKS[@]} -eq 0 ]; then
        ./scripts/log-activity.sh ERROR orchestrator batch "No valid tasks in batch"
        continue
    fi
    BATCH_ARRAY=(${VALID_TASKS[@]})

    # 4. CREATE CHECKPOINT before spawning (crash recovery)
    tasker state checkpoint create ${BATCH_ARRAY[@]}

    # 5. Mark all tasks as started
    for TASK_ID in ${BATCH_ARRAY[@]}; do
        tasker state task start $TASK_ID
    done

    # 6. SPAWN EXECUTORS IN PARALLEL
    # Use Task tool with multiple invocations in single message
    # Each executor gets: TASKER_DIR and Bundle path
    # Each executor returns: "T001: SUCCESS" or "T001: FAILED - reason"

    # 7. AS EACH EXECUTOR RETURNS, update checkpoint
    for TASK_ID in ${BATCH_ARRAY[@]}; do
        if [ -f "$TASKER_DIR/bundles/${TASK_ID}-result.json" ]; then
            STATUS=$(tasker bundle result-info "$TASKER_DIR/bundles/${TASK_ID}-result.json" | grep "^status:" | cut -d: -f2)
            tasker state checkpoint update $TASK_ID $STATUS
            ./scripts/log-activity.sh INFO orchestrator task-result "$TASK_ID: $STATUS"
        else
            ./scripts/log-activity.sh WARN orchestrator task-result "$TASK_ID: no result file"
        fi
    done

    # 8. COMPLETE CHECKPOINT for this batch
    tasker state checkpoint complete

    # 9. Check for halt AFTER batch
    tasker state check-halt
    if [ $? -ne 0 ]; then
        echo "Halt requested after batch. Stopping gracefully."
        tasker evaluate --output $TASKER_DIR/reports/evaluation-report.txt
        tasker state confirm-halt
        break
    fi

    # 10. Continue to next batch
done
```

---

## Post-Execution Commit (Defense in Depth)

Task file commits are handled **automatically** by a Claude Code hook, ensuring commits happen regardless of executor behavior.

### Hook Configuration

Configured in `.claude/settings.local.json`:
```json
"PostToolUse": [
  {
    "matcher": "Task",
    "hooks": [
      {
        "type": "command",
        "command": ".claude/hooks/post-task-commit.sh",
        "timeout": 30
      }
    ]
  }
]
```

---

## Subagent Spawn Template

Spawn task-executor with self-contained bundle. **Executors are self-completing** - they update state and write results directly, returning only a minimal status line.

```
Execute task [TASK_ID]

## Logging (MANDATORY)

```bash
./scripts/log-activity.sh INFO task-executor start "Executing task [TASK_ID]"
./scripts/log-activity.sh INFO task-executor decision "What decision and why"
./scripts/log-activity.sh INFO task-executor complete "Outcome description"
```

TASKER_DIR: {absolute path to .tasker directory}
Bundle: {TASKER_DIR}/bundles/[TASK_ID]-bundle.json

The bundle contains everything you need:
- Task definition and acceptance criteria
- Expanded behavior details (what to implement)
- File paths and purposes
- Target directory
- Constraints and patterns to follow
- Dependencies (files from prior tasks)

## Self-Completion Protocol (CRITICAL)

You are responsible for updating state and persisting results. Do NOT rely on the orchestrator.

### On Success:
1. Track all files you created/modified
2. Call: `tasker state task complete [TASK_ID] --created file1 file2 --modified file3`
3. Write result file: `{TASKER_DIR}/bundles/[TASK_ID]-result.json`
4. Return ONLY this line: `[TASK_ID]: SUCCESS`

### On Failure:
1. Call: `tasker state task fail [TASK_ID] "error message" --category <cat> --retryable`
2. Write result file with error details
3. Return ONLY this line: `[TASK_ID]: FAILED - <one-line reason>`

### Result File Schema
Write to `{TASKER_DIR}/bundles/[TASK_ID]-result.json`:
```json
{
  "version": "1.0",
  "task_id": "[TASK_ID]",
  "name": "Task name from bundle",
  "status": "success|failed",
  "started_at": "ISO timestamp",
  "completed_at": "ISO timestamp",
  "files": {
    "created": ["path1", "path2"],
    "modified": ["path3"]
  },
  "verification": {
    "verdict": "PASS|FAIL",
    "criteria": [
      {"name": "criterion", "status": "PASS|FAIL", "evidence": "..."}
    ]
  },
  "error": {
    "category": "dependency|compilation|test|validation|runtime",
    "message": "...",
    "retryable": true
  },
  "notes": "Any decisions or observations"
}
```

## Workflow Summary
1. Read the bundle file - it has ALL context
2. Implement behaviors in specified files
3. Run acceptance criteria verification
4. Call tasker state to update task status
5. Write detailed result to bundles/[TASK_ID]-result.json
6. Return ONE LINE status to orchestrator

IMPORTANT: Use the TASKER_DIR absolute path provided. Do NOT use relative paths.
```

---

## Bundle Contents

The bundle (`{TASKER_DIR}/bundles/T001-bundle.json`) includes:

| Field | Purpose |
|-------|---------|
| `task_id`, `name` | Task identification |
| `target_dir` | Where to write code |
| `behaviors` | Expanded behavior details (not just IDs) |
| `files` | Paths, actions, purposes, layers |
| `acceptance_criteria` | Verification commands |
| `constraints` | Tech stack, patterns, testing |
| `dependencies.files` | Files from prior tasks to read |
| `context` | Domain, capability, spec reference |
| `state_machine` | FSM context for adherence verification (if present) |

Generate bundles with:
```bash
tasker bundle generate T001       # Single task
tasker bundle generate-ready      # All ready tasks
```

---

## Execute Options

| Command | Behavior |
|---------|----------|
| `/execute` | Interactive, one task at a time |
| `/execute T005` | Execute specific task only |
| `/execute --batch` | All ready tasks, no prompts |
| `/execute --parallel 3` | Up to 3 tasks simultaneously |

---

## Graceful Halt and Resume

The executor supports graceful halt via two mechanisms:

### 1. STOP File (Recommended for External Control)

Create a `STOP` file in the `.tasker/` directory:

```bash
touch .tasker/STOP
```

The executor checks for this file before starting each new task and after completing each task. When detected:
1. Current task (if running) completes normally
2. No new tasks are started
3. State is saved with halt information
4. Clean exit with instructions to resume

### 2. User Message (For Interactive Sessions)

If a user sends "STOP" during an interactive `/execute` session, the orchestrator should:
1. Call `tasker state halt user_message`
2. Allow current task to complete
3. Exit gracefully

### Resuming Execution

To resume after a halt:

```bash
# Check current halt status
tasker state halt-status

# Clear halt and resume
tasker state resume

# Then run /execute again
```

---

## FSM Coverage Report (After Execution Completes)

After all tasks complete, generate the execution coverage report:

```bash
tasker fsm validate execute-coverage-report \
    {TASKER_DIR}/artifacts/fsm/index.json \
    {TASKER_DIR}/bundles \
    --output {TASKER_DIR}/artifacts/fsm-coverage.execute.json
```

This report includes:
- Which transitions were verified during execution
- Evidence type for each transition (test, runtime_assertion, manual)
- Which invariants were enforced
- Pointers to tasks and acceptance criteria that provide evidence

---

## Evaluation Report (After Completion)

**MANDATORY**: After all tasks complete (or execution halts), generate the evaluation report:

```bash
tasker evaluate --output {TASKER_DIR}/reports/evaluation-report.txt
tasker evaluate --format json --output {TASKER_DIR}/reports/evaluation-report.json
```

Display the report to the user. The report includes:
- Planning quality (verdict from task-plan-verifier)
- Execution summary (completed/failed/blocked/skipped counts)
- First-attempt success rate (measures spec quality)
- Verification breakdown (criteria pass/partial/fail, code quality)
- Cost analysis (total tokens, total cost, per-task cost)
- Failure analysis (which tasks failed and why)
- Improvement patterns (common issues for process improvement)

For metrics-only summary:
```bash
tasker evaluate --metrics-only
```

---

## Archive Execution Artifacts (After Completion)

After execution completes (all tasks done or halted), archive execution artifacts:

```bash
tasker archive execution {project_name}
```

This creates:
```
archive/{project_name}/execution/{timestamp}/
├── bundles/        # Task bundles and result files
├── logs/           # Activity logs
├── state.json      # State snapshot
└── archive-manifest.json
```

---

## State Commands Reference

```bash
# Execution
tasker state ready               # List ready tasks
tasker state task start <id>     # Mark running
tasker state task complete <id>  # Mark done
tasker state task fail <id> <e>  # Mark failed
tasker state load-tasks          # Reload from files

# Halt / Resume
tasker state halt [reason]       # Request graceful halt
tasker state check-halt          # Check if halted (exit 1 = halted)
tasker state confirm-halt        # Confirm halt completed
tasker state halt-status         # Show halt status
tasker state resume              # Clear halt, resume execution

# Checkpoint (Crash Recovery)
tasker state checkpoint create <t1> [t2 ...]  # Create batch checkpoint
tasker state checkpoint update <id> <status>  # Update task
tasker state checkpoint complete              # Mark batch done
tasker state checkpoint status                # Show current checkpoint
tasker state checkpoint recover               # Recover orphaned tasks
tasker state checkpoint clear                 # Remove checkpoint

# Bundles
tasker bundle generate <id>      # Generate bundle for task
tasker bundle generate-ready     # Generate all ready bundles
tasker bundle validate <id>      # Validate bundle against schema
tasker bundle validate-integrity <id>  # Check deps + checksums
tasker bundle list               # List existing bundles
tasker bundle clean              # Remove all bundles
```

---

## Error Recovery

If task fails:
1. `tasker state task fail` marks it failed
2. Rollback triggered (created files deleted, modified files restored)
3. Dependent tasks auto-blocked
4. Other ready tasks can continue
5. User can retry later: fix issue, then `tasker state task retry` again
