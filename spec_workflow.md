# Agentic Spec Development Workflow

**(Minimal Spec Development Protocol + Ralph Clarify + Decision/ADR Rules)**

## Purpose

Build an **agent-driven interactive workflow inside Claude Code** that helps a user (human) produce **high-quality Spec Packets and ADRs** via structured Q&A, enforcing correct ordering and avoiding over-facilitation.

The system is itself built *from this spec*.

---

## Artifacts

### Required Outputs

* **Spec Packet** — single Markdown file
* **ADR files** — 0..N Markdown files
* **Discovery file** — `.claude/clarify-session.md`

### Optional Outputs

* None (keep minimal)

---

## Core Principles (MUST)

1. **Workflows and invariants before architecture**
2. **Decision-dense, not prose-dense**
3. **No guessing** — uncertainty becomes Open Questions
4. **Minimal facilitation** — decide only when required
5. **Specs are living; ADRs are historical**
6. **Less is more** — avoid ceremony

---

## Interaction Model

* Environment: **Claude Code**
* Interaction primitive: **AskUserQuestion**
* All structured input MUST use AskUserQuestion
* Free-form chat is allowed only for explanations, not data capture

---

## High-Level Workflow (Ordered, Mandatory)

```
Scope
→ Clarify (Ralph Loop)
→ Synthesis
→ Architecture Sketch
→ Decisions / ADRs
→ Handoff-Ready Gate
→ Export
```

The agent MUST NOT skip or reorder these phases.

---

## Phase 1 — Scope

### Goal

Establish bounds before discovery.

### Required Questions (AskUserQuestion)

* Goal (what we are building)
* Non-goals (explicitly out of scope)
* “Done means” (acceptance bullets)

### Output

Populate Spec Packet sections:

* Goal
* Non-goals
* Done means

---

## Phase 2 — Clarify (Ralph Iterative Discovery Loop)

### Purpose

Exhaustively gather requirements via structured questioning.

### Setup

Create `.claude/clarify-session.md`:

```markdown
# Discovery: {TOPIC}
Started: {timestamp}

## Questions Asked
## Answers Received
## Emerging Requirements
```

### Loop Rules

* Default max iterations: **30** (user may override)
* Each iteration:

  1. Read discovery file
  2. Identify gaps
  3. Ask **2–4 new questions** (never repeat)
  4. Update discovery file
* Stop ONLY when:

  * coverage is exhaustive, OR
  * user says “enough / stop / move on”

### Question Categories (systematic)

* Core requirements
* Users & context
* Integrations / boundaries
* Edge cases / failures
* Quality attributes
* Existing patterns
* Preferences / constraints

### Completion Signal

Output `<promise>CLARIFIED</promise>` when done.

---

## Phase 3 — Synthesis (Derived, Not Asked)

From `.claude/clarify-session.md`, synthesize:

### Spec Sections

* **Workflows** (numbered steps, variants, failures, postconditions)
* **Invariants** (bulleted rules that must always hold)
* **Interfaces** (only if a boundary exists; else “none”)
* **Open Questions** (classified)

### Invariants

* No new information introduced here
* Everything must trace to discovery answers

---

## Phase 4 — Architecture Sketch

### Rule

Architecture MUST come **after** workflows, invariants, interfaces.

### AskUserQuestion

* Either:

  * ask for a brief architecture sketch, OR
  * propose one and ask for confirmation/edit

### Output

Populate **Architecture sketch** section:

* components touched
* responsibilities
* failure handling (brief)

Keep this short. No essays.

---

## Phase 5 — Decisions & ADRs

### ADR Trigger (Create ADR if ANY true)

* Hard to reverse
* Reusable standard
* Tradeoff-heavy
* Cross-cutting
* NFR-impacting

If none apply → record decision in spec only.

---

### Decision Facilitation Rules (CRITICAL)

#### Facilitate a decision ONLY IF ALL are true

1. ADR-worthy
2. Not already decided (North Star / existing ADR / explicit user preference)
3. Blocking workflows, invariants, or interfaces

#### DO NOT Facilitate if ANY are true

* Already decided
* Local / reversible / implementation detail
* Non-blocking
* User not ready to decide
* Too many options (>3)
* Premature (behavior not defined yet)

#### If facilitation is allowed

1. Present **2–3 concrete options**
2. 2–4 consequences per option
3. Ask user to choose (AskUserQuestion)
4. Outcomes:

   * Choice → write decision + create ADR (Accepted)
   * “Need more info” → Blocking Open Question (no ADR yet)

---

## Open Questions Rules

### Classification

* **Blocking**: affects workflows, invariants, interfaces
* **Non-blocking**: internal preferences only

### Gate

* ANY Blocking Open Question → NOT handoff-ready

---

## Spec Packet Template (Minimal)

```markdown
# Spec: {Title}

## Goal
## Non-goals
## Done means

## Workflows
1. …
Variants / failures:
- …

## Invariants
- …

## Interfaces
- (or “No new/changed interfaces”)

## Architecture sketch
- …

## Decisions
- Decision summary bullets
- ADR links (if any)

## Open Questions
- Blocking:
- Non-blocking:

## Agent Handoff
- What to build
- Must preserve
- Blocking conditions
```

---

## ADR Template

```markdown
# ADR-####: {Title}

## Status
Proposed | Accepted | Superseded

## Context
## Decision
## Alternatives
## Consequences
## Links
```

Rules:

* ADRs are immutable once Accepted
* Changes require a new ADR that supersedes the old

---

## Handoff-Ready Gate (Final Check)

Before export:

* All phases completed in order
* No Blocking Open Questions
* Interfaces section present
* Decisions section present

If gate fails → list exact blockers and stop.

---

## Export

Write files to disk:

* `.claude/clarify-session.md`
* `specs/<slug>.md`
* `adrs/ADR-####-<slug>.md` (0..N)

---

## Non-Goals (System-Level)

* No Git automation
* No project management
* No runtime ops/runbooks
* No over-facilitation
* No architectural debates before behavior

---

## Implementation Notes (Non-Normative)

* CLI inside Claude Code is sufficient
* In-memory draft model + deterministic Markdown renderer
* Defaults are acceptable when explicitly documented

---