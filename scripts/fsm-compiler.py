#!/usr/bin/env python3
"""
FSM Compiler

Compiles Finite State Machine artifacts from spec sections.
Derives states from workflow steps, transitions from variants/failures,
and links guards to invariants.

Usage:
    python3 fsm-compiler.py compile <spec_json> --output-dir <dir>
    python3 fsm-compiler.py from-capability-map <capability_map> <spec_md> --output-dir <dir>

Input: Structured spec data with workflows, invariants, interfaces
Output: FSM artifacts (index.json, states.json, transitions.json)
"""

import sys
import json
import argparse
import hashlib
import re
from pathlib import Path
from datetime import datetime, timezone
from dataclasses import dataclass, field, asdict
from typing import Any


@dataclass
class SpecRef:
    quote: str
    location: str = ""


@dataclass
class Guard:
    condition: str
    invariant_id: str = ""
    negated: bool = False


@dataclass
class State:
    id: str
    name: str
    type: str  # initial, normal, success, failure
    description: str = ""
    spec_ref: SpecRef | None = None
    invariants: list[str] = field(default_factory=list)
    behaviors: list[str] = field(default_factory=list)


@dataclass
class Transition:
    id: str
    from_state: str
    to_state: str
    trigger: str
    guards: list[Guard] = field(default_factory=list)
    behaviors: list[str] = field(default_factory=list)
    spec_ref: SpecRef | None = None
    is_failure_path: bool = False


@dataclass
class Machine:
    id: str
    name: str
    level: str  # steel_thread, domain, entity
    states: list[State] = field(default_factory=list)
    transitions: list[Transition] = field(default_factory=list)
    trigger_reason: str = "mandatory"
    parent_machine: str = ""


class FSMCompiler:
    def __init__(self):
        self.state_counter = 0
        self.transition_counter = 0
        self.machine_counter = 0
        self.machines: list[Machine] = []
        self.invariants: list[dict] = []

    def _next_state_id(self) -> str:
        self.state_counter += 1
        return f"S{self.state_counter}"

    def _next_transition_id(self) -> str:
        self.transition_counter += 1
        return f"TR{self.transition_counter}"

    def _next_machine_id(self) -> str:
        self.machine_counter += 1
        return f"M{self.machine_counter}"

    def compile_from_workflows(
        self,
        workflows: list[dict],
        invariants: list[dict],
        flows: list[dict] | None = None
    ) -> Machine:
        """Compile a state machine from workflow definitions."""
        self.invariants = invariants

        steel_thread_workflow = None
        for wf in workflows:
            if wf.get("is_steel_thread", False):
                steel_thread_workflow = wf
                break

        if not steel_thread_workflow and workflows:
            steel_thread_workflow = workflows[0]

        if not steel_thread_workflow:
            raise ValueError("No workflow found to compile")

        machine = self._compile_workflow(steel_thread_workflow, level="steel_thread")
        self.machines.append(machine)

        if len(machine.states) > 12:
            pass

        return machine

    def _compile_workflow(self, workflow: dict, level: str = "steel_thread") -> Machine:
        """Compile a single workflow into a state machine."""
        machine_id = self._next_machine_id()
        machine_name = workflow.get("name", "Unnamed Workflow")

        states: list[State] = []
        transitions: list[Transition] = []

        steps = workflow.get("steps", [])
        if not steps:
            raise ValueError(f"Workflow '{machine_name}' has no steps")

        initial_state = State(
            id=self._next_state_id(),
            name=f"Awaiting {workflow.get('trigger', 'start')}",
            type="initial",
            description="Initial state before workflow begins",
            spec_ref=SpecRef(
                quote=workflow.get("trigger", "Workflow start"),
                location=f"Workflow: {machine_name}"
            )
        )
        states.append(initial_state)

        prev_state = initial_state
        for i, step in enumerate(steps):
            step_name = step.get("name", step.get("action", f"Step {i+1}"))
            postcondition = step.get("postcondition", f"Completed: {step_name}")

            state = State(
                id=self._next_state_id(),
                name=postcondition,
                type="normal",
                description=step.get("description", ""),
                spec_ref=SpecRef(
                    quote=step.get("description", step_name),
                    location=f"Workflow: {machine_name}, Step {i+1}"
                ),
                behaviors=step.get("behaviors", [])
            )
            states.append(state)

            transition = Transition(
                id=self._next_transition_id(),
                from_state=prev_state.id,
                to_state=state.id,
                trigger=step_name,
                behaviors=step.get("behaviors", []),
                spec_ref=SpecRef(
                    quote=step.get("description", step_name),
                    location=f"Workflow: {machine_name}, Step {i+1}"
                )
            )
            transitions.append(transition)

            prev_state = state

        postconditions = workflow.get("postconditions", ["Workflow completed successfully"])
        success_state = State(
            id=self._next_state_id(),
            name=postconditions[0] if postconditions else "Success",
            type="success",
            description="Workflow completed successfully",
            spec_ref=SpecRef(
                quote=postconditions[0] if postconditions else "Workflow complete",
                location=f"Workflow: {machine_name}, Postconditions"
            )
        )
        states.append(success_state)

        completion_transition = Transition(
            id=self._next_transition_id(),
            from_state=prev_state.id,
            to_state=success_state.id,
            trigger="Complete workflow",
            spec_ref=SpecRef(
                quote="Workflow completion",
                location=f"Workflow: {machine_name}"
            )
        )
        transitions.append(completion_transition)

        for variant in workflow.get("variants", []):
            self._add_variant_transitions(
                variant, states, transitions, machine_name
            )

        for failure in workflow.get("failures", []):
            self._add_failure_transitions(
                failure, states, transitions, machine_name
            )

        return Machine(
            id=machine_id,
            name=machine_name,
            level=level,
            states=states,
            transitions=transitions,
            trigger_reason="mandatory" if level == "steel_thread" else "complexity"
        )

    def _add_variant_transitions(
        self,
        variant: dict,
        states: list[State],
        transitions: list[Transition],
        machine_name: str
    ) -> None:
        """Add transitions for workflow variants (conditional branches)."""
        condition = variant.get("condition", "")
        outcome = variant.get("outcome", "")
        from_step = variant.get("from_step")

        from_state = None
        if from_step is not None and from_step < len(states):
            from_state = states[from_step]
        else:
            for s in states:
                if s.type == "normal":
                    from_state = s
                    break

        if not from_state:
            return

        to_state = None
        for s in states:
            if outcome.lower() in s.name.lower():
                to_state = s
                break

        if not to_state:
            to_state = State(
                id=self._next_state_id(),
                name=outcome or f"Variant: {condition}",
                type="normal",
                description=f"Alternative path when: {condition}",
                spec_ref=SpecRef(
                    quote=f"If {condition}, then {outcome}",
                    location=f"Workflow: {machine_name}, Variants"
                )
            )
            states.append(to_state)

        guards = self._extract_guards(condition)

        transition = Transition(
            id=self._next_transition_id(),
            from_state=from_state.id,
            to_state=to_state.id,
            trigger=condition,
            guards=guards,
            spec_ref=SpecRef(
                quote=f"If {condition}, then {outcome}",
                location=f"Workflow: {machine_name}, Variants"
            )
        )
        transitions.append(transition)

    def _add_failure_transitions(
        self,
        failure: dict,
        states: list[State],
        transitions: list[Transition],
        machine_name: str
    ) -> None:
        """Add transitions for failure conditions."""
        condition = failure.get("condition", "")
        outcome = failure.get("outcome", "Error")
        from_step = failure.get("from_step")

        failure_state = None
        for s in states:
            if s.type == "failure" and outcome.lower() in s.name.lower():
                failure_state = s
                break

        if not failure_state:
            failure_state = State(
                id=self._next_state_id(),
                name=outcome,
                type="failure",
                description=f"Failure state: {condition}",
                spec_ref=SpecRef(
                    quote=f"If {condition}, then {outcome}",
                    location=f"Workflow: {machine_name}, Failures"
                )
            )
            states.append(failure_state)

        from_states = []
        if from_step is not None and from_step < len(states):
            from_states = [states[from_step]]
        else:
            from_states = [s for s in states if s.type in ("initial", "normal")]

        for from_state in from_states:
            existing = any(
                t.from_state == from_state.id and t.to_state == failure_state.id
                for t in transitions
            )
            if existing:
                continue

            guards = self._extract_guards(condition)

            transition = Transition(
                id=self._next_transition_id(),
                from_state=from_state.id,
                to_state=failure_state.id,
                trigger=condition,
                guards=guards,
                is_failure_path=True,
                spec_ref=SpecRef(
                    quote=f"If {condition}, then {outcome}",
                    location=f"Workflow: {machine_name}, Failures"
                )
            )
            transitions.append(transition)

    def _extract_guards(self, condition: str) -> list[Guard]:
        """Extract guards from condition text, linking to invariants."""
        guards = []

        for inv in self.invariants:
            inv_text = inv.get("rule", "").lower()
            if not inv_text:
                continue

            keywords = re.findall(r'\b\w+\b', inv_text)
            condition_lower = condition.lower()

            matches = sum(1 for kw in keywords if kw in condition_lower and len(kw) > 3)
            if matches >= 2 or any(kw in condition_lower for kw in ["must", "valid", "invalid", "require"]):
                guards.append(Guard(
                    condition=condition,
                    invariant_id=inv.get("id", "")
                ))
                break

        if not guards:
            guards.append(Guard(condition=condition))

        return guards

    def compile_from_capability_map(
        self,
        cap_map: dict,
        spec_text: str = ""
    ) -> Machine:
        """Compile FSM from capability map flows."""
        flows = cap_map.get("flows", [])

        steel_thread_flow = None
        for flow in flows:
            if flow.get("is_steel_thread", False):
                steel_thread_flow = flow
                break

        if not steel_thread_flow and flows:
            steel_thread_flow = flows[0]

        if not steel_thread_flow:
            raise ValueError("No flow found in capability map")

        workflow = self._flow_to_workflow(steel_thread_flow, cap_map)

        invariants = []
        for domain in cap_map.get("domains", []):
            for cap in domain.get("capabilities", []):
                if "invariants" in cap:
                    invariants.extend(cap["invariants"])

        return self.compile_from_workflows([workflow], invariants, flows)

    def _flow_to_workflow(self, flow: dict, cap_map: dict) -> dict:
        """Convert a capability map flow to workflow format."""
        behavior_lookup = {}
        for domain in cap_map.get("domains", []):
            for cap in domain.get("capabilities", []):
                for beh in cap.get("behaviors", []):
                    behavior_lookup[beh["id"]] = beh

        steps = []
        for step in flow.get("steps", []):
            beh_id = step.get("behavior_id", "")
            beh = behavior_lookup.get(beh_id, {})

            steps.append({
                "name": beh.get("name", step.get("description", f"Step {len(steps)+1}")),
                "action": beh.get("name", ""),
                "description": step.get("description", beh.get("description", "")),
                "postcondition": f"Completed: {beh.get('name', 'step')}",
                "behaviors": [beh_id] if beh_id else []
            })

        return {
            "name": flow.get("name", "Steel Thread"),
            "trigger": "User initiates flow",
            "is_steel_thread": flow.get("is_steel_thread", False),
            "steps": steps,
            "postconditions": [f"{flow.get('name', 'Flow')} completed successfully"],
            "variants": [],
            "failures": []
        }

    def export(self, output_dir: Path, spec_slug: str, spec_checksum: str) -> dict:
        """Export all compiled machines to files."""
        output_dir.mkdir(parents=True, exist_ok=True)

        primary_machine = None
        for m in self.machines:
            if m.level == "steel_thread":
                primary_machine = m.id
                break

        if not primary_machine and self.machines:
            primary_machine = self.machines[0].id

        machine_entries = []
        hierarchy = {}

        for machine in self.machines:
            machine_slug = machine.name.lower().replace(" ", "-")
            if machine.level == "steel_thread":
                machine_slug = "steel-thread"

            files = {
                "states": f"{machine_slug}.states.json",
                "transitions": f"{machine_slug}.transitions.json",
                "diagram": f"{machine_slug}.mmd",
                "notes": f"{machine_slug}.notes.md"
            }

            self._export_states(machine, output_dir / files["states"])
            self._export_transitions(machine, output_dir / files["transitions"])

            machine_entries.append({
                "id": machine.id,
                "name": machine.name,
                "level": machine.level,
                "trigger_reason": machine.trigger_reason,
                "files": files
            })

            if machine.parent_machine:
                if machine.parent_machine not in hierarchy:
                    hierarchy[machine.parent_machine] = []
                hierarchy[machine.parent_machine].append(machine.id)

        index = {
            "version": "1.0",
            "spec_slug": spec_slug,
            "spec_checksum": spec_checksum,
            "created_at": datetime.now(timezone.utc).isoformat(),
            "primary_machine": primary_machine,
            "machines": machine_entries,
            "hierarchy": hierarchy,
            "invariants": self.invariants
        }

        with open(output_dir / "index.json", "w") as f:
            json.dump(index, f, indent=2)

        return index

    def _export_states(self, machine: Machine, path: Path) -> None:
        """Export states to JSON file."""
        initial_state = next((s.id for s in machine.states if s.type == "initial"), None)
        terminal_states = [s.id for s in machine.states if s.type in ("success", "failure")]

        states_data = {
            "version": "1.0",
            "machine_id": machine.id,
            "initial_state": initial_state,
            "terminal_states": terminal_states,
            "states": []
        }

        for state in machine.states:
            state_dict = {
                "id": state.id,
                "name": state.name,
                "type": state.type,
                "description": state.description,
                "invariants": state.invariants,
                "behaviors": state.behaviors
            }
            if state.spec_ref:
                state_dict["spec_ref"] = {
                    "quote": state.spec_ref.quote,
                    "location": state.spec_ref.location
                }
            states_data["states"].append(state_dict)

        with open(path, "w") as f:
            json.dump(states_data, f, indent=2)

    def _export_transitions(self, machine: Machine, path: Path) -> None:
        """Export transitions to JSON file."""
        transitions_data = {
            "version": "1.0",
            "machine_id": machine.id,
            "transitions": [],
            "guards_index": {}
        }

        guards_index: dict[str, list[str]] = {}

        for trans in machine.transitions:
            trans_dict = {
                "id": trans.id,
                "from_state": trans.from_state,
                "to_state": trans.to_state,
                "trigger": trans.trigger,
                "guards": [],
                "behaviors": trans.behaviors,
                "is_failure_path": trans.is_failure_path
            }

            for guard in trans.guards:
                guard_dict = {
                    "condition": guard.condition,
                    "invariant_id": guard.invariant_id,
                    "negated": guard.negated
                }
                trans_dict["guards"].append(guard_dict)

                if guard.invariant_id:
                    if guard.invariant_id not in guards_index:
                        guards_index[guard.invariant_id] = []
                    guards_index[guard.invariant_id].append(trans.id)

            if trans.spec_ref:
                trans_dict["spec_ref"] = {
                    "quote": trans.spec_ref.quote,
                    "location": trans.spec_ref.location
                }

            transitions_data["transitions"].append(trans_dict)

        transitions_data["guards_index"] = guards_index

        with open(path, "w") as f:
            json.dump(transitions_data, f, indent=2)


def compute_checksum(content: str) -> str:
    """Compute SHA256 checksum (first 16 chars)."""
    return hashlib.sha256(content.encode()).hexdigest()[:16]


def cmd_compile(args: argparse.Namespace) -> int:
    spec_path = Path(args.spec_json)
    output_dir = Path(args.output_dir)

    with open(spec_path) as f:
        spec_data = json.load(f)

    spec_slug = args.slug or spec_path.stem
    spec_checksum = compute_checksum(json.dumps(spec_data))

    compiler = FSMCompiler()
    compiler.compile_from_workflows(
        workflows=spec_data.get("workflows", []),
        invariants=spec_data.get("invariants", []),
        flows=spec_data.get("flows", [])
    )

    index = compiler.export(output_dir, spec_slug, spec_checksum)
    print(json.dumps({"status": "success", "index": index}, indent=2))
    return 0


def cmd_from_capability_map(args: argparse.Namespace) -> int:
    cap_map_path = Path(args.capability_map)
    spec_md_path = Path(args.spec_md) if args.spec_md else None
    output_dir = Path(args.output_dir)

    with open(cap_map_path) as f:
        cap_map = json.load(f)

    spec_text = ""
    if spec_md_path and spec_md_path.exists():
        with open(spec_md_path) as f:
            spec_text = f.read()

    spec_slug = args.slug or cap_map_path.stem.replace(".capabilities", "")
    spec_checksum = cap_map.get("spec_checksum", compute_checksum(json.dumps(cap_map)))

    compiler = FSMCompiler()
    compiler.compile_from_capability_map(cap_map, spec_text)

    index = compiler.export(output_dir, spec_slug, spec_checksum)
    print(json.dumps({"status": "success", "index": index}, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Compile FSM artifacts from spec sections"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    compile_parser = subparsers.add_parser(
        "compile",
        help="Compile FSM from structured spec JSON"
    )
    compile_parser.add_argument("spec_json", help="Path to spec JSON with workflows")
    compile_parser.add_argument("--output-dir", required=True, help="Output directory")
    compile_parser.add_argument("--slug", help="Spec slug (default: filename)")
    compile_parser.set_defaults(func=cmd_compile)

    cap_map_parser = subparsers.add_parser(
        "from-capability-map",
        help="Compile FSM from capability map flows"
    )
    cap_map_parser.add_argument("capability_map", help="Path to capability-map.json")
    cap_map_parser.add_argument("spec_md", nargs="?", help="Path to spec markdown (optional)")
    cap_map_parser.add_argument("--output-dir", required=True, help="Output directory")
    cap_map_parser.add_argument("--slug", help="Spec slug (default: from filename)")
    cap_map_parser.set_defaults(func=cmd_from_capability_map)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
