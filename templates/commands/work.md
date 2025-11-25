# Work on Next Task

Execute the next ready task from the Task Decomposition Protocol v2 plan.

## Setup

**Planning Directory:** `{{PLANNING_DIR}}`
**Target Directory:** `{{TARGET_DIR}}`

## Execution Protocol

### 1. Check Status

```bash
python3 {{PLANNING_DIR}}/scripts/state.py status
python3 {{PLANNING_DIR}}/scripts/state.py ready-tasks
```

### 2. Select and Start Task

Pick the first ready task (or let user choose):

```bash
# Get ready tasks
READY=$(python3 {{PLANNING_DIR}}/scripts/state.py ready-tasks)

# Select first ready task
TASK_ID=$(echo "$READY" | head -1 | cut -d: -f1)

# Generate execution bundle
python3 {{PLANNING_DIR}}/scripts/bundle.py generate $TASK_ID

# Mark task started
python3 {{PLANNING_DIR}}/scripts/state.py start-task $TASK_ID
```

### 3. Load Bundle

Read the self-contained bundle:
```
{{PLANNING_DIR}}/project-planning/bundles/[TASK_ID]-bundle.json
```

The bundle contains everything needed:
- Task ID and name
- Atoms to implement (expanded details)
- Files to create/modify with purposes
- Acceptance criteria with verification commands
- Constraints (language, framework, patterns)
- Dependency files from prior tasks

### 4. Implement

Follow the bundle's specifications:
- Create/modify files as specified
- Follow constraints.patterns
- Implement each atom
- Track files created/modified for rollback

### 5. Verify

Run acceptance criteria verifications:
```bash
cd {{TARGET_DIR}}
# Run each verification command from bundle.acceptance_criteria
uv run ruff check .
uv run ty check src
uv run pytest
```

### 6. Complete or Fail

**If all criteria pass:**
```bash
python3 {{PLANNING_DIR}}/scripts/state.py complete-task $TASK_ID
```

**If criteria fail:**
```bash
python3 {{PLANNING_DIR}}/scripts/state.py fail-task $TASK_ID "error description"
```

**To retry a failed task:**
```bash
python3 {{PLANNING_DIR}}/scripts/state.py retry-task $TASK_ID
```

**To skip a blocked task:**
```bash
python3 {{PLANNING_DIR}}/scripts/state.py skip-task $TASK_ID "reason"
```

### 7. Report

Return completion report:
- Task ID and status
- Atoms implemented
- Files created/modified
- Verification results
- Notes and recommendations

## Completion Checklist

Before marking complete:
- [ ] All atoms from bundle implemented
- [ ] All acceptance criteria pass
- [ ] Linter passes
- [ ] Type checker passes
- [ ] Tests pass
- [ ] No regressions
