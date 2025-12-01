#!/usr/bin/env python3
"""
Executor Status Dashboard

Displays a formatted dashboard with execution progress, task status,
and recent activity from the state file.

Usage:
    dashboard.py              Full dashboard
    dashboard.py --compact    Compact single-line summary
    dashboard.py --json       Raw JSON output
"""

import json
import sys
from datetime import datetime, timezone
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
PLANNING_DIR = PROJECT_ROOT / "project-planning"
STATE_FILE = PLANNING_DIR / "state.json"
TASKS_DIR = PLANNING_DIR / "tasks"

# Box drawing characters
BOX_H = "─"
BOX_V = "│"
BOX_TL = "┌"
BOX_TR = "┐"
BOX_BL = "└"
BOX_BR = "┘"
BOX_T = "┬"
BOX_B = "┴"
BOX_L = "├"
BOX_R = "┤"
BOX_X = "┼"

# Status indicators
STATUS_ICONS = {
    "pending": "○",
    "ready": "◎",
    "running": "●",
    "complete": "✓",
    "failed": "✗",
    "blocked": "⊘",
    "skipped": "⊖",
}

STATUS_COLORS = {
    "pending": "\033[90m",    # Gray
    "ready": "\033[93m",      # Yellow
    "running": "\033[96m",    # Cyan
    "complete": "\033[92m",   # Green
    "failed": "\033[91m",     # Red
    "blocked": "\033[95m",    # Magenta
    "skipped": "\033[90m",    # Gray
}
RESET = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[2m"


def load_state() -> dict | None:
    if not STATE_FILE.exists():
        return None
    return json.loads(STATE_FILE.read_text())


def format_duration(start_iso: str, end_iso: str = None) -> str:
    """Format duration between two ISO timestamps."""
    try:
        start = datetime.fromisoformat(start_iso.replace("Z", "+00:00"))
        end = datetime.fromisoformat(end_iso.replace("Z", "+00:00")) if end_iso else datetime.now(timezone.utc)
        delta = end - start

        total_seconds = abs(int(delta.total_seconds()))
        if total_seconds < 60:
            return f"{total_seconds}s"
        elif total_seconds < 3600:
            return f"{total_seconds // 60}m {total_seconds % 60}s"
        else:
            hours = total_seconds // 3600
            mins = (total_seconds % 3600) // 60
            return f"{hours}h {mins}m"
    except (ValueError, TypeError):
        return "—"


def format_time_ago(iso_timestamp: str) -> str:
    """Format timestamp as 'X ago'."""
    try:
        ts = datetime.fromisoformat(iso_timestamp.replace("Z", "+00:00"))
        delta = datetime.now(timezone.utc) - ts

        total_seconds = abs(int(delta.total_seconds()))
        if total_seconds < 60:
            return f"{total_seconds}s ago"
        elif total_seconds < 3600:
            return f"{total_seconds // 60}m ago"
        elif total_seconds < 86400:
            return f"{total_seconds // 3600}h ago"
        else:
            return f"{total_seconds // 86400}d ago"
    except (ValueError, TypeError):
        return "—"


def progress_bar(completed: int, total: int, width: int = 30) -> str:
    """Create a progress bar."""
    if total == 0:
        return f"[{'─' * width}] 0%"

    pct = completed / total
    filled = int(width * pct)
    empty = width - filled

    bar = "█" * filled + "░" * empty
    return f"[{bar}] {pct * 100:.0f}%"


def box_line(left: str, fill: str, right: str, width: int) -> str:
    """Create a box line."""
    return left + fill * (width - 2) + right


def box_text(text: str, width: int, align: str = "left") -> str:
    """Create a box line with text."""
    content_width = width - 4  # Account for borders and padding
    if len(text) > content_width:
        text = text[:content_width - 1] + "…"

    if align == "center":
        padded = text.center(content_width)
    elif align == "right":
        padded = text.rjust(content_width)
    else:
        padded = text.ljust(content_width)

    return f"{BOX_V} {padded} {BOX_V}"


def render_dashboard(state: dict, use_color: bool = True) -> str:
    """Render the full dashboard."""
    lines = []
    width = 72

    def c(color_code: str) -> str:
        return color_code if use_color else ""

    # Header
    lines.append(box_line(BOX_TL, BOX_H, BOX_TR, width))
    lines.append(box_text(f"{c(BOLD)}EXECUTOR STATUS DASHBOARD{c(RESET)}", width, "center"))
    lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

    # Phase & Target
    phase = state["phase"]["current"]
    phase_display = f"{c(BOLD)}{phase.upper()}{c(RESET)}"
    target = Path(state["target_dir"]).name

    lines.append(box_text(f"Phase: {phase_display}    Target: {target}", width))
    lines.append(box_text(f"Updated: {format_time_ago(state.get('updated_at', ''))}", width))

    # Get tasks early for use in multiple sections
    tasks = state.get("tasks", {})

    # Planning Quality Section (show during planning phases or if tasks exist)
    phase_idx = ["ingestion", "logical", "physical", "definition", "validation", "sequencing", "ready", "executing", "complete"]
    current_phase_idx = phase_idx.index(phase) if phase in phase_idx else -1

    if current_phase_idx >= 3 and tasks:  # definition phase or later
        # Compute planning metrics inline
        total_behaviors = 0
        total_criteria = 0
        steel_thread_count = 0

        for tid in tasks:
            task_path = TASKS_DIR / f"{tid}.json"
            if task_path.exists():
                try:
                    task_def = json.loads(task_path.read_text())
                    total_behaviors += len(task_def.get("behaviors", []))
                    total_criteria += len(task_def.get("acceptance_criteria", []))
                    if task_def.get("context", {}).get("steel_thread"):
                        steel_thread_count += 1
                except (json.JSONDecodeError, KeyError):
                    pass

        task_count = len(tasks)
        avg_behaviors = total_behaviors / task_count if task_count > 0 else 0
        avg_criteria = total_criteria / task_count if task_count > 0 else 0

        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}PLANNING QUALITY{c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        # Color code behaviors/task: green if 2-5, yellow otherwise
        behaviors_color = STATUS_COLORS['complete'] if 2 <= avg_behaviors <= 5 else STATUS_COLORS['ready']
        plan_line = f"Tasks: {task_count}  Behaviors: {total_behaviors}  "
        plan_line += f"Avg: {c(behaviors_color)}{avg_behaviors:.1f}{c(RESET)}/task  "
        plan_line += f"Steel: {steel_thread_count}"
        lines.append(box_text(plan_line, width, "center"))

    # Execution Stats Section
    lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
    lines.append(box_text(f"{c(BOLD)}EXECUTION PROGRESS{c(RESET)}", width, "center"))
    lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

    execution = state.get("execution", {})

    # Count by status
    status_counts = {}
    for task in tasks.values():
        status = task["status"]
        status_counts[status] = status_counts.get(status, 0) + 1

    total = len(tasks)
    completed = status_counts.get("complete", 0)
    failed = status_counts.get("failed", 0)
    blocked = status_counts.get("blocked", 0)
    running = status_counts.get("running", 0)
    pending = status_counts.get("pending", 0)
    skipped = status_counts.get("skipped", 0)

    # Progress bar
    lines.append(box_text(progress_bar(completed, total), width, "center"))
    lines.append(box_text(f"{completed}/{total} tasks complete", width, "center"))

    # Status breakdown
    lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

    status_line = "  ".join([
        f"{c(STATUS_COLORS.get(s, ''))}{STATUS_ICONS.get(s, '?')} {s}: {status_counts.get(s, 0)}{c(RESET)}"
        for s in ["complete", "running", "pending", "failed", "blocked", "skipped"]
        if status_counts.get(s, 0) > 0
    ])
    lines.append(box_text(status_line, width, "center"))

    # Phase progress
    phases = {}
    for task in tasks.values():
        phase = task.get("phase", 0)
        if phase not in phases:
            phases[phase] = {"total": 0, "complete": 0}
        phases[phase]["total"] += 1
        if task["status"] == "complete":
            phases[phase]["complete"] += 1

    if phases:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}PHASE PROGRESS{c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        for phase_num in sorted(phases.keys()):
            if phase_num == 0:
                continue
            w = phases[phase_num]
            phase_bar = progress_bar(w["complete"], w["total"], 20)
            lines.append(box_text(f"Phase {phase_num}: {phase_bar} ({w['complete']}/{w['total']})", width))

    # Running tasks
    running_tasks = [t for t in tasks.values() if t["status"] == "running"]
    if running_tasks:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}{c(STATUS_COLORS['running'])}RUNNING ({len(running_tasks)}){c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        for task in running_tasks:
            duration = format_duration(task.get("started_at", ""))
            lines.append(box_text(f"{c(STATUS_COLORS['running'])}{STATUS_ICONS['running']}{c(RESET)} {task['id']}: {task.get('name', '')[:40]} ({duration})", width))

    # Failed tasks
    failed_tasks = [t for t in tasks.values() if t["status"] == "failed"]
    if failed_tasks:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}{c(STATUS_COLORS['failed'])}FAILED ({len(failed_tasks)}){c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        for task in failed_tasks[:5]:  # Limit to 5
            error = task.get("error", "Unknown error")[:45]
            lines.append(box_text(f"{c(STATUS_COLORS['failed'])}{STATUS_ICONS['failed']}{c(RESET)} {task['id']}: {error}", width))

        if len(failed_tasks) > 5:
            lines.append(box_text(f"  ... and {len(failed_tasks) - 5} more", width))

    # Blocked tasks
    blocked_tasks = [t for t in tasks.values() if t["status"] == "blocked"]
    if blocked_tasks:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}{c(STATUS_COLORS['blocked'])}BLOCKED ({len(blocked_tasks)}){c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        for task in blocked_tasks[:3]:  # Limit to 3
            lines.append(box_text(f"{c(STATUS_COLORS['blocked'])}{STATUS_ICONS['blocked']}{c(RESET)} {task['id']}: {task.get('name', '')[:50]}", width))

        if len(blocked_tasks) > 3:
            lines.append(box_text(f"  ... and {len(blocked_tasks) - 3} more", width))

    # Ready tasks
    ready_tasks = []
    for tid, task in tasks.items():
        if task["status"] == "pending":
            deps_met = all(
                tasks.get(dep, {}).get("status") in ["complete", "skipped"]
                for dep in task.get("depends_on", [])
            )
            if deps_met:
                ready_tasks.append(task)

    if ready_tasks:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}{c(STATUS_COLORS['ready'])}READY TO EXECUTE ({len(ready_tasks)}){c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        for task in ready_tasks[:5]:  # Limit to 5
            phase = task.get("phase", "?")
            lines.append(box_text(f"{c(STATUS_COLORS['ready'])}{STATUS_ICONS['ready']}{c(RESET)} {task['id']} [W{phase}]: {task.get('name', '')[:45]}", width))

        if len(ready_tasks) > 5:
            lines.append(box_text(f"  ... and {len(ready_tasks) - 5} more ready", width))

    # Token usage
    lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
    lines.append(box_text(f"{c(BOLD)}RESOURCE USAGE{c(RESET)}", width, "center"))
    lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

    total_tokens = execution.get("total_tokens", 0)
    total_cost = execution.get("total_cost_usd", 0.0)

    lines.append(box_text(f"Tokens: {total_tokens:,}    Cost: ${total_cost:.4f}", width, "center"))

    # Verification & Quality metrics
    verified_count = 0
    pass_count = 0
    block_count = 0
    for task in tasks.values():
        verification = task.get("verification", {})
        if verification:
            verified_count += 1
            if verification.get("recommendation") == "PROCEED":
                pass_count += 1
            elif verification.get("recommendation") == "BLOCK":
                block_count += 1

    if verified_count > 0:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}VERIFICATION{c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        verify_line = f"Verified: {verified_count}  "
        verify_line += f"{c(STATUS_COLORS['complete'])}PROCEED: {pass_count}{c(RESET)}  "
        if block_count > 0:
            verify_line += f"{c(STATUS_COLORS['failed'])}BLOCK: {block_count}{c(RESET)}"
        lines.append(box_text(verify_line, width, "center"))

    # Calibration metrics (if available)
    calibration = state.get("calibration", {})
    if calibration.get("total_verified", 0) > 0:
        cal_total = calibration.get("total_verified", 0)
        cal_correct = calibration.get("correct", 0)
        fp_count = len(calibration.get("false_positives", []))
        fn_count = len(calibration.get("false_negatives", []))
        cal_score = cal_correct / cal_total if cal_total > 0 else 1.0

        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}VERIFIER CALIBRATION{c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        score_color = STATUS_COLORS['complete'] if cal_score >= 0.9 else (STATUS_COLORS['ready'] if cal_score >= 0.7 else STATUS_COLORS['failed'])
        cal_line = f"Score: {c(score_color)}{cal_score:.0%}{c(RESET)}  "
        cal_line += f"Correct: {cal_correct}  "
        if fp_count > 0:
            cal_line += f"{c(STATUS_COLORS['failed'])}FP: {fp_count}{c(RESET)}  "
        if fn_count > 0:
            cal_line += f"{c(STATUS_COLORS['ready'])}FN: {fn_count}{c(RESET)}"
        lines.append(box_text(cal_line, width, "center"))

    # Recent events
    events = state.get("events", [])
    task_events = [e for e in events if e.get("type", "").startswith("task_")]

    if task_events:
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))
        lines.append(box_text(f"{c(BOLD)}RECENT ACTIVITY{c(RESET)}", width, "center"))
        lines.append(box_line(BOX_L, BOX_H, BOX_R, width))

        for event in task_events[-5:]:
            event_type = event.get("type", "").replace("task_", "")
            task_id = event.get("task_id", "")
            time_ago = format_time_ago(event.get("timestamp", ""))

            icon = STATUS_ICONS.get(event_type.replace("ed", "").replace("start", "running").replace("complet", "complete").replace("fail", "failed"), "·")
            lines.append(box_text(f"{c(DIM)}{time_ago:>8}{c(RESET)}  {icon} {task_id} {event_type}", width))

    # Footer
    lines.append(box_line(BOX_BL, BOX_H, BOX_BR, width))

    return "\n".join(lines)


def render_compact(state: dict) -> str:
    """Render compact single-line summary."""
    tasks = state.get("tasks", {})

    status_counts = {}
    for task in tasks.values():
        status = task["status"]
        status_counts[status] = status_counts.get(status, 0) + 1

    total = len(tasks)
    completed = status_counts.get("complete", 0)
    running = status_counts.get("running", 0)
    failed = status_counts.get("failed", 0)

    phase = state["phase"]["current"]

    parts = [
        f"[{phase.upper()}]",
        f"{completed}/{total} done",
    ]

    if running:
        parts.append(f"{running} running")
    if failed:
        parts.append(f"{failed} failed")

    return " | ".join(parts)


def main():
    # Check for flags
    compact = "--compact" in sys.argv
    as_json = "--json" in sys.argv
    no_color = "--no-color" in sys.argv or not sys.stdout.isatty()

    state = load_state()

    if not state:
        print("No state file found at project-planning/state.json")
        print("Run '/plan' first to initialize a decomposition.")
        sys.exit(1)

    if as_json:
        # Output relevant state as JSON
        output = {
            "phase": state["phase"]["current"],
            "target_dir": state["target_dir"],
            "updated_at": state.get("updated_at"),
            "tasks": {
                tid: {
                    "status": t["status"],
                    "phase": t.get("phase"),
                    "name": t.get("name"),
                    "error": t.get("error"),
                }
                for tid, t in state.get("tasks", {}).items()
            },
            "execution": state.get("execution", {}),
        }
        print(json.dumps(output, indent=2))
    elif compact:
        print(render_compact(state))
    else:
        print(render_dashboard(state, use_color=not no_color))


if __name__ == "__main__":
    main()
