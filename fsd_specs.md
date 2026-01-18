
# Spec: Add Behavior State Machine Modeling to Tasker `/specify` with QA in `/plan` + `/execute`
Handoff-Ready: No (until Open Questions closed)

## Goal
Make **state machine behavior models** a first-class, mandatory artifact produced by `/specify`, exported to the target project, and used by `/plan` and `/execute` as **quality assurance gates** to ensure implementation adheres to the intended behavior (“vision + adherence”).

## Non-goals
- Not a code-extraction / reverse-engineering feature (no file:line evidence requirements in `/specify`)
- Not a new state-machine runtime library recommendation
- Not a full formal verification system
- Not a replacement for existing Spec Packet, ADR, or Capability Map outputs

## Done means
- `/specify` always produces a **Behavior Model** (state machine) for the Steel Thread workflow, plus any additional models triggered by complexity rules.
- The Behavior Model is exported to the target project under a stable directory (`docs/fsm/…` by default).
- `/plan` reads the exported model(s) and enforces **coverage + completeness checks** before producing a plan.
- `/execute` uses the model(s) to run **adherence checks** during implementation (at minimum: test/verification checklist coverage per transition + invariant/guard enforcement).
- If the model is ambiguous or incomplete, `/specify` loops with `AskUserQuestion` until the model is handoff-ready.

---

## Workflows

### W1: `/specify` produces Behavior Models (mandatory)
1. `/specify` completes Phase 1–3 as usual (Scope → Clarify → Synthesis).
2. In Synthesis, `/specify` compiles the **Behavior Model** from:
   - Workflows
   - Invariants
   - Interfaces
   - Open Questions
3. `/specify` MUST produce at least:
   - One workflow-level state machine for the **Steel Thread**
4. `/specify` runs Behavior Model validation checks (see “Invariants” + “Gate”).
5. If validation fails due to ambiguity, `/specify` uses `AskUserQuestion` to close gaps.
6. On export, `/specify` writes Behavior Model artifacts into the target project under `docs/fsm/<slug>/`.

### W2: `/plan` consumes Behavior Models (mandatory QA gate)
1. `/plan` loads:
   - Spec Packet (`docs/specs/<slug>.md`)
   - Capability map (`docs/specs/<slug>.capabilities.json`)
   - Behavior Model index (`docs/fsm/<slug>/index.json`)
2. `/plan` validates:
   - Every workflow capability maps to ≥1 transition (or a declared non-transition capability type)
   - Every transition maps to one or more planned tasks (or an explicit “covered by existing system” note)
3. `/plan` outputs a Plan that includes:
   - Transition coverage checklist (which tasks cover which transitions)
   - Invariant coverage checklist (which tasks/tests enforce which invariants)

### W3: `/execute` uses Behavior Models for adherence (mandatory QA gate)
1. `/execute` loads Behavior Model artifacts (same as `/plan`).
2. During implementation, `/execute` maintains an **Adherence Checklist**:
   - For each transition: evidence of implementation + verification (usually tests)
   - For each invariant/guard: evidence of enforcement (test, runtime check, or contract)
3. `/execute` fails completion if:
   - Any “Required transition” lacks verification evidence
   - Any “Critical invariant” lacks enforcement evidence
4. Optional (future-friendly but not required initially):
   - If a code-extraction skill exists, `/execute` may run “Extracted Model vs Spec Model” diff as an additional audit step.

---

## Invariants (must always hold)

### I1: Behavior Model is mandatory
Every `/specify` run that exports a spec MUST export at least one Behavior Model for the Steel Thread workflow.

### I2: Behavior-first ordering
Behavior Models MUST be derived from workflows/invariants/interfaces and must not require architecture or implementation details to exist.

### I3: Model completeness constraints (minimum viability)
For each exported state machine:
- Has an initial state
- Has at least one terminal outcome (success and/or failure)
- Every non-terminal state has ≥1 outgoing transition
- Every referenced failure condition has an explicit failure outcome (state or terminal)

### I4: Guard ↔ Invariant linkage
If a transition is constrained by an invariant, the model MUST reflect that via:
- guard annotation, and/or
- state constraint annotation, and
- a link in the machine-readable representation

### I5: No silent ambiguity
Words like “fast”, “soon”, “handles errors”, “robust”, “etc.” must not survive into the model without a concrete state/transition interpretation or an Open Question.

---

## Interfaces

### Inputs to Behavior Modeling (from `/specify`)
- Spec Packet sections:
  - Workflows
  - Invariants
  - Interfaces
  - Open Questions
- Discovery trace (optional, for traceability IDs)

### Exported Behavior Model Artifacts (to target project)
Default directory: `docs/fsm/`

For each spec slug:
```

docs/fsm/<slug>/
├─ index.json
├─ steel-thread.mmd
├─ steel-thread.transitions.json
├─ steel-thread.states.json
├─ steel-thread.notes.md
└─ (optional additional machines)
├─ <machine-name>.mmd
├─ <machine-name>.transitions.json
├─ <machine-name>.states.json
└─ <machine-name>.notes.md

```

#### Why `docs/fsm/`?
- short, recognizable, and stable
- “FSM” is widely understood in engineering orgs
- directory name can be configurable (see Open Questions)

#### `index.json` (contract)
- spec slug
- list of machines
- primary machine (Steel Thread)
- references to file paths
- version metadata (schema version)

### Consumers
- `/plan` MUST read `docs/fsm/<slug>/index.json` and machine JSON
- `/execute` MUST read the same artifacts for adherence checks

---

## Architecture sketch
- Add a “Behavior Model Compiler” component to `/specify`:
  - Input: spec sections (workflows/invariants/interfaces)
  - Output: (1) Mermaid diagram(s), (2) machine-readable state/transition JSON, (3) findings list
- Add QA hooks:
  - `/plan`: “Transition Coverage Gate”
  - `/execute`: “Adherence Gate”

This remains spec-first. No repo scan required.

---

## Decisions

### D1: Canonical representation is machine-readable JSON; Mermaid is a rendering
- Canonical: `*.states.json` + `*.transitions.json`
- Rendering: `*.mmd` generated from canonical data
Rationale: planning/execution tooling needs structured data; Mermaid is for humans and agents.

### D2: Steel Thread is mandatory; additional machines are rule-triggered
- Always export Steel Thread
- Additional machines required when:
  - >12 states in steel thread (must split/abstract)
  - cross-boundary workflow present
  - multiple user journeys defined as “in scope”

### D3: Export location defaults to `docs/fsm/`
- Stable, short, repo-friendly
- Configurable if needed (Open Question)

#### ADR triggers
Create an ADR if we decide:
- a different canonical format (YAML vs JSON)
- a different directory convention org-wide
- a nontrivial schema versioning policy
Otherwise keep decisions in this spec.

---

## Open Questions

### Blocking
1. Should the directory be exactly `docs/fsm/`, or configurable via Tasker setting (with `docs/fsm/` default)?
2. What is the minimum acceptable “verification evidence” in `/execute`?
   - tests only?
   - tests OR runtime assertions OR documented manual verification?
3. Do we require `/plan` to map *every* capability to transitions, or allow a capability type that is explicitly “non-transition” (e.g., pure computation)?

### Non-blocking
1. Should we support an additional alias directory name like `docs/state-machines/` for readability?
2. Should we include a single combined `model.json` per machine (instead of separate states/transitions files)?
3. Do we want Mermaid `stateDiagram-v2` only, or allow other Mermaid diagrams (flowchart) for cross-service views?

---

## Agent Handoff
Use this spec to implement:
- `/specify` enhancement: compile + export Behavior Models
- `/plan` enhancement: load models + enforce transition/invariant coverage gates
- `/execute` enhancement: adherence checklist + completion gate

Implementation agents must NOT invent states/transitions. Any ambiguity must be resolved via `AskUserQuestion` and written back into the Spec Packet and Behavior Model artifacts.
