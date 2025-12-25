---
name: tasker-to-beads
description: Transform Tasker task definitions into rich, self-contained Beads issues. Bridges ephemeral execution units with persistent planning artifacts.
tools:
  - bash
  - file_read
  - file_write
---

# Tasker to Beads Transformer

Converts Tasker task definitions into enriched Beads issues that are **self-contained** and **human-readable**. This skill performs the "neural" work of understanding spec context, synthesizing narratives, and creating issues that stand alone.

## When to Use

- After completing Tasker `/plan` phase (state is "ready")
- When you want to track implementation progress in Beads
- When multiple people/agents will work on tasks and need persistent context

## Prerequisites

1. Tasker planning complete (`project-planning/state.json` exists, phase is "ready")
2. Beads initialized in project (`/beads:init` has been run)

---

## Workflow

### Step 1: Prepare Context

```bash
python3 .claude/skills/tasker-to-beads/scripts/transform.py context --all
```

This extracts structural data from task files and saves context bundles to `project-planning/beads-export/`.

### Step 2: Enrich Each Task (LLM Work)

For each task, read the context file and generate an enriched issue description.

**Read the context:**
```bash
cat project-planning/beads-export/{TASK_ID}-context.json
```

**Generate enriched content using the template below**, then save:
```bash
# Save to project-planning/beads-export/{TASK_ID}-enriched.json
```

### Step 3: Create Beads Issues

After enrichment, create issues:
```bash
python3 .claude/skills/tasker-to-beads/scripts/transform.py create {TASK_ID} project-planning/beads-export/{TASK_ID}-enriched.json
```

Or batch create from manifest:
```bash
python3 .claude/skills/tasker-to-beads/scripts/transform.py batch-create project-planning/beads-export/manifest.json
```

---

## Enrichment Template

When generating enriched issue content, produce JSON with this structure:

```json
{
  "task_id": "T001",
  "title": "Initialize Mix project with OTP dependencies",
  "priority": "critical",
  "labels": ["domain:infrastructure", "phase:1", "steel-thread"],
  "dependencies": [],
  "description": "... rich markdown description ..."
}
```

### Description Format

The description should be **self-contained** markdown with these sections:

```markdown
## Overview

[1-2 sentences explaining WHAT this task accomplishes and WHY it matters to the project]

## Spec Context

[Relevant excerpts from the specification that drive this task. Include enough context that someone can understand the requirement without reading the full spec.]

> "Direct quote from spec if available"

[Your synthesis of what the spec requires]

## Architecture Context

**Domain:** [domain name]
**Capability:** [capability name]

[Explain how this fits into the system architecture. What role does it play? What does it enable?]

## Implementation Approach

[Narrative description of how to implement this. Not just file paths, but the approach and key decisions.]

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `path/to/file.ex` | Brief purpose |

## Dependencies

[If this task depends on others, explain WHAT it needs from them, not just the ID]

- **T003 (Core data schemas):** Provides the `Session` and `Message` structs used here
- **T005 (Tool behaviour):** Defines the `Tool` behaviour this implements

## Acceptance Criteria

Human-readable checklist (not raw verification commands):

- [ ] Project compiles without warnings
- [ ] All required dependencies are declared in mix.exs
- [ ] Basic module structure is in place
- [ ] Tests pass (if applicable)

## Notes

[Any additional context, gotchas, or considerations]
```

---

## Enrichment Prompts

When processing each task context, use this reasoning approach:

### 1. Understand the Task's Purpose

Read the task name, context, and behaviors. Ask:
- What problem does this solve?
- Why does this exist in the system?
- What would be missing without it?

### 2. Extract Spec Relevance

From `relevant_spec_sections` in the context:
- Find the most relevant 1-2 sections
- Extract key quotes that define the requirement
- Synthesize the requirement in your own words

### 3. Explain Architecture Fit

From `capability_context`:
- Describe the domain and capability this belongs to
- Explain how the behaviors contribute to the capability
- Connect to the broader system design

### 4. Narrate Dependencies

From `dependency_context`:
- For each dependency, explain what this task USES from it
- Make the dependency relationship meaningful, not just structural

### 5. Humanize Acceptance Criteria

From `task.acceptance_criteria`:
- Convert verification commands to human-readable checks
- Add implicit criteria (code quality, tests, documentation)
- Make them checkable by a human reviewer

---

## Single-Task Mode

To transform a single task:

```bash
# 1. Prepare context
python3 .claude/skills/tasker-to-beads/scripts/transform.py context T001

# 2. Read and understand
cat project-planning/beads-export/T001-context.json

# 3. Generate enriched content (you do this)
# ... apply the enrichment template ...
# Save to project-planning/beads-export/T001-enriched.json

# 4. Create the issue
python3 .claude/skills/tasker-to-beads/scripts/transform.py create T001 project-planning/beads-export/T001-enriched.json
```

---

## Batch Mode

To transform all tasks:

```bash
# 1. Prepare all contexts
python3 .claude/skills/tasker-to-beads/scripts/transform.py context --all

# 2. For each context file, generate enriched content
# This is the neural loop - process each T*-context.json

# 3. Create manifest with all enriched issues
# Save to project-planning/beads-export/manifest.json with structure:
# { "issues": [ {...enriched issue 1...}, {...enriched issue 2...}, ... ] }

# 4. Batch create
python3 .claude/skills/tasker-to-beads/scripts/transform.py batch-create project-planning/beads-export/manifest.json
```

---

## Manifest Format

For batch creation, the manifest should be:

```json
{
  "created_at": "2025-01-15T10:00:00Z",
  "source": "tasker",
  "task_count": 47,
  "issues": [
    {
      "task_id": "T001",
      "title": "Initialize Mix project with OTP dependencies",
      "priority": "critical",
      "labels": ["domain:infrastructure", "phase:1", "steel-thread"],
      "dependencies": [],
      "description": "## Overview\n\n..."
    },
    {
      "task_id": "T002",
      "title": "...",
      ...
    }
  ]
}
```

---

## Quality Checklist

Before creating an issue, verify:

- [ ] Title is concise but descriptive (not just the task name)
- [ ] Overview explains WHY, not just WHAT
- [ ] Spec context includes actual quotes/references
- [ ] Architecture context explains the system role
- [ ] Dependencies are explained narratively
- [ ] Acceptance criteria are human-checkable
- [ ] Description stands alone (no external context needed)

---

## Example: T001 Enrichment

**Input context (abbreviated):**
```json
{
  "task_id": "T001",
  "task": {
    "name": "Mix project initialization",
    "phase": 1,
    "context": {
      "domain": "Infrastructure",
      "capability": "Project Setup",
      "steel_thread": true
    },
    "files": [{"path": "mix.exs", "action": "create"}],
    "acceptance_criteria": [
      {"criterion": "mix compile succeeds", "verification": "mix compile"}
    ]
  },
  "relevant_spec_sections": [
    "## Tech Stack\n\nThe agent framework uses Elixir with OTP patterns..."
  ]
}
```

**Output enriched (abbreviated):**
```json
{
  "task_id": "T001",
  "title": "Initialize Elixir/OTP project foundation",
  "priority": "critical",
  "labels": ["domain:infrastructure", "phase:1", "steel-thread"],
  "dependencies": [],
  "description": "## Overview\n\nEstablish the foundational Mix project structure for the Fathom agent framework. This is the critical first step that all other tasks depend on, providing the build system, dependency management, and basic project layout.\n\n## Spec Context\n\n> \"The agent framework uses Elixir with OTP patterns for fault-tolerant, distributed agent coordination.\"\n\nThe specification mandates Elixir/OTP as the implementation technology, leveraging its supervision trees, GenServers, and distribution capabilities for building resilient agent systems.\n\n## Architecture Context\n\n**Domain:** Infrastructure\n**Capability:** Project Setup\n\nThis task creates the skeleton that all other infrastructure and application code builds upon. It establishes:\n- Dependency versions (ecto, phoenix_pubsub, telemetry, etc.)\n- Compilation settings and warnings configuration\n- Project metadata and structure\n\n## Implementation Approach\n\nCreate a standard Mix project with umbrella-ready structure. Key dependencies include:\n- `ecto` + `postgrex` for persistence\n- `phoenix_pubsub` for inter-process messaging\n- `telemetry` + `opentelemetry` for observability\n- `libcluster` for node discovery\n\n### Files to Create/Modify\n\n| File | Purpose |\n|------|---------|\n| `mix.exs` | Project definition, dependencies, compilation settings |\n| `.formatter.exs` | Code formatting rules |\n| `.gitignore` | Standard Elixir ignores |\n| `README.md` | Project documentation |\n\n## Dependencies\n\nNone - this is the foundation task.\n\n## Acceptance Criteria\n\n- [ ] `mix deps.get` fetches all dependencies successfully\n- [ ] `mix compile --warnings-as-errors` passes\n- [ ] `mix format --check-formatted` passes\n- [ ] All required dependencies declared (ecto, postgrex, phoenix_pubsub, telemetry, libcluster, etc.)\n\n## Notes\n\nThis is on the **steel thread** path - it must complete before any other work can proceed. Keep the initial setup minimal but complete; don't add optional dependencies that aren't immediately needed."
}
```
