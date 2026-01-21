#!/usr/bin/env python3
"""
Evaluation Report Generator for Task Decomposition Protocol v2

Generates comprehensive performance reports from execution state.
Answers: "How well did tasker perform this job?"

Usage:
    evaluate.py                    Generate full evaluation report
    evaluate.py --format json      Output as JSON
    evaluate.py --metrics-only     Output only computed metrics
    evaluate.py --validate         Include validation checks (DAG, steel thread, etc.)
"""

import json
import sys
from pathlib import Path

# Paths relative to script location
SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
TASKER_DIR = PROJECT_ROOT / ".tasker"
STATE_FILE = TASKER_DIR / "state.json"

# Import validation functions
try:
    from validate import (
        compute_calibration_metrics,
        run_all_validations,
        validate_all_verification_commands,
        validate_dag,
        validate_steel_thread,
    )
    VALIDATION_AVAILABLE = True
except ImportError:
    VALIDATION_AVAILABLE = False


def load_state() -> dict | None:
    """Load state from file or return None if doesn't exist."""
    if not STATE_FILE.exists():
        return None
    return json.loads(STATE_FILE.read_text())


def compute_metrics(state: dict) -> dict:
    """Compute performance metrics from state."""
    tasks = state.get("tasks", {})
    execution = state.get("execution", {})

    completed = execution.get("completed_count", 0)
    failed = execution.get("failed_count", 0)
    total_tokens = execution.get("total_tokens", 0)
    total_cost = execution.get("total_cost_usd", 0.0)

    total_finished = completed + failed

    first_attempt_successes = 0
    quality_full_pass = 0
    total_criteria = 0
    criteria_pass = 0
    criteria_partial = 0
    criteria_fail = 0
    tasks_with_tests = 0
    edge_cases_pass = 0
    total_attempts = 0

    # Quality dimension counters
    quality_dimensions = {"types": 0, "docs": 0, "patterns": 0, "errors": 0}
    quality_totals = {"types": 0, "docs": 0, "patterns": 0, "errors": 0}

    for task in tasks.values():
        if task.get("status") == "complete":
            attempts = task.get("attempts", 1)
            total_attempts += attempts
            if attempts == 1:
                first_attempt_successes += 1

            verification = task.get("verification", {})

            # Quality pass rate
            quality = verification.get("quality", {})
            if quality:
                all_pass = True
                for dim in ["types", "docs", "patterns", "errors"]:
                    score = quality.get(dim)
                    if score:
                        quality_totals[dim] += 1
                        if score == "PASS":
                            quality_dimensions[dim] += 1
                        else:
                            all_pass = False
                if all_pass:
                    quality_full_pass += 1

            # Functional criteria pass rate
            criteria = verification.get("criteria", [])
            for c in criteria:
                total_criteria += 1
                score = c.get("score")
                if score == "PASS":
                    criteria_pass += 1
                elif score == "PARTIAL":
                    criteria_partial += 1
                else:
                    criteria_fail += 1

            # Test edge case rate
            tests = verification.get("tests", {})
            if tests:
                tasks_with_tests += 1
                if tests.get("edge_cases") == "PASS":
                    edge_cases_pass += 1

    return {
        "task_success_rate": completed / total_finished if total_finished > 0 else 0.0,
        "first_attempt_success_rate": first_attempt_successes / completed if completed > 0 else 0.0,
        "avg_attempts": total_attempts / completed if completed > 0 else 0.0,
        "tokens_per_task": total_tokens / completed if completed > 0 else 0,
        "cost_per_task": total_cost / completed if completed > 0 else 0.0,
        "quality_pass_rate": quality_full_pass / completed if completed > 0 else 0.0,
        "functional_pass_rate": criteria_pass / total_criteria if total_criteria > 0 else 0.0,
        "test_edge_case_rate": edge_cases_pass / tasks_with_tests if tasks_with_tests > 0 else 0.0,
        "completed_count": completed,
        "failed_count": failed,
        "total_tokens": total_tokens,
        "total_cost_usd": total_cost,
        "criteria_pass": criteria_pass,
        "criteria_partial": criteria_partial,
        "criteria_fail": criteria_fail,
        "total_criteria": total_criteria,
        "quality_dimensions": quality_dimensions,
        "quality_totals": quality_totals,
    }


def get_failed_tasks(state: dict) -> list[dict]:
    """Get details of failed tasks."""
    failed = []
    for tid, task in state.get("tasks", {}).items():
        if task.get("status") == "failed":
            failed.append({
                "id": tid,
                "name": task.get("name", ""),
                "error": task.get("error", "Unknown error"),
            })
    return failed


def get_improvement_patterns(state: dict) -> list[str]:
    """Identify improvement patterns from verification data."""
    patterns = []
    edge_case_issues = 0
    type_issues = 0
    doc_issues = 0

    for task in state.get("tasks", {}).values():
        verification = task.get("verification", {})

        tests = verification.get("tests", {})
        if tests.get("edge_cases") in ["PARTIAL", "FAIL"]:
            edge_case_issues += 1

        quality = verification.get("quality", {})
        if quality.get("types") in ["PARTIAL", "FAIL"]:
            type_issues += 1
        if quality.get("docs") in ["PARTIAL", "FAIL"]:
            doc_issues += 1

    if edge_case_issues > 0:
        patterns.append(f"{edge_case_issues} task(s) had issues with edge case testing")
    if type_issues > 0:
        patterns.append(f"{type_issues} task(s) had type annotation issues")
    if doc_issues > 0:
        patterns.append(f"{doc_issues} task(s) had documentation issues")

    return patterns


def print_report(state: dict, include_validation: bool = False) -> None:
    """Print full evaluation report."""
    metrics = compute_metrics(state)
    tasks = state.get("tasks", {})
    artifacts = state.get("artifacts", {})

    # Header
    print("Execution Evaluation Report")
    print("=" * 60)
    print()

    # Planning Quality
    task_validation = artifacts.get("task_validation", {})
    print("Planning Quality")
    print("-" * 40)
    verdict = task_validation.get("verdict", "N/A")
    issues = task_validation.get("issues", [])
    print(f"Plan Verdict: {verdict}")
    print(f"Issues at Planning: {len(issues)}")
    print()

    # Execution Summary
    print("Execution Summary")
    print("-" * 40)
    total_tasks = len(tasks)
    completed = metrics["completed_count"]
    failed = metrics["failed_count"]
    blocked = sum(1 for t in tasks.values() if t.get("status") == "blocked")
    skipped = sum(1 for t in tasks.values() if t.get("status") == "skipped")

    print(f"Tasks: {total_tasks} total")
    if completed > 0:
        print(f"  Completed:     {completed} ({completed/total_tasks*100:.0f}%)" if total_tasks > 0 else f"  Completed:     {completed}")
    if failed > 0:
        print(f"  Failed:        {failed} ({failed/total_tasks*100:.0f}%)" if total_tasks > 0 else f"  Failed:        {failed}")
    if blocked > 0:
        print(f"  Blocked:       {blocked} ({blocked/total_tasks*100:.0f}%)" if total_tasks > 0 else f"  Blocked:       {blocked}")
    if skipped > 0:
        print(f"  Skipped:       {skipped} ({skipped/total_tasks*100:.0f}%)" if total_tasks > 0 else f"  Skipped:       {skipped}")
    print()

    # First-attempt metrics
    first_attempt = int(metrics["first_attempt_success_rate"] * completed) if completed > 0 else 0
    print(f"First-Attempt Success: {first_attempt}/{completed} ({metrics['first_attempt_success_rate']:.0%})")
    print(f"Average Attempts: {metrics['avg_attempts']:.2f}")
    print()

    # Verification Breakdown
    print("Verification Breakdown")
    print("-" * 40)

    # Functional criteria
    print("Functional Criteria:")
    total_c = metrics["total_criteria"]
    if total_c > 0:
        print(f"  PASS:     {metrics['criteria_pass']}/{total_c} ({metrics['criteria_pass']/total_c*100:.0f}%)")
        print(f"  PARTIAL:  {metrics['criteria_partial']}/{total_c} ({metrics['criteria_partial']/total_c*100:.0f}%)")
        print(f"  FAIL:     {metrics['criteria_fail']}/{total_c} ({metrics['criteria_fail']/total_c*100:.0f}%)")
    else:
        print("  No criteria data")
    print()

    # Code quality
    print("Code Quality:")
    qd = metrics["quality_dimensions"]
    qt = metrics["quality_totals"]
    for dim in ["types", "docs", "patterns", "errors"]:
        total = qt[dim]
        passed = qd[dim]
        if total > 0:
            print(f"  {dim.capitalize():10} {passed}/{total} PASS")
    print()

    # Cost Analysis
    print("Cost Analysis")
    print("-" * 40)
    print(f"Total Tokens:  {metrics['total_tokens']:,}")
    print(f"Total Cost:    ${metrics['total_cost_usd']:.2f}")
    if completed > 0:
        print(f"Per Task:      ${metrics['cost_per_task']:.4f}")
    print()

    # Failure Analysis
    failed_tasks = get_failed_tasks(state)
    if failed_tasks:
        print("Failure Analysis")
        print("-" * 40)
        for ft in failed_tasks:
            print(f"{ft['id']}: FAIL - {ft['error']}")
        print()

    # Improvement Patterns
    patterns = get_improvement_patterns(state)
    if patterns:
        print("Improvement Patterns")
        print("-" * 40)
        for p in patterns:
            print(f"- {p}")
        print()

    # Validation Section (if requested and available)
    if include_validation and VALIDATION_AVAILABLE:
        print("Workflow Validation")
        print("-" * 40)

        # DAG validation
        dag_valid, dag_msg = validate_dag(state)
        if dag_valid:
            print("✓ DAG: No dependency cycles")
        else:
            print(f"✗ DAG: {dag_msg}")

        # Steel thread validation
        st_valid, st_issues = validate_steel_thread(state)
        if st_valid:
            print("✓ Steel Thread: Valid path")
        else:
            print("✗ Steel Thread issues:")
            for issue in st_issues:
                print(f"    - {issue}")

        # Verification commands
        vc_valid, vc_issues = validate_all_verification_commands(state)
        if vc_valid:
            print("✓ Verification Commands: All valid")
        else:
            print(f"✗ Verification Commands: {len(vc_issues)} task(s) with issues")

        # Verifier calibration
        cal = compute_calibration_metrics(state)
        print(f"\nVerifier Calibration Score: {cal['calibration_score']:.1%}")
        if cal["false_positives"]:
            print(f"  False Positives: {len(cal['false_positives'])}")
        if cal["false_negatives"]:
            print(f"  Potential False Negatives: {len(cal['false_negatives'])}")
        print()
    elif include_validation and not VALIDATION_AVAILABLE:
        print("Validation module not available")
        print()


def print_json(state: dict, include_validation: bool = False) -> None:
    """Print evaluation data as JSON."""
    metrics = compute_metrics(state)
    failed_tasks = get_failed_tasks(state)
    patterns = get_improvement_patterns(state)
    artifacts = state.get("artifacts", {})
    task_validation = artifacts.get("task_validation", {})

    output = {
        "planning": {
            "verdict": task_validation.get("verdict"),
            "issues_count": len(task_validation.get("issues", [])),
        },
        "metrics": metrics,
        "failed_tasks": failed_tasks,
        "improvement_patterns": patterns,
    }

    # Add validation if requested
    if include_validation and VALIDATION_AVAILABLE:
        validation = run_all_validations(state)
        output["validation"] = {
            "dag_valid": validation["dag"]["valid"],
            "steel_thread_valid": validation["steel_thread"]["valid"],
            "verification_commands_valid": validation["verification_commands"]["valid"],
            "calibration": validation["calibration"],
            "overall_valid": validation["overall_valid"],
        }

    print(json.dumps(output, indent=2))


def print_metrics_only(state: dict) -> None:
    """Print only computed metrics."""
    metrics = compute_metrics(state)
    # Remove internal counters from output
    output_metrics = {
        "task_success_rate": metrics["task_success_rate"],
        "first_attempt_success_rate": metrics["first_attempt_success_rate"],
        "avg_attempts": metrics["avg_attempts"],
        "tokens_per_task": metrics["tokens_per_task"],
        "cost_per_task": metrics["cost_per_task"],
        "quality_pass_rate": metrics["quality_pass_rate"],
        "functional_pass_rate": metrics["functional_pass_rate"],
        "test_edge_case_rate": metrics["test_edge_case_rate"],
    }
    print(json.dumps(output_metrics, indent=2))


def main() -> None:
    state = load_state()
    if not state:
        print("No state file found. Run 'state.py init <target_dir>' first.", file=sys.stderr)
        sys.exit(1)

    output_format = "text"
    metrics_only = False
    include_validation = False

    args = sys.argv[1:]
    i = 0
    while i < len(args):
        if args[i] == "--format" and i + 1 < len(args):
            output_format = args[i + 1]
            i += 2
        elif args[i] == "--metrics-only":
            metrics_only = True
            i += 1
        elif args[i] == "--validate":
            include_validation = True
            i += 1
        else:
            i += 1

    if metrics_only:
        print_metrics_only(state)
    elif output_format == "json":
        print_json(state, include_validation)
    else:
        print_report(state, include_validation)


if __name__ == "__main__":
    main()
