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
    state.py fail-task <task_id> <error> [--category CAT] [--subcategory SUB] [--no-retry]
                                          Mark task as failed with classification
                                          Categories: dependency, implementation,
                                          verification, environment, scope, other
    state.py retry-task <task_id>         Reset failed task to pending
    state.py skip-task <task_id> [reason] Skip task without blocking dependents
    state.py log-tokens <session> <in> <out> <cost>  Log token usage
    state.py record-verification <task_id> --verdict PASS|FAIL|CONDITIONAL
                                          --recommendation PROCEED|BLOCK
                                          [--criteria '<json>'] [--quality '<json>']
                                          [--tests '<json>']
                                          Record verification results for a task
    state.py metrics [--format text|json] Compute and display performance metrics
    state.py planning-metrics [--format text|json]
                                          Compute and display planning quality metrics
    state.py spec-coverage [--format text|json] [--save]
                                          Compute spec requirement coverage by tasks
    state.py failure-metrics [--format text|json]
                                          Show failure classification breakdown
    state.py prepare-rollback <task_id> <file1> [file2 ...]
                                          Prepare rollback data before task execution
    state.py verify-rollback <task_id> [--created f1] [--modified f2]
                                          Verify rollback restored files correctly
    state.py record-calibration <task_id> <outcome> [notes]
                                          Record verifier calibration data
    state.py calibration-score            Show verifier calibration metrics
    state.py halt [reason]                Request graceful halt of execution
    state.py check-halt                   Check if halt requested (STOP file or flag)
    state.py resume                       Clear halt flag and resume execution
    state.py halt-status                  Show current halt status
"""

import json
import hashlib
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import NoReturn


# =============================================================================
# SHIM LAYER - Forward to Go binary if available
# =============================================================================

# Commands supported by the Go binary
GO_SUPPORTED_COMMANDS = {
    "init", "status", "start-task", "complete-task", "fail-task",
    "retry-task", "skip-task", "ready-tasks", "advance", "load-tasks",
    "validate", "validate-tasks", "halt", "check-halt", "resume",
    "halt-status", "metrics", "planning-metrics", "failure-metrics",
    "log-tokens", "record-verification", "record-calibration",
    "calibration-score", "checkpoint"
}


def _find_go_binary() -> str | None:
    """Find the tasker Go binary."""
    if env_binary := os.environ.get("TASKER_BINARY"):
        if Path(env_binary).exists():
            return env_binary

    script_dir = Path(__file__).resolve().parent
    possible_paths = [
        script_dir.parent / "go" / "bin" / "tasker",
        script_dir.parent / "bin" / "tasker",
    ]
    for p in possible_paths:
        if p.exists():
            return str(p)

    import shutil
    if path := shutil.which("tasker"):
        return path

    return None


def _translate_args_to_go(args: list[str]) -> list[str]:
    """Translate Python script args to Go subcommand format."""
    if not args:
        return ["state"]

    cmd = args[0]
    rest = args[1:]

    translations = {
        "start-task": ["state", "task", "start"],
        "complete-task": ["state", "task", "complete"],
        "fail-task": ["state", "task", "fail"],
        "retry-task": ["state", "task", "retry"],
        "skip-task": ["state", "task", "skip"],
        "ready-tasks": ["state", "ready"],
        "init": ["state", "init"],
        "status": ["state", "status"],
        "advance": ["state", "advance"],
        "load-tasks": ["state", "load-tasks"],
        "validate": ["state", "validate"],
        "validate-tasks": ["state", "validate-tasks"],
        "halt": ["state", "halt"],
        "check-halt": ["state", "check-halt"],
        "resume": ["state", "resume"],
        "halt-status": ["state", "halt-status"],
        "metrics": ["state", "metrics"],
        "planning-metrics": ["state", "planning-metrics"],
        "failure-metrics": ["state", "failure-metrics"],
        "log-tokens": ["state", "log-tokens"],
        "record-verification": ["state", "record-verification"],
        "record-calibration": ["state", "record-calibration"],
        "calibration-score": ["state", "calibration-score"],
        "checkpoint": ["state", "checkpoint"],
    }

    if cmd in translations:
        go_args = translations[cmd].copy()
        translated_rest = []
        i = 0
        while i < len(rest):
            arg = rest[i]
            if arg == "--no-retry":
                translated_rest.append("--retryable=false")
            else:
                translated_rest.append(arg)
            i += 1
        go_args.extend(translated_rest)
        return go_args

    return ["state"] + args


def _try_shim_to_go() -> bool:
    """Try to forward to Go binary. Returns True if forwarded, False if fallback needed."""
    if os.environ.get("USE_PYTHON_IMPL") == "1":
        return False

    if len(sys.argv) < 2:
        return False

    cmd = sys.argv[1]
    if cmd not in GO_SUPPORTED_COMMANDS:
        return False

    binary = _find_go_binary()
    if not binary:
        return False

    go_args = _translate_args_to_go(sys.argv[1:])

    try:
        result = subprocess.run(
            [binary] + go_args,
            stdin=sys.stdin,
            stdout=sys.stdout,
            stderr=sys.stderr,
        )
        sys.exit(result.returncode)
    except Exception:
        return False


# =============================================================================
# PYTHON IMPLEMENTATION
# =============================================================================

# Paths relative to script location
SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
PLANNING_DIR = PROJECT_ROOT / "project-planning"
STATE_FILE = PLANNING_DIR / "state.json"
SCHEMAS_DIR = PROJECT_ROOT / "schemas"
STOP_FILE = PLANNING_DIR / "STOP"  # Touch file to signal halt


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
            "current_phase": 0,
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
    return ["ingestion", "spec_review", "logical", "physical", "definition", "validation", "sequencing", "ready", "executing", "complete"]


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

    elif current == "spec_review":
        review_path = PLANNING_DIR / "artifacts" / "spec-review.json"
        if not review_path.exists():
            return False, "spec-review.json not found - run spec-review.py analyze first"

        # Check if critical weaknesses are resolved
        review = json.loads(review_path.read_text())
        critical_count = review.get("summary", {}).get("by_severity", {}).get("critical", 0)

        if critical_count > 0:
            # Check for resolutions
            resolutions_path = PLANNING_DIR / "artifacts" / "spec-resolutions.json"
            if not resolutions_path.exists():
                return False, f"{critical_count} critical weaknesses require resolution"

            resolutions = json.loads(resolutions_path.read_text())
            resolved_ids = {r["weakness_id"] for r in resolutions.get("resolutions", [])}

            # Count unresolved critical weaknesses
            unresolved = sum(
                1 for w in review.get("weaknesses", [])
                if w.get("severity") == "critical" and w.get("id") not in resolved_ids
            )

            if unresolved > 0:
                return False, f"{unresolved} critical weaknesses remain unresolved"

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

        # Run automated planning gates before advancing to validation
        from validate import run_planning_gates
        results = run_planning_gates(PLANNING_DIR, spec_threshold=0.0)

        # Store validation results in state for observability
        state["artifacts"]["validation_results"] = {
            "spec_coverage": {
                "ratio": results["spec_coverage"]["report"]["coverage_ratio"],
                "passed": results["spec_coverage"]["passed"],
                "threshold": results["spec_coverage"]["report"]["threshold"],
                "timestamp": now_iso(),
            },
            "phase_leakage": {
                "passed": results["phase_leakage"]["passed"],
                "violations": results["phase_leakage"]["violations"],
            },
            "dependency_existence": {
                "passed": results["dependency_existence"]["passed"],
                "violations": results["dependency_existence"]["violations"],
            },
            "acceptance_criteria": {
                "passed": results["acceptance_criteria"]["passed"],
                "violations": results["acceptance_criteria"]["violations"],
            },
            "refactor_overrides": results["refactor_priority"]["overrides"],
            "validated_at": now_iso(),
        }

        if not results["passed"]:
            issues = "; ".join(results["blocking_issues"])
            return False, f"Planning gates failed: {issues}. Run 'python3 scripts/validate.py planning-gates' for details"

        return True, ""

    elif current == "validation":
        validation = state["artifacts"].get("task_validation", {})
        if not validation:
            return False, "Task validation not run. Spawn task-plan-verifier agent or run /verify-plan"
        if not validation.get("valid"):
            verdict = validation.get("verdict", "UNKNOWN")
            if verdict == "BLOCKED":
                return False, f"Task validation BLOCKED: {validation.get('summary', 'See task-validation-report.md')}. Fix issues and re-run /verify-plan"
            return False, "Task validation incomplete or invalid"
        return True, ""

    elif current == "sequencing":
        # Check that all tasks have phases assigned (phase can be 0, so check for None)
        for tid, task in state["tasks"].items():
            if task.get("phase") is None:
                return False, f"Task {tid} has no phase assigned"
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
        # Provide helpful guidance on the expected location
        expected_dir = artifact_path.parent
        dir_exists = expected_dir.exists()
        hint = ""
        if not dir_exists:
            hint = f" (directory {expected_dir} does not exist - agent may need to run 'mkdir -p {expected_dir}' first)"
        else:
            hint = f" (directory exists but file is missing - agent may not have written the file)"
        return False, f"Artifact not found: {path}{hint}"
    
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
                "phase": task.get("phase", 0),
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


def get_ready_tasks(state: dict, check_verification: bool = True) -> list[str]:
    """Get task IDs that are ready to execute (all deps complete and verified).

    Args:
        state: Current state dict
        check_verification: If True, also check that dependencies have PROCEED recommendation

    Returns:
        List of task IDs ready for execution
    """
    ready = []
    for tid, task in state["tasks"].items():
        if task["status"] != "pending":
            continue

        # Check all dependencies are complete
        deps_complete = True
        deps_verified = True

        for dep in task["depends_on"]:
            dep_task = state["tasks"].get(dep, {})
            dep_status = dep_task.get("status")

            # Dependency must be complete or skipped
            if dep_status not in ["complete", "skipped"]:
                deps_complete = False
                break

            # If checking verification, dependency must have PROCEED recommendation
            if check_verification and dep_status == "complete":
                verification = dep_task.get("verification", {})
                recommendation = verification.get("recommendation")
                # If verification exists and recommends BLOCK, don't allow dependents
                if recommendation == "BLOCK":
                    deps_verified = False
                    break

        if deps_complete and deps_verified:
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


FAILURE_CATEGORIES = ["dependency", "implementation", "verification", "environment", "scope", "other"]


def fail_task(
    state: dict,
    task_id: str,
    error: str,
    category: str = "other",
    subcategory: str = "",
    retryable: bool = True,
) -> tuple[bool, str]:
    """Mark task as failed with structured failure classification.

    Args:
        state: Current state dict
        task_id: Task ID to mark as failed
        error: Human-readable error message
        category: Failure category (dependency, implementation, verification,
                  environment, scope, other). Defaults to "other".
        subcategory: More specific failure type (e.g., "missing_import",
                     "test_timeout", "file_not_found")
        retryable: Whether this failure is typically recoverable

    Returns:
        (success, message) tuple
    """
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    # Validate category
    if category not in FAILURE_CATEGORIES:
        category = "other"

    task = state["tasks"][task_id]
    task["status"] = "failed"
    task["error"] = error
    task["completed_at"] = now_iso()

    # Add structured failure information
    task["failure"] = {
        "category": category,
        "subcategory": subcategory,
        "retryable": retryable,
    }

    if task_id in state["execution"]["active_tasks"]:
        state["execution"]["active_tasks"].remove(task_id)
    state["execution"]["failed_count"] += 1

    # Mark dependent tasks as blocked
    for blocked_id in task["blocks"]:
        if blocked_id in state["tasks"]:
            state["tasks"][blocked_id]["status"] = "blocked"
            state["tasks"][blocked_id]["error"] = f"Blocked by failed task {task_id}"

    add_event(state, "task_failed", task_id=task_id, details={
        "error": error,
        "category": category,
        "subcategory": subcategory,
        "retryable": retryable,
    })
    return True, f"Task {task_id} failed ({category}): {error}"


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

    # If BLOCK recommendation, mark dependent tasks as blocked
    if recommendation == "BLOCK":
        blocked_count = 0
        for blocked_id in task.get("blocks", []):
            if blocked_id in state["tasks"]:
                blocked_task = state["tasks"][blocked_id]
                if blocked_task["status"] == "pending":
                    blocked_task["status"] = "blocked"
                    blocked_task["error"] = f"Blocked by verification failure of {task_id}"
                    blocked_count += 1
        if blocked_count > 0:
            add_event(state, "tasks_blocked_by_verification", task_id=task_id, details={
                "blocked_count": blocked_count,
            })

    return True, f"Verification recorded for {task_id}: {verdict} ({recommendation})"


def prepare_rollback(state: dict, task_id: str, files_to_modify: list[str]) -> dict:
    """Prepare rollback data before task execution.

    Computes checksums for existing files that will be modified,
    enabling restoration if the task fails.

    Args:
        state: Current state dict
        task_id: Task being executed
        files_to_modify: List of relative file paths that may be modified

    Returns:
        Dict with rollback data (file checksums, timestamps)
    """
    target_dir = Path(state.get("target_dir", "."))

    rollback_data = {
        "task_id": task_id,
        "prepared_at": now_iso(),
        "target_dir": str(target_dir),
        "file_checksums": {},
        "file_existed": {},
    }

    for file_path in files_to_modify:
        full_path = target_dir / file_path
        if full_path.exists():
            rollback_data["file_checksums"][file_path] = hashlib.sha256(
                full_path.read_bytes()
            ).hexdigest()
            rollback_data["file_existed"][file_path] = True
        else:
            rollback_data["file_checksums"][file_path] = ""
            rollback_data["file_existed"][file_path] = False

    # Store in state for this task
    if task_id in state["tasks"]:
        state["tasks"][task_id]["rollback_data"] = rollback_data

    add_event(state, "rollback_prepared", task_id=task_id, details={
        "file_count": len(files_to_modify),
    })

    return rollback_data


def verify_rollback(
    state: dict,
    task_id: str,
    files_created: list[str],
    files_modified: list[str],
) -> tuple[bool, list[str]]:
    """Verify rollback restored files to original state.

    Args:
        state: Current state dict
        task_id: Task that was rolled back
        files_created: Files that should have been deleted
        files_modified: Files that should have been restored

    Returns:
        (success, issues) tuple
    """
    if task_id not in state["tasks"]:
        return False, [f"Task not found: {task_id}"]

    task = state["tasks"][task_id]
    rollback_data = task.get("rollback_data", {})

    if not rollback_data:
        return False, ["No rollback data found for task"]

    target_dir = Path(rollback_data.get("target_dir", state.get("target_dir", ".")))
    original_checksums = rollback_data.get("file_checksums", {})
    file_existed = rollback_data.get("file_existed", {})

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
        existed = file_existed.get(file_path, False)

        if not existed:
            # File didn't exist before, should not exist now
            if full_path.exists():
                issues.append(f"File should not exist after rollback: {file_path}")
        else:
            if not full_path.exists():
                issues.append(f"File should exist after rollback: {file_path}")
            else:
                current = hashlib.sha256(full_path.read_bytes()).hexdigest()
                if current != original:
                    issues.append(
                        f"File not restored to original: {file_path} "
                        f"(expected {original[:8]}..., got {current[:8]}...)"
                    )

    success = len(issues) == 0
    add_event(state, "rollback_verified", task_id=task_id, details={
        "success": success,
        "issue_count": len(issues),
    })

    return success, issues


def record_calibration(
    state: dict,
    task_id: str,
    actual_outcome: str,
    notes: str = "",
) -> tuple[bool, str]:
    """Record calibration data for verifier accuracy tracking.

    Call this when a verification verdict is later proven correct or incorrect.

    Args:
        state: Current state dict
        task_id: Task to record calibration for
        actual_outcome: "correct" if verdict matched reality, "false_positive" if
                        PROCEED but task actually failed, "false_negative" if
                        BLOCK but task would have succeeded
        notes: Optional notes about the calibration

    Returns:
        (success, message) tuple
    """
    if task_id not in state["tasks"]:
        return False, f"Task not found: {task_id}"

    valid_outcomes = ["correct", "false_positive", "false_negative"]
    if actual_outcome not in valid_outcomes:
        return False, f"Invalid outcome: {actual_outcome}. Must be one of {valid_outcomes}"

    task = state["tasks"][task_id]
    verification = task.get("verification", {})

    if not verification:
        return False, f"No verification data for {task_id}"

    # Initialize calibration tracking in state if not present
    if "calibration" not in state:
        state["calibration"] = {
            "total_verified": 0,
            "correct": 0,
            "false_positives": [],
            "false_negatives": [],
            "history": [],
        }

    cal = state["calibration"]

    # Record this calibration
    entry = {
        "task_id": task_id,
        "verdict": verification.get("verdict"),
        "recommendation": verification.get("recommendation"),
        "actual_outcome": actual_outcome,
        "notes": notes,
        "recorded_at": now_iso(),
    }

    cal["history"].append(entry)
    cal["total_verified"] += 1

    if actual_outcome == "correct":
        cal["correct"] += 1
    elif actual_outcome == "false_positive":
        cal["false_positives"].append(task_id)
    elif actual_outcome == "false_negative":
        cal["false_negatives"].append(task_id)

    add_event(state, "calibration_recorded", task_id=task_id, details={
        "actual_outcome": actual_outcome,
        "verdict": verification.get("verdict"),
    })

    return True, f"Calibration recorded for {task_id}: {actual_outcome}"


def get_calibration_score(state: dict) -> float:
    """Compute current calibration score (1.0 = perfect).

    Returns:
        Float between 0.0 and 1.0 representing verifier accuracy
    """
    cal = state.get("calibration", {})
    total = cal.get("total_verified", 0)

    if total == 0:
        return 1.0  # No data yet, assume perfect

    correct = cal.get("correct", 0)
    return correct / total


# =============================================================================
# Graceful Halt / Resume
# =============================================================================


def check_stop_file() -> bool:
    """Check if STOP file exists in project-planning directory."""
    return STOP_FILE.exists()


def remove_stop_file() -> bool:
    """Remove STOP file if it exists. Returns True if file was removed."""
    if STOP_FILE.exists():
        STOP_FILE.unlink()
        return True
    return False


def check_halt(state: dict) -> tuple[bool, str | None]:
    """Check if halt has been requested via STOP file or state flag.

    Returns:
        (is_halted, reason) tuple
    """
    # Check STOP file first (takes precedence)
    if check_stop_file():
        return True, "stop_file"

    # Check state flag
    halt = state.get("halt", {})
    if halt.get("requested"):
        return True, halt.get("reason", "manual")

    return False, None


def request_halt(
    state: dict,
    reason: str = "manual",
    active_task: str | None = None,
) -> tuple[bool, str]:
    """Request graceful halt of execution.

    Args:
        state: Current state dict
        reason: Why halt was requested (user_message, stop_file, manual)
        active_task: Task that was running when halt was requested

    Returns:
        (success, message) tuple
    """
    # Initialize halt section if not present
    if "halt" not in state:
        state["halt"] = {
            "requested": False,
            "requested_at": None,
            "reason": None,
            "halted_at": None,
            "active_task": None,
            "resumable": True,
        }

    halt = state["halt"]

    # Already halted?
    if halt.get("requested"):
        return False, f"Halt already requested at {halt.get('requested_at')}"

    halt["requested"] = True
    halt["requested_at"] = now_iso()
    halt["reason"] = reason
    halt["active_task"] = active_task
    halt["resumable"] = True

    add_event(state, "halt_requested", details={
        "reason": reason,
        "active_task": active_task,
    })

    return True, f"Halt requested: {reason}"


def confirm_halt(state: dict) -> tuple[bool, str]:
    """Confirm that halt has completed (all tasks stopped).

    Called after the executor has finished its current task and stopped.

    Returns:
        (success, message) tuple
    """
    halt = state.get("halt", {})
    if not halt.get("requested"):
        return False, "No halt was requested"

    halt["halted_at"] = now_iso()

    # Check for any running tasks
    running_tasks = [
        tid for tid, task in state.get("tasks", {}).items()
        if task.get("status") == "running"
    ]

    if running_tasks:
        halt["active_task"] = running_tasks[0]  # Track first running task

    add_event(state, "halt_confirmed", details={
        "running_tasks": running_tasks,
    })

    return True, f"Halt confirmed. Running tasks: {running_tasks}"


def resume_execution(state: dict) -> tuple[bool, str]:
    """Clear halt flag and prepare for resumed execution.

    Returns:
        (success, message) tuple
    """
    halt = state.get("halt", {})

    if not halt.get("requested"):
        return False, "No halt to resume from"

    if not halt.get("resumable", True):
        return False, "Halt is not resumable (requires manual intervention)"

    # Remove STOP file if present
    stop_file_removed = remove_stop_file()

    # Clear halt state
    previous_reason = halt.get("reason")
    previous_task = halt.get("active_task")

    state["halt"] = {
        "requested": False,
        "requested_at": None,
        "reason": None,
        "halted_at": None,
        "active_task": None,
        "resumable": True,
    }

    add_event(state, "execution_resumed", details={
        "previous_reason": previous_reason,
        "previous_active_task": previous_task,
        "stop_file_removed": stop_file_removed,
    })

    msg = "Execution resumed"
    if stop_file_removed:
        msg += " (STOP file removed)"
    if previous_task:
        msg += f". Previously active task: {previous_task}"

    return True, msg


def get_halt_status(state: dict) -> dict:
    """Get current halt status including STOP file check.

    Returns:
        Dict with halt status information
    """
    halt = state.get("halt", {})
    stop_file_exists = check_stop_file()

    is_halted, reason = check_halt(state)

    # Find any running tasks
    running_tasks = [
        tid for tid, task in state.get("tasks", {}).items()
        if task.get("status") == "running"
    ]

    return {
        "is_halted": is_halted,
        "reason": reason,
        "requested_at": halt.get("requested_at"),
        "halted_at": halt.get("halted_at"),
        "stop_file_exists": stop_file_exists,
        "active_task": halt.get("active_task"),
        "running_tasks": running_tasks,
        "resumable": halt.get("resumable", True),
    }


def compute_planning_metrics(state: dict) -> dict:
    """Compute planning quality metrics.

    Returns:
        Dict with planning metrics:
        - total_tasks: Number of tasks defined
        - total_behaviors: Number of behaviors across all tasks
        - avg_behaviors_per_task: Average behaviors per task (target: 2-5)
        - avg_criteria_per_task: Average acceptance criteria per task
        - avg_files_per_task: Average files per task
        - dependency_density: Avg dependencies per task
        - phase_count: Number of phases
        - phase_compression: Actual phases / min possible phases
        - steel_thread_coverage: % of tasks on steel thread
        - spec_coverage: % of behaviors traced to spec (if available)
    """
    tasks = state.get("tasks", {})

    if not tasks:
        return {
            "total_tasks": 0,
            "total_behaviors": 0,
            "avg_behaviors_per_task": 0.0,
            "avg_criteria_per_task": 0.0,
            "avg_files_per_task": 0.0,
            "dependency_density": 0.0,
            "phase_count": 0,
            "phase_compression": 0.0,
            "steel_thread_coverage": 0.0,
        }

    # Load task files to get full details
    tasks_dir = PLANNING_DIR / "tasks"
    total_behaviors = 0
    total_criteria = 0
    total_files = 0
    total_deps = 0
    steel_thread_tasks = 0
    phases = set()

    for tid in tasks:
        task_path = tasks_dir / f"{tid}.json"
        if task_path.exists():
            try:
                task_def = json.loads(task_path.read_text())
                total_behaviors += len(task_def.get("behaviors", []))
                total_criteria += len(task_def.get("acceptance_criteria", []))
                total_files += len(task_def.get("files", []))
                total_deps += len(task_def.get("dependencies", {}).get("tasks", []))
                if task_def.get("context", {}).get("steel_thread"):
                    steel_thread_tasks += 1
            except (json.JSONDecodeError, KeyError):
                pass

        phase = tasks[tid].get("phase", 0)
        if phase > 0:
            phases.add(phase)

    task_count = len(tasks)
    phase_count = len(phases) if phases else 0

    # Compute minimum possible phases (longest dependency chain + 1)
    # This is a simplified computation
    min_phases = 1
    if task_count > 0:
        # Count tasks with no deps as phase 1, rest need at least 1 more
        no_dep_count = sum(1 for t in tasks.values() if not t.get("depends_on"))
        if no_dep_count < task_count:
            min_phases = 2  # At least 2 phases if there are dependencies

    phase_compression = phase_count / min_phases if min_phases > 0 else 1.0

    return {
        "total_tasks": task_count,
        "total_behaviors": total_behaviors,
        "avg_behaviors_per_task": total_behaviors / task_count if task_count > 0 else 0.0,
        "avg_criteria_per_task": total_criteria / task_count if task_count > 0 else 0.0,
        "avg_files_per_task": total_files / task_count if task_count > 0 else 0.0,
        "dependency_density": total_deps / task_count if task_count > 0 else 0.0,
        "phase_count": phase_count,
        "phase_compression": phase_compression,
        "steel_thread_coverage": steel_thread_tasks / task_count if task_count > 0 else 0.0,
    }


def extract_requirements_from_spec(spec_path: Path) -> list[dict]:
    """Extract requirement markers from a spec file.

    Recognizes patterns:
    - REQ-001: Description
    - [R1] Description
    - [R1.2] Description
    - **REQ-001**: Description
    - Requirement 1: Description

    Args:
        spec_path: Path to spec.md file

    Returns:
        List of {id, text, source_line} dicts
    """
    if not spec_path.exists():
        return []

    content = spec_path.read_text()
    lines = content.split("\n")
    requirements = []

    # Patterns to match requirement markers
    patterns = [
        r"^(?:\*\*)?(?:REQ-\d+|R\d+(?:\.\d+)?)\*?\*?[:\s]+(.+)$",  # REQ-001: or [R1]:
        r"^\[(?:REQ-\d+|R\d+(?:\.\d+)?)\][:\s]*(.+)$",  # [REQ-001] or [R1.2]
        r"^Requirement\s+(\d+)[:\s]+(.+)$",  # Requirement 1:
        r"^-\s*\[(?:REQ-\d+|R\d+(?:\.\d+)?)\][:\s]*(.+)$",  # - [REQ-001] list item
    ]

    # Pattern to extract the ID
    id_pattern = r"(REQ-\d+|R\d+(?:\.\d+)?)"

    for line_num, line in enumerate(lines, 1):
        line = line.strip()
        for pattern in patterns:
            match = re.match(pattern, line, re.IGNORECASE)
            if match:
                # Extract the requirement ID
                id_match = re.search(id_pattern, line, re.IGNORECASE)
                if id_match:
                    req_id = id_match.group(1).upper()
                    # Get the text (last capture group from the pattern match)
                    text = match.group(match.lastindex) if match.lastindex else line
                    requirements.append({
                        "id": req_id,
                        "text": text.strip(),
                        "source_line": line_num,
                    })
                break

    return requirements


def compute_spec_coverage(state: dict) -> dict:
    """Compute spec requirement coverage by tasks.

    Extracts requirements from spec.md, maps to tasks via spec_requirements,
    and computes coverage metrics.

    Args:
        state: Current state dict

    Returns:
        Dict with coverage data:
        - requirements: List of {id, text, source_line, covered_by}
        - coverage_pct: Percentage of requirements covered
        - covered_count: Number of covered requirements
        - total_count: Total requirements found
        - uncovered: List of uncovered requirement IDs
    """
    spec_path = PLANNING_DIR / "inputs" / "spec.md"
    requirements = extract_requirements_from_spec(spec_path)

    if not requirements:
        return {
            "requirements": [],
            "coverage_pct": 0.0,
            "covered_count": 0,
            "total_count": 0,
            "uncovered": [],
            "computed_at": now_iso(),
        }

    # Build requirement ID -> requirement mapping
    req_by_id = {r["id"]: r for r in requirements}
    for r in requirements:
        r["covered_by"] = []

    # Load task files and find coverage
    tasks_dir = PLANNING_DIR / "tasks"
    for tid in state.get("tasks", {}):
        task_path = tasks_dir / f"{tid}.json"
        if task_path.exists():
            try:
                task_def = json.loads(task_path.read_text())
                context = task_def.get("context", {})

                # Check spec_requirements (explicit list)
                spec_reqs = context.get("spec_requirements", [])
                for req_id in spec_reqs:
                    req_id_upper = req_id.upper()
                    if req_id_upper in req_by_id:
                        req_by_id[req_id_upper]["covered_by"].append(tid)

                # Also check spec_ref for implicit coverage
                spec_ref = context.get("spec_ref", "")
                if spec_ref:
                    # Try to extract requirement IDs from spec_ref
                    id_matches = re.findall(r"(REQ-\d+|R\d+(?:\.\d+)?)", spec_ref, re.IGNORECASE)
                    for req_id in id_matches:
                        req_id_upper = req_id.upper()
                        if req_id_upper in req_by_id:
                            if tid not in req_by_id[req_id_upper]["covered_by"]:
                                req_by_id[req_id_upper]["covered_by"].append(tid)

            except (json.JSONDecodeError, KeyError):
                pass

    # Compute coverage metrics
    covered = [r for r in requirements if r["covered_by"]]
    uncovered = [r["id"] for r in requirements if not r["covered_by"]]

    total = len(requirements)
    covered_count = len(covered)
    coverage_pct = (covered_count / total * 100) if total > 0 else 0.0

    return {
        "requirements": requirements,
        "coverage_pct": round(coverage_pct, 1),
        "covered_count": covered_count,
        "total_count": total,
        "uncovered": uncovered,
        "computed_at": now_iso(),
    }


def update_spec_coverage(state: dict) -> tuple[bool, str]:
    """Compute and store spec coverage in state artifacts.

    Args:
        state: Current state dict

    Returns:
        (success, message) tuple
    """
    coverage = compute_spec_coverage(state)
    state["artifacts"]["spec_coverage"] = coverage

    add_event(state, "spec_coverage_computed", details={
        "coverage_pct": coverage["coverage_pct"],
        "covered_count": coverage["covered_count"],
        "total_count": coverage["total_count"],
    })

    return True, f"Spec coverage: {coverage['covered_count']}/{coverage['total_count']} ({coverage['coverage_pct']}%)"


def compute_failure_metrics(state: dict) -> dict:
    """Compute failure classification metrics.

    Args:
        state: Current state dict

    Returns:
        Dict with failure breakdown by category
    """
    tasks = state.get("tasks", {})

    by_category: dict[str, list[str]] = {cat: [] for cat in FAILURE_CATEGORIES}
    by_subcategory: dict[str, int] = {}
    retryable_count = 0
    non_retryable_count = 0

    for tid, task in tasks.items():
        if task.get("status") == "failed":
            failure = task.get("failure", {})
            category = failure.get("category", "other")
            subcategory = failure.get("subcategory", "")
            retryable = failure.get("retryable", True)

            by_category[category].append(tid)

            if subcategory:
                by_subcategory[subcategory] = by_subcategory.get(subcategory, 0) + 1

            if retryable:
                retryable_count += 1
            else:
                non_retryable_count += 1

    total_failed = sum(len(tasks) for tasks in by_category.values())

    return {
        "total_failed": total_failed,
        "by_category": {cat: len(tasks) for cat, tasks in by_category.items()},
        "by_category_tasks": by_category,
        "by_subcategory": by_subcategory,
        "retryable_count": retryable_count,
        "non_retryable_count": non_retryable_count,
        "category_pct": {
            cat: (len(tasks) / total_failed * 100) if total_failed > 0 else 0.0
            for cat, tasks in by_category.items()
        },
    }


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
    # Check halt status first
    is_halted, halt_reason = check_halt(state)
    if is_halted:
        print(f"⚠️  HALTED: {halt_reason}")
        halt = state.get("halt", {})
        if halt.get("active_task"):
            print(f"   Active task at halt: {halt['active_task']}")
        print()

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
        if is_halted:
            print(f"Ready tasks (paused): {ready}")
        else:
            print(f"Ready tasks: {ready}")


def main():
    # Try to forward supported commands to Go binary
    _try_shim_to_go()

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
            print(f"{tid}: {task['name']} (phase {task['phase']})")
    
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
            print("Usage: state.py fail-task <task_id> <error> [--category CAT] [--subcategory SUB] [--no-retry]")
            print(f"  Categories: {', '.join(FAILURE_CATEGORIES)}")
            sys.exit(1)
        state = load_state()

        task_id = sys.argv[2]
        error = sys.argv[3]
        category = "other"
        subcategory = ""
        retryable = True

        # Parse optional args
        args = sys.argv[4:]
        i = 0
        while i < len(args):
            if args[i] == "--category" and i + 1 < len(args):
                category = args[i + 1].lower()
                i += 2
            elif args[i] == "--subcategory" and i + 1 < len(args):
                subcategory = args[i + 1]
                i += 2
            elif args[i] == "--no-retry":
                retryable = False
                i += 1
            else:
                i += 1

        success, msg = fail_task(state, task_id, error, category, subcategory, retryable)
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

    elif cmd == "planning-metrics":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        output_format = "text"
        if len(sys.argv) > 2 and sys.argv[2] == "--format" and len(sys.argv) > 3:
            output_format = sys.argv[3]

        metrics = compute_planning_metrics(state)

        if output_format == "json":
            print(json.dumps(metrics, indent=2))
        else:
            print("Planning Quality Metrics")
            print("=" * 40)
            print(f"Total tasks:               {metrics['total_tasks']}")
            print(f"Total behaviors:           {metrics['total_behaviors']}")
            print(f"Avg behaviors/task:        {metrics['avg_behaviors_per_task']:.1f} (target: 2-5)")
            print(f"Avg criteria/task:         {metrics['avg_criteria_per_task']:.1f}")
            print(f"Avg files/task:            {metrics['avg_files_per_task']:.1f}")
            print(f"Dependency density:        {metrics['dependency_density']:.2f}")
            print(f"Phase count:                {metrics['phase_count']}")
            print(f"Phase compression:          {metrics['phase_compression']:.2f}x")
            print(f"Steel thread coverage:     {metrics['steel_thread_coverage']:.1%}")

    elif cmd == "prepare-rollback":
        if len(sys.argv) < 4:
            print("Usage: state.py prepare-rollback <task_id> <file1> [file2 ...]")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)
        task_id = sys.argv[2]
        files = sys.argv[3:]
        prepare_rollback(state, task_id, files)
        save_state(state)
        print(f"Rollback prepared for {len(files)} file(s)")

    elif cmd == "verify-rollback":
        if len(sys.argv) < 3:
            print("Usage: state.py verify-rollback <task_id> [--created f1 f2] [--modified f3 f4]")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        task_id = sys.argv[2]
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

        success, issues = verify_rollback(state, task_id, files_created, files_modified)
        save_state(state)
        if success:
            print("Rollback verified successfully")
        else:
            print("Rollback verification failed:")
            for issue in issues:
                print(f"  - {issue}")
        sys.exit(0 if success else 1)

    elif cmd == "record-calibration":
        if len(sys.argv) < 4:
            print("Usage: state.py record-calibration <task_id> <outcome> [notes]")
            print("  outcome: correct, false_positive, or false_negative")
            sys.exit(1)
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        task_id = sys.argv[2]
        outcome = sys.argv[3]
        notes = sys.argv[4] if len(sys.argv) > 4 else ""

        success, msg = record_calibration(state, task_id, outcome, notes)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "calibration-score":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        score = get_calibration_score(state)
        cal = state.get("calibration", {})

        print("Verifier Calibration")
        print("=" * 40)
        print(f"Calibration score:         {score:.1%}")
        print(f"Total verified:            {cal.get('total_verified', 0)}")
        print(f"Correct:                   {cal.get('correct', 0)}")
        print(f"False positives:           {len(cal.get('false_positives', []))}")
        print(f"False negatives:           {len(cal.get('false_negatives', []))}")

    elif cmd == "spec-coverage":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        output_format = "text"
        if len(sys.argv) > 2 and sys.argv[2] == "--format" and len(sys.argv) > 3:
            output_format = sys.argv[3]

        # Compute and optionally save
        save_to_state = "--save" in sys.argv
        coverage = compute_spec_coverage(state)

        if save_to_state:
            state["artifacts"]["spec_coverage"] = coverage
            save_state(state)

        if output_format == "json":
            print(json.dumps(coverage, indent=2))
        else:
            print("Spec Coverage Report")
            print("=" * 40)
            print(f"Total requirements:        {coverage['total_count']}")
            print(f"Covered:                   {coverage['covered_count']}")
            print(f"Coverage:                  {coverage['coverage_pct']}%")
            print()
            if coverage["uncovered"]:
                print("Uncovered Requirements:")
                for req_id in coverage["uncovered"]:
                    req = next((r for r in coverage["requirements"] if r["id"] == req_id), None)
                    if req:
                        print(f"  {req_id} (line {req['source_line']}): {req['text'][:60]}...")
            else:
                print("All requirements covered!")

    elif cmd == "failure-metrics":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        output_format = "text"
        if len(sys.argv) > 2 and sys.argv[2] == "--format" and len(sys.argv) > 3:
            output_format = sys.argv[3]

        metrics = compute_failure_metrics(state)

        if output_format == "json":
            print(json.dumps(metrics, indent=2))
        else:
            print("Failure Classification Report")
            print("=" * 40)
            print(f"Total failed tasks:        {metrics['total_failed']}")
            print()
            print("By Category:")
            for cat in FAILURE_CATEGORIES:
                count = metrics["by_category"].get(cat, 0)
                pct = metrics["category_pct"].get(cat, 0.0)
                tasks = metrics["by_category_tasks"].get(cat, [])
                if count > 0:
                    print(f"  {cat:15} {count:3} ({pct:5.1f}%)  {', '.join(tasks[:3])}")
            print()
            if metrics["by_subcategory"]:
                print("By Subcategory:")
                for sub, count in sorted(metrics["by_subcategory"].items(), key=lambda x: -x[1]):
                    print(f"  {sub:20} {count}")
            print()
            print(f"Retryable:                 {metrics['retryable_count']}")
            print(f"Non-retryable:             {metrics['non_retryable_count']}")

    elif cmd == "halt":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        reason = sys.argv[2] if len(sys.argv) > 2 else "manual"

        # Get current active task if any
        active_tasks = state["execution"].get("active_tasks", [])
        active_task = active_tasks[0] if active_tasks else None

        success, msg = request_halt(state, reason, active_task)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "check-halt":
        state = load_state()
        if not state:
            # Even without state, check for STOP file
            if check_stop_file():
                print("HALTED: stop_file")
                sys.exit(1)
            print("OK")
            sys.exit(0)

        is_halted, reason = check_halt(state)
        if is_halted:
            print(f"HALTED: {reason}")
            sys.exit(1)
        else:
            print("OK")
            sys.exit(0)

    elif cmd == "resume":
        state = load_state()
        if not state:
            # Even without state, we can remove the STOP file
            if remove_stop_file():
                print("STOP file removed. No state to resume.")
                sys.exit(0)
            else:
                print("No halt state or STOP file to clear.")
                sys.exit(1)

        success, msg = resume_execution(state)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "halt-status":
        state = load_state()
        if not state:
            # Just check STOP file
            stop_exists = check_stop_file()
            status = {
                "is_halted": stop_exists,
                "reason": "stop_file" if stop_exists else None,
                "stop_file_exists": stop_exists,
                "running_tasks": [],
                "resumable": True,
            }
        else:
            status = get_halt_status(state)

        output_format = "text"
        if len(sys.argv) > 2 and sys.argv[2] == "--format" and len(sys.argv) > 3:
            output_format = sys.argv[3]

        if output_format == "json":
            print(json.dumps(status, indent=2))
        else:
            print("Halt Status")
            print("=" * 40)
            print(f"Halted:                    {'YES' if status['is_halted'] else 'NO'}")
            if status["is_halted"]:
                print(f"Reason:                    {status['reason']}")
                if status.get("requested_at"):
                    print(f"Requested at:              {status['requested_at']}")
                if status.get("halted_at"):
                    print(f"Halted at:                 {status['halted_at']}")
            print(f"STOP file exists:          {'YES' if status['stop_file_exists'] else 'NO'}")
            if status.get("active_task"):
                print(f"Active task at halt:       {status['active_task']}")
            if status.get("running_tasks"):
                print(f"Running tasks:             {', '.join(status['running_tasks'])}")
            print(f"Resumable:                 {'YES' if status['resumable'] else 'NO'}")

    elif cmd == "confirm-halt":
        state = load_state()
        if not state:
            print("No state file.")
            sys.exit(1)

        success, msg = confirm_halt(state)
        if success:
            save_state(state)
        print(msg)
        sys.exit(0 if success else 1)

    # =========================================================================
    # CHECKPOINT COMMANDS - Orchestrator crash recovery
    # =========================================================================

    elif cmd == "checkpoint":
        if len(sys.argv) < 3:
            print("Usage: state.py checkpoint <subcommand> [args]")
            print("  create <task1> [task2 ...]  - Create checkpoint for batch")
            print("  update <task_id> <status>   - Update task status (success|failed)")
            print("  complete                    - Mark batch complete")
            print("  status                      - Show current checkpoint")
            print("  recover                     - Recover orphaned tasks")
            print("  clear                       - Remove checkpoint file")
            sys.exit(1)

        subcmd = sys.argv[2]
        checkpoint_path = PLANNING_DIR / "orchestrator-checkpoint.json"

        if subcmd == "create":
            if len(sys.argv) < 4:
                print("Usage: state.py checkpoint create <task1> [task2 ...]")
                sys.exit(1)

            tasks = sys.argv[3:]
            batch_id = f"batch-{now_iso().replace(':', '-').replace('.', '-')}"

            checkpoint = {
                "version": "1.0",
                "batch_id": batch_id,
                "spawned_at": now_iso(),
                "status": "active",
                "parallel_limit": 3,
                "tasks": {
                    "spawned": tasks,
                    "pending": tasks.copy(),
                    "completed": [],
                    "failed": []
                },
                "last_heartbeat": now_iso(),
                "orphan_timeout_seconds": 1800
            }

            with open(checkpoint_path, "w") as f:
                json.dump(checkpoint, f, indent=2)

            print(f"Checkpoint created: {batch_id}")
            print(f"Tasks: {', '.join(tasks)}")
            sys.exit(0)

        elif subcmd == "update":
            if len(sys.argv) < 5:
                print("Usage: state.py checkpoint update <task_id> <status>")
                print("  status: success | failed")
                sys.exit(1)

            task_id = sys.argv[3]
            status = sys.argv[4]

            if not checkpoint_path.exists():
                print("No active checkpoint.")
                sys.exit(1)

            with open(checkpoint_path) as f:
                checkpoint = json.load(f)

            if task_id in checkpoint["tasks"]["pending"]:
                checkpoint["tasks"]["pending"].remove(task_id)

            if status == "success":
                if task_id not in checkpoint["tasks"]["completed"]:
                    checkpoint["tasks"]["completed"].append(task_id)
            elif status == "failed":
                if task_id not in checkpoint["tasks"]["failed"]:
                    checkpoint["tasks"]["failed"].append(task_id)
            else:
                print(f"Invalid status: {status}")
                sys.exit(1)

            checkpoint["last_heartbeat"] = now_iso()

            with open(checkpoint_path, "w") as f:
                json.dump(checkpoint, f, indent=2)

            remaining = len(checkpoint["tasks"]["pending"])
            print(f"Updated {task_id}: {status} ({remaining} pending)")
            sys.exit(0)

        elif subcmd == "complete":
            if not checkpoint_path.exists():
                print("No active checkpoint.")
                sys.exit(1)

            with open(checkpoint_path) as f:
                checkpoint = json.load(f)

            checkpoint["status"] = "completed"
            checkpoint["completed_at"] = now_iso()
            checkpoint["last_heartbeat"] = now_iso()

            with open(checkpoint_path, "w") as f:
                json.dump(checkpoint, f, indent=2)

            print(f"Batch {checkpoint['batch_id']} completed.")
            print(f"  Completed: {len(checkpoint['tasks']['completed'])}")
            print(f"  Failed: {len(checkpoint['tasks']['failed'])}")
            sys.exit(0)

        elif subcmd == "status":
            if not checkpoint_path.exists():
                print("No active checkpoint.")
                sys.exit(0)

            with open(checkpoint_path) as f:
                checkpoint = json.load(f)

            output_format = "text"
            if "--format" in sys.argv and "json" in sys.argv:
                output_format = "json"

            if output_format == "json":
                print(json.dumps(checkpoint, indent=2))
            else:
                print("Orchestrator Checkpoint")
                print("=" * 40)
                print(f"Batch ID:                  {checkpoint['batch_id']}")
                print(f"Status:                    {checkpoint['status']}")
                print(f"Spawned at:                {checkpoint['spawned_at']}")
                print(f"Last heartbeat:            {checkpoint['last_heartbeat']}")
                print()
                print(f"Spawned tasks:             {len(checkpoint['tasks']['spawned'])}")
                print(f"Pending:                   {len(checkpoint['tasks']['pending'])}")
                if checkpoint['tasks']['pending']:
                    print(f"  {', '.join(checkpoint['tasks']['pending'])}")
                print(f"Completed:                 {len(checkpoint['tasks']['completed'])}")
                print(f"Failed:                    {len(checkpoint['tasks']['failed'])}")
            sys.exit(0)

        elif subcmd == "recover":
            state = load_state()
            if not state:
                print("No state file.")
                sys.exit(1)

            if not checkpoint_path.exists():
                print("No checkpoint to recover from.")
                sys.exit(0)

            with open(checkpoint_path) as f:
                checkpoint = json.load(f)

            if checkpoint["status"] != "active":
                print(f"Checkpoint status is '{checkpoint['status']}', not active.")
                sys.exit(0)

            # Find orphaned tasks: pending in checkpoint but have result files
            orphaned = []
            recovered = []
            bundles_dir = PLANNING_DIR / "bundles"

            for task_id in checkpoint["tasks"]["pending"]:
                result_file = bundles_dir / f"{task_id}-result.json"
                if result_file.exists():
                    # Task completed but checkpoint wasn't updated
                    with open(result_file) as f:
                        result = json.load(f)
                    status = result.get("status", "unknown")
                    recovered.append((task_id, status))
                else:
                    # Task may be orphaned (executor died without writing result)
                    task = state["tasks"].get(task_id, {})
                    if task.get("status") == "running":
                        orphaned.append(task_id)

            # Update checkpoint with recovered tasks
            for task_id, status in recovered:
                if task_id in checkpoint["tasks"]["pending"]:
                    checkpoint["tasks"]["pending"].remove(task_id)
                if status == "success":
                    checkpoint["tasks"]["completed"].append(task_id)
                else:
                    checkpoint["tasks"]["failed"].append(task_id)

            # Record recovery info
            if recovered or orphaned:
                checkpoint["recovery"] = {
                    "recovered_at": now_iso(),
                    "recovered_tasks": [t for t, _ in recovered],
                    "orphaned_tasks": orphaned,
                    "previous_batch_id": checkpoint["batch_id"]
                }
                checkpoint["last_heartbeat"] = now_iso()

                with open(checkpoint_path, "w") as f:
                    json.dump(checkpoint, f, indent=2)

            print("Checkpoint Recovery")
            print("=" * 40)
            if recovered:
                print(f"Recovered from result files: {len(recovered)}")
                for task_id, status in recovered:
                    print(f"  {task_id}: {status}")
            else:
                print("No tasks recovered from result files.")

            if orphaned:
                print(f"\nOrphaned tasks (need retry): {len(orphaned)}")
                for task_id in orphaned:
                    print(f"  {task_id}")
                print("\nTo reset orphaned tasks:")
                print("  python3 scripts/state.py retry-task <task_id>")
            else:
                print("No orphaned tasks.")

            print(f"\nPending: {len(checkpoint['tasks']['pending'])}")
            sys.exit(0)

        elif subcmd == "clear":
            if checkpoint_path.exists():
                checkpoint_path.unlink()
                print("Checkpoint cleared.")
            else:
                print("No checkpoint to clear.")
            sys.exit(0)

        else:
            print(f"Unknown checkpoint subcommand: {subcmd}")
            sys.exit(1)

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
