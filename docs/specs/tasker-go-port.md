# Spec: Tasker Go Port

## Related ADRs
- [ADR-0001: JSON Schema Validation Library](../adrs/ADR-0001-json-schema-library.md)

## Goal
Port Tasker from Python scripts/modules to a single Go binary executable. The agentic parts (skills, agents, prompts) remain intact, but bash script invocations will call the Go binary with a unified CLI interface.

## Non-goals
- No feature changes (pure port)
- No agent changes (skills/agents/prompts stay as-is)
- No UI/UX changes (CLI interface behavior identical)

## Done means
- Every Python script has a Go equivalent subcommand
- Existing test suite passes against Go binary
- All skills/agents work without modification

---

## Workflows

### 1. Port Core State Management (Steel Thread)
The `state.py` script is the single source of truth for all state changes. This must be ported first as other components depend on it.

1. Create Go project structure following Terraform conventions
2. Implement state.json read/write with file locking
3. Port all state.py subcommands: init, status, advance, validate, validate-tasks, load-tasks, ready-tasks, start-task, complete-task, commit-task, fail-task, retry-task, skip-task, log-tokens, record-verification, metrics, planning-metrics, spec-coverage, failure-metrics, record-calibration, calibration-score, halt, check-halt, resume, halt-status
4. Implement JSON schema validation using Go library
5. Test output compatibility with Python version

**Variants:**
- If state file is corrupted, attempt auto-recovery before failing
- If concurrent access detected, use file locking to serialize

**Failures:**
- If schema validation fails, exit with non-zero code and error to stderr
- If file lock cannot be acquired within timeout, exit with error

**Postconditions:**
- state.json format identical to Python version
- Exit codes match Python behavior
- All state.py subcommands functional

### 2. Port Validation Module
The `validate.py` script provides DAG cycle detection, calibration tracking, and planning gates.

1. Port DAG cycle detection algorithm
2. Port steel thread validation
3. Port verification command checks
4. Port planning gates: spec-coverage, phase-leakage, dependency-existence, acceptance-criteria, planning-gates, refactor-priority
5. Test output compatibility

**Variants:**
- If --threshold flag provided, use custom threshold
- If --format json specified, output JSON instead of text

**Failures:**
- If cycle detected in DAG, exit with non-zero and report cycle
- If validation fails, list all failures before exiting

**Postconditions:**
- All validation commands produce identical output to Python

### 3. Port FSM Tools
The `fsm-compiler.py`, `fsm-mermaid.py`, and `fsm-validate.py` scripts handle finite state machine artifacts.

1. Port FSM compiler (spec → states/transitions)
2. Port Mermaid diagram generator
3. Port FSM validator (I1-I5 invariants)
4. Test output compatibility

**Variants:**
- If from-capability-map command used, derive FSM from capability map
- If coverage-report requested, compute transition coverage

**Failures:**
- If FSM invariant violated, report which invariant and context
- If spec has ambiguous workflow, exit with error (no silent ambiguity)

**Postconditions:**
- FSM JSON artifacts identical format
- Mermaid output identical

### 4. Port Spec Tools
The `spec-review.py`, `spec-generate.py`, and `spec-session.py` scripts handle specification management.

1. Port spec weakness detection (W1-W7 categories)
2. Port spec generation utilities
3. Port session state management
4. Test output compatibility

**Variants:**
- If analyze command, run full weakness detection
- If status command, show current session state

**Failures:**
- If spec file not found, exit with clear error message

**Postconditions:**
- Weakness detection produces identical results
- Session state format unchanged

### 5. Port Utility Scripts
Port remaining Python scripts: bundle.py, archive.py, dashboard.py, status.py, evaluate.py.

1. Port bundle generation (task execution bundles)
2. Port archive functionality
3. Port dashboard/status display
4. Port evaluation metrics
5. Test each for output compatibility

**Variants:**
- Bundle generate-ready processes all ready tasks
- Dashboard supports different display modes

**Failures:**
- If bundle validation fails, report specific issues
- If archive target doesn't exist, exit with error

**Postconditions:**
- All utility commands functional with identical output

### 6. Port Shell Scripts
Port shell scripts to Go subcommands: ensure-git.sh, init-planning-dirs.sh, log-activity.sh.

1. Port git repository validation
2. Port directory initialization
3. Port activity logging
4. Test functionality

**Variants:**
- init-planning-dirs creates full directory tree

**Failures:**
- If not in git repo (ensure-git), exit with clear message

**Postconditions:**
- Shell script functionality available as Go subcommands

### 7. Port TUI
Port the Python Textual TUI to Go using bubbletea/lipgloss.

1. Implement main dashboard view
2. Implement task detail view
3. Implement state provider (data fetching)
4. Implement keyboard navigation
5. Test visual and functional compatibility

**Variants:**
- Different views based on current state

**Failures:**
- If terminal doesn't support TUI, fall back to text output

**Postconditions:**
- TUI functionally equivalent to Python version
- Keyboard-only navigation

### 8. Create Build System
Set up build infrastructure for the Go project.

1. Create Makefile with build targets
2. Configure cross-compilation (macOS arm64/amd64, Linux arm64/amd64)
3. Set up GitHub Actions CI/CD
4. Create Homebrew formula
5. Configure release automation

**Variants:**
- Local build vs CI build
- Debug vs release builds

**Failures:**
- If build fails, CI should report specific error

**Postconditions:**
- Single command builds all targets
- Releases automated via GitHub

### 9. Create Shim Scripts
Create compatibility shims so existing skills continue working.

1. Create shim template that execs Go binary
2. Generate shims for all Python script paths
3. Test skills work without modification

**Variants:**
- Shim translates arguments if format differs

**Failures:**
- If Go binary not found, shim exits with helpful error

**Postconditions:**
- `python3 scripts/state.py status` works via shim
- Skills unchanged, functionality preserved

### 10. Integration Testing
Validate Go binary produces identical output to Python.

1. Create comparison test framework
2. Run each command against both implementations
3. Compare outputs byte-for-byte (exact match for JSON output; whitespace-normalized for text output)
4. Document any intentional differences
5. Achieve 100% command coverage

**Variants:**
- Parameterized tests for different inputs

**Failures:**
- If output differs unexpectedly, test fails with diff

**Postconditions:**
- All commands validated for compatibility
- Test suite passes for big bang migration

---

## Invariants

- INV-001: JSON file formats (state.json, task files, bundles) MUST remain byte-compatible with Python version
- INV-002: Exit codes MUST match Python script behavior for all commands
- INV-003: Stderr error messages MUST be parseable by calling agents
- INV-004: File locking MUST be used for all state file writes
- INV-005: No Python runtime dependency after Go migration complete
- INV-006: Skills MUST work without modification via shim scripts
- INV-007: TUI MUST support keyboard-only navigation
- INV-008: Log output (when enabled) MUST NOT interfere with stdout data
- INV-009: Schema validation MUST use same JSON Schema files as Python
- INV-010: Auto-recovery from corrupted state MUST NOT lose valid data

---

## Interfaces

### CLI Interface (Primary)
```
tasker <command> [subcommand] [flags] [args]

Commands:
  state       State management (init, status, advance, validate, validate-tasks, load-tasks, ready-tasks, start-task, complete-task, commit-task, fail-task, retry-task, skip-task, log-tokens, record-verification, metrics, planning-metrics, spec-coverage, failure-metrics, record-calibration, calibration-score, halt, check-halt, resume, halt-status)
  validate    Validation (dag, steel-thread, verification-commands, calibration, all, spec-coverage, phase-leakage, dependency-existence, acceptance-criteria, planning-gates, refactor-priority)
  fsm         FSM tools (compile, mermaid, validate)
  spec        Spec tools (review, generate, session)
  bundle      Bundle management (generate, validate, list, clean)
  archive     Archive planning artifacts
  dashboard   Display dashboard
  status      Show status
  evaluate    Run evaluation
  compliance  Compliance checks
  init        Initialize planning directories
  git         Git utilities (ensure)
  log         Activity logging
  tui         Launch interactive TUI

Global Flags:
  --log-level   Log level (debug|info|warn|error), default: warn
  --help        Show help
  --version     Show version
```

### File Interface
- Reads/writes: `project-planning/state.json`
- Reads/writes: `project-planning/tasks/*.json`
- Reads/writes: `project-planning/bundles/*.json`
- Reads/writes: `project-planning/artifacts/*.json`
- Reads: `schemas/*.json`

### Environment Variables
- `TASKER_LOG_LEVEL` - Override default log level
- (No other configuration - CLI args only)

---

## Architecture sketch

```
tasker/
├── go/                          # Go source code
│   ├── cmd/
│   │   └── tasker/
│   │       └── main.go          # Entry point
│   ├── internal/
│   │   ├── command/             # CLI command implementations
│   │   │   ├── state/           # state subcommands
│   │   │   ├── validate/        # validate subcommands
│   │   │   ├── fsm/             # fsm subcommands
│   │   │   ├── spec/            # spec subcommands
│   │   │   ├── bundle/          # bundle subcommands
│   │   │   └── ...
│   │   ├── state/               # State management core
│   │   │   ├── state.go         # State file operations
│   │   │   ├── lock.go          # File locking
│   │   │   └── recovery.go      # Auto-recovery
│   │   ├── schema/              # JSON schema validation
│   │   ├── tui/                 # TUI (bubbletea)
│   │   └── logging/             # Logging infrastructure
│   ├── go.mod
│   └── go.sum
├── bin/                         # Built binaries (gitignored)
│   └── tasker
├── scripts/                     # Shim scripts (replacing Python)
│   ├── state.py                 # Shim → tasker state
│   ├── validate.py              # Shim → tasker validate
│   └── ...
├── schemas/                     # JSON schemas (shared)
└── Makefile                     # Build commands
```

**Components touched:**
- New `go/` directory with full Go project
- `scripts/` converted to shim scripts
- `bin/` for compiled binary
- `Makefile` for build automation

**Responsibilities:**
- `internal/state/` - All state file operations with locking
- `internal/command/` - CLI parsing and subcommand dispatch
- `internal/schema/` - JSON schema validation wrapper
- `internal/tui/` - Interactive terminal UI

**Failure handling:**
- File locking failures → retry with backoff, then error
- Schema validation failures → detailed error to stderr, exit 1
- Corrupted state → attempt recovery, backup, then error if unrecoverable

---

## Decisions

| Decision | Rationale | ADR |
|----------|-----------|-----|
| Use santhosh-tekuri/jsonschema | Full spec compliance, actively maintained | [ADR-0001](../adrs/ADR-0001-json-schema-library.md) |
| Use cobra for CLI | De facto standard for Go CLIs | (inline - standard choice) |
| Use bubbletea/lipgloss for TUI | Industry standard for Go TUIs | (inline - standard choice) |
| Use flock for file locking | Standard POSIX approach, Go native | (inline - standard choice) |

---

## Open Questions

### Blocking
(None identified - all critical questions answered in discovery)

### Non-blocking
- Exact Homebrew tap naming convention
- Whether to support Windows in future (currently out of scope)

### Not Applicable (CLI Tool)
- HTTP endpoints/API schemas - this is a CLI tool, not a web service
- Authentication/authorization - local CLI, agents call directly via filesystem
- Database entities/tables - uses JSON files, format defined in invariants

## Agent Handoff
- **What to build:** Single Go binary (`tasker`) with subcommands mirroring all Python scripts. TUI using bubbletea. Shim scripts for backward compatibility.
- **Must preserve:** JSON file formats (state.json, tasks/*.json), exit codes, output formats, CLI argument syntax
- **Blocking conditions:** None

## Artifacts
- **Capability Map:** [tasker-go-port.capabilities.json](./tasker-go-port.capabilities.json)
- **Behavior Model (FSM):** [state-machines/tasker-go-port/](../state-machines/tasker-go-port/) - State machine diagrams
- **Discovery Log:** Archived in `.claude/clarify-session.md`
