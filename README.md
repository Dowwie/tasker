# Tasker: Multi-Agent Planning and Execution Engine

Converts complex software specifications into executable tasks and implements them via isolated subagents.

## Overview

Tasker operates in two distinct modes:

1. **Planning Mode** (`/plan`) - Transforms a specification into a directed acyclic graph (DAG) of implementable tasks
2. **Execution Mode** (`/execute`) - Implements tasks via context-isolated subagents with verification

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PLANNING MODE (/plan)                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   spec.md ──▶ Logic Architect ──▶ Physical Architect ──▶ Task Author        │
│                     │                    │                   │               │
│                     ▼                    ▼                   ▼               │
│             capability-map.json   physical-map.json    tasks/T001.json      │
│                                                        tasks/T002.json      │
│                                                              │               │
│                                          ┌───────────────────┘               │
│                                          ▼                                   │
│                              Task-Plan-Verifier ──▶ Plan Auditor            │
│                                                          │                   │
│                                                          ▼                   │
│                                                   state.json (ready)        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                          EXECUTION MODE (/execute)                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   For each ready task:                                                       │
│                                                                              │
│   state.json ──▶ Bundle Generator ──▶ Task Executor (isolated subagent)     │
│                                              │                               │
│                                              ▼                               │
│                                       Implementation                         │
│                                              │                               │
│                                              ▼                               │
│                                      Task Verifier ──▶ state.json (updated) │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Install Dependencies

```bash
uv sync
```

### 2. Run Planning

```bash
/plan
```

The planner will ask you for:
1. **Specification** - Paste directly or provide a file path. Any format works (PRDs, bullet lists, meeting notes, etc.)
2. **Target Directory** - Where the code will be written
3. **Tech Stack** (optional) - Any constraints like "Python with FastAPI"

This triggers the planning pipeline:
1. Logic Architect extracts capabilities from spec
2. Physical Architect maps capabilities to files
3. Task Author creates task definitions
4. Task-Plan-Verifier validates task quality
5. Plan Auditor assigns execution phases and validates DAG

### 3. Review and Execute

```bash
/status              # View planning results
/verify-plan         # Run validation checks
/execute             # Begin implementation
/execute T005        # Execute a specific task
/execute --batch     # Execute all ready tasks without prompts
```

### 4. Monitor Progress

```bash
/tui                 # Launch interactive TUI dashboard
/status              # CLI status summary
```

---

## Architecture

### Directory Structure

```
tasker/
├── .claude/
│   ├── agents/                    # Subagent definitions
│   │   ├── logic-architect.md     # Phase 1: Spec → capabilities
│   │   ├── physical-architect.md  # Phase 2: Capabilities → files
│   │   ├── task-author.md         # Phase 3: Files → tasks
│   │   ├── task-plan-verifier.md  # Phase 4: Pre-execution validation
│   │   ├── plan-auditor.md        # Phase 4: DAG validation & sequencing
│   │   ├── task-executor.md       # Execution: Implement tasks
│   │   └── task-verifier.md       # Execution: Verify implementations
│   ├── commands/                  # Slash commands
│   │   ├── plan.md                # /plan - Enter planning mode
│   │   ├── execute.md             # /execute - Enter execution mode
│   │   ├── verify-plan.md         # /verify-plan - Validate planning
│   │   ├── status.md              # /status - Show dashboard
│   │   ├── tui.md                 # /tui - Launch TUI
│   │   └── evaluate.md            # /evaluate - Generate report
│   ├── hooks/                     # Event hooks
│   │   ├── launch-tui.sh          # Auto-launch TUI in tmux
│   │   ├── close-tui.sh           # Close TUI on completion
│   │   ├── detect-workflow.sh     # Detect /plan or /execute
│   │   └── subagent_stop.py       # Log token usage
│   └── skills/
│       └── orchestrator/
│           └── SKILL.md           # Main orchestrator skill
├── schemas/                       # JSON validation schemas
│   ├── capability-map.schema.json
│   ├── physical-map.schema.json
│   ├── task.schema.json
│   ├── execution-bundle.schema.json
│   └── state.schema.json
├── scripts/                       # Python utilities
│   ├── state.py                   # State management (single source of truth)
│   ├── bundle.py                  # Execution bundle generation
│   ├── validate.py                # DAG, steel thread, verification validation
│   ├── status.py                  # TUI launcher
│   ├── dashboard.py               # CLI dashboard
│   └── tui/                       # Textual TUI components
│       ├── app.py
│       ├── providers.py
│       ├── state_provider.py
│       └── views/
│           ├── dashboard.py
│           ├── task_detail.py
│           └── widgets.py
├── templates/                     # Example files (for reference only)
│   ├── example-spec.md            # Example specification format
│   ├── constraints.md.example     # Example constraints
│   ├── task.json.example          # Example task structure
│   └── README.md
└── project-planning/              # Generated during workflow (gitignored)
    ├── inputs/
    │   └── spec.md                # Your specification (stored verbatim)
    ├── artifacts/
    │   ├── capability-map.json    # Phase 1 output
    │   └── physical-map.json      # Phase 2 output
    ├── tasks/                     # Individual task definitions
    │   ├── T001.json
    │   ├── T002.json
    │   └── ...
    ├── bundles/                   # Execution bundles
    └── state.json                 # Workflow state
```

### State Machine

The workflow progresses through these phases:

```
ingestion → logical → physical → definition → validation → sequencing → ready → executing → complete
```

**Phase Progression:**

| Phase | Name | Output | Agent |
|-------|------|--------|-------|
| 0 | Ingestion | spec.md saved | Orchestrator |
| 1 | Logical | capability-map.json | logic-architect |
| 2 | Physical | physical-map.json | physical-architect |
| 3 | Definition | tasks/T*.json | task-author |
| 4 | Validation | Verification results | task-plan-verifier |
| 5 | Sequencing | Phase assignments, DAG | plan-auditor |
| 6 | Ready | Planning complete | Orchestrator |
| 7 | Executing | Implementation | task-executor + task-verifier |
| 8 | Complete | All tasks done | Orchestrator |

### Task Status Transitions

```
pending ──▶ ready ──▶ running ──▶ complete
                         │
                         ├──▶ failed
                         │
                         └──▶ blocked

skipped (manual override)
```

---

## Data Flow Detail

### Phase 1: Logical Architecture (logic-architect)

**Input:** `project-planning/inputs/spec.md`
**Output:** `project-planning/artifacts/capability-map.json`

The Logic Architect extracts the logical structure from your specification:

- **Domains** - Major functional areas (e.g., Authentication, Storage)
- **Capabilities** - Features within domains (e.g., User Login, Token Refresh)
- **Behaviors** - Atomic operations with types (Input/Process/State/Output)
- **Flows** - End-to-end user journeys that traverse behaviors

```json
{
  "version": "1.0",
  "spec_checksum": "abc123...",
  "domains": [{
    "id": "D1",
    "name": "Authentication",
    "description": "User identity and access management",
    "capabilities": [{
      "id": "C1",
      "name": "User Login",
      "spec_ref": {
        "quote": "Users must be able to log in with email and password",
        "location": "paragraph 3"
      },
      "behaviors": [
        {"id": "B1", "name": "validate_credentials", "type": "process", "description": "Verify email and password"},
        {"id": "B2", "name": "generate_token", "type": "output", "description": "Create JWT access token"}
      ]
    }]
  }],
  "flows": [{
    "id": "F1",
    "name": "Login Flow",
    "is_steel_thread": true,
    "steps": [
      {"order": 1, "behavior_id": "B1", "description": "Validate user credentials"},
      {"order": 2, "behavior_id": "B2", "description": "Generate and return token"}
    ]
  }],
  "coverage": {
    "total_requirements": 15,
    "covered_requirements": 15,
    "gaps": []
  }
}
```

**Behavior Types (I.P.S.O. Taxonomy):**

| Type | Description | Example |
|------|-------------|---------|
| **Input** | Data entering the system | `ReceiveLoginRequest` |
| **Process** | Computation/transformation | `ValidateCredentials`, `HashPassword` |
| **State** | Data persistence/mutation | `StoreSession`, `UpdateLastLogin` |
| **Output** | Data leaving the system | `ReturnAuthToken`, `SendWelcomeEmail` |

### Phase 2: Physical Architecture (physical-architect)

**Input:** `capability-map.json`, `constraints.md`
**Output:** `project-planning/artifacts/physical-map.json`

Maps behaviors to concrete file paths based on:
- Target directory structure
- Language/framework conventions (from constraints)
- Architectural layers (api, domain, data, infra, test)

```json
{
  "version": "1.0",
  "target_dir": "/path/to/project",
  "capability_map_checksum": "abc123...",
  "file_mapping": [{
    "behavior_id": "B1",
    "behavior_name": "validate_credentials",
    "files": [
      {"path": "src/auth/validator.py", "action": "create", "layer": "domain", "purpose": "Credential validation logic"}
    ],
    "tests": [
      {"path": "tests/auth/test_validator.py", "action": "create"}
    ]
  }],
  "cross_cutting": [{
    "concern": "logging",
    "files": [{"path": "src/utils/logging.py", "action": "create", "purpose": "Structured logging setup"}]
  }],
  "infrastructure": [
    {"path": "pyproject.toml", "action": "create", "purpose": "Project configuration"}
  ],
  "summary": {
    "total_behaviors": 12,
    "total_files": 24,
    "files_to_create": 22,
    "files_to_modify": 2
  }
}
```

**Architectural Layers:**

| Layer | Purpose | Example Files |
|-------|---------|---------------|
| api | HTTP/API handlers | `src/api/routes.py` |
| domain | Business logic | `src/auth/validator.py` |
| data | Data access/models | `src/models/user.py` |
| infra | Infrastructure | `src/config/settings.py` |
| test | Tests | `tests/auth/test_validator.py` |

### Phase 3: Task Definition (task-author)

**Input:** `capability-map.json`, `physical-map.json`
**Output:** `project-planning/tasks/T001.json`, `T002.json`, ...

Creates individual task files. Each task groups 2-5 related behaviors into a cohesive unit of work:

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
    {"path": "src/auth/validator.py", "action": "create", "purpose": "Credential validation logic"},
    {"path": "tests/auth/test_validator.py", "action": "create", "purpose": "Unit tests"}
  ],
  "dependencies": {
    "tasks": [],
    "external": ["pydantic>=2.0"]
  },
  "acceptance_criteria": [
    {"criterion": "Valid credentials return True", "verification": "pytest tests/auth/test_validator.py::test_valid -v"},
    {"criterion": "Invalid email raises ValidationError", "verification": "pytest tests/auth/test_validator.py::test_invalid_email -v"},
    {"criterion": "Code passes linting", "verification": "ruff check src/auth/validator.py"},
    {"criterion": "Code passes type checking", "verification": "ty check src/auth"}
  ],
  "estimate_hours": 3
}
```

### Phase 4: Validation (task-plan-verifier, plan-auditor)

**Input:** All artifacts + task files
**Output:** Updated `state.json` with validation results

**Task-Plan-Verifier** (LLM-as-judge) evaluates each task:

| Dimension | What's Checked |
|-----------|----------------|
| Spec Alignment | Does task trace to spec requirements? |
| Strategy Alignment | Does task fit the decomposition strategy? |
| Preference Compliance | Does task follow user's `~/.claude/CLAUDE.md` standards? |
| Viability | Is task properly scoped with clear acceptance criteria? |

**Verdicts:** `READY`, `READY_WITH_NOTES`, `BLOCKED`

**Plan-Auditor** then:
- Validates DAG has no cycles
- Checks steel thread forms contiguous early path
- Validates all verification commands are syntactically valid
- Assigns execution **phases** based on dependencies:
  - Phase 1: Tasks with no dependencies
  - Phase 2: Tasks depending only on Phase 1
  - And so on...

---

## State Management

`state.json` is the **single source of truth** for all workflow state:

```json
{
  "version": "2.0",
  "phase": {
    "current": "executing",
    "completed": ["ingestion", "logical", "physical", "definition", "validation", "sequencing", "ready"]
  },
  "target_dir": "/path/to/project",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T14:30:00Z",
  "artifacts": {
    "capability_map": {"path": "artifacts/capability-map.json", "checksum": "abc123...", "valid": true},
    "physical_map": {"path": "artifacts/physical-map.json", "checksum": "def456...", "valid": true},
    "task_validation": {"verdict": "READY", "valid": true, "summary": "All tasks aligned"}
  },
  "tasks": {
    "T001": {
      "id": "T001",
      "name": "Implement credential validation",
      "status": "complete",
      "phase": 1,
      "depends_on": [],
      "blocks": ["T002", "T003"],
      "started_at": "2024-01-15T11:00:00Z",
      "completed_at": "2024-01-15T11:15:00Z",
      "duration_seconds": 900,
      "attempts": 1,
      "files_created": ["src/auth/validator.py", "tests/auth/test_validator.py"],
      "files_modified": [],
      "verification": {
        "verdict": "PASS",
        "recommendation": "PROCEED",
        "criteria": [
          {"name": "Valid credentials return True", "score": "PASS", "evidence": "Test passed"}
        ],
        "quality": {"types": "PASS", "docs": "PASS", "patterns": "PASS", "errors": "PASS"},
        "tests": {"coverage": "PASS", "assertions": "PASS", "edge_cases": "PARTIAL"}
      }
    }
  },
  "execution": {
    "current_phase": 2,
    "active_tasks": ["T003"],
    "completed_count": 2,
    "failed_count": 0,
    "total_tokens": 45000,
    "total_cost_usd": 0.45
  },
  "events": [
    {"timestamp": "2024-01-15T11:00:00Z", "type": "task_started", "task_id": "T001"},
    {"timestamp": "2024-01-15T11:15:00Z", "type": "task_completed", "task_id": "T001"}
  ]
}
```

---

## Execution Bundles

Before task execution, `bundle.py` generates a **self-contained bundle** with everything the executor needs:

```json
{
  "version": "1.2",
  "bundle_created_at": "2024-01-15T11:00:00Z",
  "task_id": "T001",
  "name": "Implement credential validation",
  "phase": 1,
  "target_dir": "/path/to/project",
  "context": {
    "domain": "Authentication",
    "capability": "User Login",
    "capability_id": "C1",
    "spec_ref": {
      "quote": "Users must be able to log in with email and password",
      "location": "paragraph 3"
    },
    "steel_thread": true
  },
  "behaviors": [
    {"id": "B1", "name": "validate_credentials", "type": "process", "description": "Verify email and password"}
  ],
  "files": [
    {"path": "src/auth/validator.py", "action": "create", "layer": "domain", "purpose": "Credential validation", "behaviors": ["B1"]}
  ],
  "dependencies": {
    "tasks": [],
    "files": [],
    "external": ["pydantic>=2.0"]
  },
  "acceptance_criteria": [
    {"criterion": "Valid credentials return True", "verification": "pytest tests/auth/test_validator.py -v"}
  ],
  "constraints": {
    "language": "Python",
    "framework": "FastAPI",
    "testing": "pytest",
    "patterns": ["Use Protocol for interfaces", "Use dataclass for data structures"],
    "raw": "Full constraints.md content..."
  },
  "checksums": {
    "artifacts": {
      "capability_map": "abc123...",
      "physical_map": "def456...",
      "constraints": "ghi789...",
      "task_definition": "jkl012..."
    },
    "dependency_files": {}
  }
}
```

**Bundle Benefits:**
- **Context isolation** - Executor sees only what it needs
- **Integrity validation** - Checksums detect artifact drift
- **Reproducibility** - All inputs captured
- **Self-documentation** - Bundle explains what to build and why

---

## Task Verification

### Execution-Time Verification (task-verifier)

After each task completes, an **LLM-as-judge verifier** evaluates the implementation:

**Evidence Gathering:**
1. Read implementation files
2. Run verification commands
3. Capture all output

**Multi-Dimensional Judgment:**

| Dimension | What's Judged |
|-----------|---------------|
| Functional Correctness | Does it meet each acceptance criterion? |
| Code Quality | Types, docs, patterns, error handling |
| Test Quality | Coverage, assertions, edge cases |

**Verdicts & Recommendations:**

| Verdict | Recommendation | Meaning |
|---------|----------------|---------|
| PASS | PROCEED | All criteria met, quality acceptable |
| CONDITIONAL | PROCEED | Works, minor issues, proceed with notes |
| FAIL | BLOCK | Criteria not met or critical issues |

**When BLOCK is recommended:**
- Dependent tasks are automatically blocked
- Task cannot be marked complete until issues resolved
- Verifier provides specific feedback on what failed and how to fix

### Verifier Calibration

Tasker tracks verifier accuracy over time:

- **False positives** - PROCEED verdict but task later failed
- **False negatives** - BLOCK verdict but task would have worked

**Calibration Score** = (correct verdicts) / (total verdicts)

The dashboard displays calibration metrics to help tune verification thresholds.

---

## Key Concepts

### Steel Thread

A "steel thread" is the minimal end-to-end path through the system. Tasks marked `steel_thread: true` are prioritized in early phases to validate architecture before building out the full system.

The plan-auditor validates that:
- Steel thread tasks exist
- They form a contiguous path
- They're assigned to early phases

### Behaviors vs Tasks

| Aspect | Behavior | Task |
|--------|----------|------|
| Abstraction | Logical (what to do) | Physical (where/how) |
| Granularity | Single atomic operation | Group of 2-5 related behaviors |
| Typical count | 20-50 per project | 5-15 per project |
| Created in | Phase 1 (capability-map.json) | Phase 3 (tasks/*.json) |
| Contains | Name, type, description | Behaviors, files, criteria, dependencies |

**Behaviors/Task Metric:**
- **< 2 behaviors/task**: Tasks too granular, excessive overhead
- **2-5 behaviors/task**: Sweet spot—cohesive, verifiable units
- **> 5 behaviors/task**: Tasks too large, higher failure risk

---

## Monitoring

### TUI Dashboard

The Textual-based TUI (`/tui` or `python3 scripts/status.py`) provides real-time monitoring:

**Panels:**
- **Phase Indicator** - Current workflow phase with icon
- **Health Checks** - DAG validation, steel thread, verification commands
- **Progress** - Task completion by phase with progress bars
- **Calibration** - Verifier accuracy metrics
- **Cost** - Token usage and estimated cost (total + per-task average)
- **Current Task** - Currently running task with elapsed time
- **Task List** - All tasks sorted by phase with status icons
- **Recent Activity** - Latest completions/failures

**Keybindings:**
- `q` - Quit
- `r` - Refresh
- `a` - Toggle auto-refresh (5s interval)
- `d` - Toggle dark/light mode
- `escape` / `b` - Back (from detail view)

### CLI Dashboard

```bash
python3 scripts/dashboard.py                # Full dashboard with boxes
python3 scripts/dashboard.py --compact      # Single-line summary
python3 scripts/dashboard.py --json         # JSON output
python3 scripts/dashboard.py --no-color     # Without ANSI colors
```

---

## Scripts Reference

### state.py - State Management

```bash
# Initialization
python3 scripts/state.py init /path/to/project      # Initialize workflow

# Phase progression
python3 scripts/state.py advance-phase logical      # Move to next phase
python3 scripts/state.py show                       # Display current state

# Task management
python3 scripts/state.py task-start T001            # Mark task running
python3 scripts/state.py task-complete T001         # Mark task complete
python3 scripts/state.py task-fail T001 "error"     # Mark task failed
python3 scripts/state.py retry-task T001            # Reset failed task
python3 scripts/state.py skip-task T001 "reason"    # Skip blocked task
python3 scripts/state.py ready                      # List ready tasks

# Validation
python3 scripts/state.py validate capability_map    # Validate artifact
python3 scripts/state.py validate-tasks READY "OK"  # Record task validation

# Token tracking
python3 scripts/state.py log-tokens T001 1000 500 0.02  # Log usage

# Metrics
python3 scripts/state.py metrics                    # Execution metrics
python3 scripts/state.py planning-metrics           # Planning quality
python3 scripts/state.py calibration-score          # Verifier accuracy
```

### bundle.py - Execution Bundles

```bash
python3 scripts/bundle.py generate T001        # Generate single bundle
python3 scripts/bundle.py generate-ready       # Generate for all ready tasks
python3 scripts/bundle.py validate T001        # Validate against schema
python3 scripts/bundle.py validate-integrity T001  # Check checksums + deps
python3 scripts/bundle.py list                 # List existing bundles
python3 scripts/bundle.py clean                # Remove all bundles
```

### validate.py - Comprehensive Validation

```bash
python3 scripts/validate.py dag                    # Check for cycles
python3 scripts/validate.py steel-thread           # Validate steel thread
python3 scripts/validate.py verification-commands  # Check command syntax
python3 scripts/validate.py calibration            # Show verifier metrics
python3 scripts/validate.py all                    # Run all validations
```

---

## Hooks

Tasker integrates with Claude Code hooks for automation:

| Hook | Trigger | Action |
|------|---------|--------|
| `detect-workflow.sh` | User prompt | Detect `/plan` or `/execute`, launch TUI |
| `launch-tui.sh` | Planning/execution start | Open tmux split (30% height) with TUI |
| `close-tui.sh` | Workflow complete | Close TUI pane |
| `subagent_stop.py` | Subagent completes | Parse transcript, log token usage to state |

**Note:** Tmux hooks only work when running Claude Code inside a tmux session.

---

## Agent Responsibilities

### Planning Agents

| Agent | Input | Output | Purpose |
|-------|-------|--------|---------|
| logic-architect | spec.md | capability-map.json | Extract logical structure (domains, capabilities, behaviors) |
| physical-architect | capability-map.json, constraints.md | physical-map.json | Map behaviors to file paths |
| task-author | Both maps | tasks/T*.json | Create individual task definitions |
| task-plan-verifier | Tasks + spec | Validation report | LLM-as-judge pre-execution check |
| plan-auditor | Tasks | Updated state.json | Assign phases, validate DAG |

### Execution Agents

| Agent | Input | Output | Purpose |
|-------|-------|--------|---------|
| task-executor | Execution bundle | Implementation code | Write code in isolated context |
| task-verifier | Implementation | Verification result | LLM-as-judge post-execution check |

---

## Templates

The `templates/` directory contains example files for reference. **You don't need to copy or follow these templates** - the planner accepts any specification format.

| Template | Purpose |
|----------|---------|
| `example-spec.md` | Shows one possible spec format (not required) |
| `constraints.md.example` | Example tech stack constraints |
| `task.json.example` | Shows task structure (generated automatically) |

### Specification Input

Your spec can be in **any format**:
- Freeform requirements or PRDs
- Bullet lists or numbered lists
- Design docs or meeting notes
- Existing README files

The planner stores your spec verbatim and extracts requirements from whatever format you provide. Each extracted capability includes a `spec_ref` that quotes the original text for traceability.

### Tech Stack Constraints (Optional)

You can provide constraints conversationally when running `/plan`, or create a `constraints.md` file with:
- Language & runtime preferences
- Framework choices
- Testing requirements
- Architecture patterns to follow or avoid

---

## Development

```bash
make install    # Setup project with uv
make lint       # Run ruff check
make test       # Run pytest
make clean      # Remove artifacts
```

---

## Design Principles

1. **Single Source of Truth** - `state.json` owns all workflow state
2. **Context Isolation** - Each executor sees only its bundle
3. **Fail Fast** - Validation happens before execution
4. **Observability** - TUI + hooks provide real-time visibility
5. **Reproducibility** - Checksums detect artifact drift
6. **Steel Thread First** - Validate architecture early
7. **LLM-as-Judge** - Structured verification with calibration tracking
8. **Schema Validation** - All artifacts validated against JSON schemas

---

## Limitations

- Requires Claude Code with subagent support
- TUI requires `textual` package (`uv add textual`)
- Tmux hooks only work in tmux sessions
- Currently single-threaded execution (no parallel tasks within a phase)
- Token tracking requires `subagent_stop.py` hook

---

## License

MIT
