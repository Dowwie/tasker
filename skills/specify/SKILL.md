---
name: specify
description: Interactive specification workflow - design vision, clarify capabilities, extract behaviors. Produces spec packets, capability maps, and ADRs for /plan consumption.
tools:
  - AskUserQuestion
  - file_read
  - file_write
  - bash
---

# Specify Workflow

An **agent-driven interactive workflow** that transforms ideas into actionable specifications with extracted capabilities, ready for `/plan` to decompose into tasks.

## Core Principles

1. **Workflows and invariants before architecture** - Never discuss implementation until behavior is clear
2. **Decision-dense, not prose-dense** - Bullet points over paragraphs
3. **No guessing** — Uncertainty becomes Open Questions
4. **Minimal facilitation** — Decide only when required
5. **Specs are living; ADRs are historical**
6. **Less is more** — Avoid ceremony

## Artifacts

### Required Outputs (in TARGET project)
- **Spec Packet** — `{TARGET}/docs/specs/<slug>.md` (human-readable)
- **Capability Map** — `{TARGET}/docs/specs/<slug>.capabilities.json` (machine-readable, for `/plan`)
- **Behavior Model (FSM)** — `{TARGET}/docs/state-machines/<slug>/` (state machine artifacts, for `/plan` and `/execute`)
- **ADR files** — `{TARGET}/docs/adrs/ADR-####-<slug>.md` (0..N)

### Working Files (in target project's .tasker/)
- **Discovery file** — `$TARGET_DIR/.tasker/clarify-session.md` (ephemeral)
- **Session state** — `$TARGET_DIR/.tasker/state.json` (persistent)
- **Spec Review** — `$TARGET_DIR/.tasker/spec-review.json` (ephemeral)

### Archive
After completion, artifacts can be archived using `tasker archive` for post-hoc analysis.

---

## MANDATORY: Phase Order

```
Initialization → Scope → Clarification Loop (Discovery) → Synthesis → Architecture Sketch → Decisions/ADRs → Gate → Spec Review → Export
```

**NEVER skip or reorder these phases.**

---

# Phase 0 — Initialization

## Goal
Establish project context and session state before specification work begins.

## Step 1: Ask for Target Project Directory (MANDATORY FIRST)

**ALWAYS ask for target_dir FIRST before anything else.** No guessing, no inference from CWD.

Ask using AskUserQuestion:
```
What is the target project directory?
```
Free-form text input. User must provide an absolute or relative path.

**Validation:**
```bash
TARGET_DIR="<user-provided-path>"
# Convert to absolute path
TARGET_DIR=$(cd "$TARGET_DIR" 2>/dev/null && pwd || echo "$TARGET_DIR")

if [ ! -d "$TARGET_DIR" ]; then
    # For new projects, check parent exists
    PARENT=$(dirname "$TARGET_DIR")
    if [ -d "$PARENT" ]; then
        echo "Directory will be created: $TARGET_DIR"
    else
        echo "Error: Parent directory does not exist: $PARENT"
        # Re-ask for target_dir
    fi
fi
```

## Step 2: Ask About Existing Specs

Ask using AskUserQuestion:
```
Do you have existing specification files?
```
Options:
- **No specs yet** — Starting from scratch
- **Yes, I have specs** — I'll provide the location

### If "Yes, I have specs":
Ask for the spec location:
```
Where are your spec files located?
```
Free-form text input. User provides path (e.g., `docs/specs/`, `requirements.md`, `PRD.pdf`).

**Store the spec location for later use:**
```bash
EXISTING_SPEC_PATH="<user-provided-path>"
# Validate path exists
if [ ! -e "$TARGET_DIR/$EXISTING_SPEC_PATH" ] && [ ! -e "$EXISTING_SPEC_PATH" ]; then
    echo "Warning: Spec path not found. Will ask again during Scope phase."
fi
```

## Step 3: Check for Existing Tasker Session

After target_dir is confirmed, check for existing `.tasker/` state:

```bash
TASKER_DIR="$TARGET_DIR/.tasker"
if [ -f "$TASKER_DIR/state.json" ]; then
    echo "Found existing tasker session at $TASKER_DIR"
    echo "Resuming from saved state..."
    # Read phase from state.json and resume
else
    echo "No existing session. Starting fresh."
fi
```

If existing session found, inform user and resume from saved phase.

## Step 4: Ask User Intent

Ask using AskUserQuestion:
```
What would you like to do?
```
Options:
- **New specification** — Start a new spec from scratch
- **Continue existing spec** — Resume work on an existing spec file (use if user provided spec path in Step 2)

**Note:** If user provided a spec path in Step 2, default to "Continue existing spec" and use that path.

## Step 5: Initialize Session State

Create `.tasker/` directory structure in target project:

```bash
TASKER_DIR="$TARGET_DIR/.tasker"
mkdir -p "$TASKER_DIR"/{inputs,artifacts,tasks,bundles,reports}
```

Create or update `$TARGET_DIR/.tasker/state.json`:

```json
{
  "version": "2.0",
  "target_dir": "<absolute-path>",
  "phase": {
    "current": "initialization",
    "completed": []
  },
  "created_at": "<timestamp>",
  "spec_session": {
    "project_type": "new|existing",
    "existing_spec_path": "<path-from-step-2-or-null>",
    "spec_slug": "<slug>",
    "spec_path": "<target>/docs/specs/<slug>.md",
    "started_at": "<timestamp>",
    "resumed_from": null
  },
  "artifacts": {},
  "tasks": {}
}
```

**If user provided existing spec path in Step 2**, store it in `spec_session.existing_spec_path` for reference during Scope phase.

**For existing projects**, store discovered project context in session state for later reference during Synthesis phase.

## Output

- `.tasker/` directory structure created in target project
- Session state initialized in `$TARGET_DIR/.tasker/state.json`
- Existing spec path captured (if provided)
- Project context captured (for existing projects)
- Clear path forward: resume existing spec OR start new spec

## Proceed to Phase 1

After initialization completes, advance to Phase 1 (Scope).

---

# Phase 1 — Scope

## Goal
Establish bounds before discovery.

## Pre-Scope: Load Existing Spec (if provided)

If `spec_session.existing_spec_path` was set during initialization:

1. **Read the existing spec file** to understand prior context
2. **Extract initial answers** for the scope questions below (Goal, Non-goals, Done means)
3. **Present extracted context** to user for confirmation/refinement rather than asking from scratch

```bash
if [ -n "$EXISTING_SPEC_PATH" ]; then
    echo "Loading existing spec from: $EXISTING_SPEC_PATH"
    # Read and analyze existing spec
    # Pre-fill scope questions with extracted information
fi
```

## Required Questions (AskUserQuestion)

Ask these questions using AskUserQuestion tool with structured options.
**If existing spec was loaded**, present extracted answers for confirmation rather than blank questions:

### Question 1: Goal
```
What are we building?
```
Free-form text input.

### Question 2: Non-goals
```
What is explicitly OUT of scope?
```
Free-form text input (allow multiple items).

### Question 3: Done Means
```
What are the acceptance bullets? (When is this "done"?)
```
Free-form text input (allow multiple items).

### Question 4: Tech Stack
```
What tech stack should be used?
```
Free-form text input. Examples:
- "Python 3.12+ with FastAPI, PostgreSQL, Redis"
- "TypeScript, Next.js, Prisma, Supabase"
- "Go with Chi router, SQLite"
- "Whatever fits best" (let /specify recommend based on requirements)

**If user says "whatever fits best" or similar:**
- Note this for Phase 2 (Clarify) to recommend based on gathered requirements
- Ask clarifying questions: "Any language preferences?", "Cloud provider constraints?", "Team expertise?"

## Output
Create initial spec draft with:
- Goal
- Non-goals
- Done means
- Tech stack (or "TBD - will recommend after requirements gathering")

---

# Phase 2 — Clarify (Ralph Iterative Discovery Loop)

## Purpose
Exhaustively gather requirements via structured questioning.

## Setup

Create `$TARGET_DIR/.tasker/clarify-session.md`:

```markdown
# Discovery: {TOPIC}
Started: {timestamp}

## Questions Asked

## Answers Received

## Emerging Requirements
```

## Loop Rules

- **No iteration cap** - Continue until goals are met
- Each iteration:
  1. Read discovery file
  2. Check category goals (see checklist below)
  3. Ask **2–4 new questions** targeting incomplete goals (NEVER repeat a question)
  4. Update discovery file with Q&A
  5. Extract any new requirements discovered
  6. Update category completion status

- **Stop ONLY when:**
  - ALL category goals are met (see checklist), OR
  - User says "enough", "stop", "move on", or similar

## Category Checklist (Goal-Driven Coverage)

Each category has concrete "done" criteria. Track completion in the discovery file.

| Category | Goal (Done When) |
|----------|------------------|
| **Core requirements** | Primary workflows enumerated with inputs, outputs, and happy path steps |
| **Users & context** | User roles identified, expertise levels known, access patterns clear |
| **Integrations / boundaries** | All external systems named, data flows mapped, API contracts sketched |
| **Edge cases / failures** | Error handling defined for each workflow step, retry/fallback behavior specified |
| **Quality attributes** | Performance targets have numbers (or explicit "not critical"), security requirements stated |
| **Existing patterns** | Relevant prior art identified OR confirmed none exists, conventions to follow listed |
| **Preferences / constraints** | Tech stack decided, deployment target known, timeline/resource constraints stated |

### Tracking Format

Update discovery file with completion status:

```markdown
## Category Status

| Category | Status | Notes |
|----------|--------|-------|
| Core requirements | ✓ Complete | 3 workflows defined |
| Users & context | ✓ Complete | 2 roles: admin, user |
| Integrations | ⋯ In Progress | DB confirmed, auth TBD |
| Edge cases | ○ Not Started | — |
| Quality attributes | ○ Not Started | — |
| Existing patterns | ✓ Complete | Follow auth module pattern |
| Preferences | ⋯ In Progress | Python confirmed, framework TBD |
```

### Completion Criteria

A category is **complete** when:
1. The goal condition is satisfied (see table above)
2. User has confirmed or provided the information
3. No obvious follow-up questions remain for that category

**Do NOT mark complete** if:
- User said "I don't know" without a fallback decision
- Information is vague (e.g., "fast" instead of "<100ms")
- Dependencies on other categories are unresolved

## AskUserQuestion Format

Use AskUserQuestion with 2-4 questions per iteration:

```
questions:
  - question: "How should the system handle [specific scenario]?"
    header: "Edge case"
    options:
      - label: "Option A"
        description: "Description of approach A"
      - label: "Option B"
        description: "Description of approach B"
    multiSelect: false
```

For open-ended questions, use free-form with context:
```
questions:
  - question: "What integrations are required?"
    header: "Integrations"
    options:
      - label: "REST API"
        description: "Standard HTTP/JSON endpoints"
      - label: "Database direct"
        description: "Direct database access"
      - label: "Message queue"
        description: "Async via queue (Kafka, RabbitMQ, etc.)"
    multiSelect: true
```

## Updating Discovery File

After each Q&A round, append to `$TARGET_DIR/.tasker/clarify-session.md`:

```markdown
### Round N

**Questions:**
1. [Question text]
2. [Question text]

**Answers:**
1. [User's answer]
2. [User's answer]

**Requirements Discovered:**
- [Req 1]
- [Req 2]
```

## Completion Signal

When ALL category goals are met:

1. Verify all categories show "✓ Complete" in the status table
2. Confirm no blocking questions remain
3. Output:
```
<promise>CLARIFIED</promise>
```

**If user requests early exit:** Accept it, but note incomplete categories in the discovery file for Phase 3 to flag as assumptions.

---

# Phase 3 — Synthesis (Derived, Not Asked)

## Purpose
Derive structured requirements AND capabilities from discovery. **No new information introduced here.**

This phase produces TWO outputs:
1. **Spec sections** (human-readable) - Workflows, invariants, interfaces
2. **Capability map** (machine-readable) - For `/plan` to consume

## Process

1. Read `$TARGET_DIR/.tasker/clarify-session.md` completely
2. Extract and organize into spec sections
3. Decompose into capabilities using I.P.S.O. taxonomy
4. Everything must trace to a specific discovery answer

---

## Part A: Spec Sections

### Workflows
Numbered steps with variants and failures:
```markdown
## Workflows

### 1. [Primary Workflow Name]
1. User initiates X
2. System validates Y
3. System performs Z
4. System returns result

**Variants:**
- If [condition], then [alternative flow]

**Failures:**
- If [error], then [error handling]

**Postconditions:**
- [State after completion]
```

### Invariants
Bulleted rules that must ALWAYS hold:
```markdown
## Invariants
- [Rule that must never be violated]
- [Another invariant]
```

### Interfaces
Only if a boundary exists:
```markdown
## Interfaces
- [Interface description]

(or "No new/changed interfaces" if none)
```

### Open Questions
Classified by blocking status:
```markdown
## Open Questions

### Blocking
- [Question that affects workflows/invariants/interfaces]

### Non-blocking
- [Question about internal preferences only]
```

---

## Part A.5: Behavior Model Compilation (FSM)

After synthesizing Workflows, Invariants, and Interfaces, compile the Behavior Model (state machine).

### Purpose

The FSM serves two purposes:
1. **QA during implementation** - Shapes acceptance criteria, enables transition/guard coverage verification
2. **Documentation** - Human-readable diagrams for ongoing system understanding

### CRITICAL INVARIANT: Canonical Truth

> **FSM JSON is canonical; Mermaid is generated. `/plan` and `/execute` must fail if required transitions and invariants lack coverage evidence.**

- **Canonical artifacts**: `*.states.json`, `*.transitions.json`, `index.json`
- **Derived artifacts**: `*.mmd` (Mermaid diagrams) - generated ONLY from canonical JSON
- **NEVER** manually edit `.mmd` files - regenerate from JSON using `fsm-mermaid.py`
- If Mermaid is ever edited manually, the system loses machine trust

### Compilation Steps

1. **Identify Steel Thread Flow**: The primary end-to-end workflow
2. **Derive States**: Convert workflow steps to business states
   - Initial state from workflow trigger
   - Normal states from step postconditions
   - Success terminal from workflow completion
   - Failure terminals from failure clauses
3. **Derive Transitions**: Convert step sequences, variants, and failures
   - Happy path: step N → step N+1
   - Variants: conditional branches with guards
   - Failures: error transitions to failure states
4. **Link Guards to Invariants**: Map spec invariants to transition guards
5. **Validate Completeness**: Run I1-I5 checks (see below)
6. **Resolve Ambiguity**: Use AskUserQuestion for any gaps

### Completeness Invariants

The FSM MUST satisfy these invariants:

| ID | Invariant | Check |
|----|-----------|-------|
| I1 | Steel Thread FSM mandatory | At least one machine for primary workflow |
| I2 | Behavior-first | No architecture dependencies required |
| I3 | Completeness | Initial state, terminals, no dead ends |
| I4 | Guard-Invariant linkage | Every guard links to an invariant ID |
| I5 | No silent ambiguity | Vague terms resolved or flagged as Open Questions |

### Complexity Triggers (Splitting Rules)

Create additional machines based on **structural heuristics**, not just state count:

**State Count Triggers:**
- Steel Thread exceeds 12 states → split into domain-level sub-machines
- Any machine exceeds 20 states → mandatory split

**Structural Triggers (split even with fewer states):**
- **Divergent user journeys**: Two or more distinct journeys that share only an initial prefix, then branch into unrelated flows → separate machines for each journey
- **Unrelated failure clusters**: Multiple failure states that handle different categories of errors (e.g., validation errors vs. system errors vs. business rule violations) → group related failures into their own machines
- **Mixed abstraction levels**: Machine combines business lifecycle states (e.g., Order: Created → Paid → Shipped) with UI microstates (e.g., Modal: Open → Editing → Validating) → separate business lifecycle from UI state machines
- **Cross-boundary workflows**: Workflow that spans multiple bounded contexts or domains → domain-level machine for each context

**Hierarchy Guidelines:**
- `steel_thread` level: Primary end-to-end flow
- `domain` level: Sub-flows within a bounded context
- `entity` level: Lifecycle states for a specific entity

### Ambiguity Resolution

If the compiler detects ambiguous workflow language, use AskUserQuestion:

```json
{
  "question": "The workflow step '{step}' has ambiguous outcome. What business state results?",
  "header": "FSM State",
  "options": [
    {"label": "Define state", "description": "I'll provide the state name"},
    {"label": "Same as previous", "description": "Remains in current state"},
    {"label": "Terminal success", "description": "Workflow completes successfully"},
    {"label": "Terminal failure", "description": "Workflow fails with error"}
  ]
}
```

### FSM Output Structure

The FSM artifacts will be exported to `{TARGET}/docs/state-machines/<slug>/`:
- `index.json` - Machine list, hierarchy, primary machine
- `steel-thread.states.json` - State definitions (S1, S2, ...)
- `steel-thread.transitions.json` - Transition definitions (TR1, TR2, ...)
- `steel-thread.mmd` - Mermaid stateDiagram-v2 for visualization
- `steel-thread.notes.md` - Ambiguity resolutions and rationale

### ID Conventions (FSM-specific)

- Machines: `M1`, `M2`, `M3`...
- States: `S1`, `S2`, `S3`...
- Transitions: `TR1`, `TR2`, `TR3`...

### Traceability (Spec Provenance - MANDATORY)

Every state and transition MUST have a `spec_ref` pointing to the specific workflow step, variant, or failure that defined it. This prevents "FSM hallucination" where states/transitions are invented without spec basis.

**Required for each state:**
- `spec_ref.quote` - Verbatim text from the spec that defines this state
- `spec_ref.location` - Section reference (e.g., "Workflow 1, Step 3")

**Required for each transition:**
- `spec_ref.quote` - Verbatim text from the spec that defines this transition
- `spec_ref.location` - Section reference (e.g., "Workflow 1, Variant 2")

**If no spec text exists for an element:**
1. The element should NOT be created (likely FSM hallucination)
2. Or, use AskUserQuestion to get clarification and document the decision

---

## Part B: Capability Extraction

Extract capabilities from the synthesized workflows using **I.P.S.O. decomposition**.

### I.P.S.O. Behavior Taxonomy

For each capability, identify behaviors by type:

| Type | Description | Examples |
|------|-------------|----------|
| **Input** | Validation, parsing, authentication | Validate email format, parse JSON body |
| **Process** | Calculations, decisions, transformations | Calculate total, apply discount rules |
| **State** | Database reads/writes, cache operations | Save order, fetch user profile |
| **Output** | Responses, events, notifications | Return JSON, emit event, send email |

### Domain Grouping

Group related capabilities into domains:
- **Authentication** - Login, logout, session management
- **User Management** - Profile, preferences, settings
- **Core Feature** - The primary business capability
- etc.

### Steel Thread Identification

Identify the **steel thread** - the minimal end-to-end flow that proves the system works:
- Mark one flow as `is_steel_thread: true`
- This becomes the critical path for Phase 1 implementation

### Capability Map Output

Write to `{TARGET}/docs/specs/<slug>.capabilities.json`:

```json
{
  "version": "1.0",
  "spec_checksum": "<first 16 chars of SHA256 of spec>",

  "domains": [
    {
      "id": "D1",
      "name": "Authentication",
      "description": "User identity and access",
      "capabilities": [
        {
          "id": "C1",
          "name": "User Login",
          "discovery_ref": "Round 3, Q2: User confirmed email/password auth",
          "behaviors": [
            {"id": "B1", "name": "ValidateCredentials", "type": "input", "description": "Check email/password format"},
            {"id": "B2", "name": "VerifyPassword", "type": "process", "description": "Compare hash"},
            {"id": "B3", "name": "CreateSession", "type": "state", "description": "Store session"},
            {"id": "B4", "name": "ReturnToken", "type": "output", "description": "JWT response"}
          ]
        }
      ]
    }
  ],

  "flows": [
    {
      "id": "F1",
      "name": "Login Flow",
      "is_steel_thread": true,
      "steps": [
        {"order": 1, "behavior_id": "B1", "description": "Validate input"},
        {"order": 2, "behavior_id": "B2", "description": "Check password"},
        {"order": 3, "behavior_id": "B3", "description": "Create session"},
        {"order": 4, "behavior_id": "B4", "description": "Return JWT"}
      ]
    }
  ],

  "invariants": [
    {"id": "I1", "description": "Passwords must never be logged", "discovery_ref": "Round 5, Q1"}
  ],

  "coverage": {
    "total_requirements": 15,
    "covered_requirements": 15,
    "gaps": []
  }
}
```

### ID Conventions

- Domains: `D1`, `D2`, `D3`...
- Capabilities: `C1`, `C2`, `C3`...
- Behaviors: `B1`, `B2`, `B3`...
- Flows: `F1`, `F2`, `F3`...
- Invariants: `I1`, `I2`, `I3`...

### Traceability

Every capability and invariant MUST have a `discovery_ref` pointing to the specific round and question in `$TARGET_DIR/.tasker/clarify-session.md` that established it.

---

# Phase 4 — Architecture Sketch

## Rule
Architecture MUST come **AFTER** workflows, invariants, interfaces.

## Process

Use AskUserQuestion to either:

**Option A: Ask for sketch**
```
questions:
  - question: "Can you provide a brief architecture sketch for this feature?"
    header: "Architecture"
    options:
      - label: "I'll describe it"
        description: "User provides architecture overview"
      - label: "Propose one"
        description: "Agent proposes architecture for review"
```

**Option B: Propose and confirm**
Present a brief sketch and ask for confirmation/edits.

## Output

Populate **Architecture sketch** section:
```markdown
## Architecture sketch
- **Components touched:** [list]
- **Responsibilities:** [brief description]
- **Failure handling:** [brief description]
```

**Keep this SHORT. No essays.**

---

# Phase 5 — Decisions & ADRs

## ADR Trigger

**Create ADR if ANY of these are true:**
- Hard to reverse
- Reusable standard
- Tradeoff-heavy
- Cross-cutting
- NFR-impacting

**If none apply → record decision in spec only.**

## Decision Facilitation Rules

### FACILITATE a decision ONLY IF ALL are true:
1. ADR-worthy (meets trigger above)
2. Not already decided (no existing ADR, no explicit user preference)
3. Blocking workflows, invariants, or interfaces

### DO NOT FACILITATE if ANY are true:
- Already decided
- Local / reversible / implementation detail
- Non-blocking
- User not ready to decide
- Too many options (>3)
- Premature (behavior not defined yet)

## Facilitation Format

If facilitation is allowed:

```
questions:
  - question: "How should we approach [decision topic]?"
    header: "Decision"
    options:
      - label: "Option A: [Name]"
        description: "Consequences: [1], [2]"
      - label: "Option B: [Name]"
        description: "Consequences: [1], [2]"
      - label: "Need more info"
        description: "Defer decision, add to Open Questions"
```

## Outcomes

- **User chooses option** → Write decision to spec + create ADR (Accepted)
- **User says "need more info"** → Add as Blocking Open Question (no ADR yet)

## ADR Template

Write ADRs to `{TARGET}/docs/adrs/ADR-####-<slug>.md`:

```markdown
# ADR-####: {Title}

**Status:** Accepted
**Date:** {YYYY-MM-DD}

## Applies To
- [Spec: Feature A](../specs/feature-a.md)
- [Spec: Feature B](../specs/feature-b.md)

## Context
[Why this decision was needed - reference specific discovery round if applicable]

## Decision
[What was decided]

## Alternatives Considered
| Alternative | Pros | Cons | Why Not Chosen |
|-------------|------|------|----------------|
| [Alt 1] | ... | ... | ... |
| [Alt 2] | ... | ... | ... |

## Consequences
- [Consequence 1]
- [Consequence 2]

## Related
- Supersedes: (none | ADR-XXXX)
- Related ADRs: (none | ADR-XXXX, ADR-YYYY)
```

**ADR Rules:**
- ADRs are **immutable** once Accepted
- Changes require a **new ADR** that supersedes the old
- ADRs can apply to multiple specs (many-to-many relationship)
- When creating a new spec that uses an existing ADR, update the ADR's "Applies To" section

---

# Phase 6 — Handoff-Ready Gate

## Preliminary Check (ALL must pass)

| Check | Requirement |
|-------|-------------|
| Phases complete | All phases 1-5 completed in order |
| No blocking questions | Zero Blocking Open Questions |
| Interfaces present | Interfaces section exists (even if "none") |
| Decisions present | Decisions section exists |
| Workflows defined | At least one workflow with variants/failures |
| Invariants stated | At least one invariant |
| FSM compiled | Steel Thread FSM compiled with I1-I5 passing |

## Spec Completeness Check (Checklist C1-C11)

Run the checklist verification against the current spec draft:

```bash
tasker spec checklist /tmp/claude/spec-draft.md
```

This verifies the spec contains all expected sections:

| Category | Critical Items (must pass) |
|----------|---------------------------|
| C2: Data Model | Tables defined, fields typed, constraints stated |
| C3: API | Endpoints listed, request/response schemas, auth requirements |
| C4: Behavior | Observable behaviors, business rules |
| C7: Security | Authentication mechanism, authorization rules |

### Handling Checklist Gaps

For **critical missing items** (marked with *):

1. If the spec SHOULD have this content → return to Phase 2 (Clarify) to gather requirements
2. If the spec legitimately doesn't need this → document as N/A with rationale

Example checklist failure:
```
## Gate FAILED - Incomplete Spec

Checklist verification found critical gaps:

- [✗] C2.4*: Constraints stated (UNIQUE, CHECK, FK)?
  → Data model section exists but no constraints defined

- [✗] C3.2*: Request schemas defined?
  → API endpoints listed but request bodies not specified

Action: Return to Phase 2 to clarify data constraints and API request formats.
```

## Gate Failure

If gate fails:
1. List exact blockers
2. **STOP** - do not proceed to spec review
3. Tell user what must be resolved

Example:
```
## Gate FAILED

Cannot proceed. The following must be resolved:

1. **Blocking Open Questions:**
   - How should rate limiting work across tenants?
   - What is the retry policy for failed webhooks?

2. **Missing Sections:**
   - Interfaces section not present

3. **Checklist Gaps (Critical):**
   - C2.4: No database constraints defined
   - C3.2: API request schemas missing
```

---

# Phase 7 — Spec Review (MANDATORY)

## Purpose
Run automated weakness detection to catch issues before export. This is the final quality gate.

## Process

### Step 1: Write Draft Spec to Temp Location

Write the current spec draft to a temporary file for analysis:

```bash
# Write draft spec for analysis
cat > /tmp/claude/spec-draft.md << 'EOF'
[Current spec content]
EOF
```

### Step 2: Run Weakness Detection

```bash
tasker spec review /tmp/claude/spec-draft.md
```

This detects:
- **W1: Non-behavioral requirements** - DDL/schema not stated as behavior
- **W2: Implicit requirements** - Constraints assumed but not explicit
- **W3: Cross-cutting concerns** - Config, observability, lifecycle
- **W4: Missing acceptance criteria** - Qualitative terms without metrics
- **W5: Fragmented requirements** - Cross-references needing consolidation
- **W6: Contradictions** - Conflicting statements
- **W7: Ambiguity** - Vague quantifiers, undefined scope, weak requirements, passive voice
- **CK-*: Checklist gaps** - Critical missing content from C1-C11 categories

W7 Ambiguity patterns include:
- Vague quantifiers ("some", "many", "several")
- Undefined scope ("etc.", "and so on")
- Vague conditionals ("if applicable", "when appropriate")
- Weak requirements ("may", "might", "could")
- Passive voice hiding actor ("is handled", "will be processed")
- Vague timing ("quickly", "soon", "eventually")
- Subjective qualifiers ("reasonable", "appropriate")
- Unquantified limits ("large", "fast", "slow")

### Step 3: Handle Critical Weaknesses

For **CRITICAL** weaknesses (W1, W6, W7 with weak requirements), engage user:

#### W1: Non-Behavioral Requirements

```json
{
  "question": "The spec contains DDL/schema that isn't stated as behavioral requirement: '{spec_quote}'. How should this be treated?",
  "header": "DDL Mandate",
  "options": [
    {"label": "DB-level required", "description": "MUST be implemented as database-level constraint"},
    {"label": "App-layer OK", "description": "Application-layer validation is sufficient"},
    {"label": "Documentation only", "description": "This is reference documentation, not a requirement"}
  ]
}
```

#### W6: Contradictions

```json
{
  "question": "Conflicting statements found: {description}. Which is authoritative?",
  "header": "Conflict",
  "options": [
    {"label": "First statement", "description": "{first_quote}"},
    {"label": "Second statement", "description": "{second_quote}"},
    {"label": "Clarify", "description": "I'll provide clarification"}
  ]
}
```

#### W7: Ambiguity

Each W7 weakness includes a clarifying question. Use AskUserQuestion with the auto-generated question:

```json
{
  "question": "{weakness.question or weakness.suggested_resolution}",
  "header": "Clarify",
  "options": [
    {"label": "Specify value", "description": "I'll provide a specific value/definition"},
    {"label": "Not required", "description": "This is not a hard requirement"},
    {"label": "Use default", "description": "Use a sensible default"}
  ]
}
```

Example clarifying questions by ambiguity type:
- Vague quantifier: "How many specifically? Provide a number or range."
- Weak requirement: "Is this required or optional? If optional, under what conditions?"
- Vague timing: "What is the specific timing? (e.g., <100ms, every 5 minutes)"
- Passive voice: "What component/system performs this action?"

#### CK-*: Checklist Gaps

For critical checklist gaps that weren't caught in Phase 6 Gate:

```json
{
  "question": "The spec is missing {checklist_item}. Should this be added?",
  "header": "Missing Content",
  "options": [
    {"label": "Add it", "description": "I'll provide the missing information"},
    {"label": "N/A", "description": "This spec doesn't need this (document why)"},
    {"label": "Defer", "description": "Address in a follow-up spec"}
  ]
}
```

### Step 4: Record and Apply Resolutions

For each resolved weakness:

1. **Record the resolution** for downstream consumers (logic-architect):
```bash
tasker spec add-resolution {weakness_id} {resolution_type} \
    --response "{user_response}" \
    --notes "{context}"
```

Resolution types:
- `mandatory` - MUST be implemented as specified (W1 DDL requirements)
- `clarified` - User provided specific value/definition (W7 ambiguity)
- `not_applicable` - Doesn't apply to this spec (checklist gaps)
- `defer` - Address in follow-up work

2. **Update the spec content** to address the issue:
   - If W1 resolved as "mandatory", add explicit behavioral statement
   - If W6 resolved, remove contradictory statement
   - If W7 resolved, replace ambiguous language with specific terms
   - If CK-* resolved as "not_applicable", document rationale

### Step 5: Re-run Until Clean

```bash
# Re-run analysis
tasker spec review /tmp/claude/spec-draft.md
```

**Continue until:**
- Zero critical weaknesses remain, OR
- All critical weaknesses have been explicitly accepted by user

### Step 6: Save Review Results

Save the final review results:

```bash
tasker spec review /tmp/claude/spec-draft.md > $TARGET_DIR/.tasker/spec-review.json
```

## Spec Review Gate

| Check | Requirement |
|-------|-------------|
| No critical weaknesses | All W1, W6, critical W7, CK-* resolved or accepted |
| Resolutions recorded | `spec-resolutions.json` contains all resolution decisions |
| Review file saved | `$TARGET_DIR/.tasker/spec-review.json` exists |

Check resolution status:
```bash
tasker spec unresolved
```

If critical weaknesses remain unresolved, **STOP** and ask user to resolve.

---

# Phase 8 — Export

## Write Files

Only after spec review passes. All permanent artifacts go to the **TARGET project**.

### 1. Ensure Target Directory Structure

```bash
mkdir -p {TARGET}/docs/specs {TARGET}/docs/adrs {TARGET}/docs/state-machines/<slug>
```

### 2. Spec Packet
Write to `{TARGET}/docs/specs/<slug>.md`:

```markdown
# Spec: {Title}

## Related ADRs
- [ADR-0001: Decision Title](../adrs/ADR-0001-decision-title.md)
- [ADR-0002: Another Decision](../adrs/ADR-0002-another-decision.md)

## Goal
[From Phase 1]

## Non-goals
[From Phase 1]

## Done means
[From Phase 1]

## Tech Stack
[From Phase 1 - summarized]

**Language & Runtime:**
- [e.g., Python 3.12+, Node.js 20+, Go 1.22+]

**Frameworks:**
- [e.g., FastAPI, Next.js, Chi]

**Data:**
- [e.g., PostgreSQL, Redis, SQLite]

**Infrastructure:**
- [e.g., Docker, AWS Lambda, Kubernetes]

**Testing:**
- [e.g., pytest, Jest, go test]

(Remove sections that don't apply)

## Workflows
[From Phase 3]

## Invariants
[From Phase 3]

## Interfaces
[From Phase 3]

## Architecture sketch
[From Phase 4]

## Decisions
Summary of key decisions made during specification:

| Decision | Rationale | ADR |
|----------|-----------|-----|
| [Decision 1] | [Why] | [ADR-0001](../adrs/ADR-0001-slug.md) |
| [Decision 2] | [Why] | (inline - not ADR-worthy) |

## Open Questions

### Blocking
(none - gate passed)

### Non-blocking
- [Any remaining non-blocking questions]

## Agent Handoff
- **What to build:** [Summary]
- **Must preserve:** [Key constraints]
- **Blocking conditions:** None

## Artifacts
- **Capability Map:** [<slug>.capabilities.json](./<slug>.capabilities.json)
- **Behavior Model (FSM):** [fsm/<slug>/](../fsm/<slug>/) - State machine diagrams
- **Discovery Log:** Archived in tasker project
```

### 3. Capability Map
Write to `{TARGET}/docs/specs/<slug>.capabilities.json` (from Phase 3 Synthesis).

Validate against schema:
```bash
tasker state validate capability_map --file {TARGET}/docs/specs/<slug>.capabilities.json
```

### 4. Behavior Model (FSM)

Export FSM artifacts to `{TARGET}/docs/state-machines/<slug>/`:

```bash
# Compile FSM from capability map and spec
tasker fsm from-capability-map \
    {TARGET}/docs/specs/<slug>.capabilities.json \
    {TARGET}/docs/specs/<slug>.md \
    --output-dir {TARGET}/docs/state-machines/<slug>

# Generate Mermaid diagrams and notes
tasker fsm mermaid {TARGET}/docs/state-machines/<slug>

# Validate FSM artifacts (I1-I5 invariants)
tasker fsm validate validate {TARGET}/docs/state-machines/<slug>
```

Validate against schemas:
```bash
tasker fsm validate {TARGET}/docs/state-machines/<slug>
```

### 6. ADR Files (0..N)
Write each ADR to `{TARGET}/docs/adrs/ADR-####-<slug>.md`.

### 7. Spec Review Results
Verify `$TARGET_DIR/.tasker/spec-review.json` is saved.

## Completion Message

```markdown
## Specification Complete

**Exported to {TARGET}/docs/:**
- `specs/<slug>.md` (human-readable spec)
- `specs/<slug>.capabilities.json` (machine-readable for /plan)
- `fsm/<slug>/` (behavior model - state machine)
  - `index.json`, `steel-thread.states.json`, `steel-thread.transitions.json`
  - `steel-thread.mmd` (Mermaid diagram)
- `adrs/ADR-####-*.md` (N ADRs)

**Working files (in $TARGET_DIR/.tasker/):**
- `clarify-session.md` (discovery log)
- `spec-review.json` (weakness analysis)
- `state.json` (session state)

**Capabilities Extracted:**
- Domains: N
- Capabilities: N
- Behaviors: N
- Steel Thread: F1 (name)

**Behavior Model (FSM) Summary:**
- Machines: N (primary: M1 Steel Thread)
- States: N
- Transitions: N
- Guards linked to invariants: N

**Spec Review Summary:**
- Total weaknesses detected: X
- Critical resolved: Y
- Warnings noted: Z

**Next steps:**
- Review exported spec, capability map, and FSM diagrams
- Run `/plan {TARGET}/docs/specs/<slug>.md` to begin task decomposition
```

---

# Non-Goals (Skill-Level)

- No Git automation (user commits manually)
- No project management (no Jira/Linear integration)
- No runtime ops/runbooks
- No over-facilitation (don't ask unnecessary questions)
- No architectural debates before behavior is defined
- No file/task mapping (that's `/plan`'s job)

---

# Commands

| Command | Action |
|---------|--------|
| `/specify` | Start new specification workflow |
| `/specify resume` | Resume interrupted session from `$TARGET_DIR/.tasker/clarify-session.md` |
| `/specify status` | Show current phase and progress |

---

# Integration with /plan

After `/specify` completes, user runs:
```
/plan {TARGET}/docs/specs/<slug>.md
```

Because `/specify` already produced a capability map and FSM, `/plan` can **skip** these phases:
- Spec Review (already done)
- Capability Extraction (already done)

`/plan` starts directly at **Physical Mapping** (mapping capabilities to files).

Additionally, `/plan` will:
- Load FSM artifacts from `{TARGET}/docs/state-machines/<slug>/`
- Validate transition coverage (every FSM transition → ≥1 task)
- Generate FSM-aware acceptance criteria for tasks

# Integration with /execute

When executing tasks, `/execute` will:
- Load FSM artifacts for adherence verification
- For each task with FSM context:
  - Verify transitions are implemented
  - Verify guards are enforced
  - Verify states are reachable
- Include FSM verification results in task completion
