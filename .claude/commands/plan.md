# Plan

Decompose a specification into an executable task DAG.

## Input Required

You will be asked to provide:
1. **Specification** - Your requirements document (paste or path)
2. **Target Directory** - Where the code will eventually be written
3. **Constraints** (optional) - Tech stack, architecture rules

## What Happens

The planner runs phases 0-6:

```
Phase 0: Ingestion     → spec.md saved
Phase 1: Logical       → capability-map.json (validated)
Phase 2: Physical      → physical-map.json (validated)
Phase 3: Definition    → tasks/*.json created
Phase 4: Validation    → tasks verified against spec & user preferences
Phase 5: Sequencing    → waves assigned, DAG validated
Phase 6: Ready         → state.json shows "ready" phase
```

### Phase 4: Validation

The `task-plan-verifier` agent uses LLM-as-judge to verify each task:

| Dimension | What's Checked |
|-----------|----------------|
| Spec Alignment | Task traces to spec requirements |
| Strategy Alignment | Task fits capability-map decomposition |
| Preference Compliance | Task follows ~/.claude/CLAUDE.md patterns |
| Viability | Scope, dependencies, and criteria are valid |

If issues are found, planning blocks until they're resolved.

## Output

After planning completes, you'll have:

```
project-planning/
├── state.json              # Execution state (ready phase)
├── inputs/
│   ├── spec.md             # Your specification
│   └── constraints.md      # Tech stack rules
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
