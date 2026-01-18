#!/usr/bin/env python3
"""
Spec Development Session Manager

Manages discovery sessions for the /spec workflow.
Tracks phase progress and validates gate requirements.
"""

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path

DISCOVERY_FILE = ".claude/clarify-session.md"

# Legacy constants - use get_specs_dir/get_adrs_dir for target-aware paths
SPECS_DIR = "docs/specs"
ADRS_DIR = "docs/adrs"


def get_specs_dir(target_dir: str = None) -> Path:
    """Get specs directory for target project."""
    base = Path(target_dir) if target_dir else Path.cwd()
    return base / "docs" / "specs"


def get_adrs_dir(target_dir: str = None) -> Path:
    """Get ADRs directory for target project."""
    base = Path(target_dir) if target_dir else Path.cwd()
    return base / "docs" / "adrs"

PHASES = [
    "scope",
    "clarify",
    "synthesis",
    "architecture",
    "decisions",
    "gate",
    "spec_review",
    "export",
    "complete"
]


def init_session(topic: str, target_dir: str = None) -> dict:
    """Initialize a new discovery session."""
    Path(".claude").mkdir(exist_ok=True)

    session = {
        "topic": topic,
        "target_dir": target_dir or str(Path.cwd()),
        "started_at": datetime.now().isoformat(),
        "phase": "scope",
        "rounds": 0,
        "scope": {
            "goal": None,
            "non_goals": [],
            "done_means": []
        },
        "open_questions": {
            "blocking": [],
            "non_blocking": []
        },
        "decisions": [],
        "adrs": []
    }

    discovery_content = f"""# Discovery: {topic}
Started: {session['started_at']}

## Questions Asked

## Answers Received

## Emerging Requirements
"""

    Path(DISCOVERY_FILE).write_text(discovery_content)
    save_session_state(session)

    print(f"Session initialized for: {topic}")
    print(f"Discovery file: {DISCOVERY_FILE}")
    return session


def load_session_state() -> dict | None:
    """Load session state from .claude/spec-session.json"""
    state_file = Path(".claude/spec-session.json")
    if not state_file.exists():
        return None
    return json.loads(state_file.read_text())


def save_session_state(session: dict):
    """Save session state."""
    Path(".claude").mkdir(exist_ok=True)
    Path(".claude/spec-session.json").write_text(
        json.dumps(session, indent=2)
    )


def get_status() -> dict:
    """Get current session status."""
    session = load_session_state()
    if not session:
        return {"status": "no_session", "message": "No active session. Run `/specify` to start."}

    discovery_file = Path(DISCOVERY_FILE)
    discovery_exists = discovery_file.exists()

    discovery_rounds = 0
    if discovery_exists:
        content = discovery_file.read_text()
        discovery_rounds = len(re.findall(r"### Round \d+", content))

    # Use target_dir-aware paths for specs and ADRs
    target_dir = session.get("target_dir")
    specs_dir = get_specs_dir(target_dir)
    adrs_dir = get_adrs_dir(target_dir)

    specs_count = len(list(specs_dir.glob("*.md"))) if specs_dir.exists() else 0
    adrs_count = len(list(adrs_dir.glob("ADR-*.md"))) if adrs_dir.exists() else 0

    return {
        "status": "active",
        "topic": session.get("topic", "Unknown"),
        "target_dir": target_dir,
        "phase": session.get("phase", "scope"),
        "phase_index": PHASES.index(session.get("phase", "scope")),
        "total_phases": len(PHASES),
        "discovery_rounds": discovery_rounds,
        "open_questions": {
            "blocking": len(session.get("open_questions", {}).get("blocking", [])),
            "non_blocking": len(session.get("open_questions", {}).get("non_blocking", []))
        },
        "decisions": len(session.get("decisions", [])),
        "adrs": adrs_count,
        "specs": specs_count,
        "started_at": session.get("started_at")
    }


def advance_phase(session: dict) -> bool:
    """Advance to next phase if valid."""
    current_idx = PHASES.index(session["phase"])
    if current_idx >= len(PHASES) - 1:
        print("Already at final phase.")
        return False

    next_phase = PHASES[current_idx + 1]
    session["phase"] = next_phase
    save_session_state(session)
    print(f"Advanced to phase: {next_phase}")
    return True


def check_gate(session: dict) -> dict:
    """Check if handoff-ready gate passes."""
    issues = []

    if session.get("phase") not in ["gate", "decisions", "architecture"]:
        issues.append(f"Not ready for gate check. Current phase: {session.get('phase')}")

    blocking_qs = session.get("open_questions", {}).get("blocking", [])
    if blocking_qs:
        issues.append(f"Blocking Open Questions: {len(blocking_qs)}")
        for q in blocking_qs[:3]:
            issues.append(f"  - {q}")

    scope = session.get("scope", {})
    if not scope.get("goal"):
        issues.append("Missing: Goal not defined")
    if not scope.get("done_means"):
        issues.append("Missing: Done means not defined")

    discovery_file = Path(DISCOVERY_FILE)
    if not discovery_file.exists():
        issues.append("Missing: Discovery file not found")

    passed = len(issues) == 0
    return {
        "passed": passed,
        "issues": issues,
        "message": "Gate PASSED - ready for export" if passed else "Gate FAILED"
    }


def add_open_question(session: dict, question: str, blocking: bool):
    """Add an open question."""
    key = "blocking" if blocking else "non_blocking"
    if "open_questions" not in session:
        session["open_questions"] = {"blocking": [], "non_blocking": []}
    session["open_questions"][key].append(question)
    save_session_state(session)
    print(f"Added {'blocking' if blocking else 'non-blocking'} question: {question}")


def resolve_question(session: dict, question: str, resolution: str):
    """Resolve an open question."""
    for key in ["blocking", "non_blocking"]:
        questions = session.get("open_questions", {}).get(key, [])
        if question in questions:
            questions.remove(question)
            session["open_questions"][key] = questions
            if "resolved_questions" not in session:
                session["resolved_questions"] = []
            session["resolved_questions"].append({
                "question": question,
                "resolution": resolution,
                "resolved_at": datetime.now().isoformat()
            })
            save_session_state(session)
            print(f"Resolved: {question}")
            return True
    print(f"Question not found: {question}")
    return False


def add_decision(session: dict, decision: str, adr_id: str | None = None):
    """Add a decision."""
    if "decisions" not in session:
        session["decisions"] = []
    session["decisions"].append({
        "decision": decision,
        "adr_id": adr_id,
        "decided_at": datetime.now().isoformat()
    })
    if adr_id:
        if "adrs" not in session:
            session["adrs"] = []
        session["adrs"].append(adr_id)
    save_session_state(session)
    print(f"Added decision: {decision}" + (f" (ADR: {adr_id})" if adr_id else ""))


def get_next_adr_number(target_dir: str = None) -> int:
    """Get next available ADR number."""
    # Try to get target_dir from session if not provided
    if target_dir is None:
        session = load_session_state()
        if session:
            target_dir = session.get("target_dir")

    adrs_dir = get_adrs_dir(target_dir)
    if not adrs_dir.exists():
        return 1
    existing = list(adrs_dir.glob("ADR-*.md"))
    if not existing:
        return 1
    numbers = []
    for f in existing:
        match = re.match(r"ADR-(\d+)", f.name)
        if match:
            numbers.append(int(match.group(1)))
    return max(numbers) + 1 if numbers else 1


def print_status(status: dict):
    """Print formatted status."""
    if status["status"] == "no_session":
        print(status["message"])
        return

    phase_idx = status["phase_index"]
    total = status["total_phases"]
    progress = "=" * (phase_idx + 1) + "-" * (total - phase_idx - 1)

    print(f"\n## Spec Session Status")
    print(f"Topic: {status['topic']}")
    print(f"Target: {status.get('target_dir', 'current directory')}")
    print(f"Phase: {status['phase']} [{progress}] ({phase_idx + 1}/{total})")
    print(f"Discovery Rounds: {status['discovery_rounds']}")
    print(f"Open Questions: {status['open_questions']['blocking']} blocking, {status['open_questions']['non_blocking']} non-blocking")
    print(f"Decisions: {status['decisions']}")
    print(f"ADRs: {status['adrs']}")
    print(f"Started: {status['started_at']}")


def main():
    parser = argparse.ArgumentParser(description="Spec Session Manager")
    subparsers = parser.add_subparsers(dest="command", help="Commands")

    init_parser = subparsers.add_parser("init", help="Initialize new session")
    init_parser.add_argument("topic", help="Topic/title for the spec")
    init_parser.add_argument("--target-dir", dest="target_dir", help="Target project directory for output")

    subparsers.add_parser("status", help="Show session status")

    advance_parser = subparsers.add_parser("advance", help="Advance to next phase")

    subparsers.add_parser("gate", help="Check handoff-ready gate")

    question_parser = subparsers.add_parser("add-question", help="Add open question")
    question_parser.add_argument("question", help="The question")
    question_parser.add_argument("--blocking", action="store_true", help="Mark as blocking")

    resolve_parser = subparsers.add_parser("resolve-question", help="Resolve open question")
    resolve_parser.add_argument("question", help="The question to resolve")
    resolve_parser.add_argument("resolution", help="The resolution")

    decision_parser = subparsers.add_parser("add-decision", help="Add decision")
    decision_parser.add_argument("decision", help="The decision")
    decision_parser.add_argument("--adr", help="Associated ADR ID")

    subparsers.add_parser("next-adr", help="Get next ADR number")

    args = parser.parse_args()

    if args.command == "init":
        init_session(args.topic, getattr(args, 'target_dir', None))
    elif args.command == "status":
        status = get_status()
        print_status(status)
    elif args.command == "advance":
        session = load_session_state()
        if not session:
            print("No active session")
            sys.exit(1)
        advance_phase(session)
    elif args.command == "gate":
        session = load_session_state()
        if not session:
            print("No active session")
            sys.exit(1)
        result = check_gate(session)
        print(result["message"])
        for issue in result.get("issues", []):
            print(f"  - {issue}")
        sys.exit(0 if result["passed"] else 1)
    elif args.command == "add-question":
        session = load_session_state()
        if not session:
            print("No active session")
            sys.exit(1)
        add_open_question(session, args.question, args.blocking)
    elif args.command == "resolve-question":
        session = load_session_state()
        if not session:
            print("No active session")
            sys.exit(1)
        resolve_question(session, args.question, args.resolution)
    elif args.command == "add-decision":
        session = load_session_state()
        if not session:
            print("No active session")
            sys.exit(1)
        add_decision(session, args.decision, args.adr)
    elif args.command == "next-adr":
        print(get_next_adr_number())
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
