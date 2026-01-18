<div align="center">
<img src="/assets/logo.jpg" alt="Logo" width="100%" style="display: block; margin-top: 0; margin-bottom: 0; max-width: 100%;"/>
</div>

# Tasker

Tasker turns specifications into executable, verifiable behavior for AI-augmented software.

It uses Spec-Driven Development (SDD): precise specifications are produced with agent assistance, then compiled into executable system behavior.

Humans define logic. Agents perform translation and execution.

---

## Pipeline

Tasker enforces a three-stage pipeline: **specify → plan → execute**.

### Specify
Human intent is exhaustively clarified.  
Edge cases, state transitions, and invariants are made explicit before any implementation begins.

### Plan
The specification is decomposed into isolated, context-bounded tasks with explicit inputs, outputs, and dependencies.

### Execute
Agents implement tasks against the plan and verify results against the spec.

By the time code is written:
- Behavior is defined
- Edge cases are resolved
- Logic is verified

Implementation becomes translation, not discovery.

---

## A Compiler for Specifications

Tasker acts as a compiler for requirements.

Like a borrow checker, it enforces semantic safety:
- Agents cannot implement behavior that was not declared
- Architectural changes require explicit, recorded decisions
- Undeclared state transitions and logic drift fail early

Every artifact is traceable back to a specific requirement.
