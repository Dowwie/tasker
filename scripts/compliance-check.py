#!/usr/bin/env python3
"""
Post-Execution Compliance Check - Compare spec to implementation.

Verifies that all spec requirements were implemented after task execution
completes. Catches requirements that weren't captured as acceptance criteria.

Usage:
    compliance-check.py schema --spec <path> [--migrations <dir>]
    compliance-check.py config --spec <path> --settings <path>
    compliance-check.py api --spec <path> --routes <path>
    compliance-check.py observability --spec <path> --code <path>
    compliance-check.py all --spec <path> --target <path>

Verification Categories:
    V1: Schema Compliance    - DDL elements exist (tables, constraints, indexes)
    V2: Config Compliance    - Env vars wired to Pydantic fields
    V3: API Compliance       - Endpoints exist with correct methods
    V4: Observability        - OTel spans and metrics registered
"""

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
class ComplianceGap:
    """Detected compliance gap between spec and implementation."""

    id: str
    category: Literal["schema", "config", "api", "observability"]
    severity: Literal["critical", "warning", "info"]
    spec_requirement: str
    spec_location: str
    implementation_status: Literal["missing", "partial", "different"]
    details: str = ""
    suggested_fix: str = ""


@dataclass
class ComplianceReport:
    """Complete compliance check result."""

    version: str = "1.0"
    spec_path: str = ""
    target_path: str = ""
    checked_at: str = ""
    gaps: list[ComplianceGap] = field(default_factory=list)
    summary: dict = field(default_factory=dict)


# =============================================================================
# SPEC EXTRACTION: Schema Elements
# =============================================================================


@dataclass
class TableDef:
    name: str
    columns: list[tuple[str, str, bool]]  # (name, type, not_null)
    location: str


@dataclass
class ConstraintDef:
    name: str
    table: str
    constraint_type: str  # UNIQUE, CHECK, FK, PK
    columns: list[str]
    expression: str
    location: str


@dataclass
class IndexDef:
    name: str
    table: str
    columns: list[str]
    index_type: str  # btree, gin, hnsw
    location: str


def extract_ddl_elements(spec_content: str) -> tuple[list[TableDef], list[ConstraintDef], list[IndexDef]]:
    """Extract DDL elements from spec markdown."""
    tables: list[TableDef] = []
    constraints: list[ConstraintDef] = []
    indexes: list[IndexDef] = []

    # Find CREATE TABLE statements
    table_pattern = r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(([^;]+)\)"
    for match in re.finditer(table_pattern, spec_content, re.IGNORECASE | re.DOTALL):
        table_name = match.group(1)
        body = match.group(2)
        line_num = spec_content[: match.start()].count("\n") + 1

        columns = []
        for col_match in re.finditer(
            r"^\s*(\w+)\s+(\w+(?:\([^)]+\))?)\s*(NOT\s+NULL)?",
            body,
            re.MULTILINE | re.IGNORECASE,
        ):
            col_name = col_match.group(1)
            col_type = col_match.group(2)
            not_null = bool(col_match.group(3))
            if col_name.upper() not in ("CONSTRAINT", "PRIMARY", "FOREIGN", "UNIQUE", "CHECK"):
                columns.append((col_name, col_type, not_null))

        tables.append(TableDef(name=table_name, columns=columns, location=f"line {line_num}"))

        # Extract inline constraints from table body
        constraint_patterns = [
            (r"CONSTRAINT\s+(\w+)\s+UNIQUE\s*\(([^)]+)\)", "UNIQUE"),
            (r"CONSTRAINT\s+(\w+)\s+CHECK\s*\(([^)]+)\)", "CHECK"),
            (r"CONSTRAINT\s+(\w+)\s+PRIMARY\s+KEY\s*\(([^)]+)\)", "PK"),
            (r"CONSTRAINT\s+(\w+)\s+FOREIGN\s+KEY\s*\(([^)]+)\)", "FK"),
        ]

        for pattern, ctype in constraint_patterns:
            for cmatch in re.finditer(pattern, body, re.IGNORECASE):
                constraints.append(
                    ConstraintDef(
                        name=cmatch.group(1),
                        table=table_name,
                        constraint_type=ctype,
                        columns=_parse_columns(cmatch.group(2)),
                        expression=cmatch.group(0),
                        location=f"line {line_num}",
                    )
                )

    # Find CREATE INDEX statements
    index_pattern = r"CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(\w+)(?:\s+USING\s+(\w+))?\s*\(([^)]+)\)"
    for match in re.finditer(index_pattern, spec_content, re.IGNORECASE):
        line_num = spec_content[: match.start()].count("\n") + 1
        indexes.append(
            IndexDef(
                name=match.group(2),
                table=match.group(3),
                columns=_parse_columns(match.group(5)),
                index_type=match.group(4) or "btree",
                location=f"line {line_num}",
            )
        )

    return tables, constraints, indexes


def _parse_columns(col_str: str) -> list[str]:
    """Parse comma-separated column list."""
    return [c.strip().split()[0] for c in col_str.split(",")]


# =============================================================================
# SPEC EXTRACTION: Configuration
# =============================================================================


@dataclass
class ConfigVar:
    name: str
    var_type: str
    default: str | None
    required: bool
    description: str
    location: str


def extract_config_requirements(spec_content: str) -> list[ConfigVar]:
    """Extract environment variable requirements from spec."""
    configs: list[ConfigVar] = []

    # Pattern for markdown table rows like: | VAR_NAME | type | default | description |
    # Also handles: | VAR_NAME | type | description |
    table_pattern = r"\|\s*`?([A-Z][A-Z0-9_]+)`?\s*\|\s*(\w+)\s*\|([^|]*)\|"

    for match in re.finditer(table_pattern, spec_content):
        var_name = match.group(1)
        var_type = match.group(2).lower()
        rest = match.group(3).strip()
        line_num = spec_content[: match.start()].count("\n") + 1

        # Check if it looks like a default value or description
        default = None
        required = True
        if rest and not rest[0].isupper():
            default = rest
            required = default.lower() in ("required", "none", "")

        configs.append(
            ConfigVar(
                name=var_name,
                var_type=var_type,
                default=default,
                required=required,
                description="",
                location=f"line {line_num}",
            )
        )

    return configs


# =============================================================================
# SPEC EXTRACTION: API Endpoints
# =============================================================================


@dataclass
class EndpointDef:
    method: str
    path: str
    description: str
    location: str


def extract_api_requirements(spec_content: str) -> list[EndpointDef]:
    """Extract API endpoint requirements from spec."""
    endpoints: list[EndpointDef] = []

    # Pattern: GET /path or POST /api/path etc
    endpoint_pattern = r"(GET|POST|PUT|PATCH|DELETE)\s+(/[^\s\n]+)"

    for match in re.finditer(endpoint_pattern, spec_content):
        method = match.group(1)
        path = match.group(2)
        line_num = spec_content[: match.start()].count("\n") + 1

        # Get description from context
        context_end = min(len(spec_content), match.end() + 100)
        context = spec_content[match.end() : context_end].split("\n")[0]

        endpoints.append(
            EndpointDef(
                method=method,
                path=path,
                description=context.strip(" -:"),
                location=f"line {line_num}",
            )
        )

    return endpoints


# =============================================================================
# SPEC EXTRACTION: Observability
# =============================================================================


@dataclass
class MetricDef:
    name: str
    metric_type: str  # counter, gauge, histogram
    description: str
    location: str


@dataclass
class SpanDef:
    name: str
    operation: str
    location: str


def extract_observability_requirements(spec_content: str) -> tuple[list[MetricDef], list[SpanDef]]:
    """Extract metrics and span requirements from spec."""
    metrics: list[MetricDef] = []
    spans: list[SpanDef] = []

    # Metric patterns
    metric_patterns = [
        (r"\|\s*`?(\w+_\w+)`?\s*\|\s*(counter|gauge|histogram)\s*\|", "table"),
        (r"(counter|gauge|histogram)\s*:\s*`?(\w+)`?", "inline"),
    ]

    for pattern, style in metric_patterns:
        for match in re.finditer(pattern, spec_content, re.IGNORECASE):
            line_num = spec_content[: match.start()].count("\n") + 1
            if style == "table":
                name, mtype = match.group(1), match.group(2)
            else:
                mtype, name = match.group(1), match.group(2)

            metrics.append(
                MetricDef(
                    name=name,
                    metric_type=mtype.lower(),
                    description="",
                    location=f"line {line_num}",
                )
            )

    # Span patterns
    span_pattern = r"span[:\s]+[`'\"]?(\w+)[`'\"]?"
    for match in re.finditer(span_pattern, spec_content, re.IGNORECASE):
        line_num = spec_content[: match.start()].count("\n") + 1
        spans.append(
            SpanDef(
                name=match.group(1),
                operation="",
                location=f"line {line_num}",
            )
        )

    return metrics, spans


# =============================================================================
# IMPLEMENTATION SCANNING
# =============================================================================


def scan_migrations_for_schema(migrations_dir: Path) -> tuple[set[str], set[str], set[str]]:
    """Scan migration files for created tables, constraints, indexes."""
    tables: set[str] = set()
    constraints: set[str] = set()
    indexes: set[str] = set()

    if not migrations_dir.exists():
        return tables, constraints, indexes

    for migration_file in migrations_dir.glob("*.py"):
        content = migration_file.read_text()

        # Alembic/SQLAlchemy patterns
        for match in re.finditer(r"op\.create_table\(['\"](\w+)['\"]", content):
            tables.add(match.group(1))

        for match in re.finditer(r"op\.create_unique_constraint\(['\"](\w+)['\"]", content):
            constraints.add(match.group(1))

        for match in re.finditer(r"op\.create_check_constraint\(['\"](\w+)['\"]", content):
            constraints.add(match.group(1))

        for match in re.finditer(r"op\.create_index\(['\"](\w+)['\"]", content):
            indexes.add(match.group(1))

        # Raw SQL patterns
        for match in re.finditer(r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)", content, re.IGNORECASE):
            tables.add(match.group(1))

        for match in re.finditer(r"CONSTRAINT\s+(\w+)", content, re.IGNORECASE):
            constraints.add(match.group(1))

        for match in re.finditer(r"CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)", content, re.IGNORECASE):
            indexes.add(match.group(1))

    return tables, constraints, indexes


def scan_settings_for_config(settings_path: Path) -> dict[str, dict]:
    """Scan Pydantic settings file for environment variables."""
    if not settings_path.exists():
        return {}

    content = settings_path.read_text()
    config_vars: dict[str, dict] = {}

    # Pattern for Pydantic field definitions
    field_pattern = r"(\w+)\s*:\s*(\w+(?:\[[\w,\s]+\])?)\s*(?:=\s*(?:Field\(([^)]+)\)|([^#\n]+)))?"

    for match in re.finditer(field_pattern, content):
        name = match.group(1)
        field_type = match.group(2)
        field_args = match.group(3) or ""
        default_val = match.group(4)

        # Convert to env var name (SCREAMING_SNAKE)
        env_name = re.sub(r"([a-z])([A-Z])", r"\1_\2", name).upper()

        config_vars[env_name] = {
            "field_name": name,
            "type": field_type,
            "has_default": default_val is not None or "default" in field_args,
        }

    return config_vars


def scan_routes_for_endpoints(routes_path: Path) -> list[tuple[str, str]]:
    """Scan FastAPI routes for endpoint definitions."""
    endpoints: list[tuple[str, str]] = []

    if routes_path.is_file():
        files = [routes_path]
    elif routes_path.is_dir():
        files = list(routes_path.glob("**/*.py"))
    else:
        return endpoints

    for route_file in files:
        content = route_file.read_text()

        # FastAPI decorator patterns
        for match in re.finditer(
            r"@(?:router|app)\.(get|post|put|patch|delete)\(['\"]([^'\"]+)['\"]",
            content,
            re.IGNORECASE,
        ):
            method = match.group(1).upper()
            path = match.group(2)
            endpoints.append((method, path))

    return endpoints


def scan_code_for_observability(code_path: Path) -> tuple[set[str], set[str]]:
    """Scan code for OTel metrics and spans."""
    metrics: set[str] = set()
    spans: set[str] = set()

    if not code_path.exists():
        return metrics, spans

    for py_file in code_path.glob("**/*.py"):
        content = py_file.read_text()

        # Metric creation patterns
        for match in re.finditer(r"(?:create_|meter\.create_)(?:counter|gauge|histogram)\(['\"](\w+)['\"]", content):
            metrics.add(match.group(1))

        # Span patterns
        for match in re.finditer(r"(?:tracer\.start_span|with\s+tracer\.start_as_current_span)\(['\"](\w+)['\"]", content):
            spans.add(match.group(1))

    return metrics, spans


# =============================================================================
# COMPLIANCE CHECKS
# =============================================================================


def check_schema_compliance(
    spec_path: Path, migrations_dir: Path | None = None
) -> list[ComplianceGap]:
    """Check that all spec'd schema elements exist in migrations."""
    gaps: list[ComplianceGap] = []
    gap_counter = 0

    spec_content = spec_path.read_text()
    tables, constraints, indexes = extract_ddl_elements(spec_content)

    # If no migrations dir provided, report all as missing with info severity
    if migrations_dir is None or not migrations_dir.exists():
        for table in tables:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V1-{gap_counter:03d}",
                    category="schema",
                    severity="info",
                    spec_requirement=f"Table: {table.name}",
                    spec_location=table.location,
                    implementation_status="missing",
                    details="No migrations directory provided - cannot verify",
                    suggested_fix="Run with --migrations <dir> to verify",
                )
            )
        return gaps

    impl_tables, impl_constraints, impl_indexes = scan_migrations_for_schema(migrations_dir)

    # Check tables
    for table in tables:
        if table.name.lower() not in {t.lower() for t in impl_tables}:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V1-{gap_counter:03d}",
                    category="schema",
                    severity="critical",
                    spec_requirement=f"Table: {table.name}",
                    spec_location=table.location,
                    implementation_status="missing",
                    details=f"Table '{table.name}' not found in migrations",
                    suggested_fix=f"Add migration to create table '{table.name}'",
                )
            )

    # Check constraints
    for constraint in constraints:
        if constraint.name.lower() not in {c.lower() for c in impl_constraints}:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V1-{gap_counter:03d}",
                    category="schema",
                    severity="critical",
                    spec_requirement=f"Constraint: {constraint.name} ({constraint.constraint_type})",
                    spec_location=constraint.location,
                    implementation_status="missing",
                    details=f"Constraint '{constraint.name}' not found",
                    suggested_fix=f"Add {constraint.constraint_type} constraint on {constraint.table}({', '.join(constraint.columns)})",
                )
            )

    # Check indexes
    for index in indexes:
        if index.name.lower() not in {i.lower() for i in impl_indexes}:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V1-{gap_counter:03d}",
                    category="schema",
                    severity="warning",
                    spec_requirement=f"Index: {index.name}",
                    spec_location=index.location,
                    implementation_status="missing",
                    details=f"Index '{index.name}' not found",
                    suggested_fix=f"Add {index.index_type} index on {index.table}({', '.join(index.columns)})",
                )
            )

    return gaps


def check_config_compliance(spec_path: Path, settings_path: Path) -> list[ComplianceGap]:
    """Check that all spec'd env vars are wired in settings."""
    gaps: list[ComplianceGap] = []
    gap_counter = 0

    spec_content = spec_path.read_text()
    config_vars = extract_config_requirements(spec_content)
    impl_config = scan_settings_for_config(settings_path)

    for var in config_vars:
        if var.name not in impl_config:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V2-{gap_counter:03d}",
                    category="config",
                    severity="warning" if not var.required else "critical",
                    spec_requirement=f"Env var: {var.name} ({var.var_type})",
                    spec_location=var.location,
                    implementation_status="missing",
                    details=f"Environment variable '{var.name}' not found in settings",
                    suggested_fix=f"Add {var.name.lower()}: {var.var_type} field to settings class",
                )
            )

    return gaps


def check_api_compliance(spec_path: Path, routes_path: Path) -> list[ComplianceGap]:
    """Check that all spec'd endpoints exist in routes."""
    gaps: list[ComplianceGap] = []
    gap_counter = 0

    spec_content = spec_path.read_text()
    endpoints = extract_api_requirements(spec_content)
    impl_endpoints = scan_routes_for_endpoints(routes_path)

    impl_set = {(m.upper(), p) for m, p in impl_endpoints}

    for endpoint in endpoints:
        # Normalize path (remove trailing slashes, handle path params)
        spec_path_normalized = endpoint.path.rstrip("/")
        found = False

        for impl_method, impl_path in impl_set:
            impl_path_normalized = impl_path.rstrip("/")
            # Handle path params: /items/{id} should match /items/{item_id}
            if impl_method == endpoint.method:
                spec_parts = spec_path_normalized.split("/")
                impl_parts = impl_path_normalized.split("/")
                if len(spec_parts) == len(impl_parts):
                    match = True
                    for sp, ip in zip(spec_parts, impl_parts):
                        if sp.startswith("{") and ip.startswith("{"):
                            continue  # Both are path params
                        if sp != ip:
                            match = False
                            break
                    if match:
                        found = True
                        break

        if not found:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V3-{gap_counter:03d}",
                    category="api",
                    severity="critical",
                    spec_requirement=f"{endpoint.method} {endpoint.path}",
                    spec_location=endpoint.location,
                    implementation_status="missing",
                    details=f"Endpoint not found in routes",
                    suggested_fix=f"Add @router.{endpoint.method.lower()}('{endpoint.path}') handler",
                )
            )

    return gaps


def check_observability_compliance(spec_path: Path, code_path: Path) -> list[ComplianceGap]:
    """Check that all spec'd metrics and spans exist in code."""
    gaps: list[ComplianceGap] = []
    gap_counter = 0

    spec_content = spec_path.read_text()
    metrics, spans = extract_observability_requirements(spec_content)
    impl_metrics, impl_spans = scan_code_for_observability(code_path)

    for metric in metrics:
        if metric.name not in impl_metrics:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V4-{gap_counter:03d}",
                    category="observability",
                    severity="warning",
                    spec_requirement=f"Metric: {metric.name} ({metric.metric_type})",
                    spec_location=metric.location,
                    implementation_status="missing",
                    details=f"Metric '{metric.name}' not found in code",
                    suggested_fix=f"Add meter.create_{metric.metric_type}('{metric.name}')",
                )
            )

    for span in spans:
        if span.name not in impl_spans:
            gap_counter += 1
            gaps.append(
                ComplianceGap(
                    id=f"V4-{gap_counter:03d}",
                    category="observability",
                    severity="info",
                    spec_requirement=f"Span: {span.name}",
                    spec_location=span.location,
                    implementation_status="missing",
                    details=f"Span '{span.name}' not found in code",
                    suggested_fix=f"Add tracer.start_span('{span.name}')",
                )
            )

    return gaps


# =============================================================================
# REPORTING
# =============================================================================


def generate_report(gaps: list[ComplianceGap], spec_path: Path, target_path: Path | None) -> ComplianceReport:
    """Generate compliance report from gaps."""
    report = ComplianceReport(
        spec_path=str(spec_path),
        target_path=str(target_path) if target_path else "",
        checked_at=datetime.now(timezone.utc).isoformat(),
        gaps=gaps,
    )

    # Generate summary
    by_severity = {"critical": 0, "warning": 0, "info": 0}
    by_category: dict[str, int] = {}

    for gap in gaps:
        by_severity[gap.severity] += 1
        by_category[gap.category] = by_category.get(gap.category, 0) + 1

    report.summary = {
        "total_gaps": len(gaps),
        "by_severity": by_severity,
        "by_category": by_category,
        "compliant": by_severity["critical"] == 0,
    }

    return report


def print_report(report: ComplianceReport) -> None:
    """Print human-readable compliance report."""
    print("=" * 60)
    print("COMPLIANCE CHECK REPORT")
    print("=" * 60)
    print()
    print(f"Spec: {report.spec_path}")
    print(f"Target: {report.target_path or 'N/A'}")
    print(f"Checked: {report.checked_at}")
    print()
    print(f"Total Gaps: {report.summary['total_gaps']}")
    print(f"  Critical: {report.summary['by_severity']['critical']}")
    print(f"  Warning:  {report.summary['by_severity']['warning']}")
    print(f"  Info:     {report.summary['by_severity']['info']}")
    print()

    if report.summary["compliant"]:
        print("STATUS: COMPLIANT - No critical gaps")
    else:
        print("STATUS: NON-COMPLIANT - Critical gaps require attention")
    print()

    # Group by category
    by_cat: dict[str, list[ComplianceGap]] = {}
    for gap in report.gaps:
        if gap.category not in by_cat:
            by_cat[gap.category] = []
        by_cat[gap.category].append(gap)

    category_names = {
        "schema": "V1: Schema Compliance",
        "config": "V2: Configuration Compliance",
        "api": "V3: API Compliance",
        "observability": "V4: Observability Compliance",
    }

    for cat, name in category_names.items():
        if cat not in by_cat:
            continue

        print("-" * 60)
        print(name)
        print("-" * 60)

        for gap in by_cat[cat]:
            severity_icon = {"critical": "!", "warning": "?", "info": "."}[gap.severity]
            print(f"\n[{severity_icon}] {gap.id} ({gap.spec_location})")
            print(f"    Requirement: {gap.spec_requirement}")
            print(f"    Status: {gap.implementation_status}")
            if gap.details:
                print(f"    Details: {gap.details}")
            if gap.suggested_fix:
                print(f"    Fix: {gap.suggested_fix}")

    print()


def save_report(report: ComplianceReport, planning_dir: Path) -> Path:
    """Save compliance report to reports directory."""
    reports_dir = planning_dir / "reports"
    reports_dir.mkdir(parents=True, exist_ok=True)

    output_path = reports_dir / "compliance-report.json"

    data = {
        "version": report.version,
        "spec_path": report.spec_path,
        "target_path": report.target_path,
        "checked_at": report.checked_at,
        "summary": report.summary,
        "gaps": [asdict(g) for g in report.gaps],
    }

    output_path.write_text(json.dumps(data, indent=2))
    return output_path


# =============================================================================
# MAIN
# =============================================================================


def main() -> None:
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]
    args = sys.argv[2:]

    def get_arg(name: str) -> str | None:
        """Get argument value by name."""
        for i, arg in enumerate(args):
            if arg == f"--{name}" and i + 1 < len(args):
                return args[i + 1]
        return None

    spec_path_str = get_arg("spec")
    if not spec_path_str and cmd != "help":
        print("Error: --spec <path> is required")
        sys.exit(1)

    spec_path = Path(spec_path_str) if spec_path_str else None

    if cmd == "schema":
        if not spec_path or not spec_path.exists():
            print(f"Spec file not found: {spec_path}")
            sys.exit(1)

        migrations_str = get_arg("migrations")
        migrations_dir = Path(migrations_str) if migrations_str else None

        gaps = check_schema_compliance(spec_path, migrations_dir)
        report = generate_report(gaps, spec_path, migrations_dir)
        print_report(report)

        # Print JSON
        print("\n--- JSON Output ---")
        print(json.dumps({"gaps": [asdict(g) for g in gaps]}, indent=2))

        sys.exit(0 if report.summary["compliant"] else 1)

    elif cmd == "config":
        if not spec_path or not spec_path.exists():
            print(f"Spec file not found: {spec_path}")
            sys.exit(1)

        settings_str = get_arg("settings")
        if not settings_str:
            print("Error: --settings <path> is required")
            sys.exit(1)

        settings_path = Path(settings_str)
        gaps = check_config_compliance(spec_path, settings_path)
        report = generate_report(gaps, spec_path, settings_path)
        print_report(report)

        sys.exit(0 if report.summary["compliant"] else 1)

    elif cmd == "api":
        if not spec_path or not spec_path.exists():
            print(f"Spec file not found: {spec_path}")
            sys.exit(1)

        routes_str = get_arg("routes")
        if not routes_str:
            print("Error: --routes <path> is required")
            sys.exit(1)

        routes_path = Path(routes_str)
        gaps = check_api_compliance(spec_path, routes_path)
        report = generate_report(gaps, spec_path, routes_path)
        print_report(report)

        sys.exit(0 if report.summary["compliant"] else 1)

    elif cmd == "observability":
        if not spec_path or not spec_path.exists():
            print(f"Spec file not found: {spec_path}")
            sys.exit(1)

        code_str = get_arg("code")
        if not code_str:
            print("Error: --code <path> is required")
            sys.exit(1)

        code_path = Path(code_str)
        gaps = check_observability_compliance(spec_path, code_path)
        report = generate_report(gaps, spec_path, code_path)
        print_report(report)

        sys.exit(0 if report.summary["compliant"] else 1)

    elif cmd == "all":
        if not spec_path or not spec_path.exists():
            print(f"Spec file not found: {spec_path}")
            sys.exit(1)

        target_str = get_arg("target")
        if not target_str:
            print("Error: --target <path> is required")
            sys.exit(1)

        target_path = Path(target_str)
        all_gaps: list[ComplianceGap] = []

        # Try to find common paths
        migrations_dir = target_path / "alembic" / "versions"
        if not migrations_dir.exists():
            migrations_dir = target_path / "migrations"

        settings_path = target_path / "src" / "settings.py"
        if not settings_path.exists():
            settings_path = target_path / "settings.py"

        routes_path = target_path / "src" / "routes"
        if not routes_path.exists():
            routes_path = target_path / "routes"

        code_path = target_path / "src"
        if not code_path.exists():
            code_path = target_path

        # Run all checks
        all_gaps.extend(check_schema_compliance(spec_path, migrations_dir))
        all_gaps.extend(check_config_compliance(spec_path, settings_path))
        all_gaps.extend(check_api_compliance(spec_path, routes_path))
        all_gaps.extend(check_observability_compliance(spec_path, code_path))

        report = generate_report(all_gaps, spec_path, target_path)
        print_report(report)

        # Also output JSON
        print("\n--- JSON Output ---")
        print(json.dumps({"summary": report.summary, "gap_count": len(all_gaps)}, indent=2))

        sys.exit(0 if report.summary["compliant"] else 1)

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
