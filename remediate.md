# Tasker Validation Remediation Plan

## Problem Statement

Planning and execution workflows fail to enforce spec adherence. Current validation relies on agent instructions (soft guidelines) rather than programmatic constraints (hard enforcement). This allows material deviation from specifications during execution.

## Additional Requirement: Refactor Priority

Refactoring work takes priority over existing designs. When a task involves refactoring:
- Refactor requirements override original spec constraints
- The refactor specification becomes the authoritative source
- Original design decisions may be intentionally superseded

---

## Remediation Strategy

Three-pronged approach:
1. **Schema Enforcement** - Make critical fields required, add constraints
2. **Programmatic Gates** - Add validation functions that block on failure
3. **Refactor-Aware Validation** - Handle refactor tasks as first-class citizens

---

## Phase 1: Schema Enforcement

### 1.1 Make spec_ref Required in Task Definitions

**File:** `schemas/task.schema.json`

**Change:**
```json
{
  "context": {
    "type": "object",
    "required": ["spec_ref"],
    "properties": {
      "spec_ref": {
        "oneOf": [
          {
            "type": "object",
            "required": ["quote", "location"],
            "properties": {
              "quote": { "type": "string", "minLength": 10 },
              "location": { "type": "string" }
            }
          },
          {
            "type": "object",
            "required": ["refactor_ref"],
            "properties": {
              "refactor_ref": { "type": "string", "minLength": 10 },
              "supersedes": {
                "type": "array",
                "items": { "type": "string" },
                "description": "Original spec sections this refactor overrides"
              }
            }
          }
        ]
      }
    }
  }
}
```

**Rationale:** Every task must trace to either original spec OR a refactor directive. Refactors explicitly declare what they supersede.

### 1.2 Add Acceptance Criteria Quality Constraints

**File:** `schemas/task.schema.json`

**Change:**
```json
{
  "acceptance_criteria": {
    "type": "array",
    "minItems": 1,
    "items": {
      "type": "object",
      "required": ["criterion", "verification"],
      "properties": {
        "criterion": {
          "type": "string",
          "minLength": 20,
          "pattern": "^(?!.*(works correctly|handles errors|is correct|functions properly)).*$"
        },
        "verification": {
          "type": "string",
          "minLength": 10,
          "pattern": "^(pytest|python|bash|grep|test|curl|npm|make|go)\\s+"
        }
      }
    }
  }
}
```

**Rationale:** Block vague criteria at schema level. Verification must be an executable command.

### 1.3 Add Refactor Task Type

**File:** `schemas/task.schema.json`

**Change:**
```json
{
  "task_type": {
    "type": "string",
    "enum": ["feature", "refactor", "bugfix", "test"],
    "default": "feature"
  },
  "refactor_context": {
    "type": "object",
    "properties": {
      "original_spec_sections": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Spec sections being refactored/superseded"
      },
      "refactor_directive": {
        "type": "string",
        "description": "The refactor requirement that takes priority"
      },
      "design_changes": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Explicit deviations from original design"
      }
    }
  }
}
```

**Rationale:** Refactors are first-class citizens with explicit override semantics.

---

## Phase 2: Programmatic Gates

### 2.1 Spec Coverage Gate

**File:** `scripts/validate.py`

**Add Function:**
```python
def validate_spec_coverage(state_dir: Path, threshold: float = 0.9) -> tuple[bool, dict]:
    """
    Block planning if spec coverage falls below threshold.

    Returns:
        (passed, report) where report contains:
        - coverage_ratio: float
        - covered_requirements: list
        - uncovered_requirements: list
        - refactor_overrides: list (requirements superseded by refactors)
    """
    spec_path = state_dir / "inputs" / "spec.md"
    tasks_dir = state_dir / "tasks"

    # Extract requirements from spec
    requirements = extract_requirements(spec_path)

    # Map tasks to requirements via spec_ref
    covered = set()
    refactor_overrides = set()

    for task_file in tasks_dir.glob("T*.json"):
        task = json.loads(task_file.read_text())
        context = task.get("context", {})
        spec_ref = context.get("spec_ref", {})

        if "refactor_ref" in spec_ref:
            # Refactor tasks override original requirements
            for superseded in spec_ref.get("supersedes", []):
                refactor_overrides.add(superseded)
        elif "quote" in spec_ref:
            # Match quote to requirements
            for req_id, req_text in requirements.items():
                if spec_ref["quote"] in req_text or req_text in spec_ref["quote"]:
                    covered.add(req_id)

    # Uncovered = requirements - covered - refactor_overrides
    uncovered = set(requirements.keys()) - covered - refactor_overrides

    coverage_ratio = (len(covered) + len(refactor_overrides)) / len(requirements) if requirements else 1.0

    return coverage_ratio >= threshold, {
        "coverage_ratio": coverage_ratio,
        "covered_requirements": list(covered),
        "uncovered_requirements": list(uncovered),
        "refactor_overrides": list(refactor_overrides),
        "threshold": threshold
    }
```

**Integration Point:** Call from task-plan-verifier before READY verdict.

### 2.2 Phase Leak Detection

**File:** `scripts/validate.py`

**Add Function:**
```python
def detect_phase_leakage(state_dir: Path) -> tuple[bool, list[dict]]:
    """
    Verify no Phase 2+ content leaked into Phase 1 tasks.

    Returns:
        (passed, violations) where violations contain:
        - task_id: str
        - behavior: str
        - leaked_phase: int
        - evidence: str
    """
    cap_map = json.loads((state_dir / "artifacts" / "capability-map.json").read_text())
    tasks_dir = state_dir / "tasks"

    # Build set of excluded content
    excluded_phases = cap_map.get("phase_filtering", {}).get("excluded_phases", [])
    excluded_content = set()
    for phase in excluded_phases:
        for summary in phase.get("summaries", []):
            excluded_content.add(summary.lower())

    violations = []
    for task_file in tasks_dir.glob("T*.json"):
        task = json.loads(task_file.read_text())

        # Skip refactor tasks - they may intentionally touch excluded content
        if task.get("task_type") == "refactor":
            continue

        for behavior in task.get("behaviors", []):
            behavior_lower = behavior.lower()
            for excluded in excluded_content:
                if excluded in behavior_lower or behavior_lower in excluded:
                    violations.append({
                        "task_id": task["id"],
                        "behavior": behavior,
                        "evidence": f"Matches excluded content: {excluded}"
                    })

    return len(violations) == 0, violations
```

**Integration Point:** Call from task-plan-verifier. BLOCK if violations found.

### 2.3 Dependency Existence Validation

**File:** `scripts/validate.py`

**Add Function:**
```python
def validate_dependency_existence(state_dir: Path) -> tuple[bool, list[dict]]:
    """
    Verify all declared dependencies reference existing tasks.
    """
    tasks_dir = state_dir / "tasks"
    task_ids = {f.stem.split("-")[0] for f in tasks_dir.glob("T*.json")}

    violations = []
    for task_file in tasks_dir.glob("T*.json"):
        task = json.loads(task_file.read_text())
        deps = task.get("dependencies", {}).get("tasks", [])

        for dep in deps:
            if dep not in task_ids:
                violations.append({
                    "task_id": task["id"],
                    "missing_dependency": dep
                })

    return len(violations) == 0, violations
```

### 2.4 Bundle Drift Detection

**File:** `scripts/bundle.py`

**Add Function:**
```python
def verify_bundle_integrity(bundle_path: Path) -> tuple[bool, list[str]]:
    """
    Re-verify bundle checksums before execution.
    Block if artifacts have drifted since bundle creation.
    """
    bundle = json.loads(bundle_path.read_text())
    checksums = bundle.get("checksums", {})

    drift_detected = []
    for artifact_path, expected_checksum in checksums.items():
        actual_checksum = compute_checksum(Path(artifact_path))
        if actual_checksum != expected_checksum:
            drift_detected.append(f"{artifact_path}: expected {expected_checksum[:8]}, got {actual_checksum[:8]}")

    return len(drift_detected) == 0, drift_detected
```

**Integration Point:** Call from task-executor before implementation begins. BLOCK if drift detected.

---

## Phase 3: Refactor-Aware Validation

### 3.1 Refactor Priority Resolution

**File:** `scripts/validate.py`

**Add Function:**
```python
def resolve_refactor_priority(state_dir: Path) -> dict:
    """
    Build authoritative requirement map with refactor overrides applied.

    Returns:
        {
            "effective_requirements": dict,  # After refactor overrides
            "original_requirements": dict,   # From spec.md
            "overrides": [                   # Refactor override log
                {"task_id": str, "supersedes": list, "directive": str}
            ]
        }
    """
    spec_path = state_dir / "inputs" / "spec.md"
    tasks_dir = state_dir / "tasks"

    original_reqs = extract_requirements(spec_path)
    effective_reqs = original_reqs.copy()
    overrides = []

    for task_file in tasks_dir.glob("T*.json"):
        task = json.loads(task_file.read_text())

        if task.get("task_type") == "refactor":
            refactor_ctx = task.get("refactor_context", {})
            superseded = refactor_ctx.get("original_spec_sections", [])
            directive = refactor_ctx.get("refactor_directive", "")

            # Remove superseded requirements from effective set
            for section in superseded:
                effective_reqs.pop(section, None)

            # Add refactor directive as new requirement
            effective_reqs[f"REFACTOR-{task['id']}"] = directive

            overrides.append({
                "task_id": task["id"],
                "supersedes": superseded,
                "directive": directive
            })

    return {
        "effective_requirements": effective_reqs,
        "original_requirements": original_reqs,
        "overrides": overrides
    }
```

### 3.2 Update task-plan-verifier Rubric

**File:** `skills/agents/task-plan-verifier.md`

**Add to Rubric:**
```markdown
## 7. Refactor Compliance (REQUIRED if refactor tasks exist)

For each task with `task_type: "refactor"`:

- [ ] `refactor_context.refactor_directive` clearly states the refactor goal
- [ ] `refactor_context.original_spec_sections` lists all superseded sections
- [ ] `refactor_context.design_changes` documents intentional deviations
- [ ] Acceptance criteria verify refactor goals, NOT original spec
- [ ] No other tasks depend on superseded requirements

Verdict:
- PASS: All refactor tasks have complete context, overrides are explicit
- FAIL: Missing refactor context, implicit overrides, or orphaned dependencies
```

### 3.3 Update task-verifier for Refactor Validation

**File:** `skills/agents/task-verifier.md`

**Add to Rubric:**
```markdown
## 5. Refactor Verification (REQUIRED for refactor tasks)

For tasks with `task_type: "refactor"`:

- [ ] Implementation achieves refactor directive (not original spec)
- [ ] Superseded code/design is actually replaced, not just added to
- [ ] Design changes documented in refactor_context are implemented
- [ ] No regression in functionality that wasn't explicitly changed

Evidence to gather:
- Git diff showing removed/replaced code
- Test coverage on refactored paths
- Confirmation that superseded patterns are eliminated
```

---

## Phase 4: Integration Points

### 4.1 Planning Pipeline Gates

```
Phase 1 (logic-architect)
    ↓
Phase 2 (physical-architect)
    ↓
Phase 3 (task-author)
    ↓
Phase 4 (task-plan-verifier)
    ├── NEW: validate_spec_coverage() → BLOCK if < 90%
    ├── NEW: detect_phase_leakage() → BLOCK if violations
    ├── NEW: validate_dependency_existence() → BLOCK if missing
    └── NEW: resolve_refactor_priority() → Build effective requirements
    ↓
Phase 5 (plan-auditor)
```

### 4.2 Execution Pipeline Gates

```
Bundle Generation
    ↓
NEW: verify_bundle_integrity() → BLOCK if drift detected
    ↓
Task Execution (task-executor)
    ↓
Task Verification (task-verifier)
    └── NEW: Refactor verification rubric
```

### 4.3 State Machine Updates

**File:** `schemas/state.schema.json`

Add validation tracking:
```json
{
  "validation_results": {
    "type": "object",
    "properties": {
      "spec_coverage": {
        "type": "object",
        "properties": {
          "ratio": { "type": "number" },
          "passed": { "type": "boolean" },
          "timestamp": { "type": "string" }
        }
      },
      "phase_leakage": {
        "type": "object",
        "properties": {
          "passed": { "type": "boolean" },
          "violations": { "type": "array" }
        }
      },
      "refactor_overrides": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "task_id": { "type": "string" },
            "supersedes": { "type": "array" }
          }
        }
      }
    }
  }
}
```

---

## Implementation Order

1. **Schema changes** (task.schema.json) - Foundation for enforcement
2. **validate.py functions** - Programmatic gates
3. **task-plan-verifier updates** - Pre-execution validation
4. **bundle.py integrity check** - Execution-time validation
5. **task-verifier updates** - Post-execution refactor validation
6. **state.schema.json updates** - Tracking and observability

---

## Success Criteria

After remediation:

- [ ] Every task has traceable spec_ref OR explicit refactor_ref
- [ ] Spec coverage gate blocks planning if < 90% coverage
- [ ] Phase leakage is detected and blocked programmatically
- [ ] Refactor tasks explicitly declare what they override
- [ ] Bundle drift is detected before execution
- [ ] Verifiers use effective requirements (with refactor overrides applied)
- [ ] No implicit spec deviation is possible

---

## Metrics

Track these to measure remediation effectiveness:

| Metric | Target | Measurement |
|--------|--------|-------------|
| Spec coverage at planning | ≥ 90% | `validate_spec_coverage()` |
| Phase leakage incidents | 0 | `detect_phase_leakage()` |
| Tasks without spec_ref | 0 | Schema validation failures |
| Bundle drift at execution | 0 | `verify_bundle_integrity()` |
| Refactor override clarity | 100% | Manual audit of refactor tasks |
