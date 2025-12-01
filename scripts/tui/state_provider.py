"""
Concrete implementation of StateProvider using the existing state.py module.
"""

from __future__ import annotations

import json
import sys
from datetime import datetime
from pathlib import Path

# Add scripts to path for imports
SCRIPT_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from tui.providers import (  # noqa: E402
    CalibrationInfo,
    ExecutionInfo,
    HealthCheck,
    PhaseInfo,
    TaskInfo,
    WorkflowState,
)


def _parse_datetime(s: str | None) -> datetime | None:
    """Parse ISO datetime string."""
    if not s:
        return None
    try:
        return datetime.fromisoformat(s.replace("Z", "+00:00"))
    except (ValueError, TypeError):
        return None


def _task_from_dict(tid: str, data: dict) -> TaskInfo:
    """Convert task dict to TaskInfo."""
    return TaskInfo(
        id=tid,
        name=data.get("name", ""),
        status=data.get("status", "pending"),
        phase=data.get("phase", 0),
        depends_on=tuple(data.get("depends_on", [])),
        blocks=tuple(data.get("blocks", [])),
        attempts=data.get("attempts", 0),
        started_at=_parse_datetime(data.get("started_at")),
        completed_at=_parse_datetime(data.get("completed_at")),
        duration_seconds=data.get("duration_seconds"),
        error=data.get("error"),
        files_created=tuple(data.get("files_created", [])),
        files_modified=tuple(data.get("files_modified", [])),
        verification=data.get("verification", {}),
    )


class FileStateProvider:
    """StateProvider implementation that reads from state.json."""

    def __init__(self, state_file: Path | None = None):
        if state_file is None:
            project_root = Path(__file__).resolve().parent.parent.parent
            state_file = project_root / "project-planning" / "state.json"
        self._state_file = state_file
        self._validation_provider: "FileValidationProvider | None" = None

    def _get_validation_provider(self) -> "FileValidationProvider":
        """Lazy-load validation provider."""
        if self._validation_provider is None:
            self._validation_provider = FileValidationProvider(self._state_file)
        return self._validation_provider

    def load(self) -> WorkflowState | None:
        """Load current workflow state."""
        if not self._state_file.exists():
            return None

        try:
            data = json.loads(self._state_file.read_text())
        except (json.JSONDecodeError, OSError):
            return None

        phase_data = data.get("phase", {})
        phase = PhaseInfo(
            current=phase_data.get("current", "unknown"),
            completed=tuple(phase_data.get("completed", [])),
        )

        exec_data = data.get("execution", {})
        execution = ExecutionInfo(
            current_phase=exec_data.get("current_phase", 0),
            active_tasks=tuple(exec_data.get("active_tasks", [])),
            completed_count=exec_data.get("completed_count", 0),
            failed_count=exec_data.get("failed_count", 0),
            total_tokens=exec_data.get("total_tokens", 0),
            total_cost_usd=exec_data.get("total_cost_usd", 0.0),
        )

        tasks = {
            tid: _task_from_dict(tid, task_data)
            for tid, task_data in data.get("tasks", {}).items()
        }

        # Run health checks
        vp = self._get_validation_provider()
        health_checks = vp.run_all()
        calibration = vp.get_calibration()

        return WorkflowState(
            phase=phase,
            target_dir=data.get("target_dir", ""),
            tasks=tasks,
            execution=execution,
            created_at=_parse_datetime(data.get("created_at")) or datetime.now(),
            updated_at=_parse_datetime(data.get("updated_at")) or datetime.now(),
            health_checks=health_checks,
            calibration=calibration,
        )

    def get_ready_tasks(self) -> list[str]:
        """Get IDs of tasks ready to execute."""
        state = self.load()
        if not state:
            return []

        ready = []
        for tid, task in state.tasks.items():
            if task.status != "pending":
                continue
            deps_met = all(
                state.tasks.get(dep, TaskInfo(dep, "", "pending", 0, (), ())).status
                == "complete"
                for dep in task.depends_on
            )
            if deps_met:
                ready.append(tid)
        return ready

    def get_task(self, task_id: str) -> TaskInfo | None:
        """Get details of a specific task."""
        state = self.load()
        if not state:
            return None
        return state.tasks.get(task_id)


class FileValidationProvider:
    """ValidationProvider implementation using validate.py functions."""

    def __init__(self, state_file: Path | None = None):
        if state_file is None:
            project_root = Path(__file__).resolve().parent.parent.parent
            state_file = project_root / "project-planning" / "state.json"
        self._state_file = state_file

    def _load_raw_state(self) -> dict | None:
        """Load raw state dict for validation functions."""
        if not self._state_file.exists():
            return None
        try:
            return json.loads(self._state_file.read_text())
        except (json.JSONDecodeError, OSError):
            return None

    def validate_dag(self) -> HealthCheck:
        """Check for dependency cycles."""
        state = self._load_raw_state()
        if not state:
            return HealthCheck("DAG", False, "No state file")

        try:
            from validate import validate_dag

            valid, msg = validate_dag(state)
            return HealthCheck("DAG", valid, msg)
        except ImportError:
            return HealthCheck("DAG", False, "validate module not available")

    def validate_steel_thread(self) -> HealthCheck:
        """Check steel thread is valid."""
        state = self._load_raw_state()
        if not state:
            return HealthCheck("Steel Thread", False, "No state file")

        try:
            from validate import validate_steel_thread

            valid, issues = validate_steel_thread(state)
            msg = "Valid" if valid else f"{len(issues)} issue(s)"
            return HealthCheck("Steel Thread", valid, msg, issues)
        except ImportError:
            return HealthCheck("Steel Thread", False, "validate module not available")

    def validate_verification_commands(self) -> HealthCheck:
        """Check verification commands are valid."""
        state = self._load_raw_state()
        if not state:
            return HealthCheck("Verification Commands", False, "No state file")

        try:
            from validate import validate_all_verification_commands

            valid, issues_by_task = validate_all_verification_commands(state)
            issue_count = sum(len(issues) for issues in issues_by_task.values())
            msg = "All valid" if valid else f"{issue_count} issue(s)"
            details = [
                f"{tid}: {issue}"
                for tid, issues in issues_by_task.items()
                for issue in issues
            ]
            return HealthCheck("Verification Commands", valid, msg, details)
        except ImportError:
            return HealthCheck(
                "Verification Commands", False, "validate module not available"
            )

    def get_calibration(self) -> CalibrationInfo:
        """Get verifier calibration metrics."""
        state = self._load_raw_state()
        if not state:
            return CalibrationInfo(0, 0.0, 0, 0, {})

        try:
            from validate import compute_calibration_metrics

            metrics = compute_calibration_metrics(state)
            return CalibrationInfo(
                total_verified=metrics.get("total_verified", 0),
                calibration_score=metrics.get("calibration_score", 0.0),
                false_positive_count=len(metrics.get("false_positives", [])),
                false_negative_count=len(metrics.get("false_negatives", [])),
                verdict_distribution=metrics.get("verdict_distribution", {}),
            )
        except ImportError:
            return CalibrationInfo(0, 0.0, 0, 0, {})

    def run_all(self) -> list[HealthCheck]:
        """Run all validation checks."""
        return [
            self.validate_dag(),
            self.validate_steel_thread(),
            self.validate_verification_commands(),
        ]
