"""Reusable widgets for the TUI dashboard."""

from textual.app import ComposeResult
from textual.widgets import Label, ProgressBar, Static

from tui.providers import CalibrationInfo, ExecutionInfo, HealthCheck, TaskInfo


class HealthPanel(Static):
    """Panel showing health check status."""

    DEFAULT_CSS = """
    HealthPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    HealthPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    HealthPanel .check-pass {
        color: $success;
    }

    HealthPanel .check-fail {
        color: $error;
    }
    """

    def __init__(self, checks: list[HealthCheck], **kwargs) -> None:
        super().__init__(**kwargs)
        self._checks = checks

    def compose(self) -> ComposeResult:
        yield Label("Health Checks", classes="title")
        for check in self._checks:
            icon = "✓" if check.passed else "✗"
            css_class = "check-pass" if check.passed else "check-fail"
            yield Label(f"{icon} {check.name}: {check.message}", classes=css_class)


class ProgressPanel(Static):
    """Panel showing execution progress."""

    DEFAULT_CSS = """
    ProgressPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    ProgressPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    ProgressPanel .metric {
        margin-left: 2;
    }

    ProgressPanel .metric-label {
        width: 14;
    }

    ProgressPanel .status-complete {
        color: $success;
    }

    ProgressPanel .status-failed {
        color: $error;
    }

    ProgressPanel .status-running {
        color: $warning;
    }

    ProgressPanel .status-pending {
        color: $text-muted;
    }
    """

    def __init__(
        self,
        execution: ExecutionInfo,
        tasks: dict[str, TaskInfo],
        max_phase: int,
        **kwargs,
    ) -> None:
        super().__init__(**kwargs)
        self._execution = execution
        self._tasks = tasks
        self._max_phase = max_phase

    def compose(self) -> ComposeResult:
        total = len(self._tasks)
        completed = self._execution.completed_count
        failed = self._execution.failed_count
        running = len(self._execution.active_tasks)
        blocked = sum(1 for t in self._tasks.values() if t.status == "blocked")
        skipped = sum(1 for t in self._tasks.values() if t.status == "skipped")
        pending = total - completed - failed - running - blocked - skipped

        yield Label("Progress", classes="title")

        # Phase indicator
        current_phase = self._execution.current_phase or 1
        yield Label(f"Phase {current_phase} of {self._max_phase}")

        # Progress bar
        progress = completed / total if total > 0 else 0
        yield ProgressBar(total=100, show_eta=False)
        yield Label(f"{completed}/{total} tasks ({progress:.0%})")

        yield Label("")  # spacer

        # Status breakdown
        if completed > 0:
            yield Label(f"  Completed: {completed}", classes="status-complete")
        if running > 0:
            yield Label(f"  Running:   {running}", classes="status-running")
        if failed > 0:
            yield Label(f"  Failed:    {failed}", classes="status-failed")
        if blocked > 0:
            yield Label(f"  Blocked:   {blocked}", classes="status-failed")
        if skipped > 0:
            yield Label(f"  Skipped:   {skipped}", classes="status-pending")
        if pending > 0:
            yield Label(f"  Pending:   {pending}", classes="status-pending")


class CostPanel(Static):
    """Panel showing cost/token metrics."""

    DEFAULT_CSS = """
    CostPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    CostPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }
    """

    def __init__(self, execution: ExecutionInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._execution = execution

    def compose(self) -> ComposeResult:
        yield Label("Cost", classes="title")
        yield Label(f"Tokens: {self._execution.total_tokens:,}")
        yield Label(f"Cost:   ${self._execution.total_cost_usd:.4f}")

        if self._execution.completed_count > 0:
            avg_cost = (
                self._execution.total_cost_usd / self._execution.completed_count
            )
            avg_tokens = (
                self._execution.total_tokens // self._execution.completed_count
            )
            yield Label(f"Avg:    ${avg_cost:.4f}/task ({avg_tokens:,} tokens)")


class CalibrationPanel(Static):
    """Panel showing verifier calibration metrics."""

    DEFAULT_CSS = """
    CalibrationPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    CalibrationPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    CalibrationPanel .score-good {
        color: $success;
    }

    CalibrationPanel .score-warn {
        color: $warning;
    }

    CalibrationPanel .score-bad {
        color: $error;
    }
    """

    def __init__(self, calibration: CalibrationInfo | None, **kwargs) -> None:
        super().__init__(**kwargs)
        self._calibration = calibration

    def compose(self) -> ComposeResult:
        yield Label("Verifier Calibration", classes="title")

        if not self._calibration or self._calibration.total_verified == 0:
            yield Label("No verification data yet")
            return

        score = self._calibration.calibration_score
        if score >= 0.9:
            score_class = "score-good"
        elif score >= 0.7:
            score_class = "score-warn"
        else:
            score_class = "score-bad"

        yield Label(f"Score: {score:.0%}", classes=score_class)
        yield Label(f"Verified: {self._calibration.total_verified}")

        if self._calibration.false_positive_count > 0:
            yield Label(f"False Positives: {self._calibration.false_positive_count}")
        if self._calibration.false_negative_count > 0:
            yield Label(f"False Negatives: {self._calibration.false_negative_count}")


class CurrentTaskPanel(Static):
    """Panel showing currently running task."""

    DEFAULT_CSS = """
    CurrentTaskPanel {
        height: auto;
        border: solid $warning;
        padding: 1;
        margin-bottom: 1;
    }

    CurrentTaskPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    CurrentTaskPanel .task-id {
        color: $accent;
    }
    """

    def __init__(self, active_tasks: list[TaskInfo], **kwargs) -> None:
        super().__init__(**kwargs)
        self._active_tasks = active_tasks

    def compose(self) -> ComposeResult:
        yield Label("Current Task", classes="title")

        if not self._active_tasks:
            yield Label("No tasks running")
            return

        for task in self._active_tasks:
            yield Label(f"{task.id}: {task.name}", classes="task-id")
            info_parts = [f"Attempt {task.attempts}"]
            if task.started_at:
                from datetime import datetime, timezone

                elapsed = datetime.now(timezone.utc) - task.started_at
                minutes = int(elapsed.total_seconds() // 60)
                seconds = int(elapsed.total_seconds() % 60)
                if minutes > 0:
                    info_parts.append(f"{minutes}m {seconds}s")
                else:
                    info_parts.append(f"{seconds}s")
            yield Label("  " + " | ".join(info_parts))


class TaskRow(Static):
    """Single row in the task list."""

    DEFAULT_CSS = """
    TaskRow {
        height: 1;
        width: 100%;
    }

    TaskRow.selected {
        background: $accent;
    }

    TaskRow .status-complete {
        color: $success;
    }

    TaskRow .status-failed {
        color: $error;
    }

    TaskRow .status-blocked {
        color: $error;
    }

    TaskRow .status-running {
        color: $warning;
    }

    TaskRow .status-pending {
        color: $text-muted;
    }

    TaskRow .status-skipped {
        color: $text-muted;
    }
    """

    def __init__(self, task: TaskInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._task = task

    def compose(self) -> ComposeResult:
        status_icons = {
            "complete": "✓",
            "failed": "✗",
            "blocked": "⊘",
            "running": "▶",
            "pending": "○",
            "skipped": "⊖",
        }
        icon = status_icons.get(self._task.status, "?")
        attempts_str = f"({self._task.attempts}x)" if self._task.attempts > 1 else ""

        yield Label(
            f"{icon} {self._task.id} {self._task.name} {attempts_str}",
            classes=f"status-{self._task.status}",
        )
