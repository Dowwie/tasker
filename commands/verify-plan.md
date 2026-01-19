# Verify Plan

Spawn the `task-plan-verifier` subagent to evaluate task definitions.

## Instruction

Spawn the **task-plan-verifier** agent with:

```
Verify task definitions for planning

Spec: project-planning/inputs/spec.md
Capability Map: project-planning/artifacts/capability-map.json
Tasks Directory: project-planning/tasks/
User Preferences: ~/.claude/CLAUDE.md (if exists)
```

## When to Use

- After editing task files to check alignment
- To re-run validation after fixing BLOCKED issues
- To verify tasks match your `~/.claude/CLAUDE.md` coding preferences

## Verdicts

| Verdict | Meaning |
|---------|---------|
| READY | All tasks pass, can proceed |
| READY_WITH_NOTES | Pass with minor issues documented |
| BLOCKED | Critical issues must be fixed first |
