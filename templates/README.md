# Templates

This directory contains example files to help you get started with the Task Decomposition Protocol v2.

## Quick Start

```bash
# Create planning directories
mkdir -p project-planning/inputs

# Copy and customize templates
cp templates/spec.md.example project-planning/inputs/spec.md
cp templates/constraints.md.example project-planning/inputs/constraints.md

# Edit with your project details
# Be sure to set TARGET_DIR in spec.md
```

## Template Files

### spec.md.example

The project specification template. Contains sections for:
- Project overview and goals
- Architecture and components
- API contracts and data models
- Non-functional requirements
- Success criteria

**Required field:** `Target Directory:` must specify where code will be written.

### constraints.md.example

The project constraints template. Defines:
- Technology stack (language, frameworks, tools)
- Architecture rules (required and prohibited patterns)
- Code standards (naming, documentation)
- Definition of done

The constraints are parsed by the bundle generator and included in execution bundles.

### task.json.example

Example of an individual task file. Shows the JSON schema with:
- Task identification (id, name, wave)
- Context (domain, capability, spec reference)
- Atoms to implement
- Files to create/modify
- Dependencies (tasks and external)
- Acceptance criteria with verification commands
- Time estimate

Task files are created by the task-author agent during Phase 3 (Definition).

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

## Next Steps

After setting up your inputs:
1. Run `/plan` to decompose your spec into tasks
2. Review the generated tasks in `project-planning/tasks/`
3. Run `/execute` to implement tasks via isolated subagents
