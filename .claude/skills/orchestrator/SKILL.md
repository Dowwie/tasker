---
name: orchestrator
description: Thin orchestrator for Task Decomposition Protocol v2. Supports two modes - /plan (decompose spec into tasks) and /execute (run tasks via subagents). Delegates all state management to scripts/state.py.
tools:
  - agent
  - bash
  - file_read
  - file_write
---

# Orchestrator v2

A **thin coordination layer** with two distinct modes:
- **Plan Mode** (`/plan`) - Decompose spec into task DAG
- **Execute Mode** (`/execute`) - Run tasks via isolated subagents

## Workflow Integration

The full workflow from requirements to implementation:

```
/specify → /plan → /execute
```

| Skill | Purpose | Input | Output |
|-------|---------|-------|--------|
| `/specify` | Design vision & capabilities | User requirements | `{TARGET}/docs/specs/<slug>.md` + `.capabilities.json` + ADRs |
| `/plan` | DAG construction (files → tasks) | Spec + capability map | Task DAG in `project-planning/tasks/` |
| `/execute` | Implementation | Task DAG | Working code |

**Entry points:**
- **From scratch** → Run `/specify` first to develop spec and extract capabilities
- **Existing spec** → Run `/plan {TARGET}/docs/specs/<slug>.md` directly (may need capability extraction)
- **From /specify** → Run `/plan {TARGET}/docs/specs/<slug>.md` (skips to physical mapping)

## Philosophy

The orchestrator does NOT:
- Track state itself (state.py does this)
- Validate artifacts (state.py does this)
- Compute ready tasks (state.py does this)

The orchestrator ONLY:
- Queries state via `scripts/state.py`
- Dispatches agents based on mode/phase
- Handles user interaction
- **Ensures commits happen** via `scripts/commit-task.sh` (defense in depth)

---

## Directory Initialization (MANDATORY FIRST STEP)

**CRITICAL:** Before ANY other operation in `/plan` or `/execute` mode, the orchestrator MUST initialize the complete directory structure. This is the ONLY place where directories are created - sub-agents assume directories already exist.

### Setup (Run at start of any /plan or /execute session)

Run the initialization script:

```bash
./scripts/init-planning-dirs.sh
```

This creates:
- `project-planning/artifacts/` - For capability-map.json, physical-map.json
- `project-planning/inputs/` - For spec.md
- `project-planning/tasks/` - For T001.json, T002.json, etc.
- `project-planning/reports/` - For task-validation-report.md
- `project-planning/bundles/` - For execution bundles
- `.claude/logs/` - For activity logging

The script outputs the `PLANNING_DIR` absolute path which you must capture and pass to all sub-agents.

**Why centralized initialization?**
1. **Reliability**: Sub-agents run in isolated contexts and directory creation has been unreliable
2. **Single responsibility**: Orchestrator owns the directory structure, sub-agents own the content
3. **Fail-fast**: If directory creation fails, we catch it immediately rather than mid-workflow

**IMPORTANT:** Sub-agents must NOT create directories. They assume the directory structure already exists. If a sub-agent encounters a "directory does not exist" error, it indicates the orchestrator failed to initialize properly.

## Runtime Logging (MANDATORY)

All orchestrator activity and sub-agent activity MUST be logged using `./scripts/log-activity.sh`.

### Logging Script Usage

```bash
./scripts/log-activity.sh <LEVEL> <AGENT> <EVENT> "<MESSAGE>"
```

**Parameters:**
- `LEVEL`: INFO, WARN, ERROR
- `AGENT`: orchestrator, logic-architect, physical-architect, task-author, etc.
- `EVENT`: start, decision, tool, complete, spawn, spawn-complete, phase-transition, validation
- `MESSAGE`: Description of the activity

### Orchestrator Logging Examples

```bash
./scripts/log-activity.sh INFO orchestrator phase-transition "moving from logical to physical"
./scripts/log-activity.sh INFO orchestrator spawn "launching logic-architect for capability extraction"
./scripts/log-activity.sh INFO orchestrator spawn-complete "logic-architect finished with SUCCESS"
./scripts/log-activity.sh INFO orchestrator validation "capability_map - PASSED"
```

### Sub-Agent Logging Instructions

**CRITICAL:** Include these logging instructions in EVERY sub-agent spawn prompt. Sub-agents are context-isolated and will not see this skill file - they must receive logging instructions explicitly.

Add this block to every spawn prompt:

```
## Logging (MANDATORY)

Log your activity using the logging script:

\`\`\`bash
./scripts/log-activity.sh INFO $AGENT_NAME start "Starting task description"
./scripts/log-activity.sh INFO $AGENT_NAME decision "What decision and why"
./scripts/log-activity.sh INFO $AGENT_NAME complete "Outcome description"
./scripts/log-activity.sh WARN $AGENT_NAME warning "Warning message"
./scripts/log-activity.sh ERROR $AGENT_NAME error "Error message"
\`\`\`

Replace $AGENT_NAME with your agent name (e.g., logic-architect, task-executor).
```

## CRITICAL: Path Management

**PLANNING_DIR must be an absolute path.** Sub-agents run in isolated contexts and cannot resolve relative paths correctly.

At the start of any planning session, compute and store:
```bash
# Get absolute path to project-planning directory
PLANNING_DIR="$(pwd)/project-planning"
echo "PLANNING_DIR: $PLANNING_DIR"
```

This `PLANNING_DIR` value (e.g., `/Users/foo/tasker/project-planning`) MUST be passed to every sub-agent spawn. Do NOT use relative paths like `project-planning/` in spawn prompts.

---

# Plan Mode

Triggered by `/plan`. Runs phases 0-6 (spec review through ready).

## MANDATORY: Proactive Discovery Phase

**BEFORE asking the user anything**, you MUST perform automatic discovery. This phase gathers ALL context needed for planning.

### Step 1: Search for Existing Specifications
```bash
# Search for spec files in target project's docs/specs/ (from /specify workflow)
# and in project-planning for raw specs
echo "=== Specs from /specify workflow ==="
find . -path "*/docs/specs/*.md" -not -path "*project-planning*" 2>/dev/null | head -10

echo "=== Specs in project-planning ==="
find project-planning -name "*.md" -o -name "*.txt" 2>/dev/null | head -10
```

**Note:** If user hasn't developed a spec yet, suggest running `/specify` first to create a high-quality spec packet interactively.

### Step 2: Gather Initial Inputs

Present spec discovery results and gather basic inputs:

```markdown
## Planning Discovery

**Existing specs found:** [list what you found, or "none"]

I need a few details to proceed:

1. **Specification** - Use an existing spec above, paste requirements, or provide a file path
2. **Target Directory** - Where will the code be written?
3. **Project Type** - Is this a **new project** or **enhancing an existing project**?
4. **Tech Stack** (optional) - Any specific requirements? (e.g., "Python with FastAPI")
```

### Step 3: Existing Project Analysis (MANDATORY for existing projects)

**If enhancing an existing project**, you MUST analyze the target directory **BEFORE proceeding to ingestion**. This analysis is CRITICAL - sub-agents cannot see the codebase, so you must extract and pass this context to them.

```bash
# Check directory exists
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: Target directory does not exist"
    exit 1
fi

# Analyze structure (capture output for context)
echo "=== Project Structure ==="
tree -L 3 -I 'node_modules|__pycache__|.git|venv|.venv|dist|build|.pytest_cache' "$TARGET_DIR" 2>/dev/null || \
    find "$TARGET_DIR" -maxdepth 3 -type f | head -50

# Identify key configuration files
echo "=== Key Configuration Files ==="
for f in package.json pyproject.toml Cargo.toml go.mod Makefile requirements.txt setup.py tsconfig.json; do
    [ -f "$TARGET_DIR/$f" ] && echo "Found: $f"
done

# Detect source layout patterns
echo "=== Source Layout ==="
for d in src lib app pkg cmd internal; do
    [ -d "$TARGET_DIR/$d" ] && echo "Found directory: $d/"
done

# Detect test layout
echo "=== Test Layout ==="
for d in tests test spec __tests__; do
    [ -d "$TARGET_DIR/$d" ] && echo "Found test directory: $d/"
done

# Sample existing code files to understand patterns
echo "=== Code Samples ==="
find "$TARGET_DIR" \( -name "*.py" -o -name "*.ts" -o -name "*.js" -o -name "*.go" -o -name "*.rs" \) \
    -not -path "*node_modules*" -not -path "*__pycache__*" -not -path "*.venv*" | head -10
```

**Read key files to understand patterns:**
```bash
# Read config files to understand dependencies and structure
[ -f "$TARGET_DIR/pyproject.toml" ] && cat "$TARGET_DIR/pyproject.toml"
[ -f "$TARGET_DIR/package.json" ] && cat "$TARGET_DIR/package.json"

# Sample a few source files to understand coding patterns
# (naming conventions, import style, architecture patterns)
```

**Present findings and store context:**
```markdown
## Existing Project Analysis

**Directory:** /path/to/project
**Stack Detected:** Python 3.11+ (pyproject.toml with uv)
**Source Layout:** src/ with module structure
**Test Layout:** tests/ mirroring src/

**Key Configuration:**
- pyproject.toml: dependencies include fastapi, pydantic, loguru
- Uses ruff for linting, pytest for testing

**Discovered Patterns:**
- Naming: snake_case for files and functions
- Imports: absolute imports from src root
- Architecture: Protocol-based interfaces in src/interfaces/
- Testing: pytest with fixtures in conftest.py

**Key Files:**
- src/main.py (entry point)
- src/interfaces/ (Protocol definitions)
- src/services/ (business logic)
- tests/conftest.py (shared fixtures)

**Integration Considerations:**
- New code must follow existing module structure
- Must use existing Protocols for interfaces
- Tests should extend existing fixtures
- Must pass ruff and existing test suite

Proceed with planning? (y/n)
```

### Step 4: Store Discovery Context

**CRITICAL:** You must retain this analysis for passing to sub-agents. Store it as a structured context block:

```
PROJECT_CONTEXT = """
Directory: {TARGET_DIR}
Project Type: existing
Stack: {detected stack}
Source Layout: {layout pattern}
Test Layout: {test pattern}

Key Patterns:
- {pattern 1}
- {pattern 2}
- {pattern 3}

Integration Requirements:
- {requirement 1}
- {requirement 2}
"""
```

This `PROJECT_CONTEXT` MUST be included in every sub-agent spawn prompt (logic-architect, physical-architect, task-author). Without it, sub-agents will design solutions that conflict with existing code.

## Ingestion: Storing the Spec

First, check if the spec already exists (directory was created during initialization):

```bash
if [ -f "$PLANNING_DIR/inputs/spec.md" ]; then
    echo "Spec found, proceeding to planning..."
fi
```

**If spec already exists:** Skip ingestion, proceed to logical phase.

**If spec doesn't exist**, ask user for specification, then:

- **User provides a file path** → `cp /path/to/spec "$PLANNING_DIR/inputs/spec.md"`
- **User pastes content** → Write it to `$PLANNING_DIR/inputs/spec.md`

**Important:** Store the spec exactly as provided - no transformation, summarization, or normalization.

## Skip Phases for /specify-Generated Specs

If the spec came from `/specify` workflow, it has already been reviewed AND capabilities have been extracted. Check for artifacts:

```bash
# Derive capability map path from spec path
# e.g., /path/to/project/docs/specs/my-feature.md → /path/to/project/docs/specs/my-feature.capabilities.json
SPEC_DIR=$(dirname "$SPEC_PATH")
SPEC_SLUG=$(basename "$SPEC_PATH" .md)
CAPABILITY_MAP="${SPEC_DIR}/${SPEC_SLUG}.capabilities.json"

# Check for /specify artifacts (spec review may be in tasker's .claude/)
SPEC_REVIEW=".claude/spec-review.json"

if [ -f "$CAPABILITY_MAP" ]; then
    echo "Capability map found from /specify workflow: $CAPABILITY_MAP"
    echo "Copying artifacts and skipping to physical phase..."

    # Copy artifacts to planning directory
    cp "$CAPABILITY_MAP" "$PLANNING_DIR/artifacts/capability-map.json"
    [ -f "$SPEC_REVIEW" ] && cp "$SPEC_REVIEW" "$PLANNING_DIR/artifacts/spec-review.json"

    # Check for FSM artifacts from /specify workflow
    FSM_DIR="${SPEC_DIR}/../fsm/${SPEC_SLUG}"
    if [ -d "$FSM_DIR" ]; then
        echo "FSM artifacts found from /specify workflow: $FSM_DIR"
        mkdir -p "$PLANNING_DIR/artifacts/fsm"
        cp -r "$FSM_DIR"/* "$PLANNING_DIR/artifacts/fsm/"

        # Validate FSM artifacts
        python3 scripts/fsm-validate.py validate "$PLANNING_DIR/artifacts/fsm"
        if [ $? -ne 0 ]; then
            echo "WARNING: FSM validation failed. Review artifacts before proceeding."
        fi
    fi

    # Skip spec_review AND logical phases - advance directly to physical
    python3 scripts/state.py set-phase physical
fi
```

**Skip phases when `/specify` artifacts exist:**
- `spec_review` - Skipped (already done by `/specify` Phase 7)
- `logical` - Skipped (capability map already exists from `/specify` Phase 3)

## Plan Phase Dispatch

```bash
# Initialize if no state exists
if [ ! -f "$PLANNING_DIR/state.json" ]; then
    python3 scripts/state.py init "$TARGET_DIR"
fi

# Check current phase
python3 scripts/state.py status
```

| Phase | Agent | Output | Validation | Skip if |
|-------|-------|--------|------------|---------|
| `ingestion` | (none) | `inputs/spec.md` (verbatim) | File exists | — |
| `spec_review` | **spec-reviewer** | `artifacts/spec-review.json` | All critical resolved | `/specify` artifacts exist |
| `logical` | **logic-architect** | `artifacts/capability-map.json` | `validate capability_map` | `/specify` artifacts exist |
| `physical` | **physical-architect** | `artifacts/physical-map.json` | `validate physical_map` | — |
| `definition` | **task-author** | `tasks/*.json` | `load-tasks` | — |
| `validation` | **task-plan-verifier** | Validation report | `validate-tasks <verdict>` | — |
| `sequencing` | **plan-auditor** | Updated task phases | DAG is valid | — |
| `ready` | (done) | Planning complete | — | — |

**Entry points based on artifacts:**
- **No artifacts** → Start at `ingestion` (raw spec provided)
- **Has `.capabilities.json`** → Start at `physical` (came from `/specify`)

**Note:** When starting at `physical`, the capability-map.json is copied from `{TARGET}/docs/specs/<slug>.capabilities.json` to `artifacts/capability-map.json`.

## Plan Loop

```python
while phase not in ["ready", "executing", "complete"]:
    1. Query current phase
    2. Spawn appropriate agent WITH FULL CONTEXT (see spawn templates below)
    3. Wait for agent to complete
    4. **VERIFY OUTPUT EXISTS** (critical - see below)
       - DO NOT log spawn-complete until file verified
       - DO NOT proceed to validation until file verified
    5. If file missing: RE-SPAWN agent immediately (see recovery below)
    6. Validate output:
       - For artifacts: state.py validate <artifact>
       - For task validation: state.py validate-tasks <verdict>
    7. If valid: state.py advance
    8. If invalid: Tell agent to fix, re-validate
```

**CRITICAL: Never log "spawn-complete: SUCCESS" until the output file is verified to exist!**

## CRITICAL: Output Verification Before Validation

**MANDATORY STEP:** After each agent completes, you MUST verify its output file exists before attempting validation.

Note: `$PLANNING_DIR` below refers to the absolute path you passed to the agent (e.g., `/Users/foo/tasker/project-planning`).

```bash
# After logic-architect completes:
if [ ! -f $PLANNING_DIR/artifacts/capability-map.json ]; then
    echo "ERROR: capability-map.json not written. Agent must retry."
    # Re-spawn the agent with explicit reminder to use Write tool
fi

# After physical-architect completes:
if [ ! -f $PLANNING_DIR/artifacts/physical-map.json ]; then
    echo "ERROR: physical-map.json not written. Agent must retry."
    # Re-spawn the agent with explicit reminder to use Write tool
fi

# After task-author completes:
task_count=$(ls $PLANNING_DIR/tasks/*.json 2>/dev/null | wc -l)
if [ "$task_count" -eq 0 ]; then
    echo "ERROR: No task files written. Agent must retry."
    # Re-spawn the agent with explicit reminder to use Write tool
fi
```

**Why this matters:** Sub-agents may fail silently (e.g., output JSON to conversation instead of writing to file, or write to wrong directory). The orchestrator MUST verify files exist at the correct absolute path before calling `state.py validate`, otherwise validation will fail with "Artifact not found" which is confusing.

**Recovery procedure:** If file doesn't exist:
1. Check if directory exists: `ls -la $PLANNING_DIR/artifacts/`
2. Re-spawn the agent with this explicit reminder:
   > "IMPORTANT: You must use the Write tool to save the file to the absolute path {PLANNING_DIR}/artifacts/. Simply outputting JSON to the conversation is NOT sufficient. Do NOT use relative paths like project-planning/."

## Agent Spawn Templates

**CRITICAL:** Each sub-agent is context-isolated. They CANNOT see the orchestrator's conversation or any information you've gathered from the user. You MUST pass ALL relevant context explicitly in the spawn prompt.

**MANDATORY:** For existing projects, you MUST include the full PROJECT_CONTEXT from the discovery phase. Sub-agents have no visibility into the target codebase - they rely entirely on the context you provide.

### Spec Review Phase: spec-reviewer

**Spawn prompt for spec-reviewer:**

```
Analyze the specification for weaknesses before capability extraction.

## Logging (MANDATORY)

Log your activity using the logging script:

./scripts/log-activity.sh INFO spec-reviewer start "Analyzing spec for weaknesses"
./scripts/log-activity.sh INFO spec-reviewer decision "What decision and why"
./scripts/log-activity.sh INFO spec-reviewer complete "Outcome description"

## Context

PLANNING_DIR: {absolute path to project-planning, e.g., /Users/foo/tasker/project-planning}

## Your Task

1. Run weakness detection:
   python3 scripts/spec-review.py analyze {PLANNING_DIR}/inputs/spec.md

2. Save results to {PLANNING_DIR}/artifacts/spec-review.json

3. For CRITICAL weaknesses (W1: Non-behavioral, W6: Contradictions):
   - Use AskUserQuestion tool to engage user for resolution
   - Record resolutions to {PLANNING_DIR}/artifacts/spec-resolutions.json

4. Check status:
   python3 scripts/spec-review.py status {PLANNING_DIR}

CRITICAL WEAKNESS CATEGORIES:
- W1: Non-behavioral (DDL/schema not stated as behavior) - Ask: "Should DDL be DB-level or app-layer?"
- W6: Contradictions (conflicting statements) - Ask: "Which statement is authoritative?"

WARNING CATEGORIES (proceed with notes):
- W2: Implicit requirements
- W3: Cross-cutting concerns
- W5: Fragmented requirements

INFO CATEGORIES (log only):
- W4: Missing acceptance criteria

IMPORTANT:
- You MUST resolve all CRITICAL weaknesses before signaling completion
- Use AskUserQuestion for each critical weakness requiring user input
- Save resolutions in spec-resolutions.json for the logic-architect to consume

**Note:** The orchestrator has already created all required directories. Do NOT create directories yourself.
```

### Logical Phase: logic-architect

**Spawn prompt for logic-architect:**

```
Extract capabilities and behaviors from the specification.

## Logging (MANDATORY)

Log your activity using the logging script:

./scripts/log-activity.sh INFO logic-architect start "Extracting capabilities from spec"
./scripts/log-activity.sh INFO logic-architect decision "What decision and why"
./scripts/log-activity.sh INFO logic-architect complete "Outcome description"

## Context

PLANNING_DIR: {absolute path to project-planning, e.g., /Users/foo/tasker/project-planning}
Target Directory: {TARGET_DIR}
Project Type: {new | existing}
Tech Stack: {user-provided constraints or "none specified"}

## Project Context (CRITICAL for existing projects)

{INSERT FULL PROJECT_CONTEXT HERE - this is MANDATORY for existing projects}

Example for existing project:
"""
Directory: /Users/foo/my-app
Project Type: existing
Stack: Python 3.11+ with FastAPI, managed by uv
Source Layout: src/ with module packages
Test Layout: tests/ mirroring src structure

Key Patterns:
- Naming: snake_case for files and functions
- Imports: absolute imports from src root
- Architecture: Protocol-based interfaces in src/interfaces/
- Error handling: Custom exceptions in src/exceptions.py
- Logging: loguru with structured logging

Integration Requirements:
- New capabilities must define Protocols in src/interfaces/
- Implementations go in src/services/ or src/domain/
- Must not duplicate existing functionality
- Must integrate with existing error handling patterns
"""

For new projects, state: "New project - no existing patterns to follow"

## Specification Location

The full specification is in: {PLANNING_DIR}/inputs/spec.md

Read that file for the complete requirements. The spec has already been stored verbatim.

## Your Task

1. Read {PLANNING_DIR}/inputs/spec.md
2. Read {PLANNING_DIR}/artifacts/spec-resolutions.json if it exists
   - Resolutions marked "mandatory" MUST become explicit behaviors
   - Non-behavioral requirements (W1) should be tagged for explicit tasks
   - Cross-cutting concerns (W3) should be flagged for dedicated capabilities
3. **For existing projects:** Consider how new capabilities integrate with existing structure
4. Extract capabilities using I.P.S.O. decomposition
5. Apply phase filtering (Phase 1 only)
6. **CRITICAL: Use the Write tool** to save to {PLANNING_DIR}/artifacts/capability-map.json
7. **Verify file exists**: `ls -la {PLANNING_DIR}/artifacts/capability-map.json`
8. Validate with: `cd {PLANNING_DIR}/.. && python3 scripts/state.py validate capability_map`

IMPORTANT - YOUR TASK IS NOT COMPLETE UNTIL:
1. You MUST use the Write tool to save the file. Simply outputting JSON to the conversation is NOT sufficient.
2. Use the PLANNING_DIR absolute path provided above. Do NOT use relative paths.
3. After Write, you MUST verify: `ls -la {PLANNING_DIR}/artifacts/capability-map.json` - if file doesn't exist, Write again!
4. You MUST run validation and confirm it passes.
5. For existing projects, ensure capabilities don't duplicate what already exists in the codebase.

**Note:** The orchestrator has already created all required directories. Do NOT create directories yourself. If you encounter "directory does not exist" errors, report this to the orchestrator.

If you complete without the file existing at the absolute path, you have FAILED.
```

### Physical Phase: physical-architect

**Spawn prompt for physical-architect:**

```
Map behaviors to concrete file paths.

## Logging (MANDATORY)

Log your activity using the logging script:

./scripts/log-activity.sh INFO physical-architect start "Mapping behaviors to file paths"
./scripts/log-activity.sh INFO physical-architect decision "What decision and why"
./scripts/log-activity.sh INFO physical-architect complete "Outcome description"

## Context

PLANNING_DIR: {absolute path to project-planning, e.g., /Users/foo/tasker/project-planning}
Target Directory: {TARGET_DIR}
Project Type: {new | existing}
Tech Stack: {user-provided constraints or "infer from capability-map"}

## Project Context (CRITICAL for existing projects)

{INSERT FULL PROJECT_CONTEXT HERE - this is MANDATORY for existing projects}

This context tells you:
- Where source files should go (e.g., src/services/, src/domain/)
- Where tests should go (e.g., tests/ mirroring src/)
- Naming conventions to follow
- Existing patterns to integrate with
- Files/modules that already exist (don't recreate them)

For new projects, state: "New project - establish sensible conventions"

## Your Task

1. Read {PLANNING_DIR}/artifacts/capability-map.json
2. **For existing projects:** Map behaviors to paths that FIT the existing structure
   - Use existing directories (don't create parallel structures)
   - Follow established naming conventions
   - Integrate with existing modules where appropriate
3. For new projects: Establish clean, conventional structure
4. Add cross-cutting concerns and infrastructure
5. **CRITICAL: Use the Write tool** to save to {PLANNING_DIR}/artifacts/physical-map.json
6. **Verify file exists**: `ls -la {PLANNING_DIR}/artifacts/physical-map.json`
7. Validate with: `cd {PLANNING_DIR}/.. && python3 scripts/state.py validate physical_map`

IMPORTANT - YOUR TASK IS NOT COMPLETE UNTIL:
1. You MUST use the Write tool to save the file. Simply outputting JSON to the conversation is NOT sufficient.
2. Use the PLANNING_DIR absolute path provided above. Do NOT use relative paths.
3. After Write, you MUST verify: `ls -la {PLANNING_DIR}/artifacts/physical-map.json` - if file doesn't exist, Write again!
4. You MUST run validation and confirm it passes.
5. For existing projects, respect the established structure - don't fight it.

**Note:** The orchestrator has already created all required directories. Do NOT create directories yourself. If you encounter "directory does not exist" errors, report this to the orchestrator.

If you complete without the file existing at the absolute path, you have FAILED.
```

### Definition Phase: task-author

**Spawn prompt for task-author:**

```
Create individual task files from the physical map.

## Logging (MANDATORY)

Log your activity using the logging script:

./scripts/log-activity.sh INFO task-author start "Creating task files from physical map"
./scripts/log-activity.sh INFO task-author decision "What decision and why"
./scripts/log-activity.sh INFO task-author complete "Outcome description"

## Context

PLANNING_DIR: {absolute path to project-planning, e.g., /Users/foo/tasker/project-planning}
Target Directory: {TARGET_DIR}
Project Type: {new | existing}

## Project Context (for existing projects)

{INSERT FULL PROJECT_CONTEXT HERE if existing project}

Key information for task definitions:
- Testing patterns (what framework, where fixtures live)
- Linting/formatting requirements (ruff, eslint, etc.)
- Build/run commands (make test, uv run pytest, npm test)
- Integration points with existing code

## Tech Stack Constraints
{user-provided constraints that affect implementation}

## Your Task

1. Read {PLANNING_DIR}/artifacts/physical-map.json
2. Read {PLANNING_DIR}/artifacts/capability-map.json (for behavior details)
3. **Check for FSM artifacts**: If {PLANNING_DIR}/artifacts/fsm/index.json exists:
   - Read the FSM index and transitions files
   - For each task, identify which FSM transitions it covers
   - Add `state_machine` field to task definitions (see FSM Integration below)
   - Generate FSM-aware acceptance criteria
4. **For existing projects:** Include acceptance criteria that verify integration:
   - Tests pass with existing test suite
   - Linting passes (ruff, eslint, etc.)
   - New code follows established patterns
5. **CRITICAL: Use the Write tool** to save each task file to {PLANNING_DIR}/tasks/T001.json, etc.
6. **Verify files exist**: `ls -la {PLANNING_DIR}/tasks/`
7. Load tasks with: `cd {PLANNING_DIR}/.. && python3 scripts/state.py load-tasks`

## FSM Integration (if FSM artifacts exist)

When FSM artifacts are present at {PLANNING_DIR}/artifacts/fsm/:

### Add state_machine field to tasks
```json
{
  "id": "T001",
  "name": "Implement credential validation",
  ...
  "state_machine": {
    "transitions_covered": ["TR1", "TR2"],
    "guards_enforced": ["I1"],
    "states_reached": ["S2", "S3"]
  }
}
```

### Generate FSM-aware acceptance criteria
For each transition covered:
- "Transition TR1 (S1→S2) fires when trigger X occurs"
- "Guard I1 prevents invalid transition when condition Y fails"

Example:
```json
{
  "criterion": "Transition TR1: Unauthenticated→Validating fires on login request",
  "verification": "pytest tests/auth/test_fsm.py::test_tr1_login_triggers_validation"
},
{
  "criterion": "Guard I1: Invalid email format prevents authentication",
  "verification": "pytest tests/auth/test_fsm.py::test_guard_invalid_email_blocked"
}
```

### Coverage validation
After creating tasks, verify FSM coverage:
```bash
python3 scripts/fsm-validate.py coverage \
    {PLANNING_DIR}/artifacts/fsm/index.json \
    {PLANNING_DIR}/artifacts/capability-map.json
```

Every transition should have at least one task covering it.

IMPORTANT - YOUR TASK IS NOT COMPLETE UNTIL:
1. You MUST use the Write tool to save each file. Simply outputting JSON to the conversation is NOT sufficient.
2. Use the PLANNING_DIR absolute path provided above. Do NOT use relative paths.
3. After Write, you MUST verify: `ls {PLANNING_DIR}/tasks/*.json | wc -l` - if count is 0, Write again!
4. You MUST run load-tasks and confirm it succeeds.
5. For existing projects, tasks must include verification that new code integrates cleanly.

**Note:** The orchestrator has already created all required directories. Do NOT create directories yourself. If you encounter "directory does not exist" errors, report this to the orchestrator.

If you complete without task files existing at the absolute path, you have FAILED.
```

### Validation Phase Details

**Automated Planning Gates (Pre-Check)**

Before advancing from `definition` to `validation` phase, the system automatically runs programmatic validation gates:

```bash
# This happens automatically when calling: python3 scripts/state.py advance
# The following gates are checked:
# - Spec coverage ≥ 90% (tasks cover requirements)
# - No phase leakage (Phase 2+ content not in Phase 1 tasks)
# - All task dependencies reference existing tasks
# - Acceptance criteria quality (no vague terms like "works correctly")
```

If any gate fails, phase advancement is blocked:
```
Planning gates failed: Spec coverage 75.0% below threshold 90.0%; Acceptance criteria quality issues: 3 problem(s)
Run 'python3 scripts/validate.py planning-gates' for details
```

Results are stored in `state.json → artifacts.validation_results` for observability.

**LLM-as-Judge Verification**

After programmatic gates pass, the `validation` phase runs **task-plan-verifier** to evaluate task definitions.

**Spawn prompt for task-plan-verifier:**

```
Verify task definitions for planning

## Logging (MANDATORY)

Log your activity using the logging script:

./scripts/log-activity.sh INFO task-plan-verifier start "Verifying task definitions"
./scripts/log-activity.sh INFO task-plan-verifier decision "What decision and why"
./scripts/log-activity.sh INFO task-plan-verifier complete "Outcome description"

## Context

PLANNING_DIR: {absolute path to project-planning}
Spec: {PLANNING_DIR}/inputs/spec.md
Capability Map: {PLANNING_DIR}/artifacts/capability-map.json
Tasks Directory: {PLANNING_DIR}/tasks/
User Preferences: ~/.claude/CLAUDE.md (if exists)

## Required Command

Register verdict using: python3 scripts/state.py validate-tasks <VERDICT> "<summary>"
(IMPORTANT: The command is validate-tasks, NOT validate-complete or any other variant)
```

The verifier:
1. Evaluates all tasks against spec, strategy, and user preferences
2. Registers its verdict via `python3 scripts/state.py validate-tasks <VERDICT> ...`
3. Returns a detailed report

Verdicts:
- `READY` - All tasks pass, proceed to sequencing
- `READY_WITH_NOTES` - Tasks pass with minor issues noted
- `BLOCKED` - Critical issues found, must fix before continuing

After the verifier completes, the orchestrator:
```bash
# Check if we can advance (verifier already registered verdict)
python3 scripts/state.py advance
```

If BLOCKED, the orchestrator:
1. Displays the verifier's summary to user
2. Points user to full report: `{PLANNING_DIR}/reports/task-validation-report.md`
3. Waits for user to fix task files
4. Re-runs task-plan-verifier (or user runs `/verify-plan`)
5. Repeats until READY or READY_WITH_NOTES

### Sequencing Phase: plan-auditor

**Spawn prompt for plan-auditor:**

```
Assign phases to tasks and validate the dependency graph.

## Logging (MANDATORY)

Log your activity using the logging script:

./scripts/log-activity.sh INFO plan-auditor start "Assigning phases and validating DAG"
./scripts/log-activity.sh INFO plan-auditor decision "What decision and why"
./scripts/log-activity.sh INFO plan-auditor complete "Outcome description"

## Context

PLANNING_DIR: {absolute path to project-planning, e.g., /Users/foo/tasker/project-planning}

## Your Task

1. Read {PLANNING_DIR}/tasks/*.json
2. Read {PLANNING_DIR}/artifacts/capability-map.json (for steel thread flows)
3. Build dependency graph
4. Assign phases (1: foundations, 2: steel thread, 3+: features)
5. **CRITICAL: Update task files** using Write tool to {PLANNING_DIR}/tasks/T001.json etc.
6. Validate DAG (no cycles, deps in earlier phases)
7. Run: cd {PLANNING_DIR}/.. && python3 scripts/state.py load-tasks

IMPORTANT:
- You MUST update task files using the Write tool or jq.
- Use the PLANNING_DIR absolute path provided above. Do NOT use relative paths.
```

The report file contains:
- Per-task evaluations with evidence
- Specific issues and fix suggestions
- User preference violations (from ~/.claude/CLAUDE.md)

## Plan Completion

When phase reaches "ready":
```markdown
## Planning Complete ✓

**Tasks:** 24
**Phases:** 4
**Steel Thread:** T001 → T003 → T007 → T012

Run `/execute` to begin implementation.
```

### Archive Planning Artifacts (Automatic)

After planning completes, archive the artifacts for post-hoc analysis:

```bash
# Archive planning artifacts
python3 scripts/archive.py planning {project_name}
```

This creates:
```
archive/{project_name}/planning/{timestamp}/
├── inputs/         # spec.md (verbatim)
├── artifacts/      # capability-map.json, physical-map.json, spec-review.json
├── tasks/          # T001.json, T002.json, ...
├── reports/        # task-validation-report.md
├── state.json      # State snapshot
└── archive-manifest.json
```

The archive preserves full context for:
- Gap analysis after implementation
- Debugging spec coverage issues
- Auditing planning decisions
- Restoring planning state if needed

---

# Execute Mode

Triggered by `/execute`. Runs task execution phase, followed by optional compliance check.

## Execute Inputs

Ask user for (or detect from current directory):
1. **Planning Directory** - Where `project-planning/state.json` lives
2. **Target Directory** - Where code will be written

## Execute Prerequisites

```bash
# Verify planning is complete
python3 scripts/state.py status
# Phase must be: ready, executing, or have tasks

# Verify target directory
[ -d "$TARGET_DIR" ] || echo "Target directory not found"
```

## Git Repository Initialization (MANDATORY)

**Before any implementation begins**, check if the target repository has git initialized. If not, initialize it:

```
Bash: ./scripts/ensure-git.sh "$TARGET_DIR"
```

**Why this is required:**
- Enables automatic commit hooks to track changes per task
- Supports rollback capability if tasks fail
- Provides audit trail of all implementation changes
- Required for the post-task-commit hook to function

## Recovery on Start (CRITICAL)

**Before starting the execute loop**, always check for and recover from a previous crash:

```bash
# Check for existing checkpoint from previous run
python3 scripts/state.py checkpoint status

# If checkpoint exists and is active, recover
python3 scripts/state.py checkpoint recover

# This will:
# 1. Find tasks that completed (have result files) but weren't acknowledged
# 2. Identify orphaned tasks (still "running" with no result file)
# 3. Update checkpoint state accordingly
```

If orphaned tasks are found, ask user:
```markdown
Found 3 orphaned tasks from previous run:
- T019, T011, T006

Options:
1. Retry orphaned tasks (reset to pending)
2. Skip orphaned tasks (mark failed)
3. Abort and investigate
```

To retry orphaned tasks:
```bash
python3 scripts/state.py retry-task T019
python3 scripts/state.py retry-task T011
python3 scripts/state.py retry-task T006
python3 scripts/state.py checkpoint clear
```

## Execute Loop

**CRITICAL CONSTRAINTS:**
- **Max 3 parallel executors** - More causes orchestrator context exhaustion
- **Task-executors are self-completing** - They call `state.py complete-task` directly
- **Checkpoint before spawning** - Track batch for crash recovery
- **Minimal returns** - Executors return only `T001: SUCCESS` or `T001: FAILED - reason`

```bash
PARALLEL_LIMIT=3

while true; do
    # 0. CHECK FOR HALT
    python3 scripts/state.py check-halt
    if [ $? -ne 0 ]; then
        echo "Halt requested. Stopping gracefully."
        python3 scripts/state.py checkpoint complete
        python3 scripts/state.py confirm-halt
        break
    fi

    # 1. Get ready tasks
    READY=$(python3 scripts/state.py ready-tasks)

    if [ -z "$READY" ]; then
        python3 scripts/state.py advance
        if [ $? -eq 0 ]; then
            echo "All tasks complete!"
            python3 scripts/state.py checkpoint clear
            break
        else
            echo "No ready tasks. Check for blockers."
            python3 scripts/state.py status
            break
        fi
    fi

    # 2. Select batch (up to PARALLEL_LIMIT tasks)
    BATCH=$(echo "$READY" | head -$PARALLEL_LIMIT | cut -d: -f1)
    BATCH_ARRAY=($BATCH)
    echo "Batch: ${BATCH_ARRAY[@]}"

    # 3. Generate and validate bundles for all tasks in batch
    VALID_TASKS=()
    for TASK_ID in ${BATCH_ARRAY[@]}; do
        python3 scripts/bundle.py generate $TASK_ID
        python3 scripts/bundle.py validate-integrity $TASK_ID
        INTEGRITY_CODE=$?

        if [ $INTEGRITY_CODE -eq 0 ]; then
            # Bundle valid - add to execution batch
            VALID_TASKS+=($TASK_ID)
        elif [ $INTEGRITY_CODE -eq 2 ]; then
            # WARNING: Artifacts changed since bundle creation - regenerate
            ./scripts/log-activity.sh WARN orchestrator validation "$TASK_ID: Bundle drift detected, regenerating"
            python3 scripts/bundle.py generate $TASK_ID
            VALID_TASKS+=($TASK_ID)
        else
            # CRITICAL: Missing dependencies or validation failure
            ./scripts/log-activity.sh ERROR orchestrator validation "$TASK_ID: Bundle integrity FAILED"
            python3 scripts/state.py fail-task $TASK_ID "Bundle integrity validation failed" --category dependency
            # Task will be skipped from this batch
        fi
    done

    # Update batch to only include valid tasks
    if [ ${#VALID_TASKS[@]} -eq 0 ]; then
        ./scripts/log-activity.sh ERROR orchestrator batch "No valid tasks in batch - all failed integrity check"
        continue  # Skip to next batch
    fi
    BATCH_ARRAY=(${VALID_TASKS[@]})

    # 4. CREATE CHECKPOINT before spawning (crash recovery)
    python3 scripts/state.py checkpoint create ${BATCH_ARRAY[@]}

    # 5. Mark all tasks as started
    for TASK_ID in ${BATCH_ARRAY[@]}; do
        python3 scripts/state.py start-task $TASK_ID
    done

    # 6. SPAWN EXECUTORS IN PARALLEL
    # Use Task tool with multiple invocations in single message
    # Each executor gets: PLANNING_DIR and Bundle path
    # Each executor returns: "T001: SUCCESS" or "T001: FAILED - reason"

    # 7. AS EACH EXECUTOR RETURNS, update checkpoint
    # Parse the minimal return: "T001: SUCCESS" -> task_id=T001, status=success
    # NOTE: Commits are handled automatically by PostToolUse hook (.claude/hooks/post-task-commit.sh)
    for TASK_ID in ${BATCH_ARRAY[@]}; do
        # Executor returned - check result file exists
        if [ -f "$PLANNING_DIR/bundles/${TASK_ID}-result.json" ]; then
            STATUS=$(python3 -c "import json; print(json.load(open('$PLANNING_DIR/bundles/${TASK_ID}-result.json'))['status'])")
            python3 scripts/state.py checkpoint update $TASK_ID $STATUS
            ./scripts/log-activity.sh INFO orchestrator task-result "$TASK_ID: $STATUS"
        else
            ./scripts/log-activity.sh WARN orchestrator task-result "$TASK_ID: no result file"
        fi
    done

    # 8. COMPLETE CHECKPOINT for this batch
    python3 scripts/state.py checkpoint complete

    # 9. Check for halt AFTER batch
    python3 scripts/state.py check-halt
    if [ $? -ne 0 ]; then
        echo "Halt requested after batch. Stopping gracefully."
        python3 scripts/state.py confirm-halt
        break
    fi

    # 10. Continue to next batch (or ask user in interactive mode)
    # In batch mode: continue automatically
    # In interactive mode: read -p "Continue? (y/n): " CONTINUE
done
```

## Post-Execution Commit (Defense in Depth)

Task file commits are handled **automatically** by a Claude Code hook, ensuring commits happen regardless of executor behavior.

### Hook Configuration

Configured in `.claude/settings.local.json`:
```json
"PostToolUse": [
  {
    "matcher": "Task",
    "hooks": [
      {
        "type": "command",
        "command": ".claude/hooks/post-task-commit.sh",
        "timeout": 30
      }
    ]
  }
]
```

### How It Works

1. **Hook** (`.claude/hooks/post-task-commit.sh`) - Triggered after every Task tool completion
   - Parses task ID from output (e.g., "T001: SUCCESS")
   - Reads `target_dir` from `state.json`
   - Calls the commit script if task succeeded

2. **Script** (`scripts/commit-task.sh`) - Does the actual commit work
   - Reads `bundles/<task_id>-result.json` for file list
   - Checks if files have uncommitted changes
   - If uncommitted: stages and commits with message `<task_id>: <task_name>`
   - If already committed: no-op (idempotent)
   - Updates result file with commit SHA

### Manual Usage

```bash
./scripts/commit-task.sh <task_id> <target_dir> <planning_dir>
```

**Why hook-based (not orchestrator-called):**
- **Automatic** - No orchestrator code to forget/break
- **Single point of control** - Hook config is the source of truth
- **Resilient** - Works regardless of orchestrator implementation
- **Idempotent** - Safe to run multiple times

## Checkpoint Commands Reference

```bash
# Create checkpoint before spawning batch
python3 scripts/state.py checkpoint create T001 T002 T003

# Update after each executor returns
python3 scripts/state.py checkpoint update T001 success
python3 scripts/state.py checkpoint update T002 failed

# Mark batch complete
python3 scripts/state.py checkpoint complete

# Check current checkpoint status
python3 scripts/state.py checkpoint status

# Recover from crash (finds orphans, updates from result files)
python3 scripts/state.py checkpoint recover

# Clear checkpoint (after successful completion or manual cleanup)
python3 scripts/state.py checkpoint clear
```

## Graceful Halt and Resume

The executor supports graceful halt via two mechanisms:

### 1. STOP File (Recommended for External Control)

Create a `STOP` file in the `project-planning/` directory:

```bash
touch project-planning/STOP
```

The executor checks for this file before starting each new task and after completing each task. When detected:
1. Current task (if running) completes normally
2. No new tasks are started
3. State is saved with halt information
4. Clean exit with instructions to resume

### 2. User Message (For Interactive Sessions)

If a user sends "STOP" during an interactive `/execute` session, the orchestrator should:
1. Call `python3 scripts/state.py halt user_message`
2. Allow current task to complete
3. Exit gracefully

### Resuming Execution

To resume after a halt:

```bash
# Check current halt status
python3 scripts/state.py halt-status

# Clear halt and resume
python3 scripts/state.py resume

# Then run /execute again
```

The resume command:
- Removes the STOP file if present
- Clears the halt flag in state
- Logs the resume event

### Halt Status Commands

```bash
# Request halt (called by orchestrator when user sends STOP)
python3 scripts/state.py halt [reason]

# Check if halted (exit code 1 = halted, 0 = ok)
python3 scripts/state.py check-halt

# Confirm halt completed (after executor stops)
python3 scripts/state.py confirm-halt

# Show detailed halt status
python3 scripts/state.py halt-status [--format json]

# Clear halt and resume
python3 scripts/state.py resume
```

## Subagent Spawn

Spawn task-executor with self-contained bundle. **Executors are self-completing** - they update state and write results directly, returning only a minimal status line to the orchestrator.

```
Execute task [TASK_ID]

## Logging (MANDATORY)

Log your activity using the logging script:

```bash
./scripts/log-activity.sh INFO task-executor start "Executing task [TASK_ID]"
./scripts/log-activity.sh INFO task-executor decision "What decision and why"
./scripts/log-activity.sh INFO task-executor tool "Write - creating src/auth/validator.py"
./scripts/log-activity.sh INFO task-executor complete "Outcome description"
./scripts/log-activity.sh ERROR task-executor error "Error message if any"
```

PLANNING_DIR: {absolute path to project-planning, e.g., /Users/foo/tasker/project-planning}
Bundle: {PLANNING_DIR}/bundles/[TASK_ID]-bundle.json

The bundle contains everything you need:
- Task definition and acceptance criteria
- Expanded behavior details (what to implement)
- File paths and purposes
- Target directory
- Constraints and patterns to follow
- Dependencies (files from prior tasks)

## Self-Completion Protocol (CRITICAL)

You are responsible for updating state and persisting results. Do NOT rely on the orchestrator.

### On Success:
1. Track all files you created/modified
2. Call: `python3 scripts/state.py complete-task [TASK_ID] --created file1 file2 --modified file3`
3. Write result file: `{PLANNING_DIR}/bundles/[TASK_ID]-result.json` (see schema below)
4. Return ONLY this line: `[TASK_ID]: SUCCESS`

### On Failure:
1. Call: `python3 scripts/state.py fail-task [TASK_ID] "error message" --category <cat> --retryable`
2. Write result file with error details
3. Return ONLY this line: `[TASK_ID]: FAILED - <one-line reason>`

### Result File Schema
Write to `{PLANNING_DIR}/bundles/[TASK_ID]-result.json`:
```json
{
  "version": "1.0",
  "task_id": "[TASK_ID]",
  "name": "Task name from bundle",
  "status": "success|failed",
  "started_at": "ISO timestamp",
  "completed_at": "ISO timestamp",
  "files": {
    "created": ["path1", "path2"],
    "modified": ["path3"]
  },
  "verification": {
    "verdict": "PASS|FAIL",
    "criteria": [
      {"name": "criterion", "status": "PASS|FAIL", "evidence": "..."}
    ]
  },
  "error": {
    "category": "dependency|compilation|test|validation|runtime",
    "message": "...",
    "retryable": true
  },
  "notes": "Any decisions or observations"
}
```

## Workflow Summary
1. Read the bundle file - it has ALL context
2. Implement behaviors in specified files
3. Run acceptance criteria verification
4. Call state.py to update task status
5. Write detailed result to bundles/[TASK_ID]-result.json
6. Return ONE LINE status to orchestrator

IMPORTANT: Use the PLANNING_DIR absolute path provided above. Do NOT use relative paths.
```

The subagent:
- Has NO memory of previous tasks
- Gets full context budget
- Reads ONE file (the bundle) for complete context
- Tracks files for rollback
- **Calls state.py directly** to mark completion
- **Writes result file** for observability
- Returns **minimal status line** (not full report)

## Bundle Contents

The bundle (`{PLANNING_DIR}/bundles/T001-bundle.json`) includes:

| Field | Purpose |
|-------|---------|
| `task_id`, `name` | Task identification |
| `target_dir` | Where to write code |
| `behaviors` | Expanded behavior details (not just IDs) |
| `files` | Paths, actions, purposes, layers |
| `acceptance_criteria` | Verification commands |
| `constraints` | Tech stack, patterns, testing |
| `dependencies.files` | Files from prior tasks to read |
| `context` | Domain, capability, spec reference |
| `state_machine` | FSM context for adherence verification (if present) |

### FSM Context in Bundle (when present)

If the task has `state_machine` field, the bundle includes expanded FSM details:

```json
{
  "state_machine": {
    "transitions_covered": ["TR1", "TR2"],
    "guards_enforced": ["I1"],
    "states_reached": ["S2", "S3"],
    "transitions_detail": [
      {
        "id": "TR1",
        "from_state": "S1",
        "to_state": "S2",
        "trigger": "validate_credentials",
        "guards": [{"condition": "email_valid", "invariant_id": "I1"}]
      }
    ]
  }
}
```

The task-executor uses this for:
1. Understanding the state transitions being implemented
2. Ensuring guards are properly enforced
3. Writing tests that verify FSM behavior

Generate bundles with:
```bash
python3 scripts/bundle.py generate T001       # Single task
python3 scripts/bundle.py generate-ready      # All ready tasks
```

## Execute Options

| Command | Behavior |
|---------|----------|
| `/execute` | Interactive, one task at a time |
| `/execute T005` | Execute specific task only |
| `/execute --batch` | All ready tasks, no prompts |
| `/execute --parallel 3` | Up to 3 tasks simultaneously |

## Post-Execution Compliance Check (Optional)

After all tasks complete successfully, run the compliance check to verify the implementation matches the spec:

```bash
python3 scripts/compliance-check.py all \
    --spec $PLANNING_DIR/inputs/spec.md \
    --target $TARGET_DIR
```

This checks four categories:
- **V1: Schema Compliance** - DDL elements exist (tables, constraints, indexes)
- **V2: Config Compliance** - Env vars wired to Pydantic fields
- **V3: API Compliance** - Endpoints exist with correct methods
- **V4: Observability** - OTel spans and metrics registered

### Compliance Results

The check outputs a JSON summary and detailed report:
```json
{
  "summary": {
    "total_gaps": 5,
    "by_severity": {"critical": 1, "warning": 2, "info": 2},
    "compliant": false
  }
}
```

### Handling Gaps

If compliance check finds critical gaps:

1. **Display gaps to user** with suggested fixes
2. **Ask user how to proceed**:
   - Fix gaps (create new tasks or manual fixes)
   - Accept gaps (document as known limitations)
   - Investigate (may be false positives)

3. **For fixable gaps**, create ad-hoc tasks:
   ```bash
   # Example: Missing database constraint
   python3 scripts/state.py create-task \
       --name "Add hook_run_unique constraint" \
       --type constraint \
       --files "alembic/versions/xxx_add_constraint.py"
   ```

4. **Save compliance report** for documentation:
   ```bash
   # Report is saved to:
   $PLANNING_DIR/reports/compliance-report.json
   ```

## Archive Execution Artifacts (After Completion)

After execution completes (all tasks done or halted), archive execution artifacts:

```bash
# Archive execution artifacts
python3 scripts/archive.py execution {project_name}
```

This creates:
```
archive/{project_name}/execution/{timestamp}/
├── bundles/        # Task bundles and result files
├── logs/           # Activity logs
├── state.json      # State snapshot
└── archive-manifest.json
```

The archive preserves:
- Task execution results and timing
- Bundle contents (full context for each task)
- Logs for debugging
- State for resumption analysis

### Archive Commands Reference

```bash
# Archive planning artifacts
python3 scripts/archive.py planning {project_name}

# Archive execution artifacts
python3 scripts/archive.py execution {project_name}

# List all archives
python3 scripts/archive.py list

# List archives for specific project
python3 scripts/archive.py list --project {project_name}

# Restore planning state from archive
python3 scripts/archive.py restore {archive_id} --project {project_name}
```

---

# State Commands Reference

```bash
# General
python3 scripts/state.py status          # Current phase, task counts
python3 scripts/state.py advance         # Try to advance phase

# Planning
python3 scripts/state.py init <dir>      # Initialize new plan
python3 scripts/state.py validate <art>  # Validate artifact
python3 scripts/state.py validate-tasks <verdict> [summary] [--issues ...]
                                         # Register task validation result

# Execution
python3 scripts/state.py ready-tasks     # List ready tasks
python3 scripts/state.py start-task <id> # Mark running
python3 scripts/state.py complete-task <id>  # Mark done
python3 scripts/state.py fail-task <id> <e>  # Mark failed
python3 scripts/state.py load-tasks      # Reload from files

# Halt / Resume
python3 scripts/state.py halt [reason]   # Request graceful halt
python3 scripts/state.py check-halt      # Check if halted (exit 1 = halted)
python3 scripts/state.py confirm-halt    # Confirm halt completed
python3 scripts/state.py halt-status     # Show halt status
python3 scripts/state.py resume          # Clear halt, resume execution

# Checkpoint (Crash Recovery)
python3 scripts/state.py checkpoint create <t1> [t2 ...]  # Create batch checkpoint
python3 scripts/state.py checkpoint update <id> <status>  # Update task (success|failed)
python3 scripts/state.py checkpoint complete              # Mark batch done
python3 scripts/state.py checkpoint status                # Show current checkpoint
python3 scripts/state.py checkpoint recover               # Recover orphaned tasks
python3 scripts/state.py checkpoint clear                 # Remove checkpoint

# Bundles
python3 scripts/bundle.py generate <id>   # Generate bundle for task
python3 scripts/bundle.py generate-ready  # Generate all ready bundles
python3 scripts/bundle.py validate <id>   # Validate bundle against schema
python3 scripts/bundle.py validate-integrity <id>  # Check deps + checksums
python3 scripts/bundle.py list            # List existing bundles
python3 scripts/bundle.py clean           # Remove all bundles

# Observability
python3 scripts/state.py log-tokens <s> <i> <o> <c>  # Log usage
```

---

# Error Recovery

## Planning Errors

If agent produces invalid output:
1. Validation fails (`state.py validate` returns non-zero)
2. Report errors to agent
3. Agent fixes and re-outputs
4. Re-validate
5. Only advance on success

## Execution Errors

If task fails:
1. `state.py fail-task` marks it failed
2. Rollback triggered (created files deleted, modified files restored)
3. Dependent tasks auto-blocked
4. Other ready tasks can continue
5. User can retry later: fix issue, then `state.py start-task` again
