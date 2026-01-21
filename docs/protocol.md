# Task Decomposition Protocol

# Overview
This protocol turns software specifications into a deterministic dependency graph by mapping logical capabilities to concrete physical artifacts and sequencing work based on causal necessity and risk reduction.

**Objective**
Turn a specification + architecture into a **directed acyclic graph (DAG)** of **atomic, verifiable tasks** that can be executed in parallel where possible and sequenced by real dependencies and risk.

**Core Principles**

1. **Systems, not stories**
   Software work is a **system of dependencies**, not a flat backlog. A valid plan respects:

   * The *physics* of construction (dependencies & order).
   * The *conservation of matter* (all logic lives in real artifacts).
   * The *limits of cognition* (tasks small enough to reason about).

2. **Three lenses, one process**

   * **Logical (What):** capabilities, behavior, flows.
   * **Physical (Where/How):** files, services, DBs, infra, configs.
   * **Strategic (Risk/When):** dependencies, risk, sequencing.

3. **Tasks must be binary**

   * Every task has a clear, observable, testable “Done”.
   * No task is “work on X”; tasks produce specific, verifiable change.

---

## Phase 0 – Framing and Constraints

Before decomposition, explicitly set the ground rules.

* **Time granularity**
  Default: a task should be completable in **one sitting** (roughly 2–6 hours) by a competent engineer.
  If your team prefers larger tasks, you can relax this, but the default should be *small*.

* **Definition of Done (per task)**
  A task is done only when:

  * All identified artifacts are created/updated.
  * Code compiles/build passes.
  * Named tests pass (or equivalent verification).
  * Any required docs/notes are updated.

* **Naming convention**
  **`[Imperative Verb] + [Specific Artifact] + [Purpose]`**

  * Example: `Implement RefundService.calculate() for partial refunds`
  * Bad: `Work on refunds`

These constraints are used in Phase 3.

---

## Phase 1 – Logical Decomposition (The “What”)

**Input:** Specification
**Goal:** Identify all required *behavior* as "behaviors", mapped to domains and flows.

### 1.1 Map Capabilities to Domains

1. Extract every distinct **capability** from the spec:

   * “User can register and verify account”
   * “User can refund an order”
   * “System generates monthly statements”, etc.
2. Group capabilities by **architectural domain/component**:

   * `AuthService`, `BillingService`, `ReportingService`, `AdminUI`, etc.

> Result: a **Capability Map**: capabilities × components.

### 1.2 Define Behaviors

For each capability within a component, break it into the smallest meaningful **behaviors**:

* **Input**: validating & parsing requests/events.
* **Process**: core domain logic, calculations, decisions.
* **State**: persistence, mutations, state transitions.
* **Output**: events, responses, side-effects.

Example for “Refund order” in `BillingService`:

* `ValidateRefundRequest`
* `CalculateRefundAmount`
* `UpdateLedgerWithRefund`
* `EmitRefundProcessedEvent`

Don't create tasks yet; just define these behaviors as logical units.

### 1.3 Trace End-to-End Flows (“Happy Paths”)

Identify key flows from the spec:

* “Sign up → Pay → Access product”
* “Create order → Invoice → Pay → Confirm”
* “Request refund → Approve → Process → Notify”

For each flow:

1. List the steps across UI → API → domain → DB → external systems → notifications.
2. Map each step to one or more **behaviors**.
3. Ensure every critical flow is fully covered by behaviors; if not, define missing behaviors.

> Result: behavior is fully understood as a mesh of behaviors and flows, aligned to components.

---

## Phase 2 – Physical Decomposition (The "Matter")

**Input:** Architecture + behaviors
**Goal:** Enumerate the concrete artifacts where each behavior will live. *No floating logic.*

### 2.1 Enumerate Terminal Artifacts

For each behavior, list the concrete **artifacts** required:

* **Code**

  * `src/billing/refund_service.ts`
  * `src/api/refunds_controller.ts`
  * `src/shared/types/refund.ts`
* **Data**

  * `migrations/004_add_refunds_table.sql`
  * `seed/initial_ledger_state.sql`
* **Infra**

  * `terraform/sqs_refund_queue.tf`
  * `k8s/refund-worker-deployment.yaml`
* **Config**

  * `config/refunds.json`
  * Feature flags, secrets placeholders.

You don’t need to be perfect on file paths from day one, but you must identify **which artifacts** will exist.

### 2.2 Zero-Missing-Nodes Check

For each behavior, ask:

> “Can I point to the file / resource where this logic lives?”

If not:

* You haven’t finished decomposing.
* Add or refine artifacts until every behavior has a **home**.

> Result: a **Logical ↔ Physical mapping**: every unit of behavior maps to specific files/resources.

---

## Phase 3 – Cross-Cutting Concerns Injection

**Input:** Logical & physical maps
**Goal:** Make non-functional and systemic concerns first-class tasks, not afterthoughts.

Perform a **cross-cutting sweep** over components, flows and artifacts for:

* **Security & Auth**

  * RBAC, permissions, token handling, data access rules.
* **Observability**

  * Structured logging, metrics, tracing, dashboards.
* **Performance & Scalability**

  * Indexes, caching, pagination, bulk operations.
* **Resilience**

  * Timeouts, retries, circuit breakers, idempotency.
* **Compliance & Audit**

  * Audit logs, data retention, PII masking, encryption.
* **Deployment & Operations**

  * CI/CD, health checks, readiness/liveness, feature flags.

For each relevant area, define **what behavior** is needed and **where** it lives (artifacts). These become inputs to task creation just like core features.

> Result: cross-cutting requirements are attached to concrete components and artifacts, not left as vague “do security later”.

---

## Phase 4 – Task Definition (The Unit of Work)

**Input:** Behaviors + artifacts + cross-cutting concerns
**Goal:** Create atomic, executable tasks that respect cognitive limits and have binary outcomes.

### 4.1 Task Size and Shape

* Default size: **2–6 hours** for a competent engineer.
* A task should be **cohesive**:

  * One coherent responsibility.
  * Not mixing unrelated concerns (e.g., business logic + monitoring) unless tightly coupled.

### 4.2 Task Naming

Use the pattern:

> **`[Verb] [Specific Artifact] [Purpose]`**

Examples:

* `Implement RefundService.calculate() for full/partial refunds`
* `Add audit logging to POST /refunds handler`
* `Create 004_add_refunds_table migration and rollback`
* `Configure SQS refund-queue and DLQ in Terraform`

Avoid vague names like “Implement refunds” or “Work on billing”.

### 4.3 Task Template (What each task should contain)

For each task, capture:

1. **Context**

   * Component(s) touched.
   * Link to relevant part of spec/architecture.

2. **Outcome (One sentence)**

   * “When this is done, the system can …”
   * Example: “System can compute correct refund amounts for full and partial refunds based on spec section 3.2.”

3. **Artifacts**

   * Exact files/resources to create or modify.
   * Example: `src/billing/refund_service.ts`, `tests/billing/refund_service.test.ts`.

4. **Interfaces**

   * Inputs/outputs (types, DTOs, events, schemas).
   * Important constraints (validation rules, preconditions).

5. **Acceptance Criteria**

   * Binary, testable statements.
   * Example:

     * “Given an order with items A and B and refund amount X, `calculate()` returns Y per formula in spec 3.2.”
     * “Errors for invalid refund requests are logged with orderId and userId.”

6. **Verification / Tests**

   * Named tests or explicit verification steps:

     * “Unit tests in `refund_service.test.ts` for edge cases (over-refund, zero, partial).”
     * “Service compiles and all existing tests still pass.”

### 4.4 Cross-Cutting Tasks

Inject tasks from Phase 3 directly into this task list:

* `Add RBAC check to POST /refunds endpoint`
* `Emit metric refund.failure_reason in RefundService`
* `Mask PII in refund logs per compliance doc`

> Result: A **flat list of well-shaped tasks**, each tied to concrete artifacts and clear done criteria.

---

## Phase 5 – Dependency Graph & Sequencing (The DAG)

**Input:** Task list
**Goal:** Turn the task set into a DAG and sequence it by dependency and risk.

### 5.1 Build the Dependency Graph (Backward Pass)

For each task, ask:

> “What must exist before this can be done?”

Examples:

* You can’t implement `POST /refunds` handler until:

  * DTO/type definitions exist.
  * `RefundService` has at least the core API.
* You can’t run a DB migration until:

  * Local DB/containers and migration framework are set up.

Draw edges: `Task A → Task B` meaning “A must finish before B starts.”

This yields:

* A **DAG** of tasks.
* Visibility into the **critical path** (longest dependent chain).

### 5.2 Identify the Steel Thread(s)

Define one or more **Steel Threads**:

> The thinnest vertical slice from input → UI → API → domain → DB → external integration → observable output that proves the architecture works.

Example:

* “User submits refund request in UI → API validates → RefundService calculates → DB ledger updates → event emitted → log/metric visible.”

Create or mark tasks that implement this slice across layers:

* UI form & basic validation.
* Endpoint & DTOs.
* Domain behavior & persistence.
* Any necessary external integration stubs/mocks.
* Logging/metrics to verify behavior.

**Prioritize this thread extremely early** in the plan:

* If your architecture is flawed, you want it to fail in week 1, not month 3.

### 5.3 Sequence into Batches

Using the DAG:

* **Batch 1 – Foundations**

  * Repo structure, base tooling, CI, basic infrastructure (local DB, skeleton services, base types).
* **Batch 2 – Steel Thread(s)**

  * End-to-end flows that validate the architecture and main integration seams.
* **Batch 3 – Parallel Meat**

  * Remaining behaviors/tasks that can run in parallel given the DAG.
* **Batch 4 – Aggregation & Hardening**

  * Reports, dashboards, polish, performance tuning, resilience improvements, cleanup.

Within each batch:

* Respect dependencies from the DAG.
* Pull **high-risk tasks** earlier when possible.

> Result: an **ordered plan** driven by dependencies and risk, not arbitrary sprints or gut feel.

---

## Phase 6 – Final Audits (Completeness & Integrity)

**Input:** DAG + sequencing
**Goal:** Prove to yourself the plan is complete and consistent.

### 6.1 Spec Traceability Check

Go through the spec line by line:

* For each requirement / capability:

  * Link it to one or more tasks that fully implement it.
* If something has no corresponding task:

  * Add tasks or clarify/correct scope.

You should be able to answer:

> “Show me which tasks implement ‘User can refund an order.’”

### 6.2 Architecture “Dark Matter” Check

Walk the architecture diagram:

* For each **box** (service, module, DB, queue):

  * Are there tasks to implement/set it up?
* For each **arrow** (call, event, integration):

  * Are there tasks to wire it, configure endpoints, handle failures?

If any box/arrow has no tasks, you’re underestimating infrastructure/integration work.

### 6.3 Non-Functional Coverage Check

List the key **non-functional requirements** (from spec or architecture):

* Latency, throughput, availability, durability.
* Security/privacy requirements.
* Scalability expectations.
* Operational requirements (observability, on-call, alerting).

For each NFR, ensure there are explicit tasks that:

* Implement needed mechanisms (indexes, caching, bulk APIs, etc.).
* Provide measurement/verification (metrics, load tests).

> Result: confidence that both functional and non-functional aspects are fully represented in the DAG.

---

## Final Output and Litmus Test

The final plan you should get from this protocol:

* A **DAG of tasks**:

  * **Nodes**: atomic tasks (2–6 hours), each with:

    * Clear context, outcome, artifacts, acceptance criteria, tests.
  * **Edges**: explicit dependencies derived via backward pass.
* **Structure**:

  * Organized around **steel threads** and **risk**, not just UI/DB layers or calendar time.

**Litmus Test:**

> For any task (say, Task #42), if you hand it to a competent engineer:
>
> * Do they know exactly what files to touch?
> * Do they know how to know they’re done (tests, behavior)?
> * Can they complete it without asking for missing context?

If the answer is "yes" for essentially all tasks, and your spec and architecture are fully traceable to this graph, your decomposition is **complete and robust**.

---

## v2 Implementation Notes

This section documents how the Task Decomposition Protocol v2 implements the above specification.

### Artifact Formats

| Phase | Original (v1) | v2 |
|-------|---------------|-----|
| Phase 1 | `01-capability-map.md` | `capability-map.json` (schema-validated) |
| Phase 2 | `02-physical-map.md` | `physical-map.json` (schema-validated) |
| Phase 3-4 | `03-task-inventory.md` | `tasks/*.json` (individual files) |
| Phase 5 | `master_plan.md` | Phases in task files + state.json |
| State | `progress.md` (mutable) | `state.json` (append-only events) |

### Commands

```bash
# Planning (Phases 0-5)
/plan

# Execution (Phase 6)
/execute
/execute T005        # Specific task
/execute --batch     # All ready tasks

# State management
tasker state status
tasker state ready-tasks
tasker state retry-task T005
tasker state skip-task T005 "reason"
```

### Validation Gates

Phase transitions require schema validation:

```bash
tasker state validate capability_map
tasker state validate physical_map
tasker state advance
```

### Execution Bundles

Each task generates a self-contained bundle at `.tasker/bundles/T001-bundle.json` containing:
- Expanded behaviors (not just IDs)
- File paths with purposes and layers
- Acceptance criteria with verification commands
- Constraints (language, framework, patterns)
- Dependency files from prior tasks

### Schema Files

- `schemas/capability-map.schema.json`
- `schemas/physical-map.schema.json`
- `schemas/task.schema.json`
- `schemas/state.schema.json`
- `schemas/execution-bundle.schema.json`