# Tasker Performance Evaluation Specification

## Background and Context

### What is Tasker?

Tasker is a Claude Code-powered framework that transforms software specifications into executable task graphs. It implements a **Task Decomposition Protocol** that:

1. **Breaks specifications into behavioral atoms** (Input/Process/State/Output patterns)
2. **Maps atoms to physical files** (concrete code paths and artifacts)
3. **Sequences tasks into dependency-aware waves** (DAG structure)
4. **Executes tasks via isolated Claude Code subagents** (context-fresh execution)

### System Architecture

```
User Input (spec.md, constraints.md)
         ↓
    /plan command
         ↓
┌────────────────────────────────────────┐
│  Planning Phases (orchestrator skill)  │
├────────────────────────────────────────┤
│ Phase 1: logic-architect     → capability-map.json    │
│ Phase 2: physical-architect  → physical-map.json      │
│ Phase 3: task-author         → tasks/*.json           │
│ Phase 4: task-plan-verifier  → verification-report.md │
│ Phase 5: plan-auditor        → wave assignments       │
└────────────────────────────────────────┘
         ↓
    /execute command
         ↓
┌────────────────────────────────────────┐
│  Execution Loop                        │
├────────────────────────────────────────┤
│ For each ready task:                   │
│   1. bundle.py generates bundle        │
│   2. task-executor runs in isolation   │
│   3. task-verifier judges completion   │
│   4. state.py records result           │
└────────────────────────────────────────┘
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `scripts/state.py` | State manager | Single source of truth for all state transitions |
| `scripts/bundle.py` | Bundle generator | Creates self-contained execution contexts |
| `schemas/*.json` | JSON schemas | Validates all artifacts at phase boundaries |
| `.claude/agents/*.md` | Agent definitions | Specialized subagent prompts |
| `.claude/commands/*.md` | Slash commands | User-facing entry points |
| `project-planning/` | Working directory | Where artifacts and state live during execution |

### Current Verification Flow

Tasks are verified by an **LLM-as-judge** (`task-verifier`) that:
- Reads the implementation files
- Runs verification commands from acceptance criteria
- Evaluates against a rubric (functional correctness, code quality, test quality)
- Returns PASS/FAIL/CONDITIONAL verdict with evidence

**Problem**: This rich verification data is currently discarded after execution. We can't answer "How well did tasker perform?" because the evidence isn't persisted.

---

## Overview

This specification defines enhancements to capture, persist, and report performance metrics from tasker's multi-agent workflow. The goal is to answer: "How well did tasker perform this job?"

## Current State

### Existing Verification Infrastructure

| Component | Purpose | Output |
|-----------|---------|--------|
| `task-plan-verifier` | LLM-as-judge for task *definitions* (Phase 4) | Verdict (READY/READY_WITH_NOTES/BLOCKED), issues list, `verification-report.md` |
| `task-verifier` | LLM-as-judge for task *implementations* (Phase 6) | Verdict (PASS/FAIL/CONDITIONAL), per-criterion scores, evidence, recommendation |
| `state.json` | Execution state tracking | Task status, tokens, cost, file lists, events |

### Data Currently Captured

**In `state.json`:**
- Task status: `pending | ready | running | complete | failed | blocked`
- Timestamps: `started_at`, `completed_at`
- Files: `files_created`, `files_modified`
- Aggregates: `completed_count`, `failed_count`, `total_tokens`, `total_cost_usd`
- Events: append-only log with timestamps

**In `verification-report.md`:**
- Per-task verdicts at planning time
- Spec/strategy/preference/viability scores
- Blocking issues and recommendations

**In verifier output (not persisted):**
- Per-criterion functional scores
- Code quality assessment (types, docs, patterns, errors)
- Test quality assessment (coverage, assertions, edge cases)
- Evidence for each judgment

### Gap Analysis

| Missing Data | Current Location | Problem |
|--------------|------------------|---------|
| Per-task verifier verdict | Verifier output only | Lost after execution |
| Per-criterion scores | Verifier output only | Cannot analyze patterns |
| Attempt count | Not tracked | Cannot compute first-try success rate |
| Task duration | Derivable from events | Not explicit |
| Code quality breakdown | Verifier output only | Cannot track quality trends |
| Test quality breakdown | Verifier output only | Cannot identify weak spots |

---

## Requirements

### R1: Persist Verification Results

Store structured verification output in `state.json` for each task.

**Schema addition to task entry:**
```json
{
  "T001": {
    "id": "T001",
    "status": "complete",
    "wave": 1,
    "attempts": 1,
    "verification": {
      "verdict": "PASS",
      "criteria": [
        {
          "name": "Valid credentials return True",
          "score": "PASS",
          "evidence": "Function exists, test passes"
        }
      ],
      "quality": {
        "types": "PASS",
        "docs": "PASS",
        "patterns": "PASS",
        "errors": "PASS"
      },
      "tests": {
        "coverage": "PASS",
        "assertions": "PASS",
        "edge_cases": "PARTIAL"
      },
      "verified_at": "2025-01-15T10:15:00Z"
    }
  }
}
```

### R2: Track Attempts

Increment attempt counter on each `start-task` call. Reset on `retry-task`.

### R3: Compute Derived Metrics

From persisted data, compute:

| Metric | Formula |
|--------|---------|
| Task success rate | `completed / (completed + failed)` |
| First-attempt success rate | `tasks where attempts=1 and status=complete / completed` |
| Average attempts per task | `sum(attempts) / total_tasks` |
| Tokens per successful task | `total_tokens / completed_count` |
| Cost per successful task | `total_cost_usd / completed_count` |
| Quality pass rate | `tasks with all quality scores PASS / completed` |
| Functional pass rate | `criteria scored PASS / total criteria` |

### R4: Evaluation Report Command

Add `/evaluate` command that generates a summary report.

**Output format:**
```
Execution Evaluation Report
===========================

Planning Quality
----------------
Plan Verdict: READY_WITH_NOTES
Issues at Planning: 2

Execution Summary
-----------------
Tasks: 12 total
  Completed:     10 (83%)
  Failed:         2 (17%)

First-Attempt Success: 8/10 (80%)
Average Attempts: 1.2

Verification Breakdown
----------------------
Functional Criteria:
  PASS:     28/32 (88%)
  PARTIAL:   3/32 (9%)
  FAIL:      1/32 (3%)

Code Quality:
  Types:    10/10 PASS
  Docs:      9/10 PASS
  Patterns:  8/10 PASS
  Errors:   10/10 PASS

Test Quality:
  Coverage:    8/10 PASS
  Assertions:  9/10 PASS
  Edge Cases:  5/10 PASS (5 PARTIAL)

Cost Analysis
-------------
Total Tokens:  45,230
Total Cost:    $1.23
Per Task:      $0.12

Failure Analysis
----------------
T005: FAIL - Functional criterion "handles timeout" not implemented
T011: FAIL - Missing dependency on T003 output file

Improvement Patterns
--------------------
- 5 tasks had PARTIAL on edge case testing
- 2 tasks needed retries due to missing type annotations
```

---

## Implementation Plan

### Phase 1: State Schema Extension

1. Update `schemas/state.schema.json`:
   - Add `attempts` field to task schema
   - Add `verification` object to task schema
   - Add `duration_seconds` field

2. Update `scripts/state.py`:
   - Increment `attempts` in `start_task()`
   - Add `record_verification()` function
   - Add `compute_metrics()` function

### Phase 2: Verifier Integration

1. Update `task-executor.md`:
   - Parse structured output from task-verifier
   - Call `state.py record-verification` with parsed data

2. Update `task-verifier.md`:
   - Add structured JSON output block at end of report
   - Maintain human-readable markdown for context

**Structured output format from verifier:**
```json
{
  "task_id": "T001",
  "verdict": "PASS",
  "recommendation": "PROCEED",
  "criteria": [
    {"name": "...", "score": "PASS", "evidence": "..."}
  ],
  "quality": {"types": "PASS", "docs": "PASS", "patterns": "PASS", "errors": "PASS"},
  "tests": {"coverage": "PASS", "assertions": "PASS", "edge_cases": "PARTIAL"}
}
```

### Phase 3: Evaluation Command

1. Create `.claude/commands/evaluate.md`:
   - Read `state.json`
   - Compute all metrics
   - Format and display report

2. Add `scripts/evaluate.py` (optional):
   - CLI for metrics computation
   - JSON output for programmatic use
   - Integration with CI/CD

---

## State.py Command Additions

### `record-verification`

```bash
state.py record-verification <task_id> \
  --verdict PASS|FAIL|CONDITIONAL \
  --criteria '<json array>' \
  --quality '<json object>' \
  --tests '<json object>'
```

### `metrics`

```bash
state.py metrics [--format text|json]
```

Output:
```json
{
  "task_success_rate": 0.83,
  "first_attempt_success_rate": 0.80,
  "avg_attempts": 1.2,
  "tokens_per_task": 4523,
  "cost_per_task": 0.12,
  "quality_pass_rate": 0.90,
  "functional_pass_rate": 0.88,
  "test_edge_case_rate": 0.50
}
```

---

## Acceptance Criteria

1. **Verification persistence**: After task completion, `state.json` contains full verification data
2. **Attempt tracking**: Each task shows correct attempt count
3. **Metrics computation**: `state.py metrics` returns all defined metrics
4. **Evaluation report**: `/evaluate` command produces formatted report
5. **No regressions**: Existing tests pass, workflow unchanged
6. **Schema valid**: Updated state.json validates against updated schema

---

## Testing Strategy

### Unit Tests

- `test_record_verification()`: Verify data stored correctly
- `test_attempt_increment()`: Verify attempts tracked across retries
- `test_compute_metrics()`: Verify metric calculations
- `test_metrics_empty_state()`: Handle edge case of no completed tasks

### Integration Tests

- Run full workflow on golden spec
- Verify verification data persisted
- Verify metrics match expected values

### Golden Set

Create `tests/golden/evaluation/`:
- `state-after-execution.json`: Expected state after known execution
- `expected-metrics.json`: Expected metric values
- `expected-report.txt`: Expected evaluation report format

---

## Future Enhancements

### Spec Coverage Analysis

Compare completed tasks against original spec requirements:
- Map tasks back to spec sections via `context.spec_ref`
- Report coverage percentage
- Identify unimplemented requirements

### Trend Analysis

Track metrics across multiple runs:
- Historical success rates
- Cost trends
- Quality improvements

### Failure Pattern Detection

Analyze failures to identify systemic issues:
- Common criterion failures
- Dependency-related failures
- Quality dimension weak spots

### Verifier Calibration

Compare verifier verdicts against ground truth:
- False positive rate (PASS but broken)
- False negative rate (FAIL but working)
- Adjust verifier prompts based on findings
