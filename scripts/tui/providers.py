"""
Data providers for the TUI.

Protocols define the interface; implementations can be swapped
for testing or alternative data sources.
"""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Protocol


@dataclass(frozen=True)
class TaskInfo:
    """Immutable snapshot of task state."""

    id: str
    name: str
    status: str
    wave: int
    depends_on: tuple[str, ...]
    blocks: tuple[str, ...]
    attempts: int = 0
    started_at: datetime | None = None
    completed_at: datetime | None = None
    duration_seconds: float | None = None
    error: str | None = None
    files_created: tuple[str, ...] = ()
    files_modified: tuple[str, ...] = ()
    verification: dict = field(default_factory=dict)


@dataclass(frozen=True)
class ExecutionInfo:
    """Immutable snapshot of execution state."""

    current_wave: int
    active_tasks: tuple[str, ...]
    completed_count: int
    failed_count: int
    total_tokens: int
    total_cost_usd: float


@dataclass(frozen=True)
class PhaseInfo:
    """Immutable snapshot of phase state."""

    current: str
    completed: tuple[str, ...]


@dataclass(frozen=True)
class HealthCheck:
    """Result of a health/validation check."""

    name: str
    passed: bool
    message: str
    details: list[str] = field(default_factory=list)


@dataclass(frozen=True)
class CalibrationInfo:
    """Verifier calibration metrics."""

    total_verified: int
    calibration_score: float
    false_positive_count: int
    false_negative_count: int
    verdict_distribution: dict = field(default_factory=dict)


@dataclass(frozen=True)
class WorkflowState:
    """Complete workflow state snapshot."""

    phase: PhaseInfo
    target_dir: str
    tasks: dict[str, TaskInfo]
    execution: ExecutionInfo
    created_at: datetime
    updated_at: datetime
    health_checks: list[HealthCheck] = field(default_factory=list)
    calibration: CalibrationInfo | None = None


class StateProvider(Protocol):
    """Protocol for accessing workflow state."""

    def load(self) -> WorkflowState | None:
        """Load current workflow state."""
        ...

    def get_ready_tasks(self) -> list[str]:
        """Get IDs of tasks ready to execute."""
        ...

    def get_task(self, task_id: str) -> TaskInfo | None:
        """Get details of a specific task."""
        ...


class ValidationProvider(Protocol):
    """Protocol for running validation checks."""

    def validate_dag(self) -> HealthCheck:
        """Check for dependency cycles."""
        ...

    def validate_steel_thread(self) -> HealthCheck:
        """Check steel thread is valid."""
        ...

    def validate_verification_commands(self) -> HealthCheck:
        """Check verification commands are valid."""
        ...

    def get_calibration(self) -> CalibrationInfo:
        """Get verifier calibration metrics."""
        ...

    def run_all(self) -> list[HealthCheck]:
        """Run all validation checks."""
        ...
