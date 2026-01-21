#!/usr/bin/env python3
"""
Validation Module for Task Decomposition Protocol v2

Provides comprehensive validation for the multi-agent workflow:
- DAG cycle detection for task dependencies
- Verifier calibration tracking
- Rollback integrity validation
- Steel thread path validation
- Verification command pre-checks
- Planning gates for spec adherence enforcement

Usage:
    validate.py dag                      Check for dependency cycles
    validate.py steel-thread             Validate steel thread path
    validate.py verification-commands    Check verification command syntax
    validate.py calibration              Compute verifier calibration metrics
    validate.py all                      Run all validations

Planning Gates:
    validate.py spec-coverage [--threshold 0.9]   Check spec requirement coverage
    validate.py phase-leakage                     Detect Phase 2+ content in tasks
    validate.py dependency-existence              Verify all task dependencies exist
    validate.py acceptance-criteria               Check AC quality (no vague terms)
    validate.py planning-gates [--threshold 0.9]  Run all planning validation gates
    validate.py refactor-priority                 Show refactor override resolution
"""

import hashlib
import json
import os
import subprocess
import sys
from collections import defaultdict
from pathlib import Path


# =============================================================================
# SHIM LAYER - Forward to Go binary if available
# =============================================================================

GO_SUPPORTED_COMMANDS = {
    "dag",
    "steel-thread",
    "gates",
    "spec-coverage",
    "phase-leakage",
    "dependency-existence",
    "acceptance-criteria",
    "planning-gates",
    "verification-commands",
    "all",
    "refactor-priority",
}


def _find_go_binary() -> str | None:
    """Find the tasker Go binary."""
    if env_binary := os.environ.get("TASKER_BINARY"):
        if Path(env_binary).exists():
            return env_binary

    script_dir = Path(__file__).resolve().parent
    possible_paths = [
        script_dir.parent / "go" / "bin" / "tasker",
        script_dir.parent / "bin" / "tasker",
    ]
    for p in possible_paths:
        if p.exists():
            return str(p)

    import shutil
    if path := shutil.which("tasker"):
        return path

    return None


def _translate_args_to_go(args: list[str]) -> list[str]:
    """Translate Python script args to Go subcommand format."""
    if not args:
        return ["validate"]

    cmd = args[0]
    rest = args[1:]

    translations = {
        "dag": ["validate", "dag"],
        "steel-thread": ["validate", "steel-thread"],
        "gates": ["validate", "gates"],
        "spec-coverage": ["validate", "spec-coverage"],
        "phase-leakage": ["validate", "phase-leakage"],
        "dependency-existence": ["validate", "dependency-existence"],
        "acceptance-criteria": ["validate", "acceptance-criteria"],
        "planning-gates": ["validate", "planning-gates"],
        "verification-commands": ["validate", "verification-commands"],
        "all": ["validate", "all"],
        "refactor-priority": ["validate", "refactor-priority"],
    }

    if cmd in translations:
        go_args = translations[cmd].copy()
        go_args.extend(rest)
        return go_args

    return ["validate"] + args


def _try_shim_to_go() -> bool:
    """Try to forward to Go binary. Returns True if forwarded, False if fallback needed."""
    if os.environ.get("USE_PYTHON_IMPL") == "1":
        return False

    if len(sys.argv) < 2:
        return False

    cmd = sys.argv[1]
    if cmd not in GO_SUPPORTED_COMMANDS:
        return False

    binary = _find_go_binary()
    if not binary:
        return False

    go_args = _translate_args_to_go(sys.argv[1:])

    try:
        result = subprocess.run(
            [binary] + go_args,
            stdin=sys.stdin,
            stdout=sys.stdout,
            stderr=sys.stderr,
        )
        sys.exit(result.returncode)
    except Exception:
        return False


# =============================================================================
# PYTHON IMPLEMENTATION
# =============================================================================

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
TASKER_DIR = PROJECT_ROOT / ".tasker"
STATE_FILE = TASKER_DIR / "state.json"
TASKS_DIR = TASKER_DIR / "tasks"


def load_state() -> dict | None:
    """Load state from file or return None if doesn't exist."""
    if not STATE_FILE.exists():
        return None
    return json.loads(STATE_FILE.read_text())


def load_task(task_id: str) -> dict | None:
    """Load task definition from tasks directory."""
    task_path = TASKS_DIR / f"{task_id}.json"
    if not task_path.exists():
        return None
    return json.loads(task_path.read_text())


def file_checksum(path: Path) -> str:
    """SHA256 checksum of file contents."""
    if not path.exists():
        return ""
    return hashlib.sha256(path.read_bytes()).hexdigest()


# =============================================================================
# DAG CYCLE DETECTION
# =============================================================================


def build_dependency_graph(state: dict) -> dict[str, list[str]]:
    """Build adjacency list from task dependencies."""
    graph: dict[str, list[str]] = defaultdict(list)
    for tid, task in state.get("tasks", {}).items():
        deps = task.get("depends_on", [])
        for dep in deps:
            graph[dep].append(tid)  # dep -> tid (dep blocks tid)
        if tid not in graph:
            graph[tid] = []
    return dict(graph)


def detect_cycles(state: dict) -> tuple[bool, list[list[str]]]:
    """Detect cycles in task dependency graph using DFS.

    Returns:
        (has_cycles, list_of_cycles) tuple
    """
    tasks = state.get("tasks", {})
    if not tasks:
        return False, []

    # Build adjacency list: task -> tasks that depend on it
    graph: dict[str, list[str]] = defaultdict(list)
    for tid, task in tasks.items():
        for dep in task.get("depends_on", []):
            graph[dep].append(tid)
        if tid not in graph:
            graph[tid] = []

    # Track visited and recursion stack
    visited: set[str] = set()
    rec_stack: set[str] = set()
    cycles: list[list[str]] = []

    def dfs(node: str, path: list[str]) -> bool:
        visited.add(node)
        rec_stack.add(node)
        path.append(node)

        for neighbor in graph.get(node, []):
            if neighbor not in visited:
                if dfs(neighbor, path):
                    return True
            elif neighbor in rec_stack:
                # Found cycle - extract it
                cycle_start = path.index(neighbor)
                cycle = path[cycle_start:] + [neighbor]
                cycles.append(cycle)
                return True

        path.pop()
        rec_stack.remove(node)
        return False

    for task_id in tasks:
        if task_id not in visited:
            dfs(task_id, [])

    return len(cycles) > 0, cycles


def validate_dag(state: dict) -> tuple[bool, str]:
    """Validate task DAG has no cycles.

    Returns:
        (valid, message) tuple
    """
    has_cycles, cycles = detect_cycles(state)

    if has_cycles:
        cycle_strs = [" -> ".join(c) for c in cycles]
        return False, "Dependency cycles detected:\n" + "\n".join(cycle_strs)

    return True, "DAG is valid (no cycles)"


# =============================================================================
# STEEL THREAD VALIDATION
# =============================================================================


def validate_steel_thread(state: dict) -> tuple[bool, list[str]]:
    """Validate steel thread forms a contiguous path and is in early phases.

    Returns:
        (valid, issues) tuple
    """
    tasks = state.get("tasks", {})
    issues: list[str] = []

    # Find steel thread tasks
    steel_thread_tasks = []
    for tid, task in tasks.items():
        task_def = load_task(tid)
        if task_def and task_def.get("context", {}).get("steel_thread"):
            steel_thread_tasks.append(tid)

    if not steel_thread_tasks:
        issues.append("No steel thread tasks defined")
        return False, issues

    # Check phases - steel thread should be early
    max_phase = max(tasks[t].get("phase", 1) for t in tasks)
    for tid in steel_thread_tasks:
        phase = tasks.get(tid, {}).get("phase", 1)
        if phase > max_phase // 2 + 1:
            issues.append(f"{tid} is in phase {phase}, should be in earlier phases for steel thread")

    # Check connectivity - steel thread tasks should form a path
    st_set = set(steel_thread_tasks)
    connected = set()

    # Start from tasks with no steel thread dependencies
    def has_st_dep(tid: str) -> bool:
        deps = tasks.get(tid, {}).get("depends_on", [])
        return any(d in st_set for d in deps)

    roots = [t for t in steel_thread_tasks if not has_st_dep(t)]

    if not roots:
        issues.append("Steel thread has no root task (all have steel thread dependencies)")
    else:
        # BFS to find connected steel thread tasks
        queue = list(roots)
        while queue:
            current = queue.pop(0)
            if current in connected:
                continue
            connected.add(current)
            # Find steel thread tasks that depend on current
            for tid in steel_thread_tasks:
                if current in tasks.get(tid, {}).get("depends_on", []):
                    queue.append(tid)

        disconnected = st_set - connected
        if disconnected:
            issues.append(f"Disconnected steel thread tasks: {disconnected}")

    return len(issues) == 0, issues


# =============================================================================
# VERIFIER CALIBRATION
# =============================================================================


def compute_calibration_metrics(state: dict) -> dict:
    """Compute verifier calibration metrics.

    Tracks:
    - False positive rate: PASS verdict but task later failed
    - Verdict distribution: How often each verdict appears
    - Plan vs execution correlation: Plan verifier vs task verifier agreement

    Returns:
        Dict with calibration metrics
    """
    tasks = state.get("tasks", {})

    metrics = {
        "total_verified": 0,
        "verdict_distribution": {"PASS": 0, "FAIL": 0, "CONDITIONAL": 0},
        "recommendation_distribution": {"PROCEED": 0, "BLOCK": 0},
        "false_positives": [],  # Tasks that got PASS/PROCEED but failed
        "false_negatives": [],  # Tasks that got BLOCK but would have worked
        "calibration_score": 0.0,
    }

    for tid, task in tasks.items():
        verification = task.get("verification", {})
        if not verification:
            continue

        metrics["total_verified"] += 1
        verdict = verification.get("verdict", "")
        recommendation = verification.get("recommendation", "")

        if verdict in metrics["verdict_distribution"]:
            metrics["verdict_distribution"][verdict] += 1
        if recommendation in metrics["recommendation_distribution"]:
            metrics["recommendation_distribution"][recommendation] += 1

        # Check for false positives
        status = task.get("status")
        if recommendation == "PROCEED" and status == "failed":
            metrics["false_positives"].append({
                "task_id": tid,
                "verdict": verdict,
                "error": task.get("error", "Unknown"),
            })

        # Check for potential false negatives (blocked tasks that were retried successfully)
        if recommendation == "BLOCK" and status == "complete":
            metrics["false_negatives"].append({
                "task_id": tid,
                "verdict": verdict,
            })

    # Compute calibration score (1.0 = perfect, lower = more false positives/negatives)
    total = metrics["total_verified"]
    if total > 0:
        fp_count = len(metrics["false_positives"])
        fn_count = len(metrics["false_negatives"])
        metrics["calibration_score"] = (total - fp_count - fn_count) / total

    return metrics


def get_calibration_report(state: dict) -> str:
    """Generate human-readable calibration report."""
    metrics = compute_calibration_metrics(state)

    lines = [
        "Verifier Calibration Report",
        "=" * 40,
        "",
        f"Total Verified Tasks: {metrics['total_verified']}",
        "",
        "Verdict Distribution:",
    ]

    for verdict, count in metrics["verdict_distribution"].items():
        lines.append(f"  {verdict}: {count}")

    lines.extend([
        "",
        "Recommendation Distribution:",
    ])

    for rec, count in metrics["recommendation_distribution"].items():
        lines.append(f"  {rec}: {count}")

    lines.extend([
        "",
        f"Calibration Score: {metrics['calibration_score']:.1%}",
    ])

    if metrics["false_positives"]:
        lines.extend([
            "",
            "False Positives (PROCEED but failed):",
        ])
        for fp in metrics["false_positives"]:
            lines.append(f"  {fp['task_id']}: {fp['verdict']} - {fp['error']}")

    if metrics["false_negatives"]:
        lines.extend([
            "",
            "Potential False Negatives (BLOCK but succeeded on retry):",
        ])
        for fn in metrics["false_negatives"]:
            lines.append(f"  {fn['task_id']}: {fn['verdict']}")

    return "\n".join(lines)


# =============================================================================
# VERIFICATION COMMAND VALIDATION
# =============================================================================


def validate_verification_commands_for_criteria(
    criteria: list[dict],
) -> tuple[bool, list[str]]:
    """Validate verification commands for a list of acceptance criteria.

    This is the core validation logic used by both state-based and bundle-based
    validation functions.

    Args:
        criteria: List of acceptance criteria dicts with 'criterion' and 'verification' keys

    Returns:
        (all_valid, [issues]) tuple
    """
    import shlex

    issues: list[str] = []
    all_valid = True

    for criterion in criteria:
        cmd = criterion.get("verification", "")
        criterion_name = criterion.get("criterion", "unknown")

        if not cmd:
            issues.append(f"Empty verification for: {criterion_name}")
            all_valid = False
            continue

        try:
            parts = shlex.split(cmd)
            if not parts:
                issues.append(f"Empty command for: {criterion_name}")
                all_valid = False
        except ValueError as e:
            issues.append(f"Invalid command '{cmd}': {e}")
            all_valid = False

    return all_valid, issues


def validate_all_verification_commands(state: dict) -> tuple[bool, dict[str, list[str]]]:
    """Validate verification commands for all tasks in state.

    Returns:
        (all_valid, {task_id: [issues]}) tuple
    """
    tasks = state.get("tasks", {})
    issues_by_task: dict[str, list[str]] = {}
    all_valid = True

    for tid in tasks:
        task_def = load_task(tid)
        if not task_def:
            continue

        criteria = task_def.get("acceptance_criteria", [])
        task_valid, task_issues = validate_verification_commands_for_criteria(criteria)

        if not task_valid:
            all_valid = False
        if task_issues:
            issues_by_task[tid] = task_issues

    return all_valid, issues_by_task


# =============================================================================
# PLANNING GATES - SPEC ADHERENCE ENFORCEMENT
# =============================================================================

VAGUE_TERMS = [
    "works correctly",
    "handles errors",
    "is correct",
    "functions properly",
    "is good",
    "is clean",
    "is fast",
    "performs well",
    "is secure",
]

VALID_VERIFICATION_PREFIXES = (
    "pytest",
    "python",
    "bash",
    "grep",
    "test",
    "curl",
    "npm",
    "make",
    "go",
    "ruff",
    "cargo",
    "node",
    "yarn",
    "pnpm",
    "dotnet",
    "mvn",
    "gradle",
)


def extract_requirements(spec_path: Path) -> dict[str, str]:
    """Extract requirements from spec as {requirement_id: requirement_text}.

    Parses the spec file looking for requirement patterns:
    - Lines starting with REQ-XXX:
    - Lines starting with R#.#:
    - Numbered list items (1., 2., etc.)
    - Bullet items starting with - or *

    Returns:
        Dict mapping requirement ID to requirement text
    """
    if not spec_path.exists():
        return {}

    requirements: dict[str, str] = {}
    content = spec_path.read_text()
    lines = content.split("\n")

    req_counter = 0
    for i, line in enumerate(lines):
        line = line.strip()
        if not line:
            continue

        # Pattern: REQ-XXX: description
        if line.startswith("REQ-"):
            parts = line.split(":", 1)
            if len(parts) == 2:
                req_id = parts[0].strip()
                requirements[req_id] = parts[1].strip()
                continue

        # Pattern: R#.#: description (e.g., R1.2: ...)
        if line.startswith("R") and len(line) > 1 and line[1].isdigit():
            parts = line.split(":", 1)
            if len(parts) == 2:
                req_id = parts[0].strip()
                requirements[req_id] = parts[1].strip()
                continue

        # Pattern: numbered list (1. description)
        if line and line[0].isdigit() and "." in line[:4]:
            dot_pos = line.find(".")
            if dot_pos > 0 and dot_pos < 4:
                req_counter += 1
                req_id = f"REQ-{req_counter:03d}"
                requirements[req_id] = line[dot_pos + 1 :].strip()
                continue

        # Pattern: bullet items (- or * followed by space)
        if line.startswith("- ") or line.startswith("* "):
            req_counter += 1
            req_id = f"REQ-{req_counter:03d}"
            requirements[req_id] = line[2:].strip()

    return requirements


def validate_spec_coverage(
    state_dir: Path, threshold: float = 0.9
) -> tuple[bool, dict]:
    """Block planning if spec coverage falls below threshold.

    Returns:
        (passed, report) where report contains:
        - coverage_ratio: float
        - covered_requirements: list
        - uncovered_requirements: list
        - refactor_overrides: list (requirements superseded by refactors)
        - threshold: float
    """
    spec_path = state_dir / "inputs" / "spec.md"
    tasks_dir = state_dir / "tasks"

    requirements = extract_requirements(spec_path)
    if not requirements:
        return True, {
            "coverage_ratio": 1.0,
            "covered_requirements": [],
            "uncovered_requirements": [],
            "refactor_overrides": [],
            "threshold": threshold,
        }

    covered: set[str] = set()
    refactor_overrides: set[str] = set()

    if tasks_dir.exists():
        for task_file in tasks_dir.glob("T*.json"):
            task = json.loads(task_file.read_text())
            context = task.get("context", {})
            spec_ref = context.get("spec_ref", {})

            if isinstance(spec_ref, dict):
                if "refactor_ref" in spec_ref:
                    for superseded in spec_ref.get("supersedes", []):
                        refactor_overrides.add(superseded)
                elif "quote" in spec_ref:
                    quote = spec_ref["quote"].lower()
                    for req_id, req_text in requirements.items():
                        if quote in req_text.lower() or req_text.lower() in quote:
                            covered.add(req_id)

            # Also check spec_requirements list
            for req_id in context.get("spec_requirements", []):
                if req_id in requirements:
                    covered.add(req_id)

    uncovered = set(requirements.keys()) - covered - refactor_overrides
    total = len(requirements)
    coverage_ratio = (len(covered) + len(refactor_overrides)) / total if total else 1.0

    return coverage_ratio >= threshold, {
        "coverage_ratio": coverage_ratio,
        "covered_requirements": sorted(covered),
        "uncovered_requirements": sorted(uncovered),
        "refactor_overrides": sorted(refactor_overrides),
        "threshold": threshold,
    }


def detect_phase_leakage(state_dir: Path) -> tuple[bool, list[dict]]:
    """Verify no Phase 2+ content leaked into Phase 1 tasks.

    Returns:
        (passed, violations) where violations contain:
        - task_id: str
        - behavior: str
        - evidence: str
    """
    cap_map_path = state_dir / "artifacts" / "capability-map.json"
    if not cap_map_path.exists():
        return True, []

    cap_map = json.loads(cap_map_path.read_text())
    tasks_dir = state_dir / "tasks"

    excluded_phases = cap_map.get("phase_filtering", {}).get("excluded_phases", [])
    excluded_content: set[str] = set()
    for phase in excluded_phases:
        summary = phase.get("summary", "").lower()
        if summary:
            excluded_content.add(summary)
        heading = phase.get("heading", "").lower()
        if heading:
            excluded_content.add(heading)

    if not excluded_content or not tasks_dir.exists():
        return True, []

    violations: list[dict] = []
    for task_file in tasks_dir.glob("T*.json"):
        task = json.loads(task_file.read_text())

        # Skip refactor tasks - they may intentionally touch excluded content
        if task.get("task_type") == "refactor":
            continue

        task_id = task.get("id", task_file.stem)

        for behavior in task.get("behaviors", []):
            behavior_lower = behavior.lower()
            for excluded in excluded_content:
                if excluded in behavior_lower or behavior_lower in excluded:
                    violations.append(
                        {
                            "task_id": task_id,
                            "behavior": behavior,
                            "evidence": f"Matches excluded content: {excluded}",
                        }
                    )

    return len(violations) == 0, violations


def validate_dependency_existence(state_dir: Path) -> tuple[bool, list[dict]]:
    """Verify all declared dependencies reference existing tasks.

    Returns:
        (all_exist, violations) where violations contain:
        - task_id: str
        - missing_dependency: str
    """
    tasks_dir = state_dir / "tasks"
    if not tasks_dir.exists():
        return True, []

    task_files = list(tasks_dir.glob("T*.json"))

    # Build set of existing task IDs
    task_ids: set[str] = set()
    for task_file in task_files:
        task = json.loads(task_file.read_text())
        task_ids.add(task.get("id", task_file.stem))

    violations: list[dict] = []
    for task_file in task_files:
        task = json.loads(task_file.read_text())
        task_id = task.get("id", task_file.stem)
        deps = task.get("dependencies", {}).get("tasks", [])

        for dep in deps:
            if dep not in task_ids:
                violations.append(
                    {
                        "task_id": task_id,
                        "missing_dependency": dep,
                    }
                )

    return len(violations) == 0, violations


def validate_acceptance_criteria_quality(state_dir: Path) -> tuple[bool, list[dict]]:
    """Validate acceptance criteria are specific and measurable.

    Returns:
        (all_valid, violations) where violations contain:
        - task_id: str
        - criterion_index: int
        - issue: str
    """
    tasks_dir = state_dir / "tasks"
    if not tasks_dir.exists():
        return True, []

    violations: list[dict] = []

    for task_file in tasks_dir.glob("T*.json"):
        task = json.loads(task_file.read_text())
        task_id = task.get("id", task_file.stem)

        for idx, criterion in enumerate(task.get("acceptance_criteria", [])):
            criterion_text = criterion.get("criterion", "").lower()
            verification = criterion.get("verification", "")

            # Check for vague terms
            for vague in VAGUE_TERMS:
                if vague in criterion_text:
                    violations.append(
                        {
                            "task_id": task_id,
                            "criterion_index": idx,
                            "issue": f"Vague term '{vague}' in criterion",
                        }
                    )

            # Check verification command starts with known tool
            if verification and not any(
                verification.startswith(p) for p in VALID_VERIFICATION_PREFIXES
            ):
                violations.append(
                    {
                        "task_id": task_id,
                        "criterion_index": idx,
                        "issue": "Verification command doesn't start with recognized tool",
                    }
                )

    return len(violations) == 0, violations


def resolve_refactor_priority(state_dir: Path) -> dict:
    """Build authoritative requirement map with refactor overrides applied.

    Returns:
        {
            "effective_requirements": dict,  # After refactor overrides
            "original_requirements": dict,   # From spec.md
            "overrides": [                   # Refactor override log
                {"task_id": str, "supersedes": list, "directive": str}
            ]
        }
    """
    spec_path = state_dir / "inputs" / "spec.md"
    tasks_dir = state_dir / "tasks"

    original_reqs = extract_requirements(spec_path)
    effective_reqs = original_reqs.copy()
    overrides: list[dict] = []

    if tasks_dir.exists():
        for task_file in tasks_dir.glob("T*.json"):
            task = json.loads(task_file.read_text())

            if task.get("task_type") == "refactor":
                refactor_ctx = task.get("refactor_context", {})
                superseded = refactor_ctx.get("original_spec_sections", [])
                directive = refactor_ctx.get("refactor_directive", "")

                # Remove superseded requirements from effective set
                for section in superseded:
                    effective_reqs.pop(section, None)

                # Add refactor directive as new requirement
                if directive:
                    effective_reqs[f"REFACTOR-{task['id']}"] = directive

                overrides.append(
                    {
                        "task_id": task["id"],
                        "supersedes": superseded,
                        "directive": directive,
                    }
                )

    return {
        "effective_requirements": effective_reqs,
        "original_requirements": original_reqs,
        "overrides": overrides,
    }


def run_planning_gates(state_dir: Path, spec_threshold: float = 0.9) -> dict:
    """Run all planning validation gates.

    Returns:
        {
            "passed": bool,
            "spec_coverage": {"passed": bool, "report": dict},
            "phase_leakage": {"passed": bool, "violations": list},
            "dependency_existence": {"passed": bool, "violations": list},
            "acceptance_criteria": {"passed": bool, "violations": list},
            "refactor_priority": dict,
            "blocking_issues": list[str]
        }
    """
    results: dict = {
        "passed": True,
        "blocking_issues": [],
    }

    # Spec coverage gate
    passed, report = validate_spec_coverage(state_dir, spec_threshold)
    results["spec_coverage"] = {"passed": passed, "report": report}
    if not passed:
        results["passed"] = False
        results["blocking_issues"].append(
            f"Spec coverage {report['coverage_ratio']:.1%} below threshold {spec_threshold:.1%}"
        )

    # Phase leakage gate
    passed, violations = detect_phase_leakage(state_dir)
    results["phase_leakage"] = {"passed": passed, "violations": violations}
    if not passed:
        results["passed"] = False
        results["blocking_issues"].append(
            f"Phase leakage detected: {len(violations)} violation(s)"
        )

    # Dependency existence gate
    passed, violations = validate_dependency_existence(state_dir)
    results["dependency_existence"] = {"passed": passed, "violations": violations}
    if not passed:
        results["passed"] = False
        results["blocking_issues"].append(
            f"Missing dependencies: {len(violations)} reference(s) to non-existent tasks"
        )

    # Acceptance criteria quality gate
    passed, violations = validate_acceptance_criteria_quality(state_dir)
    results["acceptance_criteria"] = {"passed": passed, "violations": violations}
    if not passed:
        results["passed"] = False
        results["blocking_issues"].append(
            f"Acceptance criteria quality issues: {len(violations)} problem(s)"
        )

    # Refactor priority resolution (informational, doesn't block)
    results["refactor_priority"] = resolve_refactor_priority(state_dir)

    return results


# =============================================================================
# COMPREHENSIVE VALIDATION
# =============================================================================


def run_all_validations(state: dict) -> dict:
    """Run all validations and return combined results."""
    results = {
        "dag": {"valid": True, "message": ""},
        "steel_thread": {"valid": True, "issues": []},
        "verification_commands": {"valid": True, "issues_by_task": {}},
        "calibration": {},
        "overall_valid": True,
    }

    # DAG validation
    dag_valid, dag_msg = validate_dag(state)
    results["dag"]["valid"] = dag_valid
    results["dag"]["message"] = dag_msg
    if not dag_valid:
        results["overall_valid"] = False

    # Steel thread validation
    st_valid, st_issues = validate_steel_thread(state)
    results["steel_thread"]["valid"] = st_valid
    results["steel_thread"]["issues"] = st_issues
    if not st_valid:
        results["overall_valid"] = False

    # Verification commands
    vc_valid, vc_issues = validate_all_verification_commands(state)
    results["verification_commands"]["valid"] = vc_valid
    results["verification_commands"]["issues_by_task"] = vc_issues
    if not vc_valid:
        results["overall_valid"] = False

    # Calibration metrics
    results["calibration"] = compute_calibration_metrics(state)

    return results


def print_validation_report(results: dict) -> None:
    """Print validation results."""
    print("Validation Report")
    print("=" * 60)
    print()

    # DAG
    print("DAG Validation")
    print("-" * 40)
    if results["dag"]["valid"]:
        print("✓ No dependency cycles detected")
    else:
        print("✗ " + results["dag"]["message"])
    print()

    # Steel Thread
    print("Steel Thread Validation")
    print("-" * 40)
    if results["steel_thread"]["valid"]:
        print("✓ Steel thread is valid")
    else:
        print("✗ Issues found:")
        for issue in results["steel_thread"]["issues"]:
            print(f"  - {issue}")
    print()

    # Verification Commands
    print("Verification Commands")
    print("-" * 40)
    if results["verification_commands"]["valid"]:
        print("✓ All verification commands are valid")
    else:
        print("✗ Issues found:")
        for tid, issues in results["verification_commands"]["issues_by_task"].items():
            print(f"  {tid}:")
            for issue in issues:
                print(f"    - {issue}")
    print()

    # Calibration
    print("Verifier Calibration")
    print("-" * 40)
    cal = results["calibration"]
    print(f"Total verified: {cal.get('total_verified', 0)}")
    print(f"Calibration score: {cal.get('calibration_score', 0):.1%}")
    if cal.get("false_positives"):
        print(f"False positives: {len(cal['false_positives'])}")
    print()

    # Overall
    print("Overall Status")
    print("-" * 40)
    if results["overall_valid"]:
        print("✓ All validations passed")
    else:
        print("✗ Some validations failed")


def main() -> None:
    # Try to forward supported commands to Go binary
    _try_shim_to_go()

    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]

    state = load_state()
    if not state:
        print("No state file found. Run 'state.py init <target_dir>' first.", file=sys.stderr)
        sys.exit(1)

    if cmd == "dag":
        valid, msg = validate_dag(state)
        print(msg)
        sys.exit(0 if valid else 1)

    elif cmd == "steel-thread":
        valid, issues = validate_steel_thread(state)
        if valid:
            print("Steel thread is valid")
        else:
            print("Steel thread issues:")
            for issue in issues:
                print(f"  - {issue}")
        sys.exit(0 if valid else 1)

    elif cmd == "verification-commands":
        valid, issues = validate_all_verification_commands(state)
        if valid:
            print("All verification commands are valid")
        else:
            print("Verification command issues:")
            for tid, task_issues in issues.items():
                print(f"  {tid}:")
                for issue in task_issues:
                    print(f"    - {issue}")
        sys.exit(0 if valid else 1)

    elif cmd == "calibration":
        report = get_calibration_report(state)
        print(report)

    elif cmd == "all":
        results = run_all_validations(state)
        print_validation_report(results)
        sys.exit(0 if results["overall_valid"] else 1)

    elif cmd == "spec-coverage":
        threshold = 0.9
        if len(sys.argv) > 2 and sys.argv[2] == "--threshold":
            threshold = float(sys.argv[3])
        valid, report = validate_spec_coverage(TASKER_DIR, threshold)
        print("Spec Coverage Report")
        print("=" * 40)
        print(f"Coverage: {report['coverage_ratio']:.1%}")
        print(f"Threshold: {report['threshold']:.1%}")
        print(f"Covered: {len(report['covered_requirements'])}")
        print(f"Uncovered: {len(report['uncovered_requirements'])}")
        print(f"Refactor overrides: {len(report['refactor_overrides'])}")
        if report["uncovered_requirements"]:
            print("\nUncovered requirements:")
            for req in report["uncovered_requirements"]:
                print(f"  - {req}")
        sys.exit(0 if valid else 1)

    elif cmd == "phase-leakage":
        valid, violations = detect_phase_leakage(TASKER_DIR)
        if valid:
            print("No phase leakage detected")
        else:
            print("Phase leakage violations:")
            for v in violations:
                print(f"  {v['task_id']}: {v['behavior']}")
                print(f"    Evidence: {v['evidence']}")
        sys.exit(0 if valid else 1)

    elif cmd == "dependency-existence":
        valid, violations = validate_dependency_existence(TASKER_DIR)
        if valid:
            print("All dependencies exist")
        else:
            print("Missing dependency violations:")
            for v in violations:
                print(f"  {v['task_id']}: depends on non-existent {v['missing_dependency']}")
        sys.exit(0 if valid else 1)

    elif cmd == "acceptance-criteria":
        valid, violations = validate_acceptance_criteria_quality(TASKER_DIR)
        if valid:
            print("All acceptance criteria meet quality standards")
        else:
            print("Acceptance criteria quality issues:")
            for v in violations:
                print(f"  {v['task_id']} criterion {v['criterion_index']}: {v['issue']}")
        sys.exit(0 if valid else 1)

    elif cmd == "planning-gates":
        threshold = 0.9
        if len(sys.argv) > 2 and sys.argv[2] == "--threshold":
            threshold = float(sys.argv[3])
        results = run_planning_gates(TASKER_DIR, threshold)
        print("Planning Gates Report")
        print("=" * 40)
        print()
        print(f"Spec Coverage: {'PASS' if results['spec_coverage']['passed'] else 'FAIL'}")
        print(f"  Coverage: {results['spec_coverage']['report']['coverage_ratio']:.1%}")
        print()
        print(f"Phase Leakage: {'PASS' if results['phase_leakage']['passed'] else 'FAIL'}")
        if results['phase_leakage']['violations']:
            print(f"  Violations: {len(results['phase_leakage']['violations'])}")
        print()
        print(f"Dependency Existence: {'PASS' if results['dependency_existence']['passed'] else 'FAIL'}")
        if results['dependency_existence']['violations']:
            print(f"  Missing: {len(results['dependency_existence']['violations'])}")
        print()
        print(f"Acceptance Criteria: {'PASS' if results['acceptance_criteria']['passed'] else 'FAIL'}")
        if results['acceptance_criteria']['violations']:
            print(f"  Issues: {len(results['acceptance_criteria']['violations'])}")
        print()
        print("-" * 40)
        if results["passed"]:
            print("All planning gates PASSED")
        else:
            print("Planning gates BLOCKED:")
            for issue in results["blocking_issues"]:
                print(f"  - {issue}")
        sys.exit(0 if results["passed"] else 1)

    elif cmd == "refactor-priority":
        priority = resolve_refactor_priority(TASKER_DIR)
        print("Refactor Priority Resolution")
        print("=" * 40)
        print(f"Original requirements: {len(priority['original_requirements'])}")
        print(f"Effective requirements: {len(priority['effective_requirements'])}")
        print(f"Refactor overrides: {len(priority['overrides'])}")
        if priority["overrides"]:
            print("\nOverrides:")
            for o in priority["overrides"]:
                print(f"  {o['task_id']}: supersedes {o['supersedes']}")
                if o["directive"]:
                    print(f"    Directive: {o['directive'][:60]}...")

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
