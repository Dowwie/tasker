# Tasker

A Claude Code-powered task decomposition and execution framework. Transforms software specifications into executable DAGs of atomic, verifiable tasks.

## Overview

Tasker implements a **Task Decomposition Protocol** that:
- Breaks specifications into behavioral atoms (Input/Process/State/Output)
- Maps atoms to physical files and artifacts
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
   - Phase 1: `logic-architect` extracts capabilities/atoms from spec
   - Phase 2: `physical-architect` maps atoms to file paths
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
```

## Bundle Generation

Each task gets a self-contained execution bundle with:
- Expanded atom details
- File paths with purposes
- Acceptance criteria
- Constraints (language, framework, patterns)
- Dependency files from prior tasks

```bash
python3 scripts/bundle.py generate T001       # Generate for one task
python3 scripts/bundle.py generate-ready      # Generate all ready
python3 scripts/bundle.py list                # List existing bundles
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
