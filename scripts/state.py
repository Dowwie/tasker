#!/usr/bin/env python3
"""
State Manager for Task Decomposition Protocol v2

Single source of truth for decomposition state. All state changes go through here.
Validates artifacts against schemas. Computes ready tasks dynamically.

Usage:
    state.py init <target_dir>           Initialize new decomposition
    state.py status                       Show current state
    state.py advance                      Attempt to advance to next phase
    state.py validate <artifact>          Validate artifact against schema
    state.py validate-tasks <verdict> [summary] [--issues i1 i2]
                                          Register task validation results
    state.py load-tasks                   Load task files from tasks/ directory
    state.py ready-tasks                  List tasks ready for execution
    state.py start-task <task_id>         Mark task as running
    state.py complete-task <task_id> [--created f1 f2] [--modified f3 f4]
                                          Mark task as complete with file tracking
    state.py commit-task <task_id>        Commit files from completed task to git
    state.py fail-task <task_id> <error>  Mark task as failed
    state.py retry-task <task_id>         Reset failed task to pending
    state.py skip-task <task_id> [reason] Skip task without blocking dependents
    state.py log-tokens <session> <in> <out> <cost>  Log token usage
    state.py record-verification <task_id> --verdict PASS|FAIL|CONDITIONAL
                                          --recommendation PROCEED|BLOCK
                                          [--criteria '<json>'] [--quality '<json>']
                                          [--tests '<json>']
                                          Record verification results for a task
    state.py metrics [--format text|json] Compute and display performance metrics
"""

import json
import hashlib
import sys
from datetime import datetime, timezone
from pathlib import Path

# Paths relative to script location
SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
PLANNING_DIR = PROJECT_ROOT / "project-planning"
STATE_FILE = PLANNING_DIR / "state.json"
SCHEMAS_DIR = PROJECT_ROOT / "schemas"


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def file_checksum(path: Path) -> str:
    """SHA256 checksum of file contents."""
    if not path.exists():
        return ""
    return hashlib.sha256(path.read_bytes()).hexdigest()[:16]


def load_state() -> dict:
    """Load state from file or return None if doesn't exist."""
    if not STATE_FILE.exists():
        return None
    return json.loads(STATE_FILE.read_text())


def save_state(state: dict) -> None:
    """Save state to file."""
    state["updated_at"] = now_iso()
    STATE_FILE.parent.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(json.dumps(state, indent=2))


def add_event(state: dict, event_type: str, task_id: str = None, details: dict = None) -> None:
    """Append event to state's event log."""
    event = {
        "timestamp": now_iso(),
        "type": event_type,
    }
    if task_id:
        event["task_id"] = task_id
    if details:
        event["details"] = details
    state.setdefault("events", []).append(event)


def commit_task_changes(state: dict, task_id: str) -> tuple[bool, str]:
    """Commit files changed by a completed task."""
    import subprocess

    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    task = state["tasks"][task_id]
    if task["status"] != "complete":
        return False, f"Task {task_id} is {task['status']}, not complete"

    files_created = task.get("files_created", [])
    files_modified = task.get("files_modified", [])
    all_files = files_created + files_modified

    if not all_files:
        return False, f"No files to commit for task {task_id}"

    target_dir = state.get("target_dir", ".")

    # Stage the files
    for f in all_files:
        file_path = Path(target_dir) / f
        if file_path.exists():
            result = subprocess.run(
                ["git", "add", str(file_path)],
                cwd=target_dir,
                capture_output=True,
                text=True
            )
            if result.returncode != 0:
                return False, f"git add failed for {f}: {result.stderr}"

    # Build commit message
    commit_msg = f"{task_id}: {task['name']}"

    # Commit
    result = subprocess.run(
        ["git", "commit", "-m", commit_msg],
        cwd=target_dir,
        capture_output=True,
        text=True
    )

    if result.returncode != 0:
        # Check if it's "nothing to commit"
        if "nothing to commit" in result.stdout or "nothing to commit" in result.stderr:
            return True, f"No changes to commit for {task_id}"
        return False, f"git commit failed: {result.stderr}"

    add_event(state, "task_committed", task_id=task_id, details={
        "files": all_files,
        "commit_msg": commit_msg
    })

    return True, f"Committed {len(all_files)} file(s) for {task_id}"


def validate_json(data: dict, schema_name: str) -> tuple[bool, str]:
    """Validate JSON data against schema. Returns (valid, error_message)."""
    schema_path = SCHEMAS_DIR / f"{schema_name}.schema.json"
    if not schema_path.exists():
        return False, f"Schema not found: {schema_path}"

    try:
        from jsonschema import validate, ValidationError, SchemaError
    except ImportError:
        # Fallback to basic validation if jsonschema not installed
        schema = json.loads(schema_path.read_text())
        required = schema.get("required", [])
        for field in required:
            if field not in data:
                return False, f"Missing required field: {field}"
        return True, ""

    try:
        schema = json.loads(schema_path.read_text())
        validate(instance=data, schema=schema)
        return True, ""
    except SchemaError as e:
        return False, f"Invalid schema: {e.message}"
    except ValidationError as e:
        # Build a helpful error message with path to the error
        path = " -> ".join(str(p) for p in e.absolute_path) if e.absolute_path else "root"
        return False, f"Validation error at '{path}': {e.message}"


def init_state(target_dir: str) -> dict:
    """Initialize new decomposition state."""
    state = {
        "version": "2.0",
        "phase": {
            "current": "ingestion",
            "completed": []
        },
        "target_dir": str(Path(target_dir).resolve()),
        "created_at": now_iso(),
        "updated_at": now_iso(),
        "artifacts": {},
        "tasks": {},
        "execution": {
            "current_wave": 0,
            "active_tasks": [],
            "completed_count": 0,
            "failed_count": 0,
            "total_tokens": 0,
            "total_cost_usd": 0.0
        },
        "events": []
    }
    add_event(state, "initialized", details={"target_dir": target_dir})
    return state


def get_phase_order() -> list[str]:
    return ["ingestion", "logical", "physical", "definition", "validation", "sequencing", "ready", "executing", "complete"]


def get_next_phase(current: str) -> str | None:
    """Get the next phase after current."""
    order = get_phase_order()
    try:
        idx = order.index(current)
        if idx + 1 < len(order):
            return order[idx + 1]
    except ValueError:
        pass
    return None


def can_advance_phase(state: dict) -> tuple[bool, str]:
    """Check if we can advance to next phase. Returns (can_advance, reason)."""
    current = state["phase"]["current"]
    
    if current == "ingestion":
        spec_path = PLANNING_DIR / "inputs" / "spec.md"
        if not spec_path.exists():
            return False, "spec.md not found in project-planning/inputs/"
        return True, ""
    
    elif current == "logical":
        artifact = state["artifacts"].get("capability_map", {})
        if not artifact.get("valid"):
            return False, "capability_map not validated"
        return True, ""
    
    elif current == "physical":
        artifact = state["artifacts"].get("physical_map", {})
        if not artifact.get("valid"):
            return False, "physical_map not validated"
        return True, ""
    
    elif current == "definition":
        if not state["tasks"]:
            return False, "No tasks defined"
        return True, ""

    elif current == "validation":
        validation = state["artifacts"].get("task_validation", {})
        if not validation.get("valid"):
            return False, "Task validation not complete or has blocking issues"
        verdict = validation.get("verdict", "")
        if verdict == "BLOCKED":
            return False, f"Task validation blocked: {validation.get('error', 'Unknown issue')}"
        return True, ""

    elif current == "sequencing":
        # Check that all tasks have waves assigned
        for tid, task in state["tasks"].items():
            if task.get("wave", 0) == 0:
                return False, f"Task {tid} has no wave assigned"
        return True, ""
    
    elif current == "ready":
        return True, ""  # Can always start executing
    
    elif current == "executing":
        # Check if all tasks complete
        for tid, task in state["tasks"].items():
            if task["status"] not in ["complete", "blocked"]:
                return False, f"Task {tid} not complete"
        return True, ""
    
    return False, f"Unknown phase: {current}"


def advance_phase(state: dict) -> tuple[bool, str]:
    """Attempt to advance to next phase."""
    can, reason = can_advance_phase(state)
    if not can:
        return False, reason
    
    current = state["phase"]["current"]
    next_phase = get_next_phase(current)
    
    if not next_phase:
        return False, "Already at final phase"
    
    state["phase"]["completed"].append(current)
    state["phase"]["current"] = next_phase
    add_event(state, "phase_advanced", details={"from": current, "to": next_phase})
    
    return True, f"Advanced from {current} to {next_phase}"


def register_artifact(state: dict, artifact_type: str, path: str) -> tuple[bool, str]:
    """Register and validate an artifact."""
    artifact_path = Path(path)
    if not artifact_path.exists():
        return False, f"Artifact not found: {path}"
    
    # Load and validate
    try:
        data = json.loads(artifact_path.read_text())
    except json.JSONDecodeError as e:
        return False, f"Invalid JSON: {e}"
    
    valid, error = validate_json(data, artifact_type.replace("_", "-"))
    
    state["artifacts"][artifact_type] = {
        "path": str(artifact_path),
        "checksum": file_checksum(artifact_path),
        "valid": valid,
        "validated_at": now_iso(),
        "error": error if not valid else None
    }
    
    add_event(state, "artifact_registered", details={
        "type": artifact_type,
        "valid": valid,
        "error": error if not valid else None
    })

    return valid, error if not valid else "Validated successfully"


def register_task_validation(
    state: dict, verdict: str, summary: str = "", issues: list | None = None
) -> tuple[bool, str]:
    """Register task validation results from task-plan-verifier.

    Args:
        state: Current state dict
        verdict: READY, READY_WITH_NOTES, or BLOCKED
        summary: Summary of validation results
        issues: List of issues found (for BLOCKED or READY_WITH_NOTES)

    Returns:
        (success, message) tuple
    """
    valid_verdicts = ["READY", "READY_WITH_NOTES", "BLOCKED"]
    if verdict not in valid_verdicts:
        return False, f"Invalid verdict: {verdict}. Must be one of {valid_verdicts}"

    is_valid = verdict in ["READY", "READY_WITH_NOTES"]

    state["artifacts"]["task_validation"] = {
        "verdict": verdict,
        "valid": is_valid,
        "summary": summary,
        "issues": issues or [],
        "validated_at": now_iso(),
        "error": summary if not is_valid else None,
    }

    add_event(
        state,
        "task_validation_complete",
        details={"verdict": verdict, "valid": is_valid, "issue_count": len(issues or [])},
    )

    if is_valid:
        return True, f"Task validation complete: {verdict}"
    else:
        return False, f"Task validation blocked: {summary}"


def load_tasks_from_dir(state: dict) -> int:
    """Load individual task files from tasks/ directory."""
    tasks_dir = PLANNING_DIR / "tasks"
    if not tasks_dir.exists():
        return 0
    
    count = 0
    for task_file in tasks_dir.glob("*.json"):
        try:
            task = json.loads(task_file.read_text())
            task_id = task["id"]
            
            # Initialize runtime state
            state["tasks"][task_id] = {
                "id": task_id,
                "name": task.get("name", ""),
                "status": "pending",
                "wave": task.get("wave", 0),
                "depends_on": task.get("dependencies", {}).get("tasks", []),
                "blocks": [],  # Computed later
                "file": str(task_file)
            }
            count += 1
        except (json.JSONDecodeError, KeyError) as e:
            print(f"Warning: Could not load {task_file}: {e}", file=sys.stderr)
    
    # Compute reverse dependencies (blocks)
    for tid, task in state["tasks"].items():
        for dep in task["depends_on"]:
            if dep in state["tasks"]:
                state["tasks"][dep]["blocks"].append(tid)
    
    add_event(state, "tasks_loaded", details={"count": count})
    return count


def get_ready_tasks(state: dict) -> list[str]:
    """Get task IDs that are ready to execute (all deps complete)."""
    ready = []
    for tid, task in state["tasks"].items():
        if task["status"] != "pending":
            continue
        
        # Check all dependencies are complete
        deps_met = all(
            state["tasks"].get(dep, {}).get("status") == "complete"
            for dep in task["depends_on"]
        )
        
        if deps_met:
            ready.append(tid)
    
    return ready


def start_task(state: dict, task_id: str) -> tuple[bool, str]:
    """Mark task as running and increment attempt counter."""
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    task = state["tasks"][task_id]
    if task["status"] != "pending":
        return False, f"Task {task_id} is {task['status']}, not pending"

    # Check dependencies
    for dep in task["depends_on"]:
        dep_status = state["tasks"].get(dep, {}).get("status")
        if dep_status != "complete":
            return False, f"Dependency {dep} is {dep_status}, not complete"

    task["status"] = "running"
    task["started_at"] = now_iso()
    task["attempts"] = task.get("attempts", 0) + 1
    state["execution"]["active_tasks"].append(task_id)

    add_event(state, "task_started", task_id=task_id, details={"attempt": task["attempts"]})
    return True, f"Task {task_id} started (attempt {task['attempts']})"


def complete_task(state: dict, task_id: str, files_created: list = None, files_modified: list = None) -> tuple[bool, str]:
    """Mark task as complete and compute duration."""
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    task = state["tasks"][task_id]
    if task["status"] != "running":
        return False, f"Task {task_id} is {task['status']}, not running"

    task["status"] = "complete"
    task["completed_at"] = now_iso()
    task["files_created"] = files_created or []
    task["files_modified"] = files_modified or []

    # Compute duration if started_at is available
    if "started_at" in task:
        started = datetime.fromisoformat(task["started_at"].replace("Z", "+00:00"))
        completed = datetime.fromisoformat(task["completed_at"].replace("Z", "+00:00"))
        task["duration_seconds"] = (completed - started).total_seconds()

    if task_id in state["execution"]["active_tasks"]:
        state["execution"]["active_tasks"].remove(task_id)
    state["execution"]["completed_count"] += 1

    add_event(state, "task_completed", task_id=task_id, details={
        "files_created": files_created,
        "files_modified": files_modified,
        "duration_seconds": task.get("duration_seconds")
    })
    return True, f"Task {task_id} completed"


def fail_task(state: dict, task_id: str, error: str) -> tuple[bool, str]:
    """Mark task as failed."""
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"
    
    task = state["tasks"][task_id]
    task["status"] = "failed"
    task["error"] = error
    task["completed_at"] = now_iso()
    
    if task_id in state["execution"]["active_tasks"]:
        state["execution"]["active_tasks"].remove(task_id)
    state["execution"]["failed_count"] += 1
    
    # Mark dependent tasks as blocked
    for blocked_id in task["blocks"]:
        if blocked_id in state["tasks"]:
            state["tasks"][blocked_id]["status"] = "blocked"
            state["tasks"][blocked_id]["error"] = f"Blocked by failed task {task_id}"
    
    add_event(state, "task_failed", task_id=task_id, details={"error": error})
    return True, f"Task {task_id} failed: {error}"


def retry_task(state: dict, task_id: str) -> tuple[bool, str]:
    """Reset a failed task to pending and unblock its dependents."""
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    task = state["tasks"][task_id]
    if task["status"] not in ["failed", "blocked"]:
        return False, f"Task {task_id} is {task['status']}, not failed/blocked"

    # Reset this task
    old_status = task["status"]
    task["status"] = "pending"
    task.pop("error", None)
    task.pop("completed_at", None)
    task.pop("started_at", None)

    # Decrement failed count if it was failed
    if old_status == "failed":
        state["execution"]["failed_count"] = max(0, state["execution"]["failed_count"] - 1)

    # Recursively unblock dependent tasks that were blocked by this task
    unblocked = []
    for blocked_id in task.get("blocks", []):
        if blocked_id in state["tasks"]:
            blocked_task = state["tasks"][blocked_id]
            if blocked_task["status"] == "blocked" and f"Blocked by failed task {task_id}" in blocked_task.get("error", ""):
                blocked_task["status"] = "pending"
                blocked_task.pop("error", None)
                unblocked.append(blocked_id)

    add_event(state, "task_retried", task_id=task_id, details={"unblocked": unblocked})

    msg = f"Task {task_id} reset to pending"
    if unblocked:
        msg += f", unblocked: {unblocked}"
    return True, msg


def skip_task(state: dict, task_id: str, reason: str = "Manually skipped") -> tuple[bool, str]:
    """Mark a task as skipped without blocking dependents."""
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    task = state["tasks"][task_id]
    if task["status"] not in ["pending", "blocked", "failed"]:
        return False, f"Task {task_id} is {task['status']}, cannot skip"

    old_status = task["status"]
    task["status"] = "skipped"
    task["skip_reason"] = reason
    task["completed_at"] = now_iso()

    # Decrement failed count if it was failed
    if old_status == "failed":
        state["execution"]["failed_count"] = max(0, state["execution"]["failed_count"] - 1)

    # Treat skipped as "complete" for dependency purposes - unblock dependents
    # that were blocked by this task being failed
    unblocked = []
    for blocked_id in task.get("blocks", []):
        if blocked_id in state["tasks"]:
            blocked_task = state["tasks"][blocked_id]
            if blocked_task["status"] == "blocked":
                # Check if all other dependencies are now satisfied
                all_deps_ok = all(
                    state["tasks"].get(dep, {}).get("status") in ["complete", "skipped"]
                    for dep in blocked_task.get("depends_on", [])
                )
                if all_deps_ok:
                    blocked_task["status"] = "pending"
                    blocked_task.pop("error", None)
                    unblocked.append(blocked_id)

    add_event(state, "task_skipped", task_id=task_id, details={"reason": reason, "unblocked": unblocked})

    msg = f"Task {task_id} skipped: {reason}"
    if unblocked:
        msg += f", unblocked: {unblocked}"
    return True, msg


def log_tokens(state: dict, session_id: str, input_tokens: int, output_tokens: int, cost: float) -> None:
    """Log token usage from subagent."""
    state["execution"]["total_tokens"] += input_tokens + output_tokens
    state["execution"]["total_cost_usd"] += cost

    add_event(state, "tokens_logged", details={
        "session_id": session_id,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
        "cost_usd": cost
    })


def record_verification(
    state: dict,
    task_id: str,
    verdict: str,
    recommendation: str,
    criteria: list | None = None,
    quality: dict | None = None,
    tests: dict | None = None,
) -> tuple[bool, str]:
    """Record verification results for a completed task.

    Args:
        state: Current state dict
        task_id: Task ID to record verification for
        verdict: PASS, FAIL, or CONDITIONAL
        recommendation: PROCEED or BLOCK
        criteria: List of {"name", "score", "evidence"} dicts
        quality: Dict with types, docs, patterns, errors scores
        tests: Dict with coverage, assertions, edge_cases scores

    Returns:
        (success, message) tuple
    """
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    valid_verdicts = ["PASS", "FAIL", "CONDITIONAL"]
    if verdict not in valid_verdicts:
        return False, f"Invalid verdict: {verdict}. Must be one of {valid_verdicts}"

    valid_recommendations = ["PROCEED", "BLOCK"]
    if recommendation not in valid_recommendations:
        return False, f"Invalid recommendation: {recommendation}. Must be one of {valid_recommendations}"

    task = state["tasks"][task_id]
    task["verification"] = {
        "verdict": verdict,
        "recommendation": recommendation,
        "criteria": criteria or [],
        "quality": quality or {},
        "tests": tests or {},
        "verified_at": now_iso(),
    }

    add_event(state, "verification_recorded", task_id=task_id, details={
        "verdict": verdict,
        "recommendation": recommendation,
        "criteria_count": len(criteria or []),
    })

    return True, f"Verification recorded for {task_id}: {verdict} ({recommendation})"


def compute_metrics(state: dict) -> dict:
    """Compute performance metrics from state.

    Returns:
        Dict with computed metrics:
        - task_success_rate: completed / (completed + failed)
        - first_attempt_success_rate: first-try successes / completed
        - avg_attempts: average attempts per completed task
        - tokens_per_task: total_tokens / completed
        - cost_per_task: total_cost / completed
        - quality_pass_rate: tasks with all quality PASS / completed
        - functional_pass_rate: criteria PASS / total criteria
        - test_edge_case_rate: tests with edge_cases PASS / tested tasks
    """
    tasks = state.get("tasks", {})
    execution = state.get("execution", {})

    completed = execution.get("completed_count", 0)
    failed = execution.get("failed_count", 0)
    total_tokens = execution.get("total_tokens", 0)
    total_cost = execution.get("total_cost_usd", 0.0)

    # Avoid division by zero
    total_finished = completed + failed

    # Count first-attempt successes and quality metrics
    first_attempt_successes = 0
    quality_full_pass = 0
    total_criteria = 0
    criteria_pass = 0
    tasks_with_tests = 0
    edge_cases_pass = 0
    total_attempts = 0

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
                quality_scores = [quality.get(k) for k in ["types", "docs", "patterns", "errors"]]
                if all(s == "PASS" for s in quality_scores if s):
                    quality_full_pass += 1

            # Functional criteria pass rate
            criteria = verification.get("criteria", [])
            for c in criteria:
                total_criteria += 1
                if c.get("score") == "PASS":
                    criteria_pass += 1

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
    }


def print_status(state: dict) -> None:
    """Print human-readable status."""
    print(f"Phase: {state['phase']['current']}")
    print(f"Target: {state['target_dir']}")
    print(f"Tasks: {len(state['tasks'])}")
    
    if state["tasks"]:
        by_status = {}
        for task in state["tasks"].values():
            status = task["status"]
            by_status[status] = by_status.get(status, 0) + 1
        print(f"Status: {by_status}")
    
    print(f"Tokens: {state['execution']['total_tokens']:,}")
    print(f"Cost: ${state['execution']['total_cost_usd']:.4f}")
    
    ready = get_ready_tasks(state)
    if ready:
        print(f"Ready tasks: {ready}")


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    
    cmd = sys.argv[1]
    
    if cmd == "init":
        if len(sys.argv) < 3:
            print("Usage: state.py init <target_dir>")
            sys.exit(1)
        state = init_state(sys.argv[2])
        save_state(state)
        print(f"Initialized state for {sys.argv[2]}")
    
    elif cmd == "status":
        state = load_state()
        if not state:
            print("No state file. Run 'state.py init <target_dir>' first.")
            sys.exit(1)
        print_status(state)
    
    elif cmd == "advance":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        success, msg = advance_phase(state)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)
    
    elif cmd == "validate":
        if len(sys.argv) < 3:
            print("Usage: state.py validate <artifact_type>")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        
        artifact_type = sys.argv[2]
        path_map = {
            "capability_map": PLANNING_DIR / "artifacts" / "capability-map.json",
            "physical_map": PLANNING_DIR / "artifacts" / "physical-map.json",
        }
        
        if artifact_type not in path_map:
            print(f"Unknown artifact type: {artifact_type}")
            sys.exit(1)
        
        success, msg = register_artifact(state, artifact_type, str(path_map[artifact_type]))
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "validate-tasks":
        if len(sys.argv) < 3:
            print("Usage: state.py validate-tasks <verdict> [summary] [--issues i1 i2]")
            print("  verdict: READY, READY_WITH_NOTES, or BLOCKED")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        verdict = sys.argv[2].upper()
        summary = ""
        issues = []

        # Parse optional summary and --issues
        args = sys.argv[3:]
        i = 0
        while i < len(args):
            if args[i] == "--issues":
                issues = args[i + 1 :]
                break
            else:
                summary = args[i]
            i += 1

        success, msg = register_task_validation(state, verdict, summary, issues)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "load-tasks":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        count = load_tasks_from_dir(state)
        save_state(state)
        print(f"Loaded {count} tasks")
    
    elif cmd == "ready-tasks":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        ready = get_ready_tasks(state)
        for tid in ready:
            task = state["tasks"][tid]
            print(f"{tid}: {task['name']} (wave {task['wave']})")
    
    elif cmd == "start-task":
        if len(sys.argv) < 3:
            print("Usage: state.py start-task <task_id>")
            sys.exit(1)
        state = load_state()
        success, msg = start_task(state, sys.argv[2])
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)
    
    elif cmd == "complete-task":
        if len(sys.argv) < 3:
            print("Usage: state.py complete-task <task_id> [--created f1 f2] [--modified f3 f4]")
            sys.exit(1)
        state = load_state()
        task_id = sys.argv[2]

        # Parse optional --created and --modified args
        files_created = []
        files_modified = []
        args = sys.argv[3:]
        current_list = None
        for arg in args:
            if arg == "--created":
                current_list = files_created
            elif arg == "--modified":
                current_list = files_modified
            elif current_list is not None:
                current_list.append(arg)

        success, msg = complete_task(state, task_id, files_created or None, files_modified or None)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)
    
    elif cmd == "fail-task":
        if len(sys.argv) < 4:
            print("Usage: state.py fail-task <task_id> <error>")
            sys.exit(1)
        state = load_state()
        success, msg = fail_task(state, sys.argv[2], sys.argv[3])
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "retry-task":
        if len(sys.argv) < 3:
            print("Usage: state.py retry-task <task_id>")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        success, msg = retry_task(state, sys.argv[2])
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "skip-task":
        if len(sys.argv) < 3:
            print("Usage: state.py skip-task <task_id> [reason]")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        reason = sys.argv[3] if len(sys.argv) > 3 else "Manually skipped"
        success, msg = skip_task(state, sys.argv[2], reason)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "log-tokens":
        if len(sys.argv) < 6:
            print("Usage: state.py log-tokens <session> <input> <output> <cost>")
            sys.exit(1)
        state = load_state()
        log_tokens(state, sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), float(sys.argv[5]))
        save_state(state)
        print("Tokens logged")

    elif cmd == "commit-task":
        if len(sys.argv) < 3:
            print("Usage: state.py commit-task <task_id>")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        success, msg = commit_task_changes(state, sys.argv[2])
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "record-verification":
        if len(sys.argv) < 5:
            print("Usage: state.py record-verification <task_id> --verdict PASS|FAIL|CONDITIONAL --recommendation PROCEED|BLOCK [--criteria '<json>'] [--quality '<json>'] [--tests '<json>']")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        task_id = sys.argv[2]
        verdict = None
        recommendation = None
        criteria = None
        quality = None
        tests = None

        args = sys.argv[3:]
        i = 0
        while i < len(args):
            if args[i] == "--verdict" and i + 1 < len(args):
                verdict = args[i + 1].upper()
                i += 2
            elif args[i] == "--recommendation" and i + 1 < len(args):
                recommendation = args[i + 1].upper()
                i += 2
            elif args[i] == "--criteria" and i + 1 < len(args):
                criteria = json.loads(args[i + 1])
                i += 2
            elif args[i] == "--quality" and i + 1 < len(args):
                quality = json.loads(args[i + 1])
                i += 2
            elif args[i] == "--tests" and i + 1 < len(args):
                tests = json.loads(args[i + 1])
                i += 2
            else:
                i += 1

        if not verdict or not recommendation:
            print("Error: --verdict and --recommendation are required")
            sys.exit(1)

        success, msg = record_verification(state, task_id, verdict, recommendation, criteria, quality, tests)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "metrics":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        output_format = "text"
        if len(sys.argv) > 2 and sys.argv[2] == "--format" and len(sys.argv) > 3:
            output_format = sys.argv[3]

        metrics = compute_metrics(state)

        if output_format == "json":
            print(json.dumps(metrics, indent=2))
        else:
            print("Performance Metrics")
            print("=" * 40)
            print(f"Task success rate:         {metrics['task_success_rate']:.1%}")
            print(f"First-attempt success:     {metrics['first_attempt_success_rate']:.1%}")
            print(f"Average attempts:          {metrics['avg_attempts']:.2f}")
            print(f"Tokens per task:           {metrics['tokens_per_task']:,.0f}")
            print(f"Cost per task:             ${metrics['cost_per_task']:.4f}")
            print(f"Quality pass rate:         {metrics['quality_pass_rate']:.1%}")
            print(f"Functional pass rate:      {metrics['functional_pass_rate']:.1%}")
            print(f"Test edge case rate:       {metrics['test_edge_case_rate']:.1%}")
            print("-" * 40)
            print(f"Completed: {metrics['completed_count']}  Failed: {metrics['failed_count']}")
            print(f"Total tokens: {metrics['total_tokens']:,}  Total cost: ${metrics['total_cost_usd']:.4f}")

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
