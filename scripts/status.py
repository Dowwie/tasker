#!/usr/bin/env python3
"""
Tasker Status Dashboard

Terminal UI for monitoring workflow progress, health, and performance.

Usage:
    status.py              Launch interactive TUI dashboard
    status.py --once       Print status once and exit (no TUI)
    status.py --json       Print status as JSON and exit

Requirements:
    pip install textual
"""

import argparse
import json
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))


def print_status_once() -> int:
    """Print status summary and exit."""
    from tui.state_provider import FileStateProvider

    provider = FileStateProvider()
    state = provider.load()

    if not state:
        print("No workflow state found.")
        print("Run 'python3 scripts/state.py init <target_dir>' to start.")
        return 1

    # Phase
    print(f"Phase: {state.phase.current.upper()}")
    print(f"Target: {state.target_dir}")
    print()

    # Progress
    total = len(state.tasks)
    completed = state.execution.completed_count
    failed = state.execution.failed_count
    running = len(state.execution.active_tasks)
    blocked = sum(1 for t in state.tasks.values() if t.status == "blocked")
    pending = total - completed - failed - running - blocked

    max_phase = max((t.phase for t in state.tasks.values()), default=1)
    current_phase = state.execution.current_phase or 1

    print(f"Progress: {completed}/{total} tasks ({completed/total*100:.0f}%)" if total > 0 else "Progress: No tasks")
    print(f"Phase: {current_phase}/{max_phase}")
    print()

    print("Status breakdown:")
    if completed > 0:
        print(f"  ✓ Completed: {completed}")
    if running > 0:
        print(f"  ▶ Running:   {running}")
    if failed > 0:
        print(f"  ✗ Failed:    {failed}")
    if blocked > 0:
        print(f"  ⊘ Blocked:   {blocked}")
    if pending > 0:
        print(f"  ○ Pending:   {pending}")
    print()

    # Health checks
    print("Health Checks:")
    for check in state.health_checks:
        icon = "✓" if check.passed else "✗"
        print(f"  {icon} {check.name}: {check.message}")
    print()

    # Calibration
    if state.calibration and state.calibration.total_verified > 0:
        print(f"Verifier Calibration: {state.calibration.calibration_score:.0%}")
        if state.calibration.false_positive_count > 0:
            print(f"  False Positives: {state.calibration.false_positive_count}")
        print()

    # Cost
    print("Cost:")
    print(f"  Tokens: {state.execution.total_tokens:,}")
    print(f"  Cost:   ${state.execution.total_cost_usd:.4f}")
    if completed > 0:
        avg_cost = state.execution.total_cost_usd / completed
        print(f"  Avg:    ${avg_cost:.4f}/task")
    print()

    # Active tasks
    if state.execution.active_tasks:
        print("Active Tasks:")
        for tid in state.execution.active_tasks:
            task = state.tasks.get(tid)
            if task:
                print(f"  ▶ {task.id}: {task.name} (attempt {task.attempts})")
        print()

    # Recent failures
    failed_tasks = [t for t in state.tasks.values() if t.status == "failed"]
    if failed_tasks:
        print("Failed Tasks:")
        for task in failed_tasks[:5]:
            print(f"  ✗ {task.id}: {task.error or 'Unknown error'}")
        print()

    return 0


def print_status_json() -> int:
    """Print status as JSON and exit."""
    from tui.state_provider import FileStateProvider

    provider = FileStateProvider()
    state = provider.load()

    if not state:
        print(json.dumps({"error": "No workflow state found"}))
        return 1

    output = {
        "phase": state.phase.current,
        "target_dir": state.target_dir,
        "tasks": {
            "total": len(state.tasks),
            "completed": state.execution.completed_count,
            "failed": state.execution.failed_count,
            "running": len(state.execution.active_tasks),
            "blocked": sum(1 for t in state.tasks.values() if t.status == "blocked"),
        },
        "phase": {
            "current": state.execution.current_phase or 1,
            "max": max((t.phase for t in state.tasks.values()), default=1),
        },
        "health_checks": [
            {"name": c.name, "passed": c.passed, "message": c.message}
            for c in state.health_checks
        ],
        "calibration": {
            "score": state.calibration.calibration_score if state.calibration else 0,
            "total_verified": state.calibration.total_verified if state.calibration else 0,
            "false_positives": state.calibration.false_positive_count if state.calibration else 0,
        },
        "cost": {
            "tokens": state.execution.total_tokens,
            "usd": state.execution.total_cost_usd,
        },
        "active_tasks": list(state.execution.active_tasks),
    }

    print(json.dumps(output, indent=2))
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Tasker Status Dashboard",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--once",
        action="store_true",
        help="Print status once and exit (no TUI)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print status as JSON and exit",
    )
    parser.add_argument(
        "--state-file",
        type=Path,
        help="Path to state.json file (default: .tasker/state.json)",
    )

    args = parser.parse_args()

    if args.json:
        return print_status_json()

    if args.once:
        return print_status_once()

    # Launch TUI
    try:
        from tui.app import run

        run(state_file=args.state_file)
        return 0
    except ImportError as e:
        print(f"TUI requires textual: {e}")
        print("Install with: pip install textual")
        print()
        print("Falling back to --once mode:")
        print()
        return print_status_once()


if __name__ == "__main__":
    sys.exit(main())
