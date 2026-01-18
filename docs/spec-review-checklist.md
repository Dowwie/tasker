# Spec Review Checklist

Systematic verification of spec completeness before capability extraction.

---

## Ambiguity Detection (W7)

Beyond the checklist, the spec-reviewer detects ambiguous language that requires clarification:

| Ambiguity Type | Example | Clarifying Question |
|----------------|---------|---------------------|
| Vague quantifier | "several retries" | How many specifically? |
| Undefined scope | "fields etc." | What specifically is included? |
| Vague conditional | "if applicable" | Under what conditions? |
| Weak requirement | "may support X" | Is this required or optional? |
| Passive actor | "errors are handled" | What component does this? |
| Vague timing | "respond quickly" | What is the SLA? (e.g., <100ms) |
| Unspecified behavior | "handle properly" | What does 'properly' mean? |
| Unresolved or | "X or Y must" | Which one, or both? |
| Subjective qualifier | "reasonable timeout" | What value is 'reasonable'? |
| Unquantified limit | "large payload" | What size threshold? |

---

## Checklist Categories

### C1: Structural Completeness

| Item | Question | Status |
|------|----------|--------|
| C1.1 | Does the spec have a clear problem statement or purpose? | |
| C1.2 | Are functional requirements explicitly listed (not just implied)? | |
| C1.3 | Are non-functional requirements stated (performance, security, etc.)? | |
| C1.4 | Is the scope clearly bounded (what's in vs out)? | |
| C1.5 | Are phases/milestones defined if multi-phase? | |

### C2: Data Model Completeness

| Item | Question | Status |
|------|----------|--------|
| C2.1 | Are all entities/tables defined with their purpose? | |
| C2.2 | Are all fields defined with types? | |
| C2.3 | Are required vs optional fields distinguished? | |
| C2.4 | Are all constraints stated (UNIQUE, CHECK, FK)? | |
| C2.5 | Are indexes specified for query patterns? | |
| C2.6 | Are default values documented? | |
| C2.7 | Are cascade behaviors defined (ON DELETE, ON UPDATE)? | |

### C3: API Completeness

| Item | Question | Status |
|------|----------|--------|
| C3.1 | Are all endpoints listed with HTTP methods? | |
| C3.2 | Are request schemas defined (body, query params, headers)? | |
| C3.3 | Are response schemas defined for success cases? | |
| C3.4 | Are error responses defined (status codes, error bodies)? | |
| C3.5 | Are authentication requirements stated per endpoint? | |
| C3.6 | Are rate limits or quotas specified? | |

### C4: Behavior Completeness

| Item | Question | Status |
|------|----------|--------|
| C4.1 | Is each feature described as observable behavior (not just structure)? | |
| C4.2 | Are state transitions explicitly defined? | |
| C4.3 | Are business rules stated (validation, calculations)? | |
| C4.4 | Are edge cases addressed (empty, null, max values)? | |
| C4.5 | Are concurrent access behaviors defined? | |
| C4.6 | Are idempotency requirements stated? | |

### C5: Error Handling

| Item | Question | Status |
|------|----------|--------|
| C5.1 | Are all error conditions enumerated? | |
| C5.2 | Are error messages/codes defined? | |
| C5.3 | Are retry behaviors specified? | |
| C5.4 | Are fallback behaviors defined? | |
| C5.5 | Are partial failure scenarios addressed? | |

### C6: Configuration

| Item | Question | Status |
|------|----------|--------|
| C6.1 | Are all environment variables listed? | |
| C6.2 | Are types specified for each config value? | |
| C6.3 | Are defaults documented? | |
| C6.4 | Are required vs optional configs distinguished? | |
| C6.5 | Are valid value ranges/formats specified? | |

### C7: Security

| Item | Question | Status |
|------|----------|--------|
| C7.1 | Are authentication mechanisms specified? | |
| C7.2 | Are authorization rules defined (who can do what)? | |
| C7.3 | Are sensitive data handling requirements stated? | |
| C7.4 | Are audit/logging requirements for security events defined? | |
| C7.5 | Are input validation requirements specified? | |

### C8: Observability

| Item | Question | Status |
|------|----------|--------|
| C8.1 | Are logging requirements defined (what to log, levels)? | |
| C8.2 | Are metrics specified (counters, gauges, histograms)? | |
| C8.3 | Are tracing spans defined for key operations? | |
| C8.4 | Are health check endpoints specified? | |
| C8.5 | Are alerting thresholds defined? | |

### C9: Performance & Limits

| Item | Question | Status |
|------|----------|--------|
| C9.1 | Are response time SLAs defined? | |
| C9.2 | Are throughput requirements stated? | |
| C9.3 | Are resource limits specified (memory, connections)? | |
| C9.4 | Are timeout values defined? | |
| C9.5 | Are pagination/batch size limits specified? | |

### C10: Integration & Dependencies

| Item | Question | Status |
|------|----------|--------|
| C10.1 | Are external service dependencies listed? | |
| C10.2 | Are external API contracts documented? | |
| C10.3 | Are failure modes for dependencies addressed? | |
| C10.4 | Are version compatibility requirements stated? | |

### C11: Lifecycle

| Item | Question | Status |
|------|----------|--------|
| C11.1 | Is startup sequence defined? | |
| C11.2 | Is graceful shutdown behavior specified? | |
| C11.3 | Are migration/upgrade paths documented? | |
| C11.4 | Are data retention policies stated? | |

---

## Status Values

- **✓ Complete** - Requirement is fully specified
- **⚠ Partial** - Requirement exists but needs clarification
- **✗ Missing** - Requirement not addressed in spec
- **N/A** - Not applicable to this project

---

## Severity by Category

| Category | Severity if Missing |
|----------|---------------------|
| C1 (Structure) | Warning - proceed with notes |
| C2 (Data Model) | **Critical** - blocks planning |
| C3 (API) | **Critical** - blocks planning |
| C4 (Behavior) | **Critical** - blocks planning |
| C5 (Errors) | Warning - create placeholder tasks |
| C6 (Config) | Warning - create config task |
| C7 (Security) | **Critical** - must clarify auth model |
| C8 (Observability) | Info - defer to implementation |
| C9 (Performance) | Info - use sensible defaults |
| C10 (Integration) | Warning if external deps exist |
| C11 (Lifecycle) | Info - defer to implementation |

---

## Usage

The spec-reviewer agent should:

1. **Score each checklist item** against the spec
2. **Flag critical gaps** (C2, C3, C4, C7 missing items)
3. **Use AskUserQuestion** for critical gaps
4. **Document partial items** for logic-architect context
5. **Record N/A items** with justification

### Example AskUserQuestion for C2.4 (Constraints)

```json
{
  "question": "The spec defines tables but doesn't specify uniqueness constraints. Should duplicate records be allowed?",
  "header": "Constraints",
  "options": [
    {"label": "Specify constraints", "description": "I'll provide the uniqueness rules now"},
    {"label": "App-layer only", "description": "Enforce uniqueness in application code, not DB"},
    {"label": "Allow duplicates", "description": "Duplicates are acceptable for this data"}
  ]
}
```

### Example AskUserQuestion for C7.1 (Authentication)

```json
{
  "question": "The spec doesn't specify an authentication mechanism. How should users authenticate?",
  "header": "Auth Method",
  "options": [
    {"label": "API Key", "description": "Simple API key in header"},
    {"label": "JWT/OAuth", "description": "Token-based authentication"},
    {"label": "Session", "description": "Cookie-based sessions"},
    {"label": "None", "description": "No authentication required"}
  ]
}
```
