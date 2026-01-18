# Pre-Planning Spec Review Design

Addresses gap-prone transformation points discovered through tisk project gap analysis.

---

## Problem Statement

The current tasker workflow has transformation stages where requirements can be lost:

```
Spec → Capability Map → Physical Map → Tasks → Bundles → Execution
```

Gap analysis of the tisk project revealed these loss patterns:

| Root Cause | % of Gaps | Description |
|------------|-----------|-------------|
| App-layer satisficed | 25% | Agent implemented app-layer validation, skipped DB-layer constraints |
| DDL-as-documentation | 25% | Spec DDL treated as reference, not mandate |
| Cross-cutting not captured | 25% | Config, observability, startup tasks span components |
| Small details missed | 19% | Items that would be caught by verification commands |
| Actual under-developed specs | 6% | Truly ambiguous requirements |

**Key insight:** Most gaps were NOT caused by poor specs, but by the I.P.S.O. taxonomy failing to capture non-behavioral requirements (constraints, indexes, config).

---

## Solution: Two New Phases

### 1. Pre-Planning Spec Review (Phase 0)

Analyze spec for weakness categories BEFORE capability extraction. Persist weaknesses and engage user for resolution.

### 2. Post-Execution Compliance Check (Phase 7)

Compare full spec against implementation after all tasks complete. Catch requirements that weren't captured as acceptance criteria.

---

## Weakness Categories

Derived from tisk gap analysis:

### W1: Non-Behavioral Requirements

**Description:** Requirements framed as structure (DDL, schemas) rather than system behavior.

**Detection signals:**
- SQL DDL blocks (`CREATE TABLE`, `CREATE INDEX`, `CONSTRAINT`, `TRIGGER`)
- Schema definitions (JSON Schema, OpenAPI)
- Configuration file structures

**Example from tisk:**
```sql
constraint hook_run_unique unique (hook_id, event_id)
```
This is a mandate, not documentation.

**Resolution:** Convert to behavioral framing: "The system MUST reject duplicate (hook_id, event_id) combinations with an integrity error."

---

### W2: Implicit Requirements

**Description:** Requirements inferred from examples, DDL, or context but not explicitly stated.

**Detection signals:**
- DDL with constraints that have no prose equivalent
- Code examples without explanation
- "As shown above" references

**Example from tisk:**
The DDL showed `task_id NOT NULL` but prose said "Either task_id OR create_task (exactly one)" - the NOT NULL was implicit.

**Resolution:** Ask user to confirm implicit requirements are mandatory.

---

### W3: Cross-Cutting Concerns

**Description:** Requirements that span multiple components (config, observability, startup/shutdown).

**Detection signals:**
- Environment variable tables
- Configuration sections
- Observability/metrics/tracing requirements
- Startup sequence descriptions
- Shutdown/cleanup requirements

**Example from tisk:**
```
| TISK_HOOK_RETRY_MAX_ATTEMPTS | int | Max retry attempts |
```
This wasn't assigned to any specific task.

**Resolution:** Create explicit cross-cutting tasks or mark for dedicated bundling.

---

### W4: Missing Acceptance Criteria

**Description:** Requirements that can't be turned into testable verification commands.

**Detection signals:**
- Qualitative requirements ("must be fast", "should be secure")
- Requirements without measurable outcomes
- Conceptual descriptions without concrete behavior

**Example:**
"The system should handle errors gracefully" - no verification command possible.

**Resolution:** Ask user to provide measurable criteria or mark as non-functional requirement.

---

### W5: Fragmented Requirements

**Description:** Same requirement split across non-adjacent sections.

**Detection signals:**
- Forward/backward references ("see Section X")
- DDL in one section, invariants in another
- API contract separate from validation rules

**Example from tisk:**
Hook idempotency described in prose (Section 6), constraint in DDL (Section 14).

**Resolution:** Consolidate or create explicit cross-references in capability map.

---

### W6: Contradictions

**Description:** Conflicting statements within the spec.

**Detection signals:**
- Mutually exclusive requirements
- Default values that differ between sections
- Behavior described differently in different places

**Example from tisk:**
Section 11.1 listed `cancelled` as a turn status, Section 11.4 said "No cancellation: Once started, turns complete or fail."

**Resolution:** Ask user to clarify which statement is authoritative.

---

## Phase 0: Spec Review Workflow

### Inputs
- `{PLANNING_DIR}/inputs/spec.md` - Raw specification

### Outputs
- `{PLANNING_DIR}/artifacts/spec-review.json` - Detected weaknesses
- `{PLANNING_DIR}/artifacts/spec-resolutions.json` - User resolutions

### Agent: spec-reviewer

```
## Spec Reviewer

Analyze specification for weakness categories before capability extraction.

### Protocol

1. Load spec from {PLANNING_DIR}/inputs/spec.md

2. Detect weaknesses using scripts/spec-review.py:
   python3 scripts/spec-review.py analyze {PLANNING_DIR}/inputs/spec.md

3. Review detected weaknesses (the script outputs JSON)

4. For each weakness category with findings:
   - Persist to {PLANNING_DIR}/artifacts/spec-review.json
   - Engage user with AskUserQuestion tool

5. Record resolutions to {PLANNING_DIR}/artifacts/spec-resolutions.json

6. Advance to Phase 1 when all critical weaknesses resolved
```

### User Interaction Flow

For each detected weakness, use AskUserQuestion:

```python
# Example for W1: Non-Behavioral Requirement
{
  "question": "The spec contains DDL constraints that aren't stated as behavioral requirements. Should these be treated as mandatory implementation requirements?",
  "header": "DDL Mandate",
  "options": [
    {"label": "Yes, mandatory", "description": "DDL constraints MUST be implemented as DB-level constraints"},
    {"label": "App-layer OK", "description": "Application-layer validation is sufficient"},
    {"label": "Review each", "description": "I'll decide case-by-case"}
  ]
}
```

---

## Phase 7: Post-Execution Compliance Check

### Inputs
- `{PLANNING_DIR}/inputs/spec.md` - Original spec
- `{PLANNING_DIR}/artifacts/spec-resolutions.json` - User resolutions
- `{TARGET_DIR}` - Implemented code

### Outputs
- `{PLANNING_DIR}/reports/compliance-report.md` - Gap analysis

### Verification Categories

#### V1: Schema Compliance

Check that all DDL elements exist in the database:

```bash
# Extract constraints from spec DDL
python3 scripts/compliance-check.py schema --spec {SPEC} --db {DB_URL}
```

Verifies:
- Tables exist with correct columns
- Constraints exist (UNIQUE, CHECK, FK)
- Indexes exist (including HNSW, GIN)
- Triggers exist

#### V2: Configuration Compliance

Check that all spec'd environment variables are wired:

```bash
python3 scripts/compliance-check.py config --spec {SPEC} --settings {SETTINGS_FILE}
```

Verifies:
- Each spec'd env var has a Pydantic field
- Default values match spec
- Required vars are marked required

#### V3: API Compliance

Check that all spec'd endpoints exist:

```bash
python3 scripts/compliance-check.py api --spec {SPEC} --routes {ROUTES_DIR}
```

Verifies:
- Each endpoint exists
- HTTP methods match
- Request/response schemas align

#### V4: Observability Compliance

Check that all spec'd metrics/spans exist:

```bash
python3 scripts/compliance-check.py observability --spec {SPEC} --code {SRC_DIR}
```

Verifies:
- OTel spans created for spec'd operations
- Metrics registered with correct types
- Structured logging fields present

---

## Implementation Plan

### New Scripts

#### 1. `scripts/spec-review.py`

```python
"""
Spec Review - Detect weakness categories in specifications.

Usage:
    spec-review.py analyze <spec_path>     Detect weaknesses
    spec-review.py resolve <spec_path>     Interactive resolution
    spec-review.py report <spec_path>      Generate weakness report
"""
```

Key functions:
- `detect_ddl_blocks()` - Find SQL DDL sections
- `detect_config_tables()` - Find env var tables
- `detect_cross_references()` - Find fragmented requirements
- `detect_contradictions()` - Find conflicting statements
- `detect_implicit_requirements()` - Find DDL-only constraints

#### 2. `scripts/compliance-check.py`

```python
"""
Post-Execution Compliance Check - Compare spec to implementation.

Usage:
    compliance-check.py schema --spec <path> --db <url>
    compliance-check.py config --spec <path> --settings <path>
    compliance-check.py api --spec <path> --routes <path>
    compliance-check.py observability --spec <path> --code <path>
    compliance-check.py all --spec <path> --target <path>
"""
```

Key functions:
- `extract_ddl_elements()` - Parse DDL from spec
- `introspect_database()` - Get actual schema
- `compare_schemas()` - Diff expected vs actual
- `extract_config_requirements()` - Parse env var tables
- `scan_settings_class()` - Find Pydantic fields

### New Agent

#### `spec-reviewer.md`

See agent definition above. Responsibilities:
- Run spec-review.py analyze
- Present weaknesses to user via AskUserQuestion
- Record resolutions
- Gate Phase 1 until critical weaknesses resolved

### Schema Updates

#### `spec-review.schema.json`

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "SpecReview",
  "type": "object",
  "required": ["version", "spec_checksum", "weaknesses", "status"],
  "properties": {
    "version": {"const": "1.0"},
    "spec_checksum": {"type": "string"},
    "weaknesses": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "category", "severity", "location", "description"],
        "properties": {
          "id": {"type": "string", "pattern": "^W[0-9]+-[0-9]+$"},
          "category": {"enum": ["non_behavioral", "implicit", "cross_cutting", "missing_ac", "fragmented", "contradiction"]},
          "severity": {"enum": ["critical", "warning", "info"]},
          "location": {"type": "string"},
          "description": {"type": "string"},
          "spec_quote": {"type": "string"},
          "suggested_resolution": {"type": "string"}
        }
      }
    },
    "status": {"enum": ["pending", "in_review", "resolved"]}
  }
}
```

#### `spec-resolutions.schema.json`

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "SpecResolutions",
  "type": "object",
  "required": ["version", "resolutions"],
  "properties": {
    "version": {"const": "1.0"},
    "resolutions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["weakness_id", "resolution", "resolved_at"],
        "properties": {
          "weakness_id": {"type": "string"},
          "resolution": {"enum": ["mandatory", "optional", "defer", "clarified", "not_applicable"]},
          "user_response": {"type": "string"},
          "behavioral_reframe": {"type": "string"},
          "resolved_at": {"type": "string", "format": "date-time"}
        }
      }
    }
  }
}
```

### Orchestrator Updates

Update `SKILL.md` to include Phase 0:

```markdown
## Plan Phase Dispatch

| Phase | Agent | Output | Validation |
|-------|-------|--------|------------|
| `spec_review` | **spec-reviewer** | `artifacts/spec-review.json`, `artifacts/spec-resolutions.json` | All critical weaknesses resolved |
| `ingestion` | (none) | `inputs/spec.md` (verbatim) | File exists |
| `logical` | **logic-architect** | `artifacts/capability-map.json` | `state.py validate capability_map` |
...
```

### State Machine Update

Add new phase to state.py:

```python
PHASES = [
    "spec_review",   # NEW: Phase 0
    "ingestion",
    "logical",
    "physical",
    "definition",
    "validation",
    "sequencing",
    "ready",
    "executing",
    "complete"
]
```

---

## Integration with Existing Workflow

### Phase Transitions

```
spec_review → ingestion (when all critical weaknesses resolved)
ingestion → logical (unchanged)
...
sequencing → ready (unchanged)
ready → executing (unchanged)
executing → compliance_check (NEW: Phase 7)
compliance_check → complete (when all checks pass)
```

### Capability Map Enhancement

The logic-architect should receive spec resolutions:

```
## Context

PLANNING_DIR: {absolute path}
Spec Resolutions: {PLANNING_DIR}/artifacts/spec-resolutions.json

When creating capabilities, consider:
- Resolutions marked "mandatory" MUST become explicit behaviors
- Non-behavioral requirements should be tagged for explicit tasks
- Cross-cutting concerns should be flagged for dedicated bundling
```

### Task Author Enhancement

The task-author should handle non-behavioral requirements:

```json
{
  "id": "T099",
  "name": "Add database constraints",
  "task_type": "constraint",
  "context": {
    "spec_ref": {"quote": "constraint hook_run_unique unique (hook_id, event_id)"},
    "requirement_type": "non_behavioral"
  },
  "acceptance_criteria": [
    {
      "criterion": "Unique constraint exists on hook_run(hook_id, event_id)",
      "verification": "psql -c \"SELECT constraint_name FROM information_schema.table_constraints WHERE table_name='hook_run' AND constraint_type='UNIQUE'\" | grep -q hook_run"
    }
  ]
}
```

---

## Severity Classification

### Critical (blocks Phase 1)
- Contradictions (W6)
- Non-behavioral requirements without resolution (W1)

### Warning (proceed with notes)
- Implicit requirements (W2)
- Cross-cutting concerns (W3)
- Fragmented requirements (W5)

### Info (logged only)
- Missing acceptance criteria (W4) - handled during task verification

---

## Example Workflow

### 1. User runs `/plan`

### 2. Orchestrator initializes directories, prompts for spec

### 3. Spec stored to `inputs/spec.md`

### 4. Phase 0: Spec Review

```bash
python3 scripts/spec-review.py analyze $PLANNING_DIR/inputs/spec.md
```

Output:
```json
{
  "version": "1.0",
  "weaknesses": [
    {
      "id": "W1-001",
      "category": "non_behavioral",
      "severity": "critical",
      "location": "lines 1280-1281",
      "description": "DDL constraint not stated as behavioral requirement",
      "spec_quote": "constraint hook_run_unique unique (hook_id, event_id)",
      "suggested_resolution": "Reframe as: 'The system MUST reject duplicate (hook_id, event_id) combinations'"
    },
    {
      "id": "W3-001",
      "category": "cross_cutting",
      "severity": "warning",
      "location": "Section 10",
      "description": "Environment variable table not assigned to component",
      "spec_quote": "| TISK_HOOK_RETRY_MAX_ATTEMPTS | int | Max retry attempts |"
    }
  ]
}
```

### 5. Agent uses AskUserQuestion for critical weaknesses

### 6. User responds, resolutions recorded

### 7. Phase advances to ingestion when resolved

### 8. Normal planning flow continues

### 9. After execution, Phase 7 runs compliance check

```bash
python3 scripts/compliance-check.py all --spec $PLANNING_DIR/inputs/spec.md --target $TARGET_DIR
```

### 10. Gaps reported, user decides action

---

## Success Metrics

After implementing this design:

1. **Pre-planning detection rate**: % of weaknesses detected before capability extraction
2. **False positive rate**: Weaknesses flagged that weren't actually problems
3. **Post-execution gap count**: Gaps found by compliance check (should decrease over time)
4. **User resolution time**: Time spent resolving weaknesses (should be < 5 min for typical specs)

---

## Migration Path

1. Implement `scripts/spec-review.py` with detection logic
2. Add `spec-reviewer.md` agent
3. Update orchestrator SKILL.md with Phase 0
4. Update state.py with new phase
5. Implement `scripts/compliance-check.py`
6. Add Phase 7 compliance check to orchestrator
7. Test with tisk spec (known gaps)
8. Iterate on detection heuristics

---

## Open Questions

1. **Should compliance check be blocking?** If gaps found, require user acknowledgment?
2. **How to handle legacy specs?** Specs without clear DDL sections?
3. **Should resolutions persist across sessions?** For iterative spec development?
