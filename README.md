# Tasker

Tasker is a Claude Code-powered muti-agent planning and execution framework. It consists of two modes: Planning and Execution.  Planning mode transforms software specifications into executable DAGs of atomic, verifiable tasks. Execution mode executes the tasks created during planning in a manageable fashion.

## Overview

Tasker implements a **Task Decomposition Protocol** that:
- Breaks specifications into behaviors (Input/Process/State/Output)
- Maps behaviors to physical files and artifacts
- Sequences tasks into dependency-aware waves
- Executes tasks via isolated Claude Code subagents

## Quick Start

```bash
# Install dependencies
uv sync

# Create planning directories
mkdir -p project-planning/inputs

# Copy and customize templates
cp templates/spec.md.example project-planning/inputs/spec.md
cp templates/constraints.md.example project-planning/inputs/constraints.md

# Edit spec.md and constraints.md with your project details
# IMPORTANT: Set "Target Directory:" in spec.md

# In Claude Code, plan a project:
/plan

# Then execute the plan:
/execute
```

## Templates

The `templates/` directory contains starter files for defining your project:

| Template | Purpose |
|----------|---------|
| `spec.md.example` | Project specification (goals, architecture, APIs, data models, success criteria) |
| `constraints.md.example` | Technology stack, architecture rules, code standards, definition of done |
| `task.json.example` | Reference schema for task files (generated automatically during planning) |

### Template Flow

1. **Copy templates** to `project-planning/inputs/` and customize with your project details
2. **Run `/plan`** — the orchestrator reads your inputs and coordinates specialized agents:
   - Phase 1: `logic-architect` extracts capabilities/behaviors from spec
   - Phase 2: `physical-architect` maps behaviors to file paths
   - Phase 3: `task-author` creates individual task files
3. **Run `/execute`** — tasks are executed via isolated subagents

Templates themselves aren't processed programmatically—they're guidance for users. The system parses your customized files in `project-planning/inputs/` and validates generated artifacts against JSON schemas in `schemas/`.

## Commands

| Command | Description |
|---------|-------------|
| `/plan` | Decompose a spec into a task DAG (phases 0-6) |
| `/verify-plan` | Re-run task verification against spec & preferences |
| `/execute` | Run tasks via isolated subagents |
| `/execute T005` | Execute a specific task |
| `/execute --batch` | Execute all ready tasks without prompts |

## Planning Phases

```
Phase 0: Ingestion     → spec.md saved
Phase 1: Logical       → capability-map.json (what the system does)
Phase 2: Physical      → physical-map.json (where code lives)
Phase 3: Definition    → tasks/*.json (individual task files)
Phase 4: Validation    → tasks verified against spec & user preferences
Phase 5: Sequencing    → wave assignments, DAG validation
Phase 6: Ready         → planning complete, ready for execution
```

## Behaviors vs Tasks

Tasker decomposes work at two levels of abstraction:

### Behaviors (Behavioral Units)

A **behavior** is the smallest unit of *behavior* extracted from your specification. Behaviors follow the I.P.S.O. taxonomy:

| Type | Description | Example |
|------|-------------|---------|
| **Input** | Data entering the system | `ReceiveLoginRequest` |
| **Process** | Computation/transformation | `ValidateCredentials`, `HashPassword` |
| **State** | Data persistence/mutation | `StoreSession`, `UpdateLastLogin` |
| **Output** | Data leaving the system | `ReturnAuthToken`, `SendWelcomeEmail` |

Behaviors are abstract—they describe *what* the system does, not *where* the code lives. A typical project has 20-50 behaviors.

### Tasks (Execution Units)

A **task** is a concrete unit of *work* that groups related behaviors together for execution:

```json
{
  "id": "T001",
  "name": "Implement credential validation",
  "behaviors": ["B001", "B002"],
  "files": ["src/auth/validator.py", "tests/auth/test_validator.py"],
  "acceptance_criteria": [
    {"criterion": "Valid credentials return True", "verification": "pytest tests/auth/"}
  ],
  "dependencies": {"tasks": []}
}
```

Each task:
- Groups 2-5 related behaviors (the "sweet spot" for granularity)
- Maps to specific files to create/modify
- Has testable acceptance criteria
- Declares dependencies on other tasks
- Gets executed by an isolated subagent

### The Relationship

```
Specification → Behaviors → Tasks → Execution
                  (what)     (how)   (do it)
```

| Aspect | Behavior | Task |
|--------|----------|------|
| Abstraction | Logical (what to do) | Physical (where/how) |
| Granularity | Single behavior | Group of related behaviors |
| Typical count | 20-50 per project | 5-15 per project |
| Created in | Phase 1 (capability-map.json) | Phase 3 (tasks/*.json) |
| Contains | Description, type, domain | Behaviors, files, criteria, deps |

### Why This Matters

The **avg behaviors/task** metric in the dashboard indicates task granularity:
- **< 2 behaviors/task**: Tasks too granular, excessive overhead
- **2-5 behaviors/task**: Sweet spot—cohesive, verifiable units
- **> 5 behaviors/task**: Tasks too large, higher failure risk

## Project Structure

```
.claude/
├── agents/           # Specialized subagent definitions
│   ├── logic-architect.md
│   ├── physical-architect.md
│   ├── task-author.md
│   ├── task-plan-verifier.md
│   ├── plan-auditor.md
│   ├── task-executor.md
│   └── task-verifier.md
├── commands/         # Slash commands
│   ├── plan.md
│   ├── verify-plan.md
│   └── execute.md
├── skills/
│   └── orchestrator/ # Main orchestration skill
└── hooks/            # Event hooks

scripts/
├── state.py          # State management CLI
└── bundle.py         # Execution bundle generator

schemas/              # JSON Schema validation
├── capability-map.schema.json
├── physical-map.schema.json
├── task.schema.json
├── execution-bundle.schema.json
└── state.schema.json
```

## Workflow Verification

Tasker implements verification at multiple points in the workflow:

### Verification Checkpoints

| Phase | Checkpoint | What's Verified | Blocks Progress? |
|-------|------------|-----------------|------------------|
| 1-3 | Schema Validation | Artifacts match JSON schemas | Yes |
| 4 | Task Plan Verification | Tasks align with spec, strategy, preferences | Yes (if BLOCKED) |
| 5 | DAG Validation | No circular dependencies, valid wave ordering | Yes |
| 7 | Task Execution Verification | Implementation meets acceptance criteria | Yes (blocks dependents) |

### Planning-Time Verification (Phase 4)

Before any code is written, `task-plan-verifier` evaluates each task definition:

| Dimension | What's Checked |
|-----------|----------------|
| Spec Alignment | Does task trace to spec requirements? |
| Strategy Alignment | Does task fit the decomposition strategy? |
| Preference Compliance | Does task follow user's `~/.claude/CLAUDE.md` standards? |
| Viability | Is task properly scoped with clear acceptance criteria? |

**Verdicts**: `READY`, `READY_WITH_NOTES`, `BLOCKED`

### Execution-Time Verification (Phase 7)

After each task completes, `task-verifier` evaluates the implementation:

| Dimension | What's Judged |
|-----------|---------------|
| Functional Correctness | Does it meet each acceptance criterion? |
| Code Quality | Types, docs, patterns, error handling |
| Test Quality | Coverage, assertions, edge cases |

**Recommendations**: `PROCEED` or `BLOCK`

When `BLOCK` is recommended:
- Dependent tasks are automatically blocked
- Task cannot be marked complete until issues are resolved

### Verifier Calibration

Track verifier accuracy over time:

```bash
# Record whether verification was correct
python3 scripts/state.py record-calibration T001 correct
python3 scripts/state.py record-calibration T002 false_positive "Task actually failed"

# View calibration metrics
python3 scripts/state.py calibration-score
```

## Monitoring

### TUI Dashboard

Launch the real-time status dashboard:

```bash
/tui                           # Launch in tmux split
python3 scripts/dashboard.py   # Direct execution
```

**Dashboard sections**:
- **Planning Quality**: Task count, behaviors, avg behaviors/task, steel thread coverage
- **Execution Progress**: Progress bar, status breakdown, wave progress
- **Resource Usage**: Token count, cost
- **Verification**: Verified count, PROCEED/BLOCK breakdown
- **Verifier Calibration**: Accuracy score, false positive/negative counts

### CLI Status Commands

```bash
# Current state overview
python3 scripts/state.py status

# List ready tasks (considers verification recommendations)
python3 scripts/state.py ready-tasks

# Execution metrics
python3 scripts/state.py metrics

# Planning quality metrics
python3 scripts/state.py planning-metrics

# Verifier calibration
python3 scripts/state.py calibration-score
```

### Metrics Available

**Execution Metrics** (`state.py metrics`):
- Task success rate, first-attempt success rate
- Average attempts per task
- Tokens/cost per task
- Functional pass rate, quality pass rate

**Planning Metrics** (`state.py planning-metrics`):
- Total tasks and behaviors
- Avg behaviors/task (target: 2-5)
- Dependency density
- Wave count and compression ratio
- Steel thread coverage

## State Management

```bash
# Check current status
python3 scripts/state.py status

# List ready tasks
python3 scripts/state.py ready-tasks

# Register task validation result (during planning)
python3 scripts/state.py validate-tasks READY "All tasks aligned"

# Retry a failed task
python3 scripts/state.py retry-task T005

# Skip a blocked task
python3 scripts/state.py skip-task T005 "reason"

# Record verification result
python3 scripts/state.py record-verification T001 --verdict PASS --recommendation PROCEED

# Prepare rollback before task execution
python3 scripts/state.py prepare-rollback T001 src/file.py tests/test_file.py

# Verify rollback was successful
python3 scripts/state.py verify-rollback T001 --created src/new.py --modified src/file.py
```

## Bundle Generation

Each task gets a self-contained execution bundle with:
- Expanded behavior details
- File paths with purposes
- Acceptance criteria
- Constraints (language, framework, patterns)
- Dependency files from prior tasks
- Checksums for artifact and dependency validation

```bash
python3 scripts/bundle.py generate T001       # Generate for one task
python3 scripts/bundle.py generate-ready      # Generate all ready
python3 scripts/bundle.py validate T001       # Validate bundle against schema
python3 scripts/bundle.py list                # List existing bundles
```

### Bundle Validation

Bundles include SHA256 checksums of source artifacts. Before execution, validate that nothing has changed:

```python
from bundle import validate_bundle_checksums
valid, changed = validate_bundle_checksums("T001")
if not valid:
    print(f"Artifacts changed since bundle generation: {changed}")
```

## Development

```bash
make install    # Setup project
make lint       # Run ruff
make test       # Run pytest
make clean      # Remove artifacts
```

## How It Works

1. **Planning**: The orchestrator coordinates specialized agents through planning phases, producing schema-validated JSON artifacts

2. **Execution**: Tasks run in isolated subagent contexts with full context budgets, no memory of previous tasks, and automatic rollback on failure

3. **State**: All state transitions go through `scripts/state.py`, which tracks phases, task status, token usage, and events

4. **Verification**: Completed tasks are evaluated by an LLM-as-judge verifier before being marked complete

## Task Verification

Each completed task passes through an **LLM-as-judge** verifier — much richer than "tests passed/failed". The verifier reasons about whether the implementation meets the *intent*.

### Verification Rubric

**1. Evidence Gathering**
- Read implementation files
- Run verification commands
- Capture all output

**2. Multi-Dimensional Judgment**

| Dimension | What's Judged |
|-----------|---------------|
| Functional Correctness | Does it meet each criterion? |
| Code Quality | Types, docs, patterns, error handling |
| Test Quality | Coverage, assertions, edge cases |

**3. Verdicts**

| Verdict | Meaning |
|---------|---------|
| PASS | All criteria met, quality acceptable |
| CONDITIONAL PASS | Works, minor issues, proceed with notes |
| FAIL | Criteria not met or critical issues |

**4. Judgment Principles**

1. **Be objective** — Judge the code, not the approach
2. **Be specific** — Cite exact evidence for every judgment
3. **Be fair** — Partial credit for partial implementations
4. **Be helpful** — Provide actionable feedback on failures
5. **Be strict** — Functional criteria are non-negotiable
6. **Be reasonable** — Minor style issues don't block

**5. Failure Feedback**

When blocking, the verifier provides:
- **What failed** — Specific criterion
- **Why it failed** — Evidence from code/tests
- **How to fix** — Concrete suggestions
- **What to re-verify** — After fix

## License

See [LICENSE.md](LICENSE.md)
