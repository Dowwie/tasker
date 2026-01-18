#!/usr/bin/env python3
"""
Spec Review - Detect weakness categories in specifications.

Pre-planning analysis to catch gap-prone requirement patterns before
capability extraction. Based on gap analysis of tisk project.

Usage:
    spec-review.py analyze <spec_path>           Detect weaknesses in spec (outputs JSON)
    spec-review.py report <planning_dir>         Generate human-readable report
    spec-review.py status <planning_dir>         Check resolution status (READY/BLOCKED)
    spec-review.py checklist <planning_dir>      Show completeness checklist status
    spec-review.py unresolved <planning_dir>     List unresolved critical weaknesses
    spec-review.py add-resolution <dir> <id> <resolution> [--notes 'text']
                                                 Record a weakness resolution

Weakness Categories:
    W1: Non-Behavioral Requirements  - DDL/schemas framed as structure, not behavior
    W2: Implicit Requirements        - Inferred from examples/DDL, not stated
    W3: Cross-Cutting Concerns       - Config, observability spanning components
    W4: Missing Acceptance Criteria  - Qualitative requirements without measures
    W5: Fragmented Requirements      - Same requirement split across sections
    W6: Contradictions               - Conflicting statements in spec
    W7: Ambiguity                    - Vague terms requiring clarification

Checklist Categories (CK-*):
    C1: Structural Completeness      - Purpose, scope, requirements listed
    C2: Data Model Completeness      - Tables, fields, constraints, indexes
    C3: API Completeness             - Endpoints, schemas, errors, auth
    C4: Behavior Completeness        - State transitions, business rules
    C5: Error Handling               - Error conditions, codes, retries
    C6: Configuration                - Env vars, types, defaults
    C7: Security                     - Auth, authz, sensitive data
    C8: Observability                - Logging, metrics, health checks
    C9: Performance                  - SLAs, timeouts
    C10: Integration                 - External dependencies, contracts
    C11: Lifecycle                   - Startup, shutdown

Resolution Types:
    mandatory       - MUST be implemented as specified
    optional        - Nice-to-have, not blocking
    defer           - Deferred to later phase
    clarified       - User provided clarification
    not_applicable  - Not a requirement

All state is persisted to files in {planning_dir}/artifacts/.
"""

import hashlib
import json
import re
import sys
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Literal

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent


@dataclass
class Weakness:
    """Detected spec weakness."""

    id: str
    category: Literal[
        "non_behavioral",
        "implicit",
        "cross_cutting",
        "missing_ac",
        "fragmented",
        "contradiction",
    ]
    severity: Literal["critical", "warning", "info"]
    location: str
    description: str
    spec_quote: str = ""
    suggested_resolution: str = ""
    behavioral_reframe: str = ""


@dataclass
class SpecReview:
    """Complete spec review result."""

    version: str = "1.0"
    spec_checksum: str = ""
    analyzed_at: str = ""
    weaknesses: list[Weakness] = field(default_factory=list)
    status: Literal["pending", "in_review", "resolved"] = "pending"
    summary: dict = field(default_factory=dict)


# =============================================================================
# DETECTION: W1 - Non-Behavioral Requirements
# =============================================================================

DDL_PATTERNS = [
    (r"CREATE\s+TABLE", "table definition"),
    (r"CREATE\s+(UNIQUE\s+)?INDEX", "index definition"),
    (r"CREATE\s+(OR\s+REPLACE\s+)?FUNCTION", "function definition"),
    (r"CREATE\s+TRIGGER", "trigger definition"),
    (r"CONSTRAINT\s+\w+\s+(UNIQUE|CHECK|FOREIGN\s+KEY|PRIMARY\s+KEY)", "constraint"),
    (r"ALTER\s+TABLE.*ADD\s+CONSTRAINT", "constraint addition"),
]

SCHEMA_PATTERNS = [
    (r'"type"\s*:\s*"(object|array|string|integer)"', "JSON Schema"),
    (r"openapi:\s*['\"]?\d+\.\d+", "OpenAPI spec"),
    (r"schema:\s*\n\s+type:", "YAML schema"),
]


def detect_non_behavioral(content: str, lines: list[str]) -> list[Weakness]:
    """Detect DDL and schema definitions that should be behavioral requirements."""
    weaknesses = []
    weakness_counter = 0

    for pattern, desc in DDL_PATTERNS:
        for match in re.finditer(pattern, content, re.IGNORECASE | re.MULTILINE):
            weakness_counter += 1
            start = match.start()
            line_num = content[:start].count("\n") + 1

            # Extract context (the full statement)
            context_start = max(0, start - 50)
            context_end = min(len(content), match.end() + 200)
            context = content[context_start:context_end].strip()

            # Find end of statement (semicolon or closing paren for constraints)
            stmt_match = re.search(r"[^;]+;|\)[^)]*\)", context)
            if stmt_match:
                quote = stmt_match.group(0)[:150]
            else:
                quote = context[:150]

            weaknesses.append(
                Weakness(
                    id=f"W1-{weakness_counter:03d}",
                    category="non_behavioral",
                    severity="critical" if "CONSTRAINT" in pattern else "warning",
                    location=f"line {line_num}",
                    description=f"DDL {desc} not stated as behavioral requirement",
                    spec_quote=quote.replace("\n", " ").strip(),
                    suggested_resolution=f"Add prose: 'The system MUST enforce {desc}'",
                    behavioral_reframe=_suggest_behavioral_reframe(desc, quote),
                )
            )

    return weaknesses


def _suggest_behavioral_reframe(desc: str, quote: str) -> str:
    """Suggest behavioral reframing for DDL."""
    if "UNIQUE" in quote.upper():
        match = re.search(r"unique\s*\(([^)]+)\)", quote, re.IGNORECASE)
        if match:
            cols = match.group(1)
            return f"The system MUST reject duplicate ({cols}) combinations"
    if "CHECK" in quote.upper():
        match = re.search(r"check\s*\(([^)]+)\)", quote, re.IGNORECASE)
        if match:
            condition = match.group(1)
            return f"The system MUST validate that {condition}"
    if "INDEX" in quote.upper():
        return "The system SHOULD provide efficient query performance for this access pattern"
    if "TRIGGER" in quote.upper():
        return "The system MUST automatically enforce this rule on data changes"
    return f"The system MUST implement this {desc}"


# =============================================================================
# DETECTION: W2 - Implicit Requirements
# =============================================================================


def detect_implicit(content: str, lines: list[str]) -> list[Weakness]:
    """Detect requirements that are implied but not explicitly stated."""
    weaknesses = []
    weakness_counter = 0

    # Pattern: NOT NULL in DDL without corresponding prose
    for match in re.finditer(
        r"(\w+)\s+\w+.*NOT\s+NULL", content, re.IGNORECASE | re.MULTILINE
    ):
        weakness_counter += 1
        line_num = content[: match.start()].count("\n") + 1
        col_name = match.group(1)

        weaknesses.append(
            Weakness(
                id=f"W2-{weakness_counter:03d}",
                category="implicit",
                severity="warning",
                location=f"line {line_num}",
                description=f"NOT NULL constraint on '{col_name}' implied but not stated as requirement",
                spec_quote=match.group(0)[:100],
                suggested_resolution=f"Confirm: Is '{col_name}' required? Add explicit requirement if so.",
            )
        )

    # Pattern: Default values in DDL
    for match in re.finditer(
        r"(\w+).*DEFAULT\s+([^,\n]+)", content, re.IGNORECASE | re.MULTILINE
    ):
        weakness_counter += 1
        line_num = content[: match.start()].count("\n") + 1
        col_name = match.group(1)
        default_val = match.group(2).strip()

        weaknesses.append(
            Weakness(
                id=f"W2-{weakness_counter:03d}",
                category="implicit",
                severity="info",
                location=f"line {line_num}",
                description=f"Default value '{default_val}' for '{col_name}' - confirm this is intentional",
                spec_quote=match.group(0)[:100],
                suggested_resolution="Verify default value is correct and document rationale",
            )
        )

    return weaknesses


# =============================================================================
# DETECTION: W3 - Cross-Cutting Concerns
# =============================================================================

CONFIG_TABLE_PATTERNS = [
    r"\|\s*Variable\s*\|\s*Type\s*\|",  # Markdown table header
    r"\|\s*`?\w+`?\s*\|\s*(str|int|float|bool)\s*\|",  # Config row
    r"Environment\s+Variables?",  # Section header
    r"Configuration\s+(Schema|Variables?)",  # Section header
]

OBSERVABILITY_PATTERNS = [
    r"(Metrics?|Traces?|Spans?|Logs?):",
    r"\|\s*Metric\s*\|\s*Type\s*\|",
    r"OTEL|OpenTelemetry|Prometheus|Jaeger",
    r"(p50|p95|p99|latency|histogram|counter|gauge)",
]

LIFECYCLE_PATTERNS = [
    r"Startup\s+(Sequence|Tasks?|Order)",
    r"Shutdown\s+(Sequence|Tasks?|Order)",
    r"Lifespan|Lifecycle",
    r"Health\s+Check",
]


def detect_cross_cutting(content: str, lines: list[str]) -> list[Weakness]:
    """Detect cross-cutting concerns that may not be captured as tasks."""
    weaknesses = []
    weakness_counter = 0

    # Detect config tables
    for pattern in CONFIG_TABLE_PATTERNS:
        for match in re.finditer(pattern, content, re.IGNORECASE | re.MULTILINE):
            weakness_counter += 1
            line_num = content[: match.start()].count("\n") + 1

            # Extract table context
            context_end = min(len(content), match.end() + 500)
            context = content[match.start() : context_end]

            weaknesses.append(
                Weakness(
                    id=f"W3-{weakness_counter:03d}",
                    category="cross_cutting",
                    severity="warning",
                    location=f"line {line_num}",
                    description="Configuration table - ensure each var is wired to a component",
                    spec_quote=context[:200].replace("\n", " "),
                    suggested_resolution="Create dedicated configuration task or mark vars for bundling",
                )
            )
            break  # Only flag once per config section

    # Detect observability requirements
    for pattern in OBSERVABILITY_PATTERNS:
        for match in re.finditer(pattern, content, re.IGNORECASE):
            weakness_counter += 1
            line_num = content[: match.start()].count("\n") + 1

            weaknesses.append(
                Weakness(
                    id=f"W3-{weakness_counter:03d}",
                    category="cross_cutting",
                    severity="warning",
                    location=f"line {line_num}",
                    description="Observability requirement - spans multiple components",
                    spec_quote=match.group(0),
                    suggested_resolution="Create dedicated observability task",
                )
            )
            break  # Only flag once

    # Detect lifecycle requirements
    for pattern in LIFECYCLE_PATTERNS:
        for match in re.finditer(pattern, content, re.IGNORECASE):
            weakness_counter += 1
            line_num = content[: match.start()].count("\n") + 1

            weaknesses.append(
                Weakness(
                    id=f"W3-{weakness_counter:03d}",
                    category="cross_cutting",
                    severity="warning",
                    location=f"line {line_num}",
                    description="Lifecycle requirement - ensure startup/shutdown tasks exist",
                    spec_quote=match.group(0),
                    suggested_resolution="Create dedicated lifecycle task",
                )
            )
            break

    return weaknesses


# =============================================================================
# DETECTION: W4 - Missing Acceptance Criteria
# =============================================================================

QUALITATIVE_PATTERNS = [
    (r"must\s+be\s+(fast|quick|responsive)", "performance without metric"),
    (r"should\s+be\s+secure", "security without specifics"),
    (r"handle\s+errors?\s+gracefully", "error handling without behavior"),
    (r"(clean|maintainable|readable)\s+code", "code quality without measure"),
    (r"user.?friendly", "UX without specifics"),
    (r"scalable", "scalability without metric"),
]


def detect_missing_ac(content: str, lines: list[str]) -> list[Weakness]:
    """Detect requirements that can't be turned into acceptance criteria."""
    weaknesses = []
    weakness_counter = 0

    for pattern, desc in QUALITATIVE_PATTERNS:
        for match in re.finditer(pattern, content, re.IGNORECASE):
            weakness_counter += 1
            line_num = content[: match.start()].count("\n") + 1

            # Get surrounding context
            start = max(0, match.start() - 50)
            end = min(len(content), match.end() + 50)
            context = content[start:end]

            weaknesses.append(
                Weakness(
                    id=f"W4-{weakness_counter:03d}",
                    category="missing_ac",
                    severity="info",
                    location=f"line {line_num}",
                    description=f"Qualitative requirement ({desc}) - needs measurable criteria",
                    spec_quote=context.replace("\n", " ").strip(),
                    suggested_resolution="Add specific, measurable acceptance criteria",
                )
            )

    return weaknesses


# =============================================================================
# DETECTION: W5 - Fragmented Requirements
# =============================================================================


def detect_fragmented(content: str, lines: list[str]) -> list[Weakness]:
    """Detect requirements split across multiple sections."""
    weaknesses = []
    weakness_counter = 0

    # Pattern: Cross-references to other sections
    ref_patterns = [
        r"see\s+Section\s+(\d+\.?\d*)",
        r"as\s+described\s+in\s+Section\s+(\d+\.?\d*)",
        r"refer\s+to\s+Section\s+(\d+\.?\d*)",
        r"defined\s+in\s+Section\s+(\d+\.?\d*)",
    ]

    for pattern in ref_patterns:
        for match in re.finditer(pattern, content, re.IGNORECASE):
            weakness_counter += 1
            line_num = content[: match.start()].count("\n") + 1
            referenced_section = match.group(1)

            weaknesses.append(
                Weakness(
                    id=f"W5-{weakness_counter:03d}",
                    category="fragmented",
                    severity="info",
                    location=f"line {line_num}",
                    description=f"Cross-reference to Section {referenced_section} - requirement may be fragmented",
                    spec_quote=match.group(0),
                    suggested_resolution="Consider consolidating related requirements",
                )
            )

    return weaknesses


# =============================================================================
# DETECTION: W6 - Contradictions
# =============================================================================


def detect_contradictions(content: str, lines: list[str]) -> list[Weakness]:
    """Detect potentially contradictory statements."""
    weaknesses = []
    weakness_counter = 0

    # Pattern: "No X" followed by mention of X as feature
    # This is heuristic - flag for human review
    contradiction_hints = [
        (r"No\s+(\w+)", r"\1\s+(is|are|will|should|must)"),
        (r"never\s+(\w+)", r"(\1|always)\s+"),
        (r"not\s+supported", r"support(s|ed|ing)?"),
    ]

    # More sophisticated: Look for conflicting default values
    default_values: dict[str, list[tuple[str, int]]] = {}
    for match in re.finditer(
        r"(\w+).*(?:default|defaults?\s+to)\s*[:\s]*[`'\"]?(\w+)[`'\"]?",
        content,
        re.IGNORECASE,
    ):
        var_name = match.group(1).lower()
        value = match.group(2)
        line_num = content[: match.start()].count("\n") + 1

        if var_name not in default_values:
            default_values[var_name] = []
        default_values[var_name].append((value, line_num))

    for var_name, values in default_values.items():
        unique_values = set(v[0] for v in values)
        if len(unique_values) > 1:
            weakness_counter += 1
            locations = ", ".join(f"line {v[1]}" for v in values)
            value_list = ", ".join(unique_values)

            weaknesses.append(
                Weakness(
                    id=f"W6-{weakness_counter:03d}",
                    category="contradiction",
                    severity="critical",
                    location=locations,
                    description=f"Conflicting default values for '{var_name}': {value_list}",
                    spec_quote=f"Multiple defaults found: {value_list}",
                    suggested_resolution="Clarify which default value is authoritative",
                )
            )

    return weaknesses


# =============================================================================
# DETECTION: W7 - Ambiguity
# =============================================================================

# Patterns that indicate ambiguity, with clarifying question templates
AMBIGUITY_PATTERNS = [
    # Vague quantifiers
    (r"\b(some|many|few|several|various|numerous|multiple)\s+(\w+)", "vague_quantifier",
     "How many {1} specifically? Provide a number or range."),

    # Undefined scope
    (r"\b(etc\.?|and so on|and more|similar\s+\w+|like\s+\w+)\b", "undefined_scope",
     "What specifically is included? List all items explicitly."),

    # Conditional without criteria
    (r"\b(if applicable|when appropriate|as needed|when necessary|if required|where possible)\b", "vague_conditional",
     "Under what specific conditions does this apply? Define the criteria."),

    # Weasel words (weak requirements)
    (r"\b(may|might|could|possibly|optionally)\s+(be|have|include|support|allow)", "weak_requirement",
     "Is this required or optional? If optional, under what conditions?"),

    # Passive voice hiding actor
    (r"\b(is|are|will be|should be|must be)\s+(handled|processed|validated|checked|verified|managed|stored|created|updated|deleted)\b", "passive_actor",
     "What component/system performs this action?"),

    # Undefined timing
    (r"\b(quickly|soon|immediately|eventually|periodically|regularly)\b", "vague_timing",
     "What is the specific timing requirement? (e.g., <100ms, every 5 minutes)"),

    # Unspecified behavior
    (r"\b(properly|correctly|appropriately|adequately|sufficiently)\s+(handle|process|validate|manage)", "vague_behavior",
     "What does '{0}' mean specifically? Define the expected behavior."),

    # Either/or without resolution
    (r"\b(\w+)\s+or\s+(\w+)\s+(can|may|should|must|will)\b", "unresolved_or",
     "Which one: {0} or {1}? Or are both valid? Specify the rule."),

    # Reasonable/appropriate without definition
    (r"\b(reasonable|appropriate|suitable|adequate|sufficient)\s+(\w+)", "subjective_qualifier",
     "What makes a {1} '{0}'? Define the acceptance criteria."),

    # References to external knowledge
    (r"\b(standard|typical|normal|usual|common)\s+(practice|behavior|approach|way)", "external_reference",
     "Which standard specifically? Document the expected behavior."),

    # Unquantified limits
    (r"\b(large|small|long|short|high|low|fast|slow)\s+(number|amount|size|duration|latency|throughput)", "unquantified_limit",
     "What specific value constitutes '{0} {1}'? Provide a threshold."),
]


def detect_ambiguity(content: str, lines: list[str]) -> list[Weakness]:
    """Detect ambiguous language that requires clarification."""
    weaknesses = []
    weakness_counter = 0

    # Track found ambiguities to avoid duplicates
    found_contexts: set[str] = set()

    for pattern, ambiguity_type, question_template in AMBIGUITY_PATTERNS:
        for match in re.finditer(pattern, content, re.IGNORECASE):
            # Get context around the match
            start = max(0, match.start() - 100)
            end = min(len(content), match.end() + 100)
            context = content[start:end]

            # Skip if we've already flagged this context
            context_key = context[:50]
            if context_key in found_contexts:
                continue
            found_contexts.add(context_key)

            weakness_counter += 1
            line_num = content[: match.start()].count("\n") + 1
            matched_text = match.group(0)

            # Generate clarifying question
            try:
                if match.lastindex:
                    groups = match.groups()
                    clarifying_question = question_template.format(*groups)
                else:
                    clarifying_question = question_template.format(matched_text)
            except (IndexError, KeyError):
                clarifying_question = question_template

            # Determine severity based on ambiguity type
            if ambiguity_type in ("vague_quantifier", "unquantified_limit", "vague_timing"):
                severity = "warning"
            elif ambiguity_type in ("weak_requirement", "unresolved_or"):
                severity = "critical"
            else:
                severity = "warning"

            weaknesses.append(
                Weakness(
                    id=f"W7-{weakness_counter:03d}",
                    category="ambiguity",
                    severity=severity,
                    location=f"line {line_num}",
                    description=f"Ambiguous language ({ambiguity_type}): '{matched_text}'",
                    spec_quote=context.replace("\n", " ").strip()[:150],
                    suggested_resolution=clarifying_question,
                )
            )

            # Limit to avoid flooding with ambiguity warnings
            if weakness_counter >= 20:
                break

        if weakness_counter >= 20:
            break

    return weaknesses


# =============================================================================
# CHECKLIST VERIFICATION
# =============================================================================


@dataclass
class ChecklistItem:
    """Single checklist verification item."""

    id: str
    category: str
    question: str
    status: Literal["complete", "partial", "missing", "na"] = "missing"
    evidence: str = ""
    severity_if_missing: Literal["critical", "warning", "info"] = "warning"


CHECKLIST_DEFINITIONS = [
    # C1: Structural Completeness
    ("C1.1", "structure", "Problem statement or purpose defined?", "warning"),
    ("C1.2", "structure", "Functional requirements explicitly listed?", "warning"),
    ("C1.3", "structure", "Non-functional requirements stated?", "info"),
    ("C1.4", "structure", "Scope clearly bounded?", "warning"),
    # C2: Data Model Completeness
    ("C2.1", "data_model", "Entities/tables defined with purpose?", "critical"),
    ("C2.2", "data_model", "Fields defined with types?", "critical"),
    ("C2.3", "data_model", "Required vs optional fields distinguished?", "warning"),
    ("C2.4", "data_model", "Constraints stated (UNIQUE, CHECK, FK)?", "critical"),
    ("C2.5", "data_model", "Indexes specified for query patterns?", "warning"),
    ("C2.6", "data_model", "Default values documented?", "info"),
    # C3: API Completeness
    ("C3.1", "api", "Endpoints listed with HTTP methods?", "critical"),
    ("C3.2", "api", "Request schemas defined?", "critical"),
    ("C3.3", "api", "Response schemas defined?", "critical"),
    ("C3.4", "api", "Error responses defined?", "warning"),
    ("C3.5", "api", "Authentication requirements per endpoint?", "critical"),
    # C4: Behavior Completeness
    ("C4.1", "behavior", "Features described as observable behavior?", "critical"),
    ("C4.2", "behavior", "State transitions defined?", "warning"),
    ("C4.3", "behavior", "Business rules stated?", "critical"),
    ("C4.4", "behavior", "Edge cases addressed?", "warning"),
    # C5: Error Handling
    ("C5.1", "errors", "Error conditions enumerated?", "warning"),
    ("C5.2", "errors", "Error messages/codes defined?", "warning"),
    ("C5.3", "errors", "Retry behaviors specified?", "info"),
    # C6: Configuration
    ("C6.1", "config", "Environment variables listed?", "warning"),
    ("C6.2", "config", "Types for config values specified?", "warning"),
    ("C6.3", "config", "Defaults documented?", "info"),
    # C7: Security
    ("C7.1", "security", "Authentication mechanism specified?", "critical"),
    ("C7.2", "security", "Authorization rules defined?", "critical"),
    ("C7.3", "security", "Sensitive data handling stated?", "warning"),
    # C8: Observability
    ("C8.1", "observability", "Logging requirements defined?", "info"),
    ("C8.2", "observability", "Metrics specified?", "info"),
    ("C8.3", "observability", "Health check endpoints specified?", "info"),
    # C9: Performance
    ("C9.1", "performance", "Response time SLAs defined?", "info"),
    ("C9.2", "performance", "Timeout values defined?", "info"),
    # C10: Integration
    ("C10.1", "integration", "External dependencies listed?", "warning"),
    ("C10.2", "integration", "External API contracts documented?", "warning"),
    # C11: Lifecycle
    ("C11.1", "lifecycle", "Startup sequence defined?", "info"),
    ("C11.2", "lifecycle", "Graceful shutdown specified?", "info"),
]


def verify_checklist(content: str) -> list[ChecklistItem]:
    """Verify spec against completeness checklist."""
    items: list[ChecklistItem] = []
    content_lower = content.lower()

    for item_id, category, question, severity in CHECKLIST_DEFINITIONS:
        item = ChecklistItem(
            id=item_id,
            category=category,
            question=question,
            severity_if_missing=severity,
        )

        # Category-specific detection logic
        if category == "structure":
            item = _check_structure(item, content, content_lower)
        elif category == "data_model":
            item = _check_data_model(item, content, content_lower)
        elif category == "api":
            item = _check_api(item, content, content_lower)
        elif category == "behavior":
            item = _check_behavior(item, content, content_lower)
        elif category == "errors":
            item = _check_errors(item, content, content_lower)
        elif category == "config":
            item = _check_config(item, content, content_lower)
        elif category == "security":
            item = _check_security(item, content, content_lower)
        elif category == "observability":
            item = _check_observability(item, content, content_lower)
        elif category == "performance":
            item = _check_performance(item, content, content_lower)
        elif category == "integration":
            item = _check_integration(item, content, content_lower)
        elif category == "lifecycle":
            item = _check_lifecycle(item, content, content_lower)

        items.append(item)

    return items


def _check_structure(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check structural completeness items."""
    if item.id == "C1.1":
        patterns = ["purpose", "problem", "objective", "goal", "overview", "introduction"]
        if any(p in content_lower for p in patterns):
            item.status = "complete"
            item.evidence = "Found purpose/overview section"
    elif item.id == "C1.2":
        patterns = ["must", "shall", "requirement", "feature"]
        count = sum(content_lower.count(p) for p in patterns)
        if count > 5:
            item.status = "complete"
            item.evidence = f"Found {count} requirement indicators"
        elif count > 0:
            item.status = "partial"
            item.evidence = f"Found {count} requirement indicators (may need more explicit listing)"
    elif item.id == "C1.3":
        patterns = ["performance", "security", "scalab", "reliab", "availability"]
        if any(p in content_lower for p in patterns):
            item.status = "complete"
            item.evidence = "Found non-functional requirements"
    elif item.id == "C1.4":
        patterns = ["scope", "out of scope", "not included", "boundaries", "limitations"]
        if any(p in content_lower for p in patterns):
            item.status = "complete"
            item.evidence = "Found scope definition"
    return item


def _check_data_model(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check data model completeness items."""
    has_tables = bool(re.search(r"CREATE\s+TABLE", content, re.IGNORECASE))
    has_entity_desc = bool(re.search(r"(entity|table|model)\s*:", content_lower))

    if item.id == "C2.1":
        if has_tables or has_entity_desc:
            item.status = "complete"
            item.evidence = "Found table/entity definitions"
    elif item.id == "C2.2":
        if has_tables:
            item.status = "complete"
            item.evidence = "Found typed field definitions in DDL"
        elif re.search(r"\|\s*\w+\s*\|\s*(str|int|bool|float|uuid|timestamp)", content_lower):
            item.status = "complete"
            item.evidence = "Found typed fields in table"
    elif item.id == "C2.3":
        if "NOT NULL" in content.upper() or "optional" in content_lower or "required" in content_lower:
            item.status = "complete"
            item.evidence = "Found required/optional field indicators"
    elif item.id == "C2.4":
        constraints = ["UNIQUE", "CHECK", "FOREIGN KEY", "CONSTRAINT", "PRIMARY KEY"]
        found = [c for c in constraints if c in content.upper()]
        if found:
            item.status = "complete"
            item.evidence = f"Found constraints: {', '.join(found)}"
    elif item.id == "C2.5":
        if "INDEX" in content.upper() or "index" in content_lower:
            item.status = "complete"
            item.evidence = "Found index definitions"
    elif item.id == "C2.6":
        if "DEFAULT" in content.upper() or "default" in content_lower:
            item.status = "complete"
            item.evidence = "Found default value specifications"
    return item


def _check_api(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check API completeness items."""
    has_endpoints = bool(re.search(r"(GET|POST|PUT|PATCH|DELETE)\s+/", content))

    if item.id == "C3.1":
        if has_endpoints:
            item.status = "complete"
            item.evidence = "Found endpoint definitions with HTTP methods"
    elif item.id == "C3.2":
        if "request" in content_lower and ("body" in content_lower or "param" in content_lower or "schema" in content_lower):
            item.status = "complete"
            item.evidence = "Found request schema references"
        elif has_endpoints:
            item.status = "partial"
            item.evidence = "Endpoints found but request schemas may be incomplete"
    elif item.id == "C3.3":
        if "response" in content_lower and ("body" in content_lower or "return" in content_lower or "schema" in content_lower):
            item.status = "complete"
            item.evidence = "Found response schema references"
        elif has_endpoints:
            item.status = "partial"
            item.evidence = "Endpoints found but response schemas may be incomplete"
    elif item.id == "C3.4":
        error_patterns = ["error response", "4xx", "5xx", "400", "401", "404", "500", "error code"]
        if any(p in content_lower for p in error_patterns):
            item.status = "complete"
            item.evidence = "Found error response definitions"
    elif item.id == "C3.5":
        auth_patterns = ["authentication", "authorization", "auth", "bearer", "api key", "jwt", "token"]
        if any(p in content_lower for p in auth_patterns):
            item.status = "complete"
            item.evidence = "Found authentication references"
    return item


def _check_behavior(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check behavior completeness items."""
    if item.id == "C4.1":
        behavior_words = ["when", "then", "must", "shall", "should", "will"]
        count = sum(content_lower.count(w) for w in behavior_words)
        if count > 10:
            item.status = "complete"
            item.evidence = f"Found {count} behavioral indicators"
        elif count > 3:
            item.status = "partial"
            item.evidence = f"Found {count} behavioral indicators (may need more explicit behaviors)"
    elif item.id == "C4.2":
        state_patterns = ["state", "status", "transition", "workflow", "lifecycle"]
        if any(p in content_lower for p in state_patterns):
            item.status = "complete"
            item.evidence = "Found state/transition references"
    elif item.id == "C4.3":
        rule_patterns = ["rule", "validation", "must be", "cannot", "allowed", "prohibited"]
        if any(p in content_lower for p in rule_patterns):
            item.status = "complete"
            item.evidence = "Found business rule indicators"
    elif item.id == "C4.4":
        edge_patterns = ["edge case", "empty", "null", "zero", "maximum", "minimum", "boundary"]
        if any(p in content_lower for p in edge_patterns):
            item.status = "complete"
            item.evidence = "Found edge case handling"
    return item


def _check_errors(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check error handling completeness items."""
    if item.id == "C5.1":
        if "error" in content_lower and any(p in content_lower for p in ["condition", "case", "when", "if"]):
            item.status = "complete"
            item.evidence = "Found error condition descriptions"
    elif item.id == "C5.2":
        if re.search(r"error.*code|error.*message|\d{3}\s", content_lower):
            item.status = "complete"
            item.evidence = "Found error codes/messages"
    elif item.id == "C5.3":
        if "retry" in content_lower:
            item.status = "complete"
            item.evidence = "Found retry behavior"
    return item


def _check_config(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check configuration completeness items."""
    has_env_vars = bool(re.search(r"[A-Z][A-Z0-9_]{3,}", content))

    if item.id == "C6.1":
        if "environment" in content_lower or has_env_vars:
            item.status = "complete"
            item.evidence = "Found environment variable references"
    elif item.id == "C6.2":
        if has_env_vars and any(t in content_lower for t in ["str", "int", "bool", "float", "string", "integer"]):
            item.status = "complete"
            item.evidence = "Found typed config values"
    elif item.id == "C6.3":
        if "default" in content_lower:
            item.status = "complete"
            item.evidence = "Found default value specifications"
    return item


def _check_security(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check security completeness items."""
    if item.id == "C7.1":
        auth_patterns = ["authentication", "authn", "login", "bearer", "jwt", "api key", "oauth"]
        if any(p in content_lower for p in auth_patterns):
            item.status = "complete"
            item.evidence = "Found authentication mechanism"
    elif item.id == "C7.2":
        authz_patterns = ["authorization", "authz", "permission", "role", "access control", "rbac"]
        if any(p in content_lower for p in authz_patterns):
            item.status = "complete"
            item.evidence = "Found authorization rules"
    elif item.id == "C7.3":
        data_patterns = ["sensitive", "encrypt", "hash", "pii", "secret", "credential"]
        if any(p in content_lower for p in data_patterns):
            item.status = "complete"
            item.evidence = "Found sensitive data handling"
    return item


def _check_observability(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check observability completeness items."""
    if item.id == "C8.1":
        if "log" in content_lower:
            item.status = "complete"
            item.evidence = "Found logging requirements"
    elif item.id == "C8.2":
        metric_patterns = ["metric", "counter", "gauge", "histogram", "prometheus", "otel"]
        if any(p in content_lower for p in metric_patterns):
            item.status = "complete"
            item.evidence = "Found metrics requirements"
    elif item.id == "C8.3":
        if "health" in content_lower or "/health" in content:
            item.status = "complete"
            item.evidence = "Found health check requirements"
    return item


def _check_performance(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check performance completeness items."""
    if item.id == "C9.1":
        perf_patterns = ["latency", "response time", "sla", "p50", "p95", "p99", "millisecond"]
        if any(p in content_lower for p in perf_patterns):
            item.status = "complete"
            item.evidence = "Found performance SLAs"
    elif item.id == "C9.2":
        if "timeout" in content_lower:
            item.status = "complete"
            item.evidence = "Found timeout specifications"
    return item


def _check_integration(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check integration completeness items."""
    if item.id == "C10.1":
        dep_patterns = ["external", "dependency", "third-party", "integration", "api call"]
        if any(p in content_lower for p in dep_patterns):
            item.status = "complete"
            item.evidence = "Found external dependency references"
        else:
            item.status = "na"
            item.evidence = "No external dependencies apparent"
    elif item.id == "C10.2":
        if "contract" in content_lower or "external api" in content_lower:
            item.status = "complete"
            item.evidence = "Found external API contracts"
        elif item.status != "na":
            item.status = "na"
            item.evidence = "No external API contracts needed"
    return item


def _check_lifecycle(item: ChecklistItem, content: str, content_lower: str) -> ChecklistItem:
    """Check lifecycle completeness items."""
    if item.id == "C11.1":
        startup_patterns = ["startup", "initialization", "boot", "lifespan"]
        if any(p in content_lower for p in startup_patterns):
            item.status = "complete"
            item.evidence = "Found startup sequence"
    elif item.id == "C11.2":
        shutdown_patterns = ["shutdown", "graceful", "cleanup", "termination"]
        if any(p in content_lower for p in shutdown_patterns):
            item.status = "complete"
            item.evidence = "Found shutdown behavior"
    return item


# =============================================================================
# MAIN ANALYSIS
# =============================================================================


def analyze_spec(spec_path: Path) -> SpecReview:
    """Run all weakness detectors and checklist verification on a spec file."""
    content = spec_path.read_text()
    lines = content.split("\n")
    checksum = hashlib.sha256(content.encode()).hexdigest()[:16]

    review = SpecReview(
        spec_checksum=checksum,
        analyzed_at=datetime.now(timezone.utc).isoformat(),
    )

    # Run all weakness detectors
    detectors = [
        ("non_behavioral", detect_non_behavioral),
        ("implicit", detect_implicit),
        ("cross_cutting", detect_cross_cutting),
        ("missing_ac", detect_missing_ac),
        ("fragmented", detect_fragmented),
        ("contradiction", detect_contradictions),
        ("ambiguity", detect_ambiguity),
    ]

    for category, detector in detectors:
        weaknesses = detector(content, lines)
        review.weaknesses.extend(weaknesses)

    # Run checklist verification
    checklist_items = verify_checklist(content)

    # Convert missing critical checklist items to weaknesses
    for item in checklist_items:
        if item.status == "missing" and item.severity_if_missing == "critical":
            review.weaknesses.append(
                Weakness(
                    id=f"CK-{item.id}",
                    category="checklist_gap",
                    severity="critical",
                    location="spec-wide",
                    description=f"Checklist gap: {item.question}",
                    spec_quote="",
                    suggested_resolution=f"Address checklist item {item.id}: {item.question}",
                )
            )

    # Generate summary
    by_severity = {"critical": 0, "warning": 0, "info": 0}
    by_category: dict[str, int] = {}

    for w in review.weaknesses:
        by_severity[w.severity] += 1
        by_category[w.category] = by_category.get(w.category, 0) + 1

    # Checklist summary
    checklist_summary = {
        "total": len(checklist_items),
        "complete": sum(1 for i in checklist_items if i.status == "complete"),
        "partial": sum(1 for i in checklist_items if i.status == "partial"),
        "missing": sum(1 for i in checklist_items if i.status == "missing"),
        "na": sum(1 for i in checklist_items if i.status == "na"),
        "critical_missing": sum(
            1 for i in checklist_items
            if i.status == "missing" and i.severity_if_missing == "critical"
        ),
    }

    review.summary = {
        "total": len(review.weaknesses),
        "by_severity": by_severity,
        "by_category": by_category,
        "blocking": by_severity["critical"] > 0,
        "checklist": checklist_summary,
        "checklist_items": [asdict(i) for i in checklist_items],
    }

    return review


def save_review(review: SpecReview, planning_dir: Path) -> Path:
    """Save review to artifacts directory. All state persisted to files."""
    artifacts_dir = planning_dir / "artifacts"
    artifacts_dir.mkdir(parents=True, exist_ok=True)

    output_path = artifacts_dir / "spec-review.json"

    # Convert to dict - includes checklist in summary
    data = {
        "version": review.version,
        "spec_checksum": review.spec_checksum,
        "analyzed_at": review.analyzed_at,
        "status": review.status,
        "summary": review.summary,  # Now includes checklist and checklist_items
        "weaknesses": [asdict(w) for w in review.weaknesses],
    }

    output_path.write_text(json.dumps(data, indent=2))
    return output_path


def load_review(planning_dir: Path) -> SpecReview | None:
    """Load review from artifacts directory. Restores state from file."""
    review_path = planning_dir / "artifacts" / "spec-review.json"
    if not review_path.exists():
        return None

    data = json.loads(review_path.read_text())
    return SpecReview(
        version=data["version"],
        spec_checksum=data["spec_checksum"],
        analyzed_at=data["analyzed_at"],
        status=data["status"],
        summary=data["summary"],
        weaknesses=[Weakness(**w) for w in data["weaknesses"]],
    )


def save_resolutions(resolutions: list[dict], planning_dir: Path) -> Path:
    """Save resolutions to artifacts directory."""
    artifacts_dir = planning_dir / "artifacts"
    artifacts_dir.mkdir(parents=True, exist_ok=True)

    output_path = artifacts_dir / "spec-resolutions.json"

    data = {
        "version": "1.0",
        "resolutions": resolutions,
    }

    output_path.write_text(json.dumps(data, indent=2))
    return output_path


def load_resolutions(planning_dir: Path) -> list[dict]:
    """Load resolutions from artifacts directory."""
    resolutions_path = planning_dir / "artifacts" / "spec-resolutions.json"
    if not resolutions_path.exists():
        return []

    data = json.loads(resolutions_path.read_text())
    return data.get("resolutions", [])


def print_report(review: SpecReview) -> None:
    """Print human-readable weakness report."""
    print("=" * 60)
    print("SPEC REVIEW REPORT")
    print("=" * 60)
    print()
    print(f"Checksum: {review.spec_checksum}")
    print(f"Analyzed: {review.analyzed_at}")
    print()

    # Checklist summary
    checklist = review.summary.get("checklist", {})
    if checklist:
        print("CHECKLIST VERIFICATION")
        print("-" * 40)
        print(f"  Complete: {checklist.get('complete', 0)}")
        print(f"  Partial:  {checklist.get('partial', 0)}")
        print(f"  Missing:  {checklist.get('missing', 0)} ({checklist.get('critical_missing', 0)} critical)")
        print(f"  N/A:      {checklist.get('na', 0)}")
        print()

    # Weakness summary
    print("WEAKNESS DETECTION")
    print("-" * 40)
    print(f"Total Weaknesses: {review.summary['total']}")
    print(f"  Critical: {review.summary['by_severity']['critical']}")
    print(f"  Warning:  {review.summary['by_severity']['warning']}")
    print(f"  Info:     {review.summary['by_severity']['info']}")
    print()

    if review.summary["blocking"]:
        print("STATUS: BLOCKED - Critical weaknesses require resolution")
    else:
        print("STATUS: READY - No blocking weaknesses")
    print()

    # Group by category
    by_cat: dict[str, list[Weakness]] = {}
    for w in review.weaknesses:
        if w.category not in by_cat:
            by_cat[w.category] = []
        by_cat[w.category].append(w)

    category_names = {
        "non_behavioral": "W1: Non-Behavioral Requirements",
        "implicit": "W2: Implicit Requirements",
        "cross_cutting": "W3: Cross-Cutting Concerns",
        "missing_ac": "W4: Missing Acceptance Criteria",
        "fragmented": "W5: Fragmented Requirements",
        "contradiction": "W6: Contradictions",
        "ambiguity": "W7: Ambiguity (Clarification Needed)",
        "checklist_gap": "CK: Checklist Gaps (Critical Missing Items)",
    }

    for cat, name in category_names.items():
        if cat not in by_cat:
            continue

        print("-" * 60)
        print(name)
        print("-" * 60)

        for w in by_cat[cat]:
            severity_icon = {"critical": "!", "warning": "?", "info": "."}[w.severity]
            print(f"\n[{severity_icon}] {w.id} ({w.location})")
            print(f"    {w.description}")
            if w.spec_quote:
                quote = w.spec_quote[:80] + "..." if len(w.spec_quote) > 80 else w.spec_quote
                print(f"    Quote: {quote}")
            if w.suggested_resolution:
                print(f"    Resolution: {w.suggested_resolution}")
            if w.behavioral_reframe:
                print(f"    Reframe: {w.behavioral_reframe}")

    print()


def main() -> None:
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "analyze":
        if len(sys.argv) < 3:
            print("Usage: spec-review.py analyze <spec_path>")
            sys.exit(1)

        spec_path = Path(sys.argv[2])
        if not spec_path.exists():
            print(f"Spec file not found: {spec_path}", file=sys.stderr)
            sys.exit(1)

        review = analyze_spec(spec_path)

        # Print JSON to stdout for programmatic use
        data = {
            "version": review.version,
            "spec_checksum": review.spec_checksum,
            "analyzed_at": review.analyzed_at,
            "status": review.status,
            "summary": review.summary,
            "weaknesses": [asdict(w) for w in review.weaknesses],
        }
        print(json.dumps(data, indent=2))

        # Exit with non-zero if blocking
        sys.exit(1 if review.summary.get("blocking") else 0)

    elif cmd == "report":
        if len(sys.argv) < 3:
            print("Usage: spec-review.py report <planning_dir>")
            sys.exit(1)

        planning_dir = Path(sys.argv[2])
        review_path = planning_dir / "artifacts" / "spec-review.json"

        if not review_path.exists():
            # Run analysis first
            spec_path = planning_dir / "inputs" / "spec.md"
            if not spec_path.exists():
                print(f"No spec found at {spec_path}", file=sys.stderr)
                sys.exit(1)

            review = analyze_spec(spec_path)
            save_review(review, planning_dir)
        else:
            data = json.loads(review_path.read_text())
            review = SpecReview(
                version=data["version"],
                spec_checksum=data["spec_checksum"],
                analyzed_at=data["analyzed_at"],
                status=data["status"],
                summary=data["summary"],
                weaknesses=[Weakness(**w) for w in data["weaknesses"]],
            )

        print_report(review)

    elif cmd == "status":
        if len(sys.argv) < 3:
            print("Usage: spec-review.py status <planning_dir>")
            sys.exit(1)

        planning_dir = Path(sys.argv[2])
        review_path = planning_dir / "artifacts" / "spec-review.json"
        resolutions_path = planning_dir / "artifacts" / "spec-resolutions.json"

        if not review_path.exists():
            print("No spec review found. Run 'analyze' first.")
            sys.exit(1)

        review_data = json.loads(review_path.read_text())
        total = review_data["summary"]["total"]
        critical = review_data["summary"]["by_severity"]["critical"]

        if resolutions_path.exists():
            resolutions = json.loads(resolutions_path.read_text())
            resolved_count = len(resolutions.get("resolutions", []))
        else:
            resolved_count = 0

        print(f"Weaknesses: {total} ({critical} critical)")
        print(f"Resolved: {resolved_count}")
        print(f"Remaining: {total - resolved_count}")

        # Check resolutions against critical weaknesses
        critical_weaknesses = [
            w for w in review_data.get("weaknesses", [])
            if w.get("severity") == "critical"
        ]
        resolved_ids = set()
        if resolutions_path.exists():
            resolutions_data = json.loads(resolutions_path.read_text())
            resolved_ids = {r["weakness_id"] for r in resolutions_data.get("resolutions", [])}

        unresolved_critical = [w for w in critical_weaknesses if w["id"] not in resolved_ids]

        if unresolved_critical:
            print(f"Status: BLOCKED ({len(unresolved_critical)} unresolved critical)")
            for w in unresolved_critical:
                print(f"  - {w['id']}: {w['description'][:60]}")
            sys.exit(1)
        else:
            print("Status: READY")
            sys.exit(0)

    elif cmd == "checklist":
        if len(sys.argv) < 3:
            print("Usage: spec-review.py checklist <planning_dir>")
            sys.exit(1)

        planning_dir = Path(sys.argv[2])
        review_path = planning_dir / "artifacts" / "spec-review.json"

        if not review_path.exists():
            print("No spec review found. Run 'analyze' first.")
            sys.exit(1)

        review_data = json.loads(review_path.read_text())
        checklist_items = review_data.get("summary", {}).get("checklist_items", [])

        if not checklist_items:
            print("No checklist data found. Re-run analyze.")
            sys.exit(1)

        print("=" * 60)
        print("SPEC COMPLETENESS CHECKLIST")
        print("=" * 60)
        print()

        current_category = None
        status_icons = {"complete": "✓", "partial": "◐", "missing": "✗", "na": "-"}

        for item in checklist_items:
            cat = item["category"]
            if cat != current_category:
                current_category = cat
                print(f"\n{cat.upper().replace('_', ' ')}")
                print("-" * 40)

            icon = status_icons.get(item["status"], "?")
            severity_marker = "*" if item["severity_if_missing"] == "critical" else ""
            print(f"  [{icon}] {item['id']}{severity_marker}: {item['question']}")
            if item.get("evidence"):
                print(f"      → {item['evidence']}")

        print()
        print("Legend: ✓=complete  ◐=partial  ✗=missing  -=N/A  *=critical if missing")

    elif cmd == "add-resolution":
        if len(sys.argv) < 5:
            print("Usage: spec-review.py add-resolution <planning_dir> <weakness_id> <resolution> [--notes 'text']")
            print("Resolutions: mandatory, optional, defer, clarified, not_applicable")
            sys.exit(1)

        planning_dir = Path(sys.argv[2])
        weakness_id = sys.argv[3]
        resolution = sys.argv[4]

        valid_resolutions = ["mandatory", "optional", "defer", "clarified", "not_applicable"]
        if resolution not in valid_resolutions:
            print(f"Invalid resolution: {resolution}")
            print(f"Valid: {', '.join(valid_resolutions)}")
            sys.exit(1)

        # Get notes if provided
        notes = ""
        if "--notes" in sys.argv:
            idx = sys.argv.index("--notes")
            if idx + 1 < len(sys.argv):
                notes = sys.argv[idx + 1]

        # Load existing resolutions
        existing = load_resolutions(planning_dir)

        # Check if already resolved
        if any(r["weakness_id"] == weakness_id for r in existing):
            print(f"Weakness {weakness_id} already resolved. Remove first to update.")
            sys.exit(1)

        # Add new resolution
        new_resolution = {
            "weakness_id": weakness_id,
            "resolution": resolution,
            "user_response": resolution,
            "notes": notes,
            "resolved_at": datetime.now(timezone.utc).isoformat(),
        }
        existing.append(new_resolution)

        # Save
        save_resolutions(existing, planning_dir)
        print(f"Added resolution: {weakness_id} → {resolution}")

    elif cmd == "unresolved":
        if len(sys.argv) < 3:
            print("Usage: spec-review.py unresolved <planning_dir>")
            sys.exit(1)

        planning_dir = Path(sys.argv[2])
        review_path = planning_dir / "artifacts" / "spec-review.json"

        if not review_path.exists():
            print("No spec review found. Run 'analyze' first.")
            sys.exit(1)

        review_data = json.loads(review_path.read_text())
        weaknesses = review_data.get("weaknesses", [])
        resolved_ids = {r["weakness_id"] for r in load_resolutions(planning_dir)}

        # Show unresolved critical weaknesses
        unresolved = [w for w in weaknesses if w["id"] not in resolved_ids and w["severity"] == "critical"]

        if not unresolved:
            print("No unresolved critical weaknesses!")
            sys.exit(0)

        print(f"Unresolved Critical Weaknesses: {len(unresolved)}")
        print()
        for w in unresolved:
            print(f"[{w['id']}] {w['description']}")
            if w.get("suggested_resolution"):
                print(f"    Suggested: {w['suggested_resolution']}")
            print()

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
