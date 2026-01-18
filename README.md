<div align="center">
<img src="/assets/logo.jpg" alt="Logo" width="100%" style="display: block; margin-top: 0; margin-bottom: 0; max-width: 100%;"/>
</div>

# Tasker

**Tasker turns specifications into executable control systems for AI software agents.**

Instead of “prompting and hoping,” you define **exact behavior, constraints, and failure modes** once — and agents execute reliably, repeatedly, and audibly.

This is **Spec-Driven Development (SDD)** done correctly.

Your job shifts from *writing code* to **compiling intent into logic**.

---

## What Problem Tasker Solves

Modern AI agents fail for predictable reasons:

* Specs are vague, oversized, or contradictory
* Requirements leak mid-implementation
* Behavior lives in people’s heads, not documents
* “Common sense” breaks under automation

**Tasker eliminates these failure modes by treating specs as first-class, executable artifacts.**

If it’s not explicit, it doesn’t exist.
If it can’t be verified, it doesn’t ship.

---

## The Core Idea

> **A specification is not documentation.
> It is the source code for agent behavior.**

Tasker provides a strict protocol that converts messy human intent into:

* Deterministic workflows
* Explicit state machines
* Enforced invariants
* Machine-verifiable acceptance criteria

By the time implementation starts, **the system already exists — on paper**.

---

## The SDD Architecture (3-Tier Spec System)

Tasker replaces monolithic docs with a **stable, composable spec hierarchy**.

### 1. North Star (Global Context)

The immutable foundation:

* Tech stack + versions
* Global business rules
* Non-negotiables (“never do X”)

This changes rarely and prevents drift.

### 2. Blueprints (System Architecture)

The structural truth:

* Data models (SQL / ERDs)
* API contracts (OpenAPI)
* **Behavior models (finite state machines)**

New features must fit this skeleton.

### 3. Action Specs (Task Units)

The unit of execution:

* One feature, one intent
* Clear inputs / outputs
* Explicit definition of done

Agents work only at this level.

---

## The Requirements Compiler Workflow

Tasker enforces a **mandatory, ordered pipeline**.
Skipping steps is not allowed.

### Phase 1 — Scope

Define the box before filling it.

* Goal
* Non-goals
* What “done” objectively means

If it’s not scoped, it’s ignored.

---

### Phase 2 — Clarify (Adversarial Extraction)

No brainstorming. No vibes.

The agent **interrogates you** with pressure-test questions:

* Edge cases
* State transitions
* Failure scenarios
* Constraints and limits

This continues until every category is explicitly closed.

---

### Phase 3 — Synthesis

Structure, no invention.

* Numbered workflows (including failures)
* Invariants (rules that must *always* hold)
* Typed interfaces
* **Mandatory Steel-Thread FSM**

Every transition must reference a rule.
Every rule must be traceable to source text.

---

### Phase 4 — Architecture Sketch

Only now do we talk components.

* Map behavior → responsibility
* Define ownership and failure handling
* Keep it minimal

Architecture follows behavior — never the reverse.

---

### Phase 5 — Decisions (ADRs)

Hard choices are recorded once.

* Tradeoffs
* Rationale
* Why this path was chosen

Specs describe *what is*.
ADRs explain *why*.

---

### Phase 6 — Quality Gates

Before any code exists, the spec must pass.

* No open questions
* No contradictions
* No vague language

Tasker automatically flags:

* Ambiguity
* Conflicts
* Missing behavioral rules

If it fails, implementation is blocked.

---

### Phase 7 — Export

The spec becomes permanent system input:

* Human-readable spec packet
* Machine-readable capability map
* Canonical FSM + diagrams

This is what agents execute against.

---

## What Makes a Spec “Agent-Ready”

A good spec explains intent.
A **Tasker spec enforces it**.

| Principle             | Why It Matters              |
| --------------------- | --------------------------- |
| Single interpretation | No branching guesswork      |
| Explicit constraints  | Prevents silent violations  |
| Verifiable outcomes   | Enables automation          |
| Typed interfaces      | Eliminates mismatch errors  |
| Full traceability     | Every behavior is justified |

If a human can “interpret” it, an agent will misinterpret it.

---

## The Shift Tasker Enables

| Old Model      | Tasker Model            |
| -------------- | ----------------------- |
| Prompt → hope  | Specify → verify        |
| One giant doc  | Composable spec library |
| Implicit logic | Explicit state machines |
| Code as truth  | Spec as truth           |
| Debugging code | Auditing decisions      |

**You become a decision maker and logic auditor.
The agent becomes a compiler and executor.**

---

## The End State

By the time implementation starts:

* The behavior is already defined
* The edge cases are already handled
* The failure modes are already known
* The success criteria are already testable

**Code becomes a mechanical translation, not a creative act.**

That’s Tasker.
