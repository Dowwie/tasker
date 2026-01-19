# Tasker Plugin Packaging Plan

## Overview

Package Tasker as an installable Claude Code plugin with three commands: `tasker:specify`, `tasker:plan`, `tasker:execute`. The architecture consolidates all Python scripts into a single Go binary, with a thin Claude Code skill layer for LLM orchestration.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Claude Code Plugin (.claude/skills/tasker/)                │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ SKILL.md                                              │  │
│  │  • Mode detection (specify/plan/execute)              │  │
│  │  • LLM orchestration & agent spawning                 │  │
│  │  • User interaction (AskUserQuestion)                 │  │
│  │  • Workflow logic (phases, gates)                     │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ agents/*.md - 8 agent definitions                     │  │
│  │  • task-executor, task-verifier, logic-architect      │  │
│  │  • physical-architect, task-author, plan-auditor      │  │
│  │  • spec-reviewer, task-plan-verifier                  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  tasker binary (Go)                                         │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Commands (all output JSON for skill consumption)      │  │
│  │                                                       │  │
│  │ State:     init, status, advance, halt, resume        │  │
│  │ Tasks:     ready, start, complete, fail, skip, retry  │  │
│  │ Bundles:   generate, validate, list                   │  │
│  │ Validate:  artifact, tasks, planning-gates            │  │
│  │ FSM:       compile, validate, coverage, mermaid       │  │
│  │ Spec:      review, import, export                     │  │
│  │ Checkpoint: create, recover, status, complete, clear  │  │
│  │ Metrics:   execution, planning, coverage, calibration │  │
│  │ Git:       commit-task                                │  │
│  │ Archive:   create, list                               │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Embedded:  schemas/*.json (13 schemas)                │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Working Directory (user's project)                         │
│  └── .tasker/              (configurable via TASKER_DIR)    │
│      ├── state.json        State file                       │
│      ├── inputs/           Spec and constraints             │
│      ├── artifacts/        Capability map, physical map     │
│      ├── tasks/            Task definitions (T001.json)     │
│      ├── bundles/          Execution bundles                │
│      ├── reports/          Validation reports               │
│      └── checkpoints/      Crash recovery                   │
└─────────────────────────────────────────────────────────────┘
```

## Go Binary Design

### CLI Structure

```
tasker <command> [subcommand] [flags]

Global Flags:
  --dir, -d <path>     Working directory (default: $TASKER_DIR or .tasker)
  --json               Force JSON output (default for most commands)
  --quiet, -q          Suppress non-essential output

Commands:

STATE MANAGEMENT
  tasker init [target]              Initialize state in working directory
  tasker status                     Show phase, task counts, progress
  tasker advance                    Attempt phase advancement
  tasker halt [reason]              Request graceful halt
  tasker resume                     Clear halt, resume execution
  tasker check-halt                 Check if halted (exit code 0=halted)

TASK LIFECYCLE
  tasker task list                  List all tasks with status
  tasker task ready                 List tasks ready for execution
  tasker task get <id>              Get task definition
  tasker task start <id>            Mark as running
  tasker task complete <id> [--created f1] [--modified f2]
  tasker task fail <id> <error> [--category CAT]
  tasker task skip <id> [reason]    Skip without blocking dependents
  tasker task retry <id>            Reset failed task to pending
  tasker task load                  Reload tasks from disk

BUNDLE MANAGEMENT
  tasker bundle generate <id>       Generate execution bundle
  tasker bundle generate-ready      Generate for all ready tasks
  tasker bundle validate <id>       Validate bundle schema
  tasker bundle integrity <id>      Check dependencies + checksums
  tasker bundle list                List existing bundles
  tasker bundle clean               Remove all bundles

VALIDATION
  tasker validate artifact <name>   Validate artifact against schema
  tasker validate tasks <verdict> [--issues i1 i2]
  tasker validate planning-gates [--threshold 0.9]
  tasker validate dag               Check for cycles, missing deps

FSM (FINITE STATE MACHINE)
  tasker fsm compile <spec>         Compile FSM from spec file
  tasker fsm validate <fsm-dir>     Validate FSM completeness (I1-I5)
  tasker fsm coverage               Check task coverage of transitions
  tasker fsm mermaid <fsm-dir>      Generate Mermaid diagrams

SPEC OPERATIONS
  tasker spec import <file>         Import spec to inputs/
  tasker spec review                Run weakness detection (W1-W7)
  tasker spec export <target>       Export to target project

CHECKPOINT (CRASH RECOVERY)
  tasker checkpoint create <t1> [t2...]
  tasker checkpoint recover         Find orphaned tasks
  tasker checkpoint status
  tasker checkpoint complete
  tasker checkpoint clear

METRICS & REPORTING
  tasker metrics execution          Task timing, success rates
  tasker metrics planning           Planning quality metrics
  tasker metrics coverage           Spec coverage analysis
  tasker metrics calibration        Verifier calibration score
  tasker metrics failures           Failure classification breakdown

GIT INTEGRATION
  tasker git commit-task <id>       Commit files from completed task

ARCHIVE
  tasker archive create [name]      Archive current state
  tasker archive list               List archives

VERIFICATION
  tasker verify record <id> --verdict PASS|FAIL [--criteria json]
  tasker verify calibration <id> <outcome> [notes]

UTILITY
  tasker version                    Show version
  tasker schema list                List embedded schemas
  tasker schema show <name>         Print schema content
```

### Go Package Structure

```
cmd/tasker/
├── main.go                    Entry point, CLI setup (cobra)
├── commands/
│   ├── init.go
│   ├── status.go
│   ├── task.go                Task subcommands
│   ├── bundle.go              Bundle subcommands
│   ├── validate.go            Validation subcommands
│   ├── fsm.go                 FSM subcommands
│   ├── spec.go                Spec subcommands
│   ├── checkpoint.go
│   ├── metrics.go
│   ├── git.go
│   └── archive.go

internal/
├── state/
│   ├── state.go               State struct, load/save
│   ├── events.go              Event logging
│   ├── phases.go              Phase transitions
│   └── tasks.go               Task lifecycle
├── bundle/
│   ├── generator.go           Bundle generation
│   ├── integrity.go           Checksum verification
│   └── dependencies.go        Dependency resolution
├── validate/
│   ├── schema.go              JSON schema validation
│   ├── dag.go                 DAG cycle detection
│   ├── coverage.go            Spec coverage analysis
│   └── gates.go               Planning gates
├── fsm/
│   ├── compiler.go            Compile from spec
│   ├── validator.go           I1-I5 invariants
│   ├── coverage.go            Transition coverage
│   └── mermaid.go             Diagram generation
├── spec/
│   ├── review.go              Weakness detection W1-W7
│   ├── import.go              Import handling
│   └── export.go              Export to target
├── checkpoint/
│   ├── manager.go             Checkpoint lifecycle
│   └── recovery.go            Orphan detection
├── metrics/
│   ├── execution.go
│   ├── planning.go
│   └── calibration.go
├── git/
│   └── commit.go              Git operations
├── archive/
│   └── archive.go             Archiving logic
└── config/
    ├── config.go              Configuration management
    └── paths.go               Path resolution

schemas/                        Embedded via go:embed
├── state.schema.json
├── task.schema.json
├── capability-map.schema.json
├── physical-map.schema.json
├── execution-bundle.schema.json
├── spec-review.schema.json
├── fsm-index.schema.json
├── fsm-states.schema.json
├── fsm-transitions.schema.json
└── ... (13 total)
```

### Configuration

Environment variables:
- `TASKER_DIR` - Override default .tasker directory
- `TASKER_TARGET` - Target project directory (for spec export)

Config file (optional): `.tasker/config.json`
```json
{
  "target_dir": "/path/to/project",
  "schemas_dir": null,
  "hooks": {
    "post_complete": "script.sh"
  }
}
```

## Skill Layer Design

### Directory Structure

```
.claude/skills/tasker/
├── SKILL.md                   Unified skill definition
├── agents/
│   ├── task-executor.md
│   ├── task-verifier.md
│   ├── logic-architect.md
│   ├── physical-architect.md
│   ├── task-author.md
│   ├── plan-auditor.md
│   ├── spec-reviewer.md
│   └── task-plan-verifier.md
└── README.md                  Installation/usage docs
```

### SKILL.md Structure

```markdown
---
name: tasker
description: Spec-driven development. Commands: specify, plan, execute
tools:
  - AskUserQuestion
  - Task
  - Bash
  - Read
  - Write
  - Glob
  - Grep
---

# Tasker

## Commands

| Command | Purpose |
|---------|---------|
| `tasker:specify` | Interactive specification workflow |
| `tasker:plan` | Decompose spec into task DAG |
| `tasker:execute` | Run tasks via isolated subagents |

## Mode Detection

Parse invocation to determine mode:
- Contains "specify" → Specify Mode
- Contains "plan" → Plan Mode
- Contains "execute" → Execute Mode

## Binary Location

The tasker binary is at: `~/.local/bin/tasker` (or in PATH)

All deterministic operations delegate to the binary:
- `tasker status` → Get current state
- `tasker task ready` → Get ready tasks
- `tasker bundle generate T001` → Generate bundle
- etc.

---

## Specify Mode

[Merged content from current specify/SKILL.md]

### Phase 0: Spec Acquisition (NEW)

Before entering Phase 1, determine starting point:

**Ask user via AskUserQuestion:**
> How would you like to begin?
> 1. **Start fresh** - Begin interactive discovery from scratch
> 2. **Import existing spec** - You have a spec file/document
> 3. **Paste requirements** - Provide requirements in chat

**If Import:**
1. Ask for file path
2. Run: `tasker spec import <path>`
3. Run: `tasker spec review`
4. Based on review, skip to Phase 6 or start at Phase 2

**If Paste:**
1. Accept pasted content
2. Write to temp file, run `tasker spec import`
3. Continue as above

[Rest of specify phases 1-8...]

---

## Plan Mode

[Merged content from orchestrator SKILL.md - Plan section]

### Prerequisites
- Run `tasker init` if not initialized
- Verify spec exists: `tasker status`

### Phase Flow
1. Ingestion - `tasker spec import`
2. Spec Review - Spawn spec-reviewer agent
3. Logical - Spawn logic-architect agent
4. Physical - Spawn physical-architect agent
5. Definition - Spawn task-author agent
6. Validation - Spawn task-plan-verifier agent
7. Sequencing - Spawn plan-auditor agent

[Detailed phase instructions...]

---

## Execute Mode

[Merged content from orchestrator SKILL.md - Execute section]

### Prerequisites
- Planning complete: `tasker status` shows "executing" phase
- Git initialized in target

### Execution Loop
1. `tasker task ready` → Get ready tasks
2. `tasker checkpoint create T001 T002 T003`
3. Spawn up to 3 task-executor agents in parallel
4. Monitor result files
5. `tasker task complete` or `tasker task fail`
6. Repeat until all tasks done

[Detailed execution instructions...]

---

## Agent Definitions

Agents are spawned with the Task tool. Each agent file in `agents/`
contains the full prompt template.

[Reference to agents/*.md files...]
```

## Installation Mechanism

### Install Script

```bash
#!/bin/bash
# install.sh - Install Tasker Claude Code plugin

set -e

VERSION="${TASKER_VERSION:-latest}"
BINARY_NAME="tasker"
SKILL_NAME="tasker"

# Detect OS/Arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
esac

# Directories
BIN_DIR="${TASKER_BIN_DIR:-$HOME/.local/bin}"
SKILL_DIR="${TASKER_SKILL_DIR:-$HOME/.claude/skills}"

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Install Tasker as a Claude Code plugin.

Options:
  --version VER    Install specific version (default: latest)
  --bin-dir DIR    Binary install location (default: ~/.local/bin)
  --skill-dir DIR  Skill install location (default: ~/.claude/skills)
  --uninstall      Remove Tasker
  --help           Show this help

Examples:
  $0                    # Install latest
  $0 --version v1.0.0   # Install specific version
  $0 --uninstall        # Remove
EOF
}

install_binary() {
    echo "Installing tasker binary..."
    mkdir -p "$BIN_DIR"

    # Download from releases (placeholder URL)
    DOWNLOAD_URL="https://github.com/OWNER/tasker/releases/download/${VERSION}/tasker-${OS}-${ARCH}"

    curl -fsSL "$DOWNLOAD_URL" -o "$BIN_DIR/$BINARY_NAME"
    chmod +x "$BIN_DIR/$BINARY_NAME"

    echo "Binary installed: $BIN_DIR/$BINARY_NAME"
}

install_skill() {
    echo "Installing skill..."
    mkdir -p "$SKILL_DIR"

    # Clone or download skill files
    SKILL_URL="https://github.com/OWNER/tasker/releases/download/${VERSION}/skill.tar.gz"

    curl -fsSL "$SKILL_URL" | tar -xz -C "$SKILL_DIR"

    echo "Skill installed: $SKILL_DIR/$SKILL_NAME"
}

show_next_steps() {
    cat << EOF

Tasker installed successfully!

Next steps:

1. Ensure $BIN_DIR is in your PATH:
   export PATH="\$PATH:$BIN_DIR"

2. Add to your project's .claude/settings.local.json:
   {
     "permissions": {
       "allow": [
         "Bash($BIN_DIR/tasker:*)",
         "Skill(tasker)"
       ]
     }
   }

3. Start using Tasker:
   /tasker:specify  - Create a new specification
   /tasker:plan     - Decompose spec into tasks
   /tasker:execute  - Run the tasks

EOF
}

uninstall() {
    echo "Uninstalling Tasker..."
    rm -f "$BIN_DIR/$BINARY_NAME"
    rm -rf "$SKILL_DIR/$SKILL_NAME"
    echo "Tasker uninstalled."
}

# Parse args
while [[ $# -gt 0 ]]; do
    case $1 in
        --version) VERSION="$2"; shift 2 ;;
        --bin-dir) BIN_DIR="$2"; shift 2 ;;
        --skill-dir) SKILL_DIR="$2"; shift 2 ;;
        --uninstall) uninstall; exit 0 ;;
        --help) usage; exit 0 ;;
        *) echo "Unknown option: $1"; usage; exit 1 ;;
    esac
done

install_binary
install_skill
show_next_steps
```

## Implementation Phases

### Phase 1: Go Binary Foundation

**Goal**: Core state management and CLI structure

Files to create:
- `cmd/tasker/main.go` - CLI entry with cobra
- `internal/config/` - Configuration and path resolution
- `internal/state/` - State management (port state.py)
- `commands/init.go`, `status.go`, `task.go`

Port from Python:
- `scripts/state.py` → `internal/state/`

Deliverable: `tasker init`, `tasker status`, `tasker task *` commands working

### Phase 2: Bundle & Validation

**Goal**: Bundle generation and schema validation

Files to create:
- `internal/bundle/` - Bundle generation
- `internal/validate/` - Schema validation, DAG checks
- `commands/bundle.go`, `validate.go`

Port from Python:
- `scripts/bundle.py` → `internal/bundle/`
- `scripts/validate.py` → `internal/validate/`

Deliverable: `tasker bundle *`, `tasker validate *` commands working

### Phase 3: FSM System

**Goal**: FSM compilation, validation, coverage

Files to create:
- `internal/fsm/` - Full FSM subsystem
- `commands/fsm.go`

Port from Python:
- `scripts/fsm-compiler.py` → `internal/fsm/compiler.go`
- `scripts/fsm-mermaid.py` → `internal/fsm/mermaid.go`
- `scripts/fsm-validate.py` → `internal/fsm/validator.go`

Deliverable: `tasker fsm *` commands working

### Phase 4: Spec Operations

**Goal**: Spec import, review, export

Files to create:
- `internal/spec/` - Spec handling
- `commands/spec.go`

Port from Python:
- `scripts/spec-review.py` → `internal/spec/review.go`
- `scripts/spec-session.py` → `internal/spec/`
- `scripts/spec-generate.py` → `internal/spec/`

Deliverable: `tasker spec *` commands working

### Phase 5: Supporting Features

**Goal**: Checkpoints, metrics, archive, git

Files to create:
- `internal/checkpoint/`
- `internal/metrics/`
- `internal/archive/`
- `internal/git/`
- Corresponding commands

Port from Python:
- `scripts/archive.py`
- Metrics from state.py

Deliverable: All remaining commands working

### Phase 6: Skill Layer

**Goal**: Claude Code skill integration

Files to create:
- `.claude/skills/tasker/SKILL.md` - Unified skill
- `.claude/skills/tasker/agents/*.md` - Agent definitions
- `.claude/skills/tasker/README.md`

Actions:
- Merge specify + orchestrator SKILL.md content
- Add spec acquisition phase
- Update all references to use `tasker` binary
- Test all three modes

### Phase 7: Installation & Distribution

**Goal**: Easy installation for users

Files to create:
- `install.sh` - Installation script
- `Makefile` or `goreleaser.yaml` - Build automation
- GitHub Actions for releases

Actions:
- Set up GitHub releases
- Build binaries for darwin/linux x amd64/arm64
- Package skill files
- Write installation docs

## Scripts to Port (15 total)

| Python Script | Go Location | Priority |
|---------------|-------------|----------|
| `state.py` | `internal/state/` | P0 - Core |
| `bundle.py` | `internal/bundle/` | P0 - Core |
| `validate.py` | `internal/validate/` | P1 - Essential |
| `fsm-compiler.py` | `internal/fsm/compiler.go` | P1 - Essential |
| `fsm-validate.py` | `internal/fsm/validator.go` | P1 - Essential |
| `fsm-mermaid.py` | `internal/fsm/mermaid.go` | P2 - Important |
| `spec-review.py` | `internal/spec/review.go` | P1 - Essential |
| `spec-session.py` | `internal/spec/` | P2 - Important |
| `spec-generate.py` | `internal/spec/` | P2 - Important |
| `archive.py` | `internal/archive/` | P3 - Nice to have |
| `status.py` | Merged into TUI | P0 - Core |
| `evaluate.py` | `internal/metrics/` | P3 - Nice to have |

## Verification Plan

### Unit Tests
- Each Go package has `*_test.go` files
- Test state transitions, bundle generation, validation logic
- Use table-driven tests for schema validation

### Integration Tests
- End-to-end workflow: init → plan → execute
- Test with sample specs from `tests/fixtures/`
- Verify JSON output matches expected schemas

### Manual Testing
1. Install in fresh project
2. Run `/tasker:specify` - create spec interactively
3. Run `/tasker:plan` - decompose into tasks
4. Run `/tasker:execute` - implement tasks
5. Verify `.tasker/` directory structure
6. Test crash recovery via checkpoint

### Compatibility
- Test on macOS (arm64, amd64)
- Test on Linux (amd64, arm64)
- Verify skill works in Claude Code

## Success Criteria

- [ ] Single `tasker` binary with all commands
- [ ] All 15 Python scripts functionality ported
- [ ] SKILL.md exposes `tasker:specify`, `tasker:plan`, `tasker:execute`
- [ ] Works when installed in any project via install.sh
- [ ] State stored in configurable `.tasker/` directory
- [ ] JSON schemas embedded in binary
- [ ] Cross-platform binaries (darwin/linux x amd64/arm64)
