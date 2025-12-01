# Templates

This directory contains example files for the Task Decomposition Protocol v2.

## Quick Start

No setup required. Just run `/plan` and the orchestrator will ask for:

1. **Your specification** - paste it or provide a file path
2. **Target directory** - where to write the code
3. **Tech stack** (optional) - any constraints

The spec is stored verbatim. Any format works.

## Template Files

### example-spec.md

An example specification showing one possible format. **You don't need to follow this format.**
The planner accepts any input:
- Freeform requirements
- PRDs or design docs
- Bullet lists
- Meeting notes

This file is purely for inspiration.

### task.json.example

Example of an individual task file. Shows the JSON schema with:
- Task identification (id, name, phase)
- Context (domain, capability, spec reference)
- Behaviors to implement
- Files to create/modify
- Dependencies (tasks and external)
- Acceptance criteria with verification commands
- Time estimate

Task files are created by the task-author agent during Phase 3 (Definition).

### constraints.md.example

Example constraints file. **Optional.** You can provide tech stack constraints conversationally when running `/plan` instead of creating this file.

## Validation

All artifacts are validated against JSON schemas:
- `schemas/capability-map.schema.json`
- `schemas/physical-map.schema.json`
- `schemas/task.schema.json`
- `schemas/state.schema.json`

Run validation manually:
```bash
python3 scripts/state.py validate capability_map
python3 scripts/state.py validate physical_map
```

## Workflow

1. Run `/plan` - provide spec, target dir, optional constraints
2. Review generated tasks in `project-planning/tasks/`
3. Run `/execute` to implement tasks via isolated subagents
