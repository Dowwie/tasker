#!/usr/bin/env python3
"""
FSM Mermaid Generator

Generates Mermaid stateDiagram-v2 diagrams from canonical FSM JSON files.

Usage:
    python3 fsm-mermaid.py generate <states_json> <transitions_json> [--output <file>]
    python3 fsm-mermaid.py generate-all <fsm_dir> [--output-dir <dir>]

Output: Mermaid markdown files (.mmd) for rendering state diagrams.
"""

import sys
import json
import argparse
import re
from pathlib import Path


def sanitize_name(name: str) -> str:
    """Sanitize state name for Mermaid compatibility."""
    sanitized = re.sub(r'[^a-zA-Z0-9_]', '_', name)
    sanitized = re.sub(r'_+', '_', sanitized)
    sanitized = sanitized.strip('_')
    if sanitized and sanitized[0].isdigit():
        sanitized = f"s_{sanitized}"
    return sanitized[:40] or "unnamed"


def truncate_text(text: str, max_len: int = 30) -> str:
    """Truncate text for diagram labels."""
    if len(text) <= max_len:
        return text
    return text[:max_len - 3] + "..."


def escape_mermaid(text: str) -> str:
    """Escape special characters for Mermaid labels."""
    text = text.replace('"', "'")
    text = text.replace('\n', ' ')
    return text


def generate_mermaid(states_data: dict, transitions_data: dict) -> str:
    """Generate Mermaid stateDiagram-v2 from states and transitions JSON."""
    lines = ["stateDiagram-v2"]
    lines.append("    direction TB")
    lines.append("")

    state_lookup = {s["id"]: s for s in states_data.get("states", [])}
    initial_state = states_data.get("initial_state")
    terminal_states = set(states_data.get("terminal_states", []))

    if initial_state:
        lines.append(f"    [*] --> {initial_state}")
        lines.append("")

    lines.append("    %% State definitions")
    for state in states_data.get("states", []):
        sid = state["id"]
        name = sanitize_name(state["name"])
        state_type = state.get("type", "normal")

        if state_type == "failure":
            lines.append(f"    state \"{escape_mermaid(state['name'])}\" as {sid}")
            lines.append(f"    {sid}:::failure")
        elif state_type == "success":
            lines.append(f"    state \"{escape_mermaid(state['name'])}\" as {sid}")
            lines.append(f"    {sid}:::success")
        elif state_type == "initial":
            lines.append(f"    state \"{escape_mermaid(state['name'])}\" as {sid}")
            lines.append(f"    {sid}:::initial")
        else:
            lines.append(f"    state \"{escape_mermaid(state['name'])}\" as {sid}")

    lines.append("")
    lines.append("    %% Transitions")

    for trans in transitions_data.get("transitions", []):
        from_state = trans["from_state"]
        to_state = trans["to_state"]
        trigger = truncate_text(escape_mermaid(trans["trigger"]), 25)

        guards = trans.get("guards", [])
        if guards:
            guard_conditions = [truncate_text(g["condition"], 15) for g in guards[:2]]
            guard_text = " & ".join(guard_conditions)
            label = f"{trigger} [{guard_text}]"
        else:
            label = trigger

        if trans.get("is_failure_path", False):
            lines.append(f"    {from_state} --> {to_state}: {label}")
        else:
            lines.append(f"    {from_state} --> {to_state}: {label}")

    lines.append("")
    lines.append("    %% Terminal state connections")
    for state in states_data.get("states", []):
        if state["type"] in ("success", "failure"):
            lines.append(f"    {state['id']} --> [*]")

    lines.append("")
    lines.append("    %% Styling")
    lines.append("    classDef initial fill:#e1f5fe,stroke:#01579b")
    lines.append("    classDef success fill:#e8f5e9,stroke:#2e7d32")
    lines.append("    classDef failure fill:#ffebee,stroke:#c62828")

    return "\n".join(lines)


def generate_notes(states_data: dict, transitions_data: dict, machine_name: str = "") -> str:
    """Generate markdown notes file for a state machine."""
    lines = [f"# FSM Notes: {machine_name or 'State Machine'}"]
    lines.append("")
    lines.append("## States")
    lines.append("")

    for state in states_data.get("states", []):
        state_type = state.get("type", "normal")
        type_badge = f"[{state_type.upper()}]" if state_type != "normal" else ""
        lines.append(f"### {state['id']}: {state['name']} {type_badge}")

        if state.get("description"):
            lines.append(f"\n{state['description']}")

        if state.get("spec_ref"):
            ref = state["spec_ref"]
            lines.append(f"\n**Source:** \"{ref.get('quote', '')}\" ({ref.get('location', '')})")

        if state.get("invariants"):
            lines.append(f"\n**Invariants:** {', '.join(state['invariants'])}")

        if state.get("behaviors"):
            lines.append(f"\n**Behaviors:** {', '.join(state['behaviors'])}")

        lines.append("")

    lines.append("## Transitions")
    lines.append("")

    for trans in transitions_data.get("transitions", []):
        failure_badge = "[FAILURE]" if trans.get("is_failure_path") else ""
        lines.append(f"### {trans['id']}: {trans['from_state']} → {trans['to_state']} {failure_badge}")
        lines.append(f"\n**Trigger:** {trans['trigger']}")

        guards = trans.get("guards", [])
        if guards:
            guard_list = [f"- {g['condition']}" + (f" (enforces {g['invariant_id']})" if g.get('invariant_id') else "") for g in guards]
            lines.append("\n**Guards:**")
            lines.extend(guard_list)

        if trans.get("behaviors"):
            lines.append(f"\n**Behaviors:** {', '.join(trans['behaviors'])}")

        if trans.get("spec_ref"):
            ref = trans["spec_ref"]
            lines.append(f"\n**Source:** \"{ref.get('quote', '')}\" ({ref.get('location', '')})")

        lines.append("")

    guards_index = transitions_data.get("guards_index", {})
    if guards_index:
        lines.append("## Guard-Invariant Index")
        lines.append("")
        for inv_id, trans_ids in guards_index.items():
            lines.append(f"- **{inv_id}**: used by {', '.join(trans_ids)}")
        lines.append("")

    return "\n".join(lines)


def cmd_generate(args: argparse.Namespace) -> int:
    states_path = Path(args.states_json)
    transitions_path = Path(args.transitions_json)

    with open(states_path) as f:
        states_data = json.load(f)
    with open(transitions_path) as f:
        transitions_data = json.load(f)

    mermaid = generate_mermaid(states_data, transitions_data)

    if args.output:
        with open(args.output, "w") as f:
            f.write(mermaid)
        print(f"Generated: {args.output}")
    else:
        print(mermaid)

    if args.notes:
        notes = generate_notes(states_data, transitions_data, args.machine_name or "")
        with open(args.notes, "w") as f:
            f.write(notes)
        print(f"Generated notes: {args.notes}")

    return 0


def cmd_generate_all(args: argparse.Namespace) -> int:
    fsm_dir = Path(args.fsm_dir)
    output_dir = Path(args.output_dir) if args.output_dir else fsm_dir

    index_path = fsm_dir / "index.json"
    if not index_path.exists():
        print(f"Error: No index.json found in {fsm_dir}", file=sys.stderr)
        return 1

    with open(index_path) as f:
        index = json.load(f)

    generated = []
    for machine in index.get("machines", []):
        files = machine.get("files", {})

        states_path = fsm_dir / files.get("states", "")
        transitions_path = fsm_dir / files.get("transitions", "")

        if not states_path.exists() or not transitions_path.exists():
            print(f"Warning: Missing files for machine {machine['id']}", file=sys.stderr)
            continue

        with open(states_path) as f:
            states_data = json.load(f)
        with open(transitions_path) as f:
            transitions_data = json.load(f)

        mermaid = generate_mermaid(states_data, transitions_data)
        diagram_path = output_dir / files.get("diagram", f"{machine['id']}.mmd")
        with open(diagram_path, "w") as f:
            f.write(mermaid)
        generated.append(str(diagram_path))

        notes_file = files.get("notes")
        if notes_file:
            notes = generate_notes(states_data, transitions_data, machine.get("name", ""))
            notes_path = output_dir / notes_file
            with open(notes_path, "w") as f:
                f.write(notes)
            generated.append(str(notes_path))

    print(json.dumps({"status": "success", "generated": generated}, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Generate Mermaid diagrams from FSM JSON files"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    gen_parser = subparsers.add_parser(
        "generate",
        help="Generate Mermaid diagram from states and transitions"
    )
    gen_parser.add_argument("states_json", help="Path to states JSON file")
    gen_parser.add_argument("transitions_json", help="Path to transitions JSON file")
    gen_parser.add_argument("--output", "-o", help="Output .mmd file (default: stdout)")
    gen_parser.add_argument("--notes", help="Also generate notes markdown file")
    gen_parser.add_argument("--machine-name", help="Machine name for notes header")
    gen_parser.set_defaults(func=cmd_generate)

    all_parser = subparsers.add_parser(
        "generate-all",
        help="Generate diagrams for all machines in FSM directory"
    )
    all_parser.add_argument("fsm_dir", help="Path to FSM directory with index.json")
    all_parser.add_argument("--output-dir", help="Output directory (default: same as input)")
    all_parser.set_defaults(func=cmd_generate_all)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
