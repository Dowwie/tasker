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

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
