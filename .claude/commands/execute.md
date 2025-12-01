# Execute

Execute tasks from a completed plan using isolated subagents.

## Input Required

You will be asked to provide:
1. **Planning Directory** - Where `project-planning/state.json` lives
2. **Target Directory** - Where code will be written

If state.json exists in current directory's `project-planning/`, that's used by default.

## Prerequisites

Before executing:
- Planning must be complete (state.json phase = "ready" or "executing")
- Target directory must exist and be writable

```bash
# Verify planning is complete
python3 scripts/state.py status
# Should show: Phase: ready (or executing)
```

## What Happens

The executor runs phase 6:

```
1. Query ready tasks     → state.py ready-tasks
2. Select task           → First ready, or user choice
3. Generate bundle       → bundle.py generate T001
4. Mark started          → state.py start-task T001
5. Spawn subagent        → With self-contained bundle
6. Implement task        → Creates/modifies files in target
7. Verify acceptance     → Runs verification commands
8. Complete or rollback  → state.py complete-task / fail-task
9. Loop                  → Until all tasks done
```

### Execution Bundles

Each task gets a self-contained bundle (`project-planning/bundles/T001-bundle.json`) containing:
- Expanded behavior details (what to implement)
- File paths with purposes and layers
- Acceptance criteria
- Constraints (language, framework, patterns)
- Dependency files from prior tasks

## Execution Modes

### Sequential (Default)
```
> /execute
Executes one task at a time, asks to continue after each.
```

### Specific Task
```
> /execute T005
Executes only task T005 (must be ready - deps complete).
```

### Batch
```
> /execute --batch
Executes all ready tasks without prompting.
Stop on first failure.
```

## Subagent Isolation

Each task runs in a **fresh subagent context**:
- No memory of previous tasks
- Full context window available
- Clean failure isolation
- Token usage logged automatically

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Task fails | Rollback changes, mark failed, continue to other ready tasks |
| Dependency fails | Dependent tasks auto-blocked |
| All tasks blocked | Report status, suggest manual intervention |

## Commands During Execution

```bash
# See what's ready to run
python3 scripts/state.py ready-tasks

# Check overall status
python3 scripts/state.py status

# Bundle management
python3 scripts/bundle.py generate T005      # Generate bundle for task
python3 scripts/bundle.py generate-ready     # Generate all ready bundles
python3 scripts/bundle.py list               # List existing bundles
python3 scripts/bundle.py clean              # Remove all bundles

# Retry a failed task (after fixing issue)
python3 scripts/state.py retry-task T005

# Skip a blocked task
python3 scripts/state.py skip-task T005
```

## Output

After each task:
- Completion report (files created, tests passed)
- Token usage logged to state.json
- Progress updated

After all tasks:
- State advances to "complete"
- Summary of execution (tasks completed, failed, tokens used)

## Execute from Target Project

Alternatively, install the `/work` command in your target project:

```bash
python3 scripts/install_work_command.py /path/to/target/project
```

Then work from the target project:
```bash
cd /path/to/target/project
claude
> /work
```

This installs:
- A `/work` command configured for this planning directory
- Necessary permissions in settings.local.json
