# Plan

Decompose a specification into an executable task DAG.

**IMPORTANT:** This command uses the `orchestrator` skill. You MUST follow the full plan loop defined in `.claude/skills/orchestrator/SKILL.md`, executing ALL phases 0-5 sequentially. Do NOT stop after task definition - Phase 4 (validation) is mandatory.

## FIRST: Automatic Discovery (MANDATORY)

**You MUST immediately perform these actions before asking the user anything:**

1. **Search for existing specification files in project-planning/:**
   ```bash
   find project-planning -name "*.md" -o -name "*.txt" 2>/dev/null | head -10
   ```

2. **Present your findings** to the user with a summary like:
   ```
   ## Planning Discovery

   Found specs: [list what you found, or "none"]

   [Then ask if they want to use an existing spec or provide a new one]
   ```

## Input Required

After discovery, gather any remaining inputs:

1. **Specification** - Paste your requirements directly, or provide a file path. Any format works:
   - PRDs, design docs, Notion exports
   - Bullet lists, freeform descriptions
   - Slack thread summaries, meeting notes
   - Existing README or spec files

2. **Target Directory** - Where the code will be written (e.g., `/path/to/my-project`)

3. **Tech Stack** (optional) - Any constraints like:
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
