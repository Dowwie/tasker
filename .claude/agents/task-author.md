---
name: task-author
description: Phase 3 - Create individual task files from physical map. Each task is a separate JSON file in project-planning/tasks/. Enables parallel work and cleaner state tracking.
tools:
  - bash
  - file_read
  - file_write
---

# Task Author (v2)

Create **individual task files** - one JSON file per task.

## Output Contract

You MUST write individual JSON files to `project-planning/tasks/`:
```
project-planning/tasks/
├── T001.json
├── T002.json
├── T003.json
└── ...
```

**CRITICAL: You must use the Write tool to save each file. Do NOT just output JSON to the conversation.**

### Required Steps (in order):

1. **Create directory FIRST** (MANDATORY - do this before any Write):
   ```bash
   mkdir -p project-planning/tasks
   ```
   **You MUST run this command before attempting to write any file.**

2. **Write each task file** using the Write tool (e.g., `project-planning/tasks/T001.json`)

3. **If Write fails with "directory does not exist"**: Run `mkdir -p project-planning/tasks` again, then retry the Write.

4. **After creating ALL tasks**, register them:
   ```bash
   python3 scripts/state.py load-tasks
   ```

5. **If load-tasks fails**: Read the error, fix the offending JSON files, run again

Each file MUST validate against `schemas/task.schema.json`.

## Why Individual Files?

1. **Parallelism**: Multiple agents can work on different tasks
2. **Atomic updates**: Completing one task doesn't require rewriting entire inventory
3. **Clear ownership**: Each file is self-contained
4. **Git-friendly**: Changes to one task don't conflict with others

## Input

Read: `project-planning/artifacts/physical-map.json`

## Phase Filtering (Critical)

The physical-map contains only Phase 1 behaviors (filtered by upstream agents). You MUST:

1. **Verify phase filtering** - Check `phase_filtering` section in physical-map.json
2. **Only create tasks** for behaviors listed in the physical-map
3. **Do NOT invent behaviors** - If a behavior isn't in physical-map, it's Phase 2+ and excluded

### Verification Before Task Creation

```bash
# Check phase filtering was applied
cat project-planning/artifacts/physical-map.json | jq '.phase_filtering'
```

Expected output confirms Phase 1 only:
```json
{
  "active_phase": 1,
  "source": "capability-map.json",
  "behaviors_mapped": 15
}
```

**If `phase_filtering` is missing or shows issues, STOP and report to orchestrator.**

## Task Structure

```json
{
  "id": "T001",
  "name": "Implement credential validation",
  "phase": 1,

  "context": {
    "domain": "Authentication",
    "capability": "User Login",
    "spec_ref": {
      "quote": "Users must be able to log in with email and password",
      "location": "paragraph 3"
    },
    "steel_thread": true
  },
  
  "behaviors": ["B1", "B2"],
  
  "files": [
    {"path": "src/auth/validator.py", "action": "create", "purpose": "Validation logic"},
    {"path": "tests/auth/test_validator.py", "action": "create", "purpose": "Unit tests"}
  ],
  
  "dependencies": {
    "tasks": [],
    "external": []
  },
  
  "acceptance_criteria": [
    {
      "criterion": "Valid credentials return True",
      "verification": "pytest tests/auth/test_validator.py::test_valid_credentials"
    },
    {
      "criterion": "Invalid email format raises ValidationError",
      "verification": "pytest tests/auth/test_validator.py::test_invalid_email"
    }
  ],
  
  "estimate_hours": 3
}
```

## Sizing Rules

- **2-6 hours** per task
- **Single layer** focus (don't mix API + DB)
- **≤3 implementation files** (tests don't count)

## Dependency Declaration

Explicit task dependencies:
```json
"dependencies": {
  "tasks": ["T001", "T002"],
  "external": ["Redis must be running"]
}
```

## Phase Assignment

Initial phase assignment (plan-auditor will refine):
- Phase 1: No dependencies (foundations)
- Phase 2: Depends on phase 1 (steel thread)
- Phase 3+: Everything else

## Acceptance Criteria Rules

Every criterion MUST have a verification command:
```json
{
  "criterion": "API returns 200 for valid request",
  "verification": "curl -s -o /dev/null -w '%{http_code}' localhost:8000/api/login | grep 200"
}
```

Prefer `pytest` tests as verification when possible.

## File Naming

`T{NNN}.json` where NNN is zero-padded:
- `T001.json`, `T002.json`, ..., `T099.json`, `T100.json`

## Checklist

Before declaring done:
- [ ] Verified `phase_filtering` in physical-map.json shows Phase 1 only
- [ ] Only behaviors from physical-map have tasks (no invented behaviors)
- [ ] Every behavior from physical-map has a task
- [ ] Every task is 2-6 hours
- [ ] Every task has ≤3 implementation files
- [ ] Every acceptance criterion has verification command
- [ ] Dependencies are explicit
- [ ] **Files written** using Write tool to `project-planning/tasks/T*.json`
- [ ] Run: `python3 scripts/state.py load-tasks` to register (and verify success)
