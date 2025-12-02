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

## Philosophy

The orchestrator does NOT:
- Track state itself (state.py does this)
- Validate artifacts (state.py does this)
- Compute ready tasks (state.py does this)

The orchestrator ONLY:
- Queries state via `scripts/state.py`
- Dispatches agents based on mode/phase
- Handles user interaction

---

# Plan Mode

Triggered by `/plan`. Runs phases 0-5.

## MANDATORY: Proactive Discovery Phase

**BEFORE asking the user anything**, you MUST perform automatic discovery:

### Step 1: Search for Existing Specifications
```bash
# Search project-planning directory for spec files
find project-planning -name "*.md" -o -name "*.txt" 2>/dev/null | head -10
```

### Step 2: Present Discovery Results

**ALWAYS present what you found** before asking questions:

```markdown
## Planning Discovery

**Existing specs found:**
- `project-planning/my_spec.md` (2.3KB)
- `project-planning/design_doc.md` (5.1KB)

Would you like me to use one of these existing specs, or do you have a new specification to provide?
```

If NO specs found, say so explicitly:
```markdown
## Planning Discovery

No existing specification files found in project-planning/.

I'll need you to provide a specification. You can:
1. Paste requirements directly into chat
2. Provide a file path to an existing document
```

## Plan Inputs

After discovery, gather any remaining inputs:

1. **Specification** - Requirements in any format:
   - Paste directly into chat, OR
   - Provide a file path to an existing doc
   - Accept: PRDs, bullet lists, design docs, freeform descriptions, meeting notes

2. **Target Directory** - Where code will be written (required)

3. **Project Type** - Ask: "Is this a **new project** (create fresh directory) or **enhancing an existing project**?"
   - **New project**: Confirm/create target directory, proceed normally
   - **Existing project**: Run directory structure analysis before proceeding (see below)

4. **Tech Stack** (optional, conversational) - Ask: "Any tech stack requirements?"
   - If yes: note them for the physical-architect
   - If no: let agents infer from spec or use sensible defaults

## Existing Project Analysis

When enhancing an existing project, **BEFORE proceeding to ingestion**, analyze the target directory:

```bash
# Check directory exists
if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: Target directory does not exist"
    exit 1
fi

# Analyze structure
echo "=== Project Structure ==="
tree -L 2 -I 'node_modules|__pycache__|.git|venv|.venv' "$TARGET_DIR" 2>/dev/null || \
    find "$TARGET_DIR" -maxdepth 2 -type f | head -50

# Identify key files
echo "=== Key Configuration Files ==="
for f in package.json pyproject.toml Cargo.toml go.mod Makefile requirements.txt setup.py; do
    [ -f "$TARGET_DIR/$f" ] && echo "Found: $f"
done

# Check for existing patterns
echo "=== Existing Code Patterns ==="
find "$TARGET_DIR" -name "*.py" -o -name "*.ts" -o -name "*.js" | head -5 | while read f; do
    echo "Sample: $f"
done
```

**Present findings to user:**
```markdown
## Existing Project Analysis

**Directory:** /path/to/project
**Stack Detected:** Python (pyproject.toml, src/ layout)
**Key Files:**
- pyproject.toml (dependencies)
- src/main.py (entry point)
- tests/ (existing test suite)

**Considerations:**
- Will integrate with existing module structure
- Should follow established patterns in codebase
- Tests should extend existing test framework

Proceed with planning? (y/n)
```

This analysis informs the physical-architect about existing structure, ensuring new code integrates properly rather than conflicting with established patterns.

## Ingestion: Storing the Spec

First, check if the spec already exists:

```bash
mkdir -p project-planning/inputs
if [ -f project-planning/inputs/spec.md ]; then
    echo "Spec found, proceeding to planning..."
fi
```

**If spec already exists:** Skip ingestion, proceed to logical phase.

**If spec doesn't exist**, ask user for specification, then:

- **User provides a file path** → `cp /path/to/spec project-planning/inputs/spec.md`
- **User pastes content** → Write it to `project-planning/inputs/spec.md`

**Important:** Store the spec exactly as provided - no transformation, summarization, or normalization.

## Plan Phase Dispatch

```bash
# Initialize if no state exists
if [ ! -f project-planning/state.json ]; then
    python3 scripts/state.py init "$TARGET_DIR"
fi

# Check current phase
python3 scripts/state.py status
```

| Phase | Agent | Output | Validation |
|-------|-------|--------|------------|
| `ingestion` | (none) | `inputs/spec.md` (verbatim) | File exists |
| `logical` | **logic-architect** | `artifacts/capability-map.json` | `state.py validate capability_map` |
| `physical` | **physical-architect** | `artifacts/physical-map.json` | `state.py validate physical_map` |
| `definition` | **task-author** | `tasks/*.json` | `state.py load-tasks` |
| `validation` | **task-plan-verifier** | Validation report | `state.py validate-tasks <verdict>` |
| `sequencing` | **plan-auditor** | Updated task phases | DAG is valid |
| `ready` | (done) | Planning complete | — |

**Note on ingestion:** The spec is stored exactly as provided. Tech stack constraints (if any) are passed to physical-architect via state metadata, not a separate constraints file.

## Plan Loop

```python
while phase not in ["ready", "executing", "complete"]:
    1. Query current phase
    2. Spawn appropriate agent WITH FULL CONTEXT (see spawn templates below)
    3. Wait for agent to complete
    4. **VERIFY OUTPUT EXISTS** (critical - see below)
    5. Validate output:
       - For artifacts: state.py validate <artifact>
       - For task validation: state.py validate-tasks <verdict>
    6. If valid: state.py advance
    7. If invalid: Tell agent to fix, re-validate
```

## CRITICAL: Output Verification Before Validation

**MANDATORY STEP:** After each agent completes, you MUST verify its output file exists before attempting validation.

```bash
# After logic-architect completes:
if [ ! -f project-planning/artifacts/capability-map.json ]; then
    echo "ERROR: capability-map.json not written. Agent must retry."
    # Re-spawn the agent with explicit reminder to use Write tool
fi

# After physical-architect completes:
if [ ! -f project-planning/artifacts/physical-map.json ]; then
    echo "ERROR: physical-map.json not written. Agent must retry."
    # Re-spawn the agent with explicit reminder to use Write tool
fi

# After task-author completes:
task_count=$(ls project-planning/tasks/*.json 2>/dev/null | wc -l)
if [ "$task_count" -eq 0 ]; then
    echo "ERROR: No task files written. Agent must retry."
    # Re-spawn the agent with explicit reminder to use Write tool
fi
```

**Why this matters:** Sub-agents may fail silently (e.g., output JSON to conversation instead of writing to file). The orchestrator MUST verify files exist before calling `state.py validate`, otherwise validation will fail with "Artifact not found" which is confusing.

**Recovery procedure:** If file doesn't exist:
1. Check if directory exists: `ls -la project-planning/artifacts/`
2. Re-spawn the agent with this explicit reminder:
   > "IMPORTANT: You must use the Write tool to save the file. Simply outputting JSON to the conversation is NOT sufficient. The file must exist at the specified path."

## Agent Spawn Templates

**CRITICAL:** Each sub-agent is context-isolated. They CANNOT see the orchestrator's conversation or any information you've gathered from the user. You MUST pass ALL relevant context explicitly in the spawn prompt.

### Logical Phase: logic-architect

```
Extract capabilities and behaviors from the specification.

## Context

Target Directory: {TARGET_DIR}
Project Type: {new | existing}
Tech Stack: {user-provided constraints or "none specified"}

## Existing Project Analysis (if applicable)
{paste the project structure analysis you performed}

## Specification Location

The full specification is in: project-planning/inputs/spec.md

Read that file for the complete requirements. The spec has already been stored verbatim.

## Your Task

1. Read project-planning/inputs/spec.md
2. Extract capabilities using I.P.S.O. decomposition
3. Apply phase filtering (Phase 1 only)
4. **CRITICAL: Create directory first**: `mkdir -p project-planning/artifacts`
5. **CRITICAL: Use the Write tool** to save to project-planning/artifacts/capability-map.json
6. **Verify file exists**: `ls -la project-planning/artifacts/capability-map.json`
7. Validate with: python3 scripts/state.py validate capability_map

IMPORTANT: You MUST use the Write tool to save the file. Simply outputting JSON to the conversation is NOT sufficient.
```

### Physical Phase: physical-architect

```
Map behaviors to concrete file paths.

## Context

Target Directory: {TARGET_DIR}
Project Type: {new | existing}
Tech Stack: {user-provided constraints or "infer from capability-map"}

## Existing Project Analysis (if applicable)
{paste the project structure analysis - this informs where new files should go}

## Key Patterns to Follow
{if existing project, list discovered patterns like:
- Source layout: src/ with modules
- Test layout: tests/ mirroring src/
- Naming: snake_case for files
}

## Your Task

1. Read project-planning/artifacts/capability-map.json
2. Map each behavior to file paths respecting existing project structure
3. Add cross-cutting concerns and infrastructure
4. **CRITICAL: Create directory first**: `mkdir -p project-planning/artifacts`
5. **CRITICAL: Use the Write tool** to save to project-planning/artifacts/physical-map.json
6. **Verify file exists**: `ls -la project-planning/artifacts/physical-map.json`
7. Validate with: python3 scripts/state.py validate physical_map

IMPORTANT: You MUST use the Write tool to save the file. Simply outputting JSON to the conversation is NOT sufficient.
```

### Definition Phase: task-author

```
Create individual task files from the physical map.

## Context

Target Directory: {TARGET_DIR}
Project Type: {new | existing}

## Tech Stack Constraints
{user-provided constraints that affect implementation}

## Your Task

1. Read project-planning/artifacts/physical-map.json
2. Read project-planning/artifacts/capability-map.json (for behavior details)
3. **CRITICAL: Create directory first**: `mkdir -p project-planning/tasks`
4. **CRITICAL: Use the Write tool** to save each task file to project-planning/tasks/T001.json, etc.
5. **Verify files exist**: `ls -la project-planning/tasks/`
6. Load tasks with: python3 scripts/state.py load-tasks

IMPORTANT: You MUST use the Write tool to save each file. Simply outputting JSON to the conversation is NOT sufficient.
```

### Validation Phase Details

The `validation` phase runs **task-plan-verifier** to evaluate task definitions:

```bash
# Spawn task-plan-verifier with context
Verify task definitions for planning

Spec: project-planning/inputs/spec.md
Capability Map: project-planning/artifacts/capability-map.json
Tasks Directory: project-planning/tasks/
User Preferences: ~/.claude/CLAUDE.md (if exists)
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
2. Points user to full report: `project-planning/reports/verification-report.md`
3. Waits for user to fix task files
4. Re-runs task-plan-verifier (or user runs `/verify-plan`)
5. Repeats until READY or READY_WITH_NOTES

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

---

# Execute Mode

Triggered by `/execute`. Runs phase 6.

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

## Execute Loop

```bash
while true; do
    # 1. Get ready tasks
    READY=$(python3 scripts/state.py ready-tasks)

    if [ -z "$READY" ]; then
        # Try to advance (might complete)
        python3 scripts/state.py advance
        if [ $? -eq 0 ]; then
            echo "All tasks complete!"
            break
        else
            echo "No ready tasks. Check for blockers."
            python3 scripts/state.py status
            break
        fi
    fi

    # 2. Select task (first ready, or user choice)
    TASK_ID=$(echo "$READY" | head -1 | cut -d: -f1)

    # 3. Generate execution bundle (BEFORE starting task)
    python3 scripts/bundle.py generate $TASK_ID

    # 4. Validate bundle integrity (dependencies + checksums)
    python3 scripts/bundle.py validate-integrity $TASK_ID
    if [ $? -eq 1 ]; then
        echo "Bundle validation failed for $TASK_ID - missing dependencies"
        continue  # Skip to next task
    fi
    # Exit code 2 = warnings (checksum mismatch) - proceed but notify user

    # 5. Mark started
    python3 scripts/state.py start-task $TASK_ID

    # 6. Spawn isolated task-executor subagent with bundle
    # (See "Subagent Spawn" section below)

    # 7. Handle result
    if [ $SUCCESS ]; then
        python3 scripts/state.py complete-task $TASK_ID
    else
        python3 scripts/state.py fail-task $TASK_ID "$ERROR"
    fi

    # 8. Ask to continue (unless --batch mode)
    read -p "Continue? (y/n): " CONTINUE
done
```

## Subagent Spawn

Spawn task-executor with self-contained bundle:

```
Execute task [TASK_ID]

Bundle: project-planning/bundles/[TASK_ID]-bundle.json

The bundle contains everything you need:
- Task definition and acceptance criteria
- Expanded behavior details (what to implement)
- File paths and purposes
- Target directory
- Constraints and patterns to follow
- Dependencies (files from prior tasks)

Instructions:
1. Read the bundle file - it has ALL context
2. Implement behaviors in specified files
3. Run acceptance criteria verification
4. Report: files created, tests passed, any issues
```

The subagent:
- Has NO memory of previous tasks
- Gets full context budget
- Reads ONE file (the bundle) for complete context
- Tracks files for rollback
- Returns structured completion report

## Bundle Contents

The bundle (`project-planning/bundles/T001-bundle.json`) includes:

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
