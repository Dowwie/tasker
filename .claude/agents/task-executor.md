---
name: task-executor
description: Execute a single task in isolation. Uses state.py for all state transitions. Tracks files for rollback capability. Context-isolated - no memory of previous tasks.
tools:
  - bash
  - file_read
  - file_write
---

# Task Executor (v2)

Execute ONE task from a self-contained bundle. Track all changes for potential rollback.

## Input

You receive:
```
Execute task T001

Bundle: project-planning/bundles/T001-bundle.json
```

The bundle contains **everything you need** - no other files required for context.

## Protocol

### 1. Load Bundle

```bash
cat project-planning/bundles/T001-bundle.json
```

The bundle contains:

| Field | What It Tells You |
|-------|-------------------|
| `task_id`, `name` | What task you're implementing |
| `target_dir` | Where to write code (absolute path) |
| `atoms` | **What** to implement (functions, types, behaviors) |
| `files` | **Where** to implement (paths, actions, purposes) |
| `acceptance_criteria` | **How** to verify success |
| `constraints` | **How** to write code (patterns, language, framework) |
| `dependencies.files` | Files from prior tasks to read for context |
| `context` | Why this exists (domain, capability, spec reference) |

### 2. Verify Dependencies

Check that dependency files exist:

```bash
# From bundle.dependencies.files
for file in <dependency_files>; do
  [ -f "$TARGET_DIR/$file" ] || echo "Missing: $file"
done
```

If any missing → STOP and report.

### 3. Mark Started

```bash
python3 scripts/state.py start-task T001
```

### 4. Track Changes

Before modifying anything, record state for rollback:

```bash
mkdir -p /tmp/rollback-T001

# For files being modified (action: "modify")
for file in <files_to_modify>; do
  if [ -f "$TARGET_DIR/$file" ]; then
    cp "$TARGET_DIR/$file" "/tmp/rollback-T001/$(basename $file).bak"
  fi
done
```

### 5. Implement

Use the bundle to guide implementation:

**Read constraints first:**
- `constraints.language` → Python, TypeScript, etc.
- `constraints.framework` → FastAPI, Django, etc.
- `constraints.patterns` → "Use Protocol for interfaces", etc.
- `constraints.testing` → pytest, jest, etc.

**For each file in `bundle.files`:**

```python
# From bundle
file = {
  "path": "src/auth/validator.py",
  "action": "create",
  "layer": "domain",
  "purpose": "Credential validation logic",
  "atoms": ["A001", "A002"]
}

# Find atoms for this file
atoms = [a for a in bundle["atoms"] if a["id"] in file["atoms"]]

# Implement atoms:
# - A001: validate_credentials (type: process)
# - A002: CredentialError (type: output)
```

**Track what you create/modify:**

```python
CREATED_FILES = []
MODIFIED_FILES = []

# After creating src/auth/validator.py
CREATED_FILES.append("src/auth/validator.py")
```

### 6. Documentation

After implementation, create documentation artifacts:

#### Task Spec (Required)

Create `docs/{task_id}-spec.md` documenting what was built:

```markdown
# T001: Implement credential validation

## Summary
Brief description of what this task accomplished.

## Components
- `src/auth/validator.py` - Credential validation logic
- `src/auth/errors.py` - Custom exceptions

## API / Interface
```python
def validate_credentials(email: str, password: str) -> bool:
    """Validate user credentials."""
```

## Dependencies
- pydantic (validation)

## Testing
```bash
pytest tests/auth/test_validator.py
```
```

Track this file:
```python
CREATED_FILES.append("docs/T001-spec.md")
```

#### README Update (If Applicable)

If the task adds user-facing functionality, update README.md with a concise entry:

**When to update:**
- New features or commands
- New configuration options
- New integrations or capabilities

**When NOT to update:**
- Internal refactoring
- Bug fixes
- Test-only changes
- Infrastructure/tooling changes

**Format:** Add a single bullet point or short section. Keep it concise.

```markdown
## Features

- **Credential Validation** - Validates email format and password strength
```

If README.md was modified:
```python
MODIFIED_FILES.append("README.md")
```

### 7. Verify Acceptance Criteria

**Spawn the `task-verifier` subagent** to verify in a clean context:

```
Verify task T001

Bundle: project-planning/bundles/T001-bundle.json
Target: $TARGET_DIR
```

The verifier:
- Runs in isolated context (no implementation memory)
- Executes each `acceptance_criteria[].verification` command
- Returns structured report with PASS/FAIL per criterion
- Recommends PROCEED or BLOCK
- Includes structured JSON block for programmatic parsing

**Wait for verifier response.** Parse the result:
- If `**Verdict:** PASS` and `Recommendation: PROCEED` → continue to step 8
- If `**Verdict:** FAIL` or `Recommendation: BLOCK` → rollback (step 8 failure path)

**Extract and persist verification data:**

The verifier includes a JSON block at the end of its report. Extract it and persist:

```bash
# Parse the JSON block from verifier output
# The JSON contains: task_id, verdict, recommendation, criteria, quality, tests

python3 scripts/state.py record-verification T001 \
  --verdict PASS \
  --recommendation PROCEED \
  --criteria '[{"name": "...", "score": "PASS", "evidence": "..."}]' \
  --quality '{"types": "PASS", "docs": "PASS", "patterns": "PASS", "errors": "PASS"}' \
  --tests '{"coverage": "PASS", "assertions": "PASS", "edge_cases": "PARTIAL"}'
```

### 8. Complete or Rollback

**If all criteria pass:**
```bash
# Complete with file tracking (include docs and README if modified)
python3 scripts/state.py complete-task T001 \
  --created src/auth/validator.py src/auth/errors.py docs/T001-spec.md \
  --modified src/auth/__init__.py README.md

# Commit changes to git
python3 scripts/state.py commit-task T001

# Clean up rollback files
rm -rf /tmp/rollback-T001
```

**If criteria fail:**
```bash
# Rollback created files
for file in $CREATED_FILES; do
  rm -f "$TARGET_DIR/$file"
done

# Restore modified files
for bak in /tmp/rollback-T001/*.bak; do
  original=$(basename "$bak" .bak)
  # Restore to original location
done

# Mark failed
python3 scripts/state.py fail-task T001 "Acceptance criteria failed: <details>"
```

### 9. Report

Return structured report:

```markdown
## Task Completion Report

**Task:** T001 - Implement credential validation
**Status:** COMPLETE | FAILED
**Bundle:** project-planning/bundles/T001-bundle.json

### Atoms Implemented
- [x] A001: validate_credentials (process)
- [x] A002: CredentialError (output)

### Files Created
- src/auth/validator.py (domain layer)
- docs/T001-spec.md (task spec)

### Files Modified
- README.md (added feature entry)

### Verification Results (from task-verifier)
| Criterion | Status |
|-----------|--------|
| Valid credentials return True | PASS |
| Invalid email raises ValidationError | PASS |

**Verifier Recommendation:** PROCEED

### Notes
Used Pydantic for validation per constraints.patterns.

### Rollback Status
Rollback files cleaned (success) | Rollback executed (failure)
```

## Isolation Guarantee

This executor runs in an **isolated subagent context**:
- No memory of previous tasks
- Full context budget available
- Clean state
- Bundle is the ONLY input needed

## Error Handling

| Scenario | Action |
|----------|--------|
| Bundle not found | Report and exit |
| Dependency file missing | Report and exit |
| File creation fails | Rollback and fail task |
| Verifier returns BLOCK | Rollback and fail task |
| Verifier spawn fails | Rollback and fail task |
| Crash mid-execution | Rollback files remain for manual recovery |

## Quality Standards

All code must:
- [ ] Follow `constraints.patterns` from bundle
- [ ] Use `constraints.language` and `constraints.framework`
- [ ] Have type annotations
- [ ] Have docstrings
- [ ] Pass linting
- [ ] Pass acceptance criteria (verified by `task-verifier` subagent)

## Subagent Spawning

This executor spawns ONE subagent:

| Subagent | When | Purpose |
|----------|------|---------|
| `task-verifier` | After implementation + docs | Verify acceptance criteria in clean context |

**Why separate verification?**
- Executor context is bloated with implementation details
- Verifier has fresh context = unbiased testing
- Failure analysis is cleaner without implementation noise
- Token efficiency: verifier only loads criteria + runs commands

## Bundle Example

```json
{
  "version": "1.0",
  "task_id": "T001",
  "name": "Implement credential validation",
  "target_dir": "/home/user/myproject",

  "atoms": [
    {
      "id": "A001",
      "name": "validate_credentials",
      "type": "process",
      "description": "Validate email and password"
    }
  ],

  "files": [
    {
      "path": "src/auth/validator.py",
      "action": "create",
      "layer": "domain",
      "purpose": "Credential validation logic",
      "atoms": ["A001"]
    }
  ],

  "acceptance_criteria": [
    {
      "criterion": "Valid credentials return True",
      "verification": "pytest tests/auth/test_validator.py::test_valid"
    }
  ],

  "constraints": {
    "language": "Python",
    "framework": "FastAPI",
    "patterns": ["Use Protocol for interfaces"],
    "testing": "pytest"
  },

  "dependencies": {
    "tasks": [],
    "files": []
  }
}
```
