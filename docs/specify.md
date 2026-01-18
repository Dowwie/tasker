
## The `/specify` Phase — Turning Intent into a Source of Truth

The **`/specify` phase** is where raw ideas, half-formed requirements, and implicit assumptions are transformed into a **precise, unambiguous specification** that the rest of the agentic workflow can safely build from.

In Tasker’s pipeline:

```
/specify → /plan → /execute
```

`/specify` is deliberately focused on **understanding and definition**, not execution.

---

### What `/specify` Produces

By the end of `/specify`, the system produces:

* **A Spec Packet** (human-readable)
  The authoritative description of:

  * what the system must do
  * why it exists
  * what rules must always hold
  * what is explicitly out of scope

* **ADR files** (Architecture Decision Records, as needed)
  Durable records of *why* significant decisions were made, separate from the spec itself.

* **A Capability Map** (machine-readable)
  A structured representation of system capabilities, derived from the spec and used by `/plan` to generate execution strategies.

Together, these artifacts form the **Source of Truth** for the rest of the workflow.

---

### What `/specify` Is (and Is Not)

**`/specify` is:**

* A **requirements compiler**
* An **interactive discovery process**
* A way to extract decisions, rules, and boundaries from human intent
* Strictly focused on the *what*, *why*, and *constraints* of the system

**`/specify` is not:**

* Task decomposition
* File planning
* Implementation strategy
* Execution ordering
* Optimization or refactoring advice

Those belong to `/plan` and `/execute`.

---

### How `/specify` Works

The `/specify` phase runs as an **agent-driven, interactive workflow** with strict ordering:

1. **Scope**
   Establishes goals, non-goals, and what “done” means.

2. **Clarify (Iterative Discovery)**
   Uses structured questioning to exhaustively surface:

   * workflows
   * edge cases
   * integrations
   * quality requirements
   * preferences and constraints
     Ambiguity is reduced by presenting concrete trade-offs instead of vague questions.

3. **Synthesis**
   Discovery answers are compiled into:

   * explicit workflows
   * **testable invariants** (rules written as checkable predicates)
   * defined interfaces
   * clearly classified open questions

4. **Architecture Sketch**
   A brief structural outline, created *only after* behavior and rules are known.

5. **Decisions & ADRs**
   Significant, hard-to-reverse, or reusable decisions are captured as ADRs.
   Decisions are facilitated **only when required**, never by default.

6. **Spec Review (Quality Gate)**
   The spec is analyzed for ambiguity, contradictions, missing behavior, and implicit assumptions.
   Critical issues are resolved before proceeding.

7. **Handoff-Ready Gate**
   The spec must be internally consistent, complete, and free of blocking uncertainty before it can move on.

---

### Why This Matters

Most development failures don’t come from bad code — they come from:

* missing requirements
* implicit assumptions
* premature architecture
* decisions made too early or not recorded at all

`/specify` exists to eliminate those failure modes *before* any planning or execution begins.

It ensures that:

* **Workflows and invariants come before architecture**
* **Uncertainty is explicit, not hidden**
* **Decisions are intentional and traceable**
* **Downstream agents don’t have to guess**

When `/specify` finishes successfully, `/plan` no longer needs discovery — it can focus entirely on mapping clearly defined capabilities to concrete work.

---

### Mental Model

Think of `/specify` as a **compiler for requirements**:

* Human intent goes in
* A validated, decision-dense specification comes out
* Anything ambiguous or contradictory is rejected at compile time

Only after that does the system move on to planning and execution.
