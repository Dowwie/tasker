# Task Decomposition Protocol v2

A **schema-validated, state-managed** approach to task decomposition.

## Why v2?

The original design has weaknesses that compound in practice:

| Problem | Original | v2 Solution |
|---------|----------|-------------|
| **State detection** | Scan filesystem for file existence | Explicit `state.json` with phase tracking |
| **Artifact validation** | None - trust agent output | JSON schemas with validation gates |
| **Task inventory** | Monolithic markdown file | Individual JSON files per task |
| **Progress tracking** | Mutable progress.md | Append-only event log in state.json |
| **Error recovery** | Manual cleanup | Built-in rollback tracking |
| **Ready task computation** | Hardcoded wave order | Dynamic DAG resolution |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         COMMANDS                                │
│                                                                 │
│          /plan                              /execute            │
│            │                                    │               │
│            ▼                                    ▼               │
│    ┌───────────────┐                   ┌───────────────┐       │
│    │ Ask for spec, │                   │ Ask for plan  │       │
│    │ target dir,   │                   │ dir, target   │       │
│    │ constraints   │                   │ dir           │       │
│    └───────┬───────┘                   └───────┬───────┘       │
│            │                                    │               │
│            ▼                                    ▼               │
├────────────────────────────────────────────────────────────────┤
│                    scripts/state.py                             │
│           (Single source of truth for all state)                │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Planning Commands:              Execution Commands:            │
│    init, validate, advance         ready-tasks, start-task,    │
│    load-tasks                      complete-task, fail-task    │
│                                                                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │
           ┌────────────────┴────────────────┐
           │                                 │
           ▼                                 ▼
  ┌─────────────────┐               ┌─────────────────┐
  │  Planning Mode  │               │ Execution Mode  │
  │  (Phases 0-5)   │               │   (Phase 6)     │
  │                 │               │                 │
  │  Agents:        │               │  Subagents:     │
  │  • logic-arch   │               │  • task-executor│
  │  • physical-arch│               │    (isolated)   │
  │  • task-author  │               │                 │
  │  • plan-auditor │               │  Hook:          │
  │                 │               │  • token logging│
  │  Output:        │               │                 │
  │  • JSON artifacts│              │  Output:        │
  │  • Task files   │               │  • Code in target│
  │  • state.json   │               │  • Updated state│
  └─────────────────┘               └─────────────────┘
```

## Key Improvements

### 1. Schema-Validated Artifacts

Agents output JSON that **must validate** against schemas:

```bash
# Agent outputs capability-map.json
python3 scripts/state.py validate capability_map

# Returns exit code 0 (valid) or 1 (invalid)
# Invalid = agent must fix before advancing
```

No more "file exists but is garbage" problems.

### 2. Individual Task Files

Instead of one giant inventory:
```
project-planning/tasks/
├── T001.json    # Can work on this
├── T002.json    # While someone works on this  
├── T003.json    # And this
└── ...
```

Benefits:
- Parallel task creation
- Atomic state updates
- Clean git diffs
- Easy to re-run single task

### 3. Explicit State Machine

State is **never inferred** from filesystem:

```json
{
  "phase": {
    "current": "executing",
    "completed": ["ingestion", "logical", "physical", "definition", "sequencing", "ready"]
  },
  "tasks": {
    "T001": {"status": "complete", "completed_at": "..."},
    "T002": {"status": "running", "started_at": "..."},
    "T003": {"status": "pending"}
  }
}
```

### 4. Validation Gates

Phase transitions require validation:

```
logical phase
    │
    ▼
[Agent outputs capability-map.json]
    │
    ▼
[state.py validate capability_map]
    │
    ├── FAIL → Agent fixes output
    │
    └── PASS → [state.py advance] → physical phase
```

### 5. Dynamic Ready-Task Resolution

Instead of hardcoded wave execution:

```bash
$ python3 scripts/state.py ready-tasks
T005: Implement user service (wave 2)
T006: Add logging middleware (wave 2)
T007: Create health endpoint (wave 2)
```

Tasks are "ready" when all dependencies complete - computed at runtime.

### 6. Built-in Rollback

Task executor tracks all changes:

```python
# Before modifying
BACKUP: /tmp/rollback-T001/

# On failure
→ Delete created files
→ Restore modified files
→ Mark task failed
→ Dependent tasks auto-blocked
```

### 7. Append-Only Event Log

All state changes logged:

```json
"events": [
  {"timestamp": "...", "type": "task_started", "task_id": "T001"},
  {"timestamp": "...", "type": "task_completed", "task_id": "T001"},
  {"timestamp": "...", "type": "tokens_logged", "details": {"cost": 0.0234}}
]
```

Enables: auditing, debugging, replaying state.

## Usage

### Plan (Decompose Spec → Tasks)

```bash
claude
> /plan
```

You'll be asked for:
1. Your specification (paste or path)
2. Target directory (where code will be written)
3. Constraints (optional - tech stack, rules)

The planner runs through phases 0-5, producing:
- `capability-map.json` (validated)
- `physical-map.json` (validated)
- `tasks/*.json` (individual task files)
- `state.json` (ready for execution)

### Execute (Run Tasks via Subagents)

```bash
> /execute
```

You'll be asked for:
1. Planning directory (where state.json lives)
2. Target directory (where code gets written)

The executor:
- Queries ready tasks dynamically
- Spawns isolated subagent per task
- Tracks files for rollback
- Logs token usage
- Continues until complete

### Execution Options

```bash
> /execute           # Interactive, one at a time
> /execute T005      # Specific task only
> /execute --batch   # All ready, no prompts
```

### Check Status

```bash
python3 scripts/state.py status
python3 scripts/state.py ready-tasks
```

### Error Recovery

```bash
# Retry a failed task (resets to pending, unblocks dependents)
python3 scripts/state.py retry-task T005

# Skip a blocked task (treats as complete for dependency purposes)
python3 scripts/state.py skip-task T005 "Not needed for MVP"
```

### Execute from Target Project

Install `/work` command in your target project:

```bash
# Install /work command
python3 scripts/install_work_command.py /path/to/target/project

# Then from target project:
cd /path/to/target/project
claude
> /work
```

This allows you to work from the target project context while still using the tasker planning infrastructure.

## Templates

Copy templates to get started:

```bash
mkdir -p project-planning/inputs
cp templates/spec.md.example project-planning/inputs/spec.md
cp templates/constraints.md.example project-planning/inputs/constraints.md
# Edit with your project details
```

See `templates/README.md` for full documentation.

## Documentation

- `docs/protocol.md` - Full protocol specification with phase details
- `templates/README.md` - Template usage guide
- `.claude/skills/orchestrator/SKILL.md` - Orchestrator implementation details

## Directory Structure

```
tasker/
├── .claude/
│   ├── agents/
│   │   ├── logic-architect.md      # Phase 1: spec → capabilities
│   │   ├── physical-architect.md   # Phase 2: atoms → files
│   │   ├── task-author.md          # Phase 3: files → tasks
│   │   ├── plan-auditor.md         # Phase 4: tasks → waves
│   │   └── task-executor.md        # Phase 6: task → code
│   ├── commands/
│   │   ├── plan.md                 # /plan - decomposition
│   │   └── execute.md              # /execute - run tasks
│   ├── hooks/
│   │   └── subagent_stop.py        # Token tracking
│   └── skills/
│       └── orchestrator/
│           └── SKILL.md            # Thin coordination layer
├── docs/
│   └── protocol.md                 # Full protocol specification
├── schemas/
│   ├── state.schema.json           # Execution state
│   ├── capability-map.schema.json  # Phase 1 output
│   ├── physical-map.schema.json    # Phase 2 output
│   ├── task.schema.json            # Individual task
│   └── execution-bundle.schema.json # Task execution bundle
├── scripts/
│   ├── state.py                    # All state management
│   ├── bundle.py                   # Execution bundle generator
│   └── install_work_command.py     # Install /work in target project
├── templates/
│   ├── spec.md.example             # Specification template
│   ├── constraints.md.example      # Constraints template
│   ├── task.json.example           # Task file example
│   ├── commands/
│   │   └── work.md                 # /work command template
│   └── README.md                   # Template usage guide
└── project-planning/               # Created at runtime
    ├── state.json                  # Single source of truth
    ├── inputs/
    │   ├── spec.md
    │   └── constraints.md
    ├── artifacts/
    │   ├── capability-map.json
    │   └── physical-map.json
    ├── tasks/
    │   ├── T001.json
    │   └── ...
    └── bundles/
        ├── T001-bundle.json
        └── ...
```

## Comparison Summary

| Aspect | Original | v2 |
|--------|----------|-----|
| **Commands** | `/task-decompose`, `/work` | `/plan`, `/execute` |
| **State detection** | Filesystem scan | Explicit state.json |
| **Artifact format** | Markdown | Validated JSON |
| **Task storage** | Single file | Individual files |
| **Validation** | None | Schema-based gates |
| **Ready tasks** | Wave-based | DAG-computed |
| **Error recovery** | Manual | Built-in rollback |
| **Event history** | Mutable progress.md | Append-only log |
| **Orchestrator** | Heavy (all logic) | Thin (delegates to state.py) |

## When to Use Original vs v2

**Use Original** if:
- Simple project, few tasks
- Prefer markdown readability
- Don't need parallel execution

**Use v2** if:
- Complex project, many tasks
- Need robust error recovery
- Want parallel task execution
- Need audit trail
- Agents produce unreliable output (validation gates help)
