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

# In Claude Code, plan a project:
/plan

# Then execute the plan:
/execute
```

## Commands

| Command | Description |
|---------|-------------|
| `/plan` | Decompose a spec into a task DAG (phases 0-5) |
| `/execute` | Run tasks via isolated subagents |
| `/execute T005` | Execute a specific task |
| `/execute --batch` | Execute all ready tasks without prompts |

## Planning Phases

```
Phase 0: Ingestion     → spec.md saved
Phase 1: Logical       → capability-map.json (what the system does)
Phase 2: Physical      → physical-map.json (where code lives)
Phase 3: Definition    → tasks/*.json (individual task files)
Phase 4: Sequencing    → wave assignments, DAG validation
Phase 5: Ready         → planning complete, ready for execution
```

## Project Structure

```
.claude/
├── agents/           # Specialized subagent definitions
│   ├── logic-architect.md
│   ├── physical-architect.md
│   ├── task-author.md
│   ├── plan-auditor.md
│   └── task-executor.md
├── commands/         # Slash commands
│   ├── plan.md
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

## License

See [LICENSE.md](LICENSE.md)
