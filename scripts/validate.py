#!/usr/bin/env python3
"""
Validation Module for Task Decomposition Protocol v2

Provides comprehensive validation for the multi-agent workflow:
- DAG cycle detection for task dependencies
- Verifier calibration tracking
- Rollback integrity validation
- Steel thread path validation
- Verification command pre-checks

Usage:
    validate.py dag                      Check for dependency cycles
    validate.py steel-thread             Validate steel thread path
    validate.py verification-commands    Check verification command syntax
    validate.py calibration              Compute verifier calibration metrics
    validate.py rollback <task_id>       Validate rollback integrity
    validate.py all                      Run all validations
"""

import hashlib
import json
import sys
from collections import defaultdict
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
PLANNING_DIR = PROJECT_ROOT / "project-planning"
STATE_FILE = PLANNING_DIR / "state.json"
TASKS_DIR = PLANNING_DIR / "tasks"


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
    """Validate steel thread forms a contiguous path and is in early waves.

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

    # Check waves - steel thread should be early
    max_wave = max(tasks[t].get("wave", 1) for t in tasks)
    for tid in steel_thread_tasks:
        wave = tasks.get(tid, {}).get("wave", 1)
        if wave > max_wave // 2 + 1:
            issues.append(f"{tid} is in wave {wave}, should be in earlier waves for steel thread")

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
# ROLLBACK INTEGRITY VALIDATION
# =============================================================================


def prepare_rollback_checksums(
    target_dir: Path, files_to_modify: list[str]
) -> dict[str, str]:
    """Compute checksums for files before modification.

    Args:
        target_dir: Base directory of target project
        files_to_modify: List of relative file paths

    Returns:
        Dict mapping file path to checksum (empty string for new files)
    """
    checksums = {}
    for file_path in files_to_modify:
        full_path = target_dir / file_path
        checksums[file_path] = file_checksum(full_path)
    return checksums


def verify_rollback_integrity(
    target_dir: Path,
    original_checksums: dict[str, str],
    files_created: list[str],
    files_modified: list[str],
) -> tuple[bool, list[str]]:
    """Verify rollback restored files to original state.

    Args:
        target_dir: Base directory of target project
        original_checksums: Checksums before task execution
        files_created: Files that should have been deleted
        files_modified: Files that should have been restored

    Returns:
        (success, issues) tuple
    """
    issues = []

    # Check created files were deleted
    for file_path in files_created:
        full_path = target_dir / file_path
        if full_path.exists():
            issues.append(f"Created file not deleted: {file_path}")

    # Check modified files were restored
    for file_path in files_modified:
        full_path = target_dir / file_path
        original = original_checksums.get(file_path, "")

        if not original:
            # File didn't exist before, should not exist now
            if full_path.exists():
                issues.append(f"File should not exist after rollback: {file_path}")
        else:
            current = file_checksum(full_path)
            if current != original:
                issues.append(
                    f"File not restored to original: {file_path} "
                    f"(expected {original[:8]}..., got {current[:8]}...)"
                )

    return len(issues) == 0, issues


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

    elif cmd == "rollback":
        if len(sys.argv) < 3:
            print("Usage: validate.py rollback <task_id>")
            sys.exit(1)
        task_id = sys.argv[2]
        task = state.get("tasks", {}).get(task_id)
        if not task:
            print(f"Task not found: {task_id}")
            sys.exit(1)
        # This would need to be called with stored checksums
        print("Rollback validation requires pre-stored checksums.")
        print("Use prepare_rollback_checksums() before execution and")
        print("verify_rollback_integrity() after rollback.")

    elif cmd == "all":
        results = run_all_validations(state)
        print_validation_report(results)
        sys.exit(0 if results["overall_valid"] else 1)

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
