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

Create files in `project-planning/tasks/`:
```
project-planning/tasks/
├── T001.json
├── T002.json
├── T003.json
└── ...
```

Each file MUST validate against `schemas/task.schema.json`.

**After creating all tasks:**
```bash
python3 scripts/state.py load-tasks
```

## Why Individual Files?

1. **Parallelism**: Multiple agents can work on different tasks
2. **Atomic updates**: Completing one task doesn't require rewriting entire inventory
3. **Clear ownership**: Each file is self-contained
4. **Git-friendly**: Changes to one task don't conflict with others

## Input

Read: `project-planning/artifacts/physical-map.json`

## Task Structure

```json
{
  "id": "T001",
  "name": "Implement credential validation",
  "wave": 1,
  
  "context": {
    "domain": "Authentication",
    "capability": "User Login",
    "spec_ref": "Section 2.1",
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

## Wave Assignment

Initial wave assignment (plan-auditor will refine):
- Wave 1: No dependencies (foundations)
- Wave 2: Depends on wave 1 (steel thread)
- Wave 3+: Everything else

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
- [ ] Every behavior from physical-map has a task
- [ ] Every task is 2-6 hours
- [ ] Every task has ≤3 implementation files
- [ ] Every acceptance criterion has verification command
- [ ] Dependencies are explicit
- [ ] Individual JSON files created in `tasks/`
- [ ] Run: `python3 scripts/state.py load-tasks` to register
