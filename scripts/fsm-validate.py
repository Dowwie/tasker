#!/usr/bin/env python3
"""
FSM Validation Script

Validates FSM artifacts against I1-I5 invariants:
- I1: Steel Thread FSM mandatory
- I2: Behavior-first (enforced by compilation order)
- I3: Completeness (initial state, terminals, no dead ends)
- I4: Guard-Invariant linkage
- I5: No silent ambiguity (enforced during compilation)

Usage:
    python3 fsm-validate.py validate <fsm_dir>
    python3 fsm-validate.py completeness <states_file> <transitions_file>
    python3 fsm-validate.py coverage <fsm_index> <capability_map>
"""

import sys
import json
import argparse
from pathlib import Path
from typing import Any


class ValidationResult:
    def __init__(self):
        self.passed = True
        self.issues: list[dict[str, Any]] = []
        self.warnings: list[dict[str, Any]] = []

    def add_issue(self, invariant: str, message: str, context: dict | None = None):
        self.passed = False
        self.issues.append({
            "invariant": invariant,
            "message": message,
            "context": context or {}
        })

    def add_warning(self, message: str, context: dict | None = None):
        self.warnings.append({
            "message": message,
            "context": context or {}
        })

    def to_dict(self) -> dict:
        return {
            "passed": self.passed,
            "issue_count": len(self.issues),
            "warning_count": len(self.warnings),
            "issues": self.issues,
            "warnings": self.warnings
        }


def load_json(path: Path) -> dict | None:
    try:
        with open(path, "r") as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError) as e:
        return None


def validate_i1_steel_thread(index: dict, result: ValidationResult) -> None:
    """I1: Steel Thread FSM mandatory."""
    primary = index.get("primary_machine")
    if not primary:
        result.add_issue("I1", "No primary_machine defined in index")
        return

    machines = {m["id"]: m for m in index.get("machines", [])}
    if primary not in machines:
        result.add_issue("I1", f"primary_machine '{primary}' not found in machines list")
        return

    primary_machine = machines[primary]
    if primary_machine.get("level") != "steel_thread":
        result.add_issue(
            "I1",
            f"primary_machine '{primary}' has level '{primary_machine.get('level')}', expected 'steel_thread'",
            {"machine": primary_machine}
        )


def validate_i3_completeness(states: dict, transitions: dict, result: ValidationResult) -> None:
    """I3: Model completeness constraints."""
    machine_id = states.get("machine_id", "unknown")

    state_list = states.get("states", [])
    state_ids = {s["id"] for s in state_list}
    state_types = {s["id"]: s["type"] for s in state_list}

    initial_state = states.get("initial_state")
    if not initial_state:
        result.add_issue("I3", f"Machine {machine_id}: No initial_state defined")
    elif initial_state not in state_ids:
        result.add_issue("I3", f"Machine {machine_id}: initial_state '{initial_state}' not in states list")

    terminal_states = set(states.get("terminal_states", []))
    if not terminal_states:
        result.add_issue("I3", f"Machine {machine_id}: No terminal_states defined")

    for term in terminal_states:
        if term not in state_ids:
            result.add_issue("I3", f"Machine {machine_id}: terminal_state '{term}' not in states list")

    has_success = any(state_types.get(t) == "success" for t in terminal_states)
    has_failure = any(state_types.get(t) == "failure" for t in terminal_states)
    if not has_success and not has_failure:
        result.add_warning(
            f"Machine {machine_id}: No success or failure terminal states",
            {"terminal_states": list(terminal_states)}
        )

    transition_list = transitions.get("transitions", [])
    sources = {t["from_state"] for t in transition_list}
    targets = {t["to_state"] for t in transition_list}

    for trans in transition_list:
        if trans["from_state"] not in state_ids:
            result.add_issue(
                "I3",
                f"Machine {machine_id}: Transition {trans['id']} references unknown from_state '{trans['from_state']}'"
            )
        if trans["to_state"] not in state_ids:
            result.add_issue(
                "I3",
                f"Machine {machine_id}: Transition {trans['id']} references unknown to_state '{trans['to_state']}'"
            )

    for state in state_list:
        sid = state["id"]
        stype = state["type"]
        if stype not in ("success", "failure") and sid not in sources:
            result.add_issue(
                "I3",
                f"Machine {machine_id}: Non-terminal state '{sid}' ({state['name']}) has no outgoing transitions"
            )

    reachable = set()
    if initial_state:
        to_visit = [initial_state]
        while to_visit:
            current = to_visit.pop()
            if current in reachable:
                continue
            reachable.add(current)
            for trans in transition_list:
                if trans["from_state"] == current and trans["to_state"] not in reachable:
                    to_visit.append(trans["to_state"])

    unreachable = state_ids - reachable
    if unreachable:
        result.add_issue(
            "I3",
            f"Machine {machine_id}: States unreachable from initial: {sorted(unreachable)}"
        )


def validate_i4_guard_linkage(transitions: dict, result: ValidationResult) -> None:
    """I4: Guard-Invariant linkage."""
    machine_id = transitions.get("machine_id", "unknown")

    for trans in transitions.get("transitions", []):
        guards = trans.get("guards", [])
        for guard in guards:
            if not guard.get("invariant_id"):
                result.add_warning(
                    f"Machine {machine_id}: Guard on {trans['id']} has no invariant_id linkage",
                    {"transition": trans["id"], "guard": guard.get("condition")}
                )


def validate_fsm_directory(fsm_dir: Path) -> ValidationResult:
    """Validate all FSM artifacts in a directory."""
    result = ValidationResult()

    index_path = fsm_dir / "index.json"
    index = load_json(index_path)
    if not index:
        result.add_issue("I1", f"Cannot load FSM index from {index_path}")
        return result

    validate_i1_steel_thread(index, result)

    for machine in index.get("machines", []):
        files = machine.get("files", {})

        states_path = fsm_dir / files.get("states", "")
        transitions_path = fsm_dir / files.get("transitions", "")

        states = load_json(states_path)
        transitions = load_json(transitions_path)

        if not states:
            result.add_issue("I3", f"Cannot load states from {states_path}")
            continue
        if not transitions:
            result.add_issue("I3", f"Cannot load transitions from {transitions_path}")
            continue

        validate_i3_completeness(states, transitions, result)
        validate_i4_guard_linkage(transitions, result)

    return result


def validate_coverage(fsm_index_path: Path, capability_map_path: Path) -> ValidationResult:
    """Validate that FSM transitions are covered by capability behaviors."""
    result = ValidationResult()

    index = load_json(fsm_index_path)
    cap_map = load_json(capability_map_path)

    if not index:
        result.add_issue("COVERAGE", f"Cannot load FSM index from {fsm_index_path}")
        return result
    if not cap_map:
        result.add_issue("COVERAGE", f"Cannot load capability map from {capability_map_path}")
        return result

    all_behaviors = set()
    for domain in cap_map.get("domains", []):
        for cap in domain.get("capabilities", []):
            for beh in cap.get("behaviors", []):
                all_behaviors.add(beh["id"])

    fsm_dir = fsm_index_path.parent

    uncovered_transitions = []
    for machine in index.get("machines", []):
        files = machine.get("files", {})
        transitions_path = fsm_dir / files.get("transitions", "")
        transitions = load_json(transitions_path)

        if not transitions:
            continue

        for trans in transitions.get("transitions", []):
            trans_behaviors = trans.get("behaviors", [])
            if not trans_behaviors:
                uncovered_transitions.append({
                    "machine": machine["id"],
                    "transition": trans["id"],
                    "trigger": trans["trigger"]
                })
            else:
                missing = [b for b in trans_behaviors if b not in all_behaviors]
                if missing:
                    result.add_warning(
                        f"Transition {trans['id']} references unknown behaviors: {missing}",
                        {"transition": trans["id"], "missing_behaviors": missing}
                    )

    if uncovered_transitions:
        result.add_warning(
            f"{len(uncovered_transitions)} transitions have no linked behaviors",
            {"uncovered": uncovered_transitions}
        )

    return result


def validate_task_coverage(
    fsm_index_path: Path,
    tasks_dir: Path,
    steel_thread_threshold: float = 1.0,
    non_steel_thread_threshold: float = 0.9
) -> ValidationResult:
    """
    Validate that FSM transitions are covered by tasks (HARD PLANNING GATE).

    - Steel thread transitions: 100% coverage required (default)
    - Non-steel thread transitions: 90% coverage required (configurable)

    Returns coverage report with pass/fail status.
    """
    result = ValidationResult()

    index = load_json(fsm_index_path)
    if not index:
        result.add_issue("TASK_COVERAGE", f"Cannot load FSM index from {fsm_index_path}")
        return result

    task_files = list(tasks_dir.glob("T*.json"))
    if not task_files:
        result.add_issue("TASK_COVERAGE", f"No task files found in {tasks_dir}")
        return result

    tasks_by_transition: dict[str, list[str]] = {}
    for task_file in task_files:
        task = load_json(task_file)
        if not task:
            continue
        task_id = task.get("id", task_file.stem)
        state_machine = task.get("state_machine", {})
        for tr_id in state_machine.get("transitions_covered", []):
            if tr_id not in tasks_by_transition:
                tasks_by_transition[tr_id] = []
            tasks_by_transition[tr_id].append(task_id)

    fsm_dir = fsm_index_path.parent
    primary_machine = index.get("primary_machine")

    steel_thread_transitions: list[str] = []
    non_steel_thread_transitions: list[str] = []

    for machine in index.get("machines", []):
        files = machine.get("files", {})
        transitions_path = fsm_dir / files.get("transitions", "")
        transitions = load_json(transitions_path)
        if not transitions:
            continue

        is_steel_thread = machine["id"] == primary_machine or machine.get("level") == "steel_thread"

        for trans in transitions.get("transitions", []):
            tr_id = trans["id"]
            if is_steel_thread:
                steel_thread_transitions.append(tr_id)
            else:
                non_steel_thread_transitions.append(tr_id)

    steel_thread_covered = [t for t in steel_thread_transitions if t in tasks_by_transition]
    steel_thread_uncovered = [t for t in steel_thread_transitions if t not in tasks_by_transition]
    non_steel_covered = [t for t in non_steel_thread_transitions if t in tasks_by_transition]
    non_steel_uncovered = [t for t in non_steel_thread_transitions if t not in tasks_by_transition]

    steel_coverage = len(steel_thread_covered) / len(steel_thread_transitions) if steel_thread_transitions else 1.0
    non_steel_coverage = len(non_steel_covered) / len(non_steel_thread_transitions) if non_steel_thread_transitions else 1.0

    if steel_coverage < steel_thread_threshold:
        result.add_issue(
            "TASK_COVERAGE",
            f"Steel thread coverage {steel_coverage:.1%} below required {steel_thread_threshold:.0%}",
            {
                "required": steel_thread_threshold,
                "actual": steel_coverage,
                "uncovered": steel_thread_uncovered
            }
        )

    if non_steel_coverage < non_steel_thread_threshold:
        result.add_issue(
            "TASK_COVERAGE",
            f"Non-steel-thread coverage {non_steel_coverage:.1%} below required {non_steel_thread_threshold:.0%}",
            {
                "required": non_steel_thread_threshold,
                "actual": non_steel_coverage,
                "uncovered": non_steel_uncovered
            }
        )

    return result


def generate_coverage_report(
    fsm_index_path: Path,
    tasks_dir: Path,
    output_path: Path | None = None,
    phase: str = "plan"
) -> dict:
    """
    Generate FSM coverage report for /plan or /execute phase.

    Output: fsm-coverage.plan.json or fsm-coverage.execute.json
    """
    index = load_json(fsm_index_path)
    if not index:
        return {"error": f"Cannot load FSM index from {fsm_index_path}"}

    task_files = list(tasks_dir.glob("T*.json"))

    tasks_by_transition: dict[str, list[str]] = {}
    tasks_by_guard: dict[str, list[str]] = {}

    for task_file in task_files:
        task = load_json(task_file)
        if not task:
            continue
        task_id = task.get("id", task_file.stem)
        state_machine = task.get("state_machine", {})

        for tr_id in state_machine.get("transitions_covered", []):
            if tr_id not in tasks_by_transition:
                tasks_by_transition[tr_id] = []
            tasks_by_transition[tr_id].append(task_id)

        for inv_id in state_machine.get("guards_enforced", []):
            if inv_id not in tasks_by_guard:
                tasks_by_guard[inv_id] = []
            tasks_by_guard[inv_id].append(task_id)

    fsm_dir = fsm_index_path.parent
    machines_report = []

    for machine in index.get("machines", []):
        files = machine.get("files", {})
        transitions_path = fsm_dir / files.get("transitions", "")
        transitions_data = load_json(transitions_path)

        transitions_report = []
        if transitions_data:
            for trans in transitions_data.get("transitions", []):
                tr_id = trans["id"]
                covering_tasks = tasks_by_transition.get(tr_id, [])
                transitions_report.append({
                    "id": tr_id,
                    "trigger": trans.get("trigger"),
                    "covered": len(covering_tasks) > 0,
                    "covering_tasks": covering_tasks
                })

        covered_count = sum(1 for t in transitions_report if t["covered"])
        total_count = len(transitions_report)

        machines_report.append({
            "machine_id": machine["id"],
            "name": machine.get("name"),
            "level": machine.get("level"),
            "transitions": {
                "total": total_count,
                "covered": covered_count,
                "coverage_pct": (covered_count / total_count * 100) if total_count else 100,
                "details": transitions_report
            }
        })

    invariants = index.get("invariants", [])
    invariants_report = []
    for inv in invariants:
        inv_id = inv["id"]
        enforcing_tasks = tasks_by_guard.get(inv_id, [])
        invariants_report.append({
            "id": inv_id,
            "rule": inv.get("rule"),
            "enforced": len(enforcing_tasks) > 0,
            "enforcing_tasks": enforcing_tasks
        })

    report = {
        "version": "1.0",
        "phase": phase,
        "spec_slug": index.get("spec_slug"),
        "machines": machines_report,
        "invariants": {
            "total": len(invariants_report),
            "enforced": sum(1 for i in invariants_report if i["enforced"]),
            "details": invariants_report
        },
        "summary": {
            "total_transitions": sum(m["transitions"]["total"] for m in machines_report),
            "covered_transitions": sum(m["transitions"]["covered"] for m in machines_report),
            "total_invariants": len(invariants_report),
            "enforced_invariants": sum(1 for i in invariants_report if i["enforced"])
        }
    }

    if output_path:
        with open(output_path, "w") as f:
            json.dump(report, f, indent=2)

    return report


def generate_execute_coverage_report(
    fsm_index_path: Path,
    bundles_dir: Path,
    output_path: Path | None = None
) -> dict:
    """
    Generate FSM coverage report for /execute phase.

    Includes evidence from verification results in bundle result files.
    Output: fsm-coverage.execute.json
    """
    index = load_json(fsm_index_path)
    if not index:
        return {"error": f"Cannot load FSM index from {fsm_index_path}"}

    result_files = list(bundles_dir.glob("T*-result.json"))

    transitions_evidence: dict[str, list[dict]] = {}
    guards_evidence: dict[str, list[dict]] = {}

    for result_file in result_files:
        result = load_json(result_file)
        if not result:
            continue

        task_id = result.get("task_id")
        verification = result.get("verification", {})
        fsm_adherence = verification.get("fsm_adherence", {})

        for tr_info in fsm_adherence.get("transitions_verified", []):
            if isinstance(tr_info, dict):
                tr_id = tr_info.get("id")
                if tr_id:
                    if tr_id not in transitions_evidence:
                        transitions_evidence[tr_id] = []
                    transitions_evidence[tr_id].append({
                        "task_id": task_id,
                        "evidence_type": tr_info.get("evidence_type"),
                        "evidence": tr_info.get("evidence")
                    })
            elif isinstance(tr_info, str):
                if tr_info not in transitions_evidence:
                    transitions_evidence[tr_info] = []
                transitions_evidence[tr_info].append({
                    "task_id": task_id,
                    "evidence_type": "unknown",
                    "evidence": "Verified (legacy format)"
                })

        for guard_info in fsm_adherence.get("guards_verified", []):
            if isinstance(guard_info, dict):
                guard_id = guard_info.get("id")
                if guard_id:
                    if guard_id not in guards_evidence:
                        guards_evidence[guard_id] = []
                    guards_evidence[guard_id].append({
                        "task_id": task_id,
                        "evidence_type": guard_info.get("evidence_type"),
                        "evidence": guard_info.get("evidence")
                    })
            elif isinstance(guard_info, str):
                if guard_info not in guards_evidence:
                    guards_evidence[guard_info] = []
                guards_evidence[guard_info].append({
                    "task_id": task_id,
                    "evidence_type": "unknown",
                    "evidence": "Verified (legacy format)"
                })

    fsm_dir = fsm_index_path.parent
    machines_report = []

    for machine in index.get("machines", []):
        files = machine.get("files", {})
        transitions_path = fsm_dir / files.get("transitions", "")
        transitions_data = load_json(transitions_path)

        transitions_report = []
        if transitions_data:
            for trans in transitions_data.get("transitions", []):
                tr_id = trans["id"]
                evidence_list = transitions_evidence.get(tr_id, [])
                transitions_report.append({
                    "id": tr_id,
                    "trigger": trans.get("trigger"),
                    "verified": len(evidence_list) > 0,
                    "evidence": evidence_list
                })

        verified_count = sum(1 for t in transitions_report if t["verified"])
        total_count = len(transitions_report)

        machines_report.append({
            "machine_id": machine["id"],
            "name": machine.get("name"),
            "level": machine.get("level"),
            "transitions": {
                "total": total_count,
                "verified": verified_count,
                "verification_pct": (verified_count / total_count * 100) if total_count else 100,
                "details": transitions_report
            }
        })

    invariants = index.get("invariants", [])
    invariants_report = []
    for inv in invariants:
        inv_id = inv["id"]
        evidence_list = guards_evidence.get(inv_id, [])
        invariants_report.append({
            "id": inv_id,
            "rule": inv.get("rule"),
            "verified": len(evidence_list) > 0,
            "evidence": evidence_list
        })

    report = {
        "version": "1.0",
        "phase": "execute",
        "spec_slug": index.get("spec_slug"),
        "machines": machines_report,
        "invariants": {
            "total": len(invariants_report),
            "verified": sum(1 for i in invariants_report if i["verified"]),
            "details": invariants_report
        },
        "summary": {
            "total_transitions": sum(m["transitions"]["total"] for m in machines_report),
            "verified_transitions": sum(m["transitions"]["verified"] for m in machines_report),
            "total_invariants": len(invariants_report),
            "verified_invariants": sum(1 for i in invariants_report if i["verified"])
        }
    }

    if output_path:
        with open(output_path, "w") as f:
            json.dump(report, f, indent=2)

    return report


def cmd_validate(args: argparse.Namespace) -> int:
    fsm_dir = Path(args.fsm_dir)
    if not fsm_dir.is_dir():
        print(f"Error: {fsm_dir} is not a directory", file=sys.stderr)
        return 1

    result = validate_fsm_directory(fsm_dir)

    output = result.to_dict()
    print(json.dumps(output, indent=2))

    return 0 if result.passed else 1


def cmd_completeness(args: argparse.Namespace) -> int:
    states_path = Path(args.states_file)
    transitions_path = Path(args.transitions_file)

    states = load_json(states_path)
    transitions = load_json(transitions_path)

    if not states:
        print(f"Error: Cannot load states from {states_path}", file=sys.stderr)
        return 1
    if not transitions:
        print(f"Error: Cannot load transitions from {transitions_path}", file=sys.stderr)
        return 1

    result = ValidationResult()
    validate_i3_completeness(states, transitions, result)
    validate_i4_guard_linkage(transitions, result)

    output = result.to_dict()
    print(json.dumps(output, indent=2))

    return 0 if result.passed else 1


def cmd_coverage(args: argparse.Namespace) -> int:
    fsm_index = Path(args.fsm_index)
    cap_map = Path(args.capability_map)

    result = validate_coverage(fsm_index, cap_map)

    output = result.to_dict()
    print(json.dumps(output, indent=2))

    return 0 if result.passed else 1


def cmd_task_coverage(args: argparse.Namespace) -> int:
    """Validate task coverage of FSM transitions (hard planning gate)."""
    fsm_index = Path(args.fsm_index)
    tasks_dir = Path(args.tasks_dir)

    steel_threshold = args.steel_threshold if hasattr(args, 'steel_threshold') else 1.0
    other_threshold = args.other_threshold if hasattr(args, 'other_threshold') else 0.9

    result = validate_task_coverage(
        fsm_index,
        tasks_dir,
        steel_thread_threshold=steel_threshold,
        non_steel_thread_threshold=other_threshold
    )

    output = result.to_dict()
    print(json.dumps(output, indent=2))

    return 0 if result.passed else 1


def cmd_coverage_report(args: argparse.Namespace) -> int:
    """Generate FSM coverage report for plan phase."""
    fsm_index = Path(args.fsm_index)
    tasks_dir = Path(args.tasks_dir)
    output_path = Path(args.output) if args.output else None

    report = generate_coverage_report(fsm_index, tasks_dir, output_path, phase="plan")

    if not output_path:
        print(json.dumps(report, indent=2))
    else:
        print(f"Coverage report written to: {output_path}")

    return 0


def cmd_execute_coverage_report(args: argparse.Namespace) -> int:
    """Generate FSM coverage report for execute phase with verification evidence."""
    fsm_index = Path(args.fsm_index)
    bundles_dir = Path(args.bundles_dir)
    output_path = Path(args.output) if args.output else None

    report = generate_execute_coverage_report(fsm_index, bundles_dir, output_path)

    if not output_path:
        print(json.dumps(report, indent=2))
    else:
        print(f"Execute coverage report written to: {output_path}")

    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Validate FSM artifacts against I1-I5 invariants"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    validate_parser = subparsers.add_parser(
        "validate",
        help="Validate all FSM artifacts in a directory"
    )
    validate_parser.add_argument("fsm_dir", help="Path to FSM directory")
    validate_parser.set_defaults(func=cmd_validate)

    completeness_parser = subparsers.add_parser(
        "completeness",
        help="Validate completeness of a single machine"
    )
    completeness_parser.add_argument("states_file", help="Path to states JSON")
    completeness_parser.add_argument("transitions_file", help="Path to transitions JSON")
    completeness_parser.set_defaults(func=cmd_completeness)

    coverage_parser = subparsers.add_parser(
        "coverage",
        help="Validate transition coverage against capability map"
    )
    coverage_parser.add_argument("fsm_index", help="Path to FSM index.json")
    coverage_parser.add_argument("capability_map", help="Path to capability-map.json")
    coverage_parser.set_defaults(func=cmd_coverage)

    task_coverage_parser = subparsers.add_parser(
        "task-coverage",
        help="Validate task coverage of FSM transitions (HARD PLANNING GATE)"
    )
    task_coverage_parser.add_argument("fsm_index", help="Path to FSM index.json")
    task_coverage_parser.add_argument("tasks_dir", help="Path to tasks directory")
    task_coverage_parser.add_argument(
        "--steel-threshold",
        type=float,
        default=1.0,
        help="Coverage threshold for steel-thread transitions (default: 1.0 = 100%%)"
    )
    task_coverage_parser.add_argument(
        "--other-threshold",
        type=float,
        default=0.9,
        help="Coverage threshold for non-steel-thread transitions (default: 0.9 = 90%%)"
    )
    task_coverage_parser.set_defaults(func=cmd_task_coverage)

    report_parser = subparsers.add_parser(
        "coverage-report",
        help="Generate FSM coverage report for plan phase"
    )
    report_parser.add_argument("fsm_index", help="Path to FSM index.json")
    report_parser.add_argument("tasks_dir", help="Path to tasks directory")
    report_parser.add_argument(
        "--output", "-o",
        help="Output file path (default: stdout)"
    )
    report_parser.set_defaults(func=cmd_coverage_report)

    exec_report_parser = subparsers.add_parser(
        "execute-coverage-report",
        help="Generate FSM coverage report for execute phase with verification evidence"
    )
    exec_report_parser.add_argument("fsm_index", help="Path to FSM index.json")
    exec_report_parser.add_argument("bundles_dir", help="Path to bundles directory")
    exec_report_parser.add_argument(
        "--output", "-o",
        help="Output file path (default: stdout)"
    )
    exec_report_parser.set_defaults(func=cmd_execute_coverage_report)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
