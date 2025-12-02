# Plan

Decompose a specification into an executable task DAG.

**IMPORTANT:** This command uses the `orchestrator` skill. You MUST follow the full plan loop defined in `.claude/skills/orchestrator/SKILL.md`, executing ALL phases 0-5 sequentially. Do NOT stop after task definition - Phase 4 (validation) is mandatory.

## FIRST: Automatic Discovery (MANDATORY)

**You MUST immediately perform these actions before asking the user anything:**

### Step 1: Search for Existing Specs
```bash
find project-planning -name "*.md" -o -name "*.txt" 2>/dev/null | head -10
```

### Step 2: Gather Initial Inputs

Present findings and ask:
```
## Planning Discovery

**Existing specs found:** [list what you found, or "none"]

I need a few details:
1. **Specification** - Use existing spec, paste requirements, or provide a file path
2. **Target Directory** - Where will the code be written?
3. **Project Type** - Is this a **new project** or **enhancing an existing project**?
4. **Tech Stack** (optional) - Any specific requirements?
```

### Step 3: Existing Project Analysis (MANDATORY for existing projects)

**If the user says "existing project"**, you MUST analyze the target directory BEFORE proceeding. Sub-agents cannot see the codebase - you must extract this context for them.

```bash
# Analyze structure
tree -L 3 -I 'node_modules|__pycache__|.git|venv|.venv' "$TARGET_DIR" 2>/dev/null

# Find config files
for f in package.json pyproject.toml Cargo.toml go.mod Makefile; do
    [ -f "$TARGET_DIR/$f" ] && echo "Found: $f"
done

# Read key config files to understand stack and patterns
[ -f "$TARGET_DIR/pyproject.toml" ] && cat "$TARGET_DIR/pyproject.toml"
[ -f "$TARGET_DIR/package.json" ] && cat "$TARGET_DIR/package.json"
```

**Present analysis to user:**
```
## Existing Project Analysis

**Directory:** /path/to/project
**Stack Detected:** [what you found]
**Source Layout:** [src/, lib/, etc.]
**Test Layout:** [tests/, __tests__, etc.]

**Discovered Patterns:**
- [naming conventions]
- [architecture patterns]
- [existing modules]

**Integration Considerations:**
- [how new code should fit]

Proceed with planning? (y/n)
```

**Store this as PROJECT_CONTEXT** - you MUST pass it to every sub-agent spawn.

## Input Summary

After discovery, you should have:

1. **Specification** - Requirements in any format:
   - PRDs, design docs, Notion exports
   - Bullet lists, freeform descriptions
   - Existing README or spec files

2. **Target Directory** - Where the code will be written

3. **Project Type** - New or existing (if existing, you have PROJECT_CONTEXT)

4. **Tech Stack** (optional) - Any constraints like:
   - "Python with FastAPI"
   - "Use existing React setup"
   - "Must integrate with PostgreSQL"

## What Happens

Follow the orchestrator's Plan Loop (see `.claude/skills/orchestrator/SKILL.md`):

```
Phase 0: Ingestion     → spec.md saved
Phase 1: Logical       → spawn logic-architect → capability-map.json (validated)
Phase 2: Physical      → spawn physical-architect → physical-map.json (validated)
Phase 3: Definition    → spawn task-author → tasks/*.json created
Phase 4: Validation    → spawn task-plan-verifier → verification-report.md
Phase 5: Sequencing    → spawn plan-auditor → phases assigned, DAG validated
Phase 6: Ready         → state.json shows "ready" phase
```

**Critical:** After each phase, call `python3 scripts/state.py advance` to proceed. The state machine enforces that validation MUST complete before sequencing.

### Phase 4: Validation (Mandatory)

When the phase reaches `validation`, spawn the `task-plan-verifier` agent:

```
Verify task definitions for planning

Spec: project-planning/inputs/spec.md
Capability Map: project-planning/artifacts/capability-map.json
Tasks Directory: project-planning/tasks/
User Preferences: ~/.claude/CLAUDE.md (if exists)
```

The verifier evaluates each task across these dimensions:

| Dimension | What's Checked |
|-----------|----------------|
| Spec Alignment | Task traces to spec requirements |
| Strategy Alignment | Task fits capability-map decomposition |
| Preference Compliance | Task follows ~/.claude/CLAUDE.md patterns |
| Viability | Scope, dependencies, and criteria are valid |

The verifier registers its verdict via `python3 scripts/state.py validate-tasks <VERDICT>`. If BLOCKED, planning halts until issues are fixed and `/verify-plan` is re-run.

## Output

After planning completes, you'll have:

```
project-planning/
├── state.json              # Execution state (ready phase)
├── inputs/
│   └── spec.md             # Your specification (verbatim)
├── artifacts/
│   ├── capability-map.json # What the system does
│   └── physical-map.json   # Where code lives
├── reports/
│   └── verification-report.md  # Task verification results
└── tasks/
    ├── T001.json           # Individual task definitions
    ├── T002.json
    └── ...
```

## Next Step

After planning, run `/execute` to implement the tasks.

## Commands

```bash
# Check planning status
python3 scripts/state.py status

# View the task list
ls project-planning/tasks/

# See task details
cat project-planning/tasks/T001.json | jq .

# Re-run task validation after fixes
python3 scripts/state.py validate-tasks READY "All tasks aligned"
```
