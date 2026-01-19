# Tasker: Spec-Driven Development System

A multi-agent architecture for transforming specifications into implemented code through three integrated workflows: `/specify`, `/plan`, and `/execute`.

## System Philosophy

Tasker implements **spec-driven development**: specifications are the source of truth, and all implementation flows from verified, structured requirements. The system prioritizes:

- **Traceability**: Every line of code traces back to a spec requirement
- **Verification**: LLM-as-judge evaluation at planning and execution stages
- **Observability**: Persistent state enables crash recovery and progress monitoring
- **Isolation**: Subagents run in isolated contexts with explicit inputs only

---

## Three Workflows

```
/specify → /plan → /execute
```

| Workflow | Purpose | Output |
|----------|---------|--------|
| `/specify` | Interactive requirements discovery and structured synthesis | Spec doc, capability map, FSM diagrams, ADRs |
| `/plan` | Decompose specification into executable task DAG | Task files with dependencies and acceptance criteria |
| `/execute` | Implement tasks via isolated subagents with verification | Working code committed to git |

---

## Workflow 1: `/specify`

**Purpose**: Transform vague requirements into structured, machine-readable specifications.

### Phase Progression

```
Scope → Clarify → Synthesis → Architecture → Decisions → Gate → Review → Export
```

### Key Phases

**Phase 2: Clarify (Ralph Loop)**
- Exhaustive requirements gathering via structured questioning
- Categories: core requirements, users, integrations, edge cases, quality attributes, patterns, constraints
- Loop continues until all categories complete (no iteration cap)

**Phase 3: Synthesis**
- Extract **two artifacts** from discovery:
  1. **Spec sections** (human-readable): Workflows, Invariants, Interfaces
  2. **Capability Map** (machine-readable): Domains → Capabilities → Behaviors

**Part A.5: Behavior Model (FSM) Compilation**
- Compile finite state machine from workflows
- Identify steel thread (primary end-to-end flow)
- Validate completeness invariants (I1-I5)

**Phase 8: Export**
```
{TARGET}/docs/
├── specs/<slug>.md                    # Human-readable spec
├── specs/<slug>.capabilities.json     # Machine-readable capability map
├── fsm/<slug>/                        # Behavior model
│   ├── index.json
│   ├── steel-thread.states.json
│   ├── steel-thread.transitions.json
│   └── steel-thread.mmd               # Mermaid diagram
└── adrs/ADR-####-<slug>.md            # Decision records
```

### I.P.S.O. Behavior Taxonomy

Every behavior is classified into one of four types:

| Type | Examples |
|------|----------|
| **Input** | Validation, parsing, authentication |
| **Process** | Calculations, decisions, transformations |
| **State** | Database reads/writes, cache operations |
| **Output** | Responses, events, notifications |

---

## Workflow 2: `/plan`

**Purpose**: Decompose specification into a directed acyclic graph (DAG) of implementable tasks.

### Phase Progression

| Phase | Agent | Output |
|-------|-------|--------|
| 0: Ingestion | Orchestrator | `inputs/spec.md` |
| 1: Spec Review | spec-reviewer | `spec-review.json` |
| 2: Logical | logic-architect | `capability-map.json` |
| 3: Physical | physical-architect | `physical-map.json` |
| 4: Definition | task-author | `tasks/T###.json` |
| 5: Validation | task-plan-verifier | Validation report |
| 6: Sequencing | plan-auditor | Phase assignments |
| 7: Ready | Orchestrator | State = ready |

### Skip Optimization

If `/specify` artifacts exist in target project:
- Copy `capability-map.json` to planning artifacts
- Skip phases 1-2 (already complete)
- Start at physical phase

### Task Structure

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
    {"path": "src/auth/validator.py", "action": "create", "purpose": "..."}
  ],
  "dependencies": {
    "tasks": [],
    "external": ["pydantic>=2.0"]
  },
  "acceptance_criteria": [
    {
      "criterion": "Valid email format accepted",
      "verification": "pytest tests/test_validator.py::test_valid_email -v"
    }
  ],
  "state_machine": {
    "transitions_covered": ["TR1", "TR2"],
    "guards_enforced": ["I1"],
    "states_reached": ["S2", "S3"]
  }
}
```

### Validation Gates

**Programmatic (automatic):**
- ≥90% spec coverage by tasks
- No phase leakage (Phase 2+ content in Phase 1 tasks)
- All declared dependencies exist
- Acceptance criteria use valid verification commands

**LLM-as-Judge (task-plan-verifier):**
- Spec Alignment
- Phase Compliance
- Strategy Alignment
- Acceptance Criteria Quality

---

## Workflow 3: `/execute`

**Purpose**: Implement tasks via isolated subagents with verification after each task.

### Execution Loop

```
Initialize → Recovery Check → Loop {
  Get ready tasks
  Generate bundles
  Validate integrity
  Create checkpoint
  Spawn executors (max 3 parallel)
  Update state + commit
  Check halt signal
}
```

### Bundle Generation

Each task is packaged into a self-contained **execution bundle**:

```json
{
  "version": "1.3",
  "task_id": "T001",
  "name": "Implement credential validation",
  "target_dir": "/absolute/path/to/project",

  "behaviors": [
    {"id": "B1", "name": "validate_credentials", "type": "process"}
  ],

  "files": [
    {"path": "src/auth/validator.py", "action": "create", "behaviors": ["B1"]}
  ],

  "acceptance_criteria": [...],
  "constraints": {...},
  "checksums": {...},

  "state_machine": {
    "transitions_covered": ["TR1"],
    "transitions_detail": [
      {"id": "TR1", "from_state": "S1", "to_state": "S2", "trigger": "validate"}
    ]
  }
}
```

### Task Executor Protocol

The task-executor is **self-completing**: it updates state directly rather than relying on orchestrator acknowledgment.

1. Load bundle from `bundles/{task_id}-bundle.json`
2. Mark started via `tasker state start-task`
3. Track changes for rollback
4. Implement behaviors in specified files
5. Create documentation: `docs/{task_id}-spec.md` (mandatory)
6. Spawn task-verifier subagent
7. If all criteria pass:
   - Call `tasker state complete-task`
   - Commit to git
   - Write result file
8. If criteria fail:
   - Rollback files
   - Call `tasker state fail-task`

### Task Verifier

The task-verifier runs in an **isolated context** (clean from implementation details):

**Functional Correctness:**
- Run each acceptance criterion's verification command
- PASS / PARTIAL / FAIL per criterion

**Code Quality:**
| Dimension | Check |
|-----------|-------|
| Types | Annotations present and correct? |
| Docs | Docstrings present and accurate? |
| Patterns | Follows constraints.patterns? |
| Errors | Error handling appropriate? |

**FSM Adherence (if state_machine present):**
| Check | Requirement |
|-------|-------------|
| Transitions Implemented | All `transitions_covered` have code paths |
| Guards Enforced | All `guards_enforced` checked in code |
| States Reachable | All `states_reached` reachable from transitions |

**Verdict**: PASS / CONDITIONAL PASS / FAIL
**Recommendation**: PROCEED / BLOCK

---

## State Management

### Single Source of Truth

All workflow state is managed through the `tasker` CLI with persistence to `project-planning/state.json`.

### State Structure

```json
{
  "version": "2.0",
  "phase": {
    "current": "executing",
    "completed": ["ingestion", "logical", "physical", "definition", "validation", "sequencing", "ready"]
  },
  "target_dir": "/path/to/project",

  "tasks": {
    "T001": {
      "status": "complete",
      "phase": 1,
      "depends_on": [],
      "blocks": ["T003"],
      "verification": {"verdict": "PASS"}
    }
  },

  "checkpoint": {
    "batch": ["T001", "T002"],
    "status": {"T001": "success", "T002": "running"}
  }
}
```

### Task Status Transitions

```
pending → ready → running → complete
                     ├─→ failed
                     └─→ blocked
```

### Key Commands

```bash
# Planning
tasker state init <target_dir>
tasker state advance
tasker state load-tasks

# Execution
tasker state ready-tasks
tasker state start-task T001
tasker state complete-task T001 --files src/auth/validator.py
tasker state fail-task T001 "Test failed" --category test

# Recovery
tasker state checkpoint create T001 T002
tasker state checkpoint recover
tasker state retry-task T001
```

---

## FSM Behavior Modeling

### Purpose

The finite state machine (FSM) serves two purposes:
1. **QA during implementation**: Shapes acceptance criteria; verification confirms coverage
2. **Documentation**: Human-readable diagrams for ongoing system understanding

### FSM Levels

| Level | Scope | Trigger |
|-------|-------|---------|
| Steel Thread | End-to-end workflow | Always (mandatory) |
| Domain FSM | Subsystem flow | >12 states or cross-boundary |
| Entity FSM | Object lifecycle | Entity has state invariants |

### Completeness Invariants (I1-I5)

| ID | Invariant |
|----|-----------|
| I1 | Steel thread FSM exists and is primary_machine |
| I2 | Behavior-first (no architecture dependencies) |
| I3 | Complete (initial state, terminals, no dead ends) |
| I4 | Guard-invariant linkage (every guard has invariant_id) |
| I5 | No silent ambiguity (resolved during compilation) |

### Artifact Structure

```
docs/state-machines/<slug>/
├── index.json                    # Machine list, hierarchy
├── steel-thread.states.json      # S1, S2, ... with types/invariants
├── steel-thread.transitions.json # TR1, TR2, ... with guards/behaviors
├── steel-thread.mmd              # Mermaid stateDiagram-v2
└── steel-thread.notes.md         # Ambiguity resolutions
```

### Integration Flow

```
/specify compiles FSM
    ↓
/plan loads FSM → validates coverage → adds state_machine to tasks
    ↓
/execute bundles FSM context → verifier checks adherence
```

---

## Agent Architecture

### Planning Agents

| Agent | Responsibility |
|-------|----------------|
| **spec-reviewer** | Weakness detection (W1-W7), user engagement |
| **logic-architect** | I.P.S.O. decomposition, capability extraction |
| **physical-architect** | File path mapping, layer assignment |
| **task-author** | Task definition, behavior grouping |
| **task-plan-verifier** | LLM-as-judge task evaluation |
| **plan-auditor** | Phase assignment, DAG validation |

### Execution Agents

| Agent | Responsibility |
|-------|----------------|
| **task-executor** | Isolated implementation, self-completion |
| **task-verifier** | Clean verification, quality assessment |

### Isolation Principle

Each subagent runs in **isolated context**:
- Full context budget for the task
- Bundle is the ONLY input needed
- No inter-agent memory
- Prevents information leakage

---

## Directory Structure

```
tasker/
├── .claude/
│   ├── agents/                # Agent definitions
│   │   ├── task-executor.md
│   │   ├── task-verifier.md
│   │   └── ...
│   └── skills/
│       ├── specify/SKILL.md   # /specify workflow
│       └── orchestrator/SKILL.md  # /plan + /execute
│
├── schemas/                   # JSON validation schemas
│   ├── capability-map.schema.json
│   ├── task.schema.json
│   ├── execution-bundle.schema.json
│   ├── fsm-*.schema.json
│   └── ...
│
├── go/                        # Go CLI implementation
│   ├── cmd/tasker/            # Main entry point
│   └── internal/              # State, bundle, validation, FSM commands
│
├── scripts/                   # Python shims (forward to Go CLI)
│   ├── state.py               # Shim → tasker state
│   ├── bundle.py              # Shim → tasker bundle
│   └── validate.py            # Shim → tasker validate
│
└── project-planning/          # Generated during workflow
    ├── inputs/spec.md
    ├── artifacts/
    │   ├── capability-map.json
    │   ├── physical-map.json
    │   └── fsm/
    ├── tasks/T###.json
    ├── bundles/
    │   ├── T###-bundle.json
    │   └── T###-result.json
    └── state.json
```

---

## Key Design Decisions

### 1. Self-Completing Executors

Executors update state directly rather than relying on orchestrator acknowledgment. This enables:
- Graceful recovery after crashes
- Minimal orchestrator context usage
- Detailed observability via result files

### 2. Checkpoint-Based Recovery

```bash
# Before spawning batch
tasker state checkpoint create T001 T002 T003

# On crash and restart
tasker state checkpoint recover  # Finds orphaned tasks
```

### 3. Phase Filtering

Specs can contain multiple development phases. Only Phase 1 content is extracted during `/plan`:
- Prevents scope creep
- Enables phased development
- Future phases documented but excluded

### 4. Steel Thread First

The primary end-to-end workflow is executed early (Phase 2 tasks):
- Validates architecture before building features
- Marked with `steel_thread: true`
- Forms contiguous path through task DAG

### 5. Content-Based Spec References

Tasks link to spec via quoted text rather than section numbers:

```json
{
  "spec_ref": {
    "quote": "Users must be able to log in with email and password",
    "location": "paragraph 3"
  }
}
```

This survives spec reformatting and enables traceability verification.

### 6. Executable Acceptance Criteria

Every acceptance criterion includes a verification command:

```json
{
  "criterion": "Valid email format accepted",
  "verification": "pytest tests/test_validator.py::test_valid_email -v"
}
```

Commands must return exit code 0 on success.

### 7. FSM Canonical Contract

**FSM JSON is canonical; Mermaid is generated. `/plan` and `/execute` must fail if required transitions and invariants lack coverage evidence.**

- Canonical artifacts: `*.states.json`, `*.transitions.json`, `index.json`
- Derived artifacts: `*.mmd` (Mermaid diagrams) - generated ONLY from canonical JSON
- NEVER manually edit `.mmd` files
- Steel-thread transitions require 100% task coverage
- Each transition/invariant must have verifiable evidence (test preferred)

---

## Failure Recovery

### Planning Failures

| Failure | Recovery |
|---------|----------|
| Artifact validation fails | Agent re-runs and fixes output |
| task-plan-verifier blocks | User fixes task files, re-runs validation |

### Execution Failures

| Failure | Recovery |
|---------|----------|
| Acceptance criteria fail | Automatic rollback, task marked failed |
| Executor crashes | Checkpoint recovery on restart |
| Verifier returns BLOCK | Automatic rollback, user can retry |

```bash
# Retry a failed task
tasker state retry-task T001

# Skip a blocked task
tasker state skip-task T001 "reason"

# Manual halt
tasker state halt "reason"

# Resume after halt
tasker state resume
```

---

## Observability

### State File

`project-planning/state.json` is the single source of truth:
- Persisted after every significant operation
- Enables resumability after crashes
- Human-readable JSON

### Activity Logging

```bash
./scripts/log-activity.sh INFO task-executor "Starting T001"
```

### Result Files

Each task produces `bundles/{task_id}-result.json`:

```json
{
  "task_id": "T001",
  "status": "success",
  "files": {
    "created": ["src/auth/validator.py"],
    "modified": ["README.md"]
  },
  "verification": {
    "verdict": "PASS",
    "criteria": [...]
  },
  "git": {
    "commit_sha": "abc123"
  }
}
```

### Dashboard

```bash
tasker tui                 # Interactive TUI
tasker state status        # CLI status
```

---

## Quick Start

### From Scratch (Full Workflow)

```bash
# 1. Develop specification interactively
/specify

# 2. Decompose into tasks
/plan

# 3. Execute tasks
/execute
```

### With Existing Spec

```bash
# 1. Place spec in target project
cp my-spec.md ~/projects/myapp/docs/specs/myapp.md

# 2. Plan (reads spec from target)
/plan

# 3. Execute
/execute
```

### Resume After Interruption

```bash
# Check current state
/status

# Resume execution
/execute
```

---

## Schema Reference

| Schema | Purpose |
|--------|---------|
| `capability-map.schema.json` | Domains, capabilities, behaviors, flows |
| `physical-map.schema.json` | Behavior → file path mappings |
| `task.schema.json` | Individual task definitions |
| `execution-bundle.schema.json` | Self-contained execution context |
| `state.schema.json` | Workflow state |
| `fsm-index.schema.json` | FSM machine index |
| `fsm-states.schema.json` | FSM state definitions |
| `fsm-transitions.schema.json` | FSM transition definitions |
| `task-result.schema.json` | Execution result artifact |

All schemas use JSON Schema draft-07 with consistent ID prefixes:
- `D` for domains
- `C` for capabilities
- `B` for behaviors
- `T` for tasks
- `F` for flows
- `S` for states
- `TR` for transitions
- `M` for machines
