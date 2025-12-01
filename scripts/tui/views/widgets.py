"""Reusable widgets for the TUI dashboard."""

from textual.app import ComposeResult
from textual.widgets import Label, ProgressBar, Static

from tui.providers import (
    ArtifactInfo,
    CalibrationInfo,
    ExecutionInfo,
    HealthCheck,
    PlanningInfo,
    TaskInfo,
)


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


# =============================================================================
# Planning Widgets
# =============================================================================


class PlanningPhasePanel(Static):
    """Panel showing planning phase progression."""

    DEFAULT_CSS = """
    PlanningPhasePanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    PlanningPhasePanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    PlanningPhasePanel .phase-complete {
        color: $success;
    }

    PlanningPhasePanel .phase-current {
        color: $warning;
        text-style: bold;
    }

    PlanningPhasePanel .phase-pending {
        color: $text-muted;
    }
    """

    PHASE_NAMES = {
        "ingestion": "Ingestion",
        "logical": "Logical Design",
        "physical": "Physical Mapping",
        "definition": "Task Definition",
        "validation": "Validation",
        "sequencing": "Sequencing",
    }

    def __init__(self, planning: PlanningInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._planning = planning

    def compose(self) -> ComposeResult:
        yield Label("Planning Progress", classes="title")

        for phase in self._planning.phases:
            display_name = self.PHASE_NAMES.get(phase, phase.title())

            if phase in self._planning.completed_phases:
                icon = "✓"
                css_class = "phase-complete"
            elif phase == self._planning.current_phase:
                icon = "●"
                css_class = "phase-current"
            else:
                icon = "○"
                css_class = "phase-pending"

            yield Label(f"{icon} {display_name}", classes=css_class)


class ArtifactPanel(Static):
    """Panel showing artifact status."""

    DEFAULT_CSS = """
    ArtifactPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    ArtifactPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    ArtifactPanel .artifact-valid {
        color: $success;
    }

    ArtifactPanel .artifact-invalid {
        color: $error;
    }

    ArtifactPanel .artifact-pending {
        color: $text-muted;
    }

    ArtifactPanel .artifact-generating {
        color: $warning;
    }

    ArtifactPanel .detail {
        color: $text-muted;
        margin-left: 2;
    }
    """

    def __init__(self, artifacts: tuple[ArtifactInfo, ...], **kwargs) -> None:
        super().__init__(**kwargs)
        self._artifacts = artifacts

    def compose(self) -> ComposeResult:
        yield Label("Artifacts", classes="title")

        for artifact in self._artifacts:
            if not artifact.exists:
                icon = "○"
                css_class = "artifact-pending"
            elif artifact.valid is None:
                icon = "⋯"
                css_class = "artifact-generating"
            elif artifact.valid:
                icon = "✓"
                css_class = "artifact-valid"
            else:
                icon = "✗"
                css_class = "artifact-invalid"

            yield Label(f"{icon} {artifact.name}", classes=css_class)

            # Show details for key artifacts
            details = artifact.details
            if artifact.name == "capability-map.json" and details.get("behaviors"):
                yield Label(
                    f"  {details.get('capabilities', 0)} capabilities, "
                    f"{details.get('behaviors', 0)} behaviors",
                    classes="detail",
                )
            elif artifact.name == "physical-map.json" and details.get("files"):
                yield Label(f"  {details.get('files', 0)} files", classes="detail")
            elif artifact.name == "tasks/" and details.get("count"):
                yield Label(f"  {details.get('count', 0)} tasks", classes="detail")
            elif artifact.name == "spec.md" and details.get("size"):
                size_kb = details.get("size", 0) / 1024
                yield Label(f"  {size_kb:.1f} KB", classes="detail")


class PlanningMetricsPanel(Static):
    """Panel showing planning quality metrics."""

    DEFAULT_CSS = """
    PlanningMetricsPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    PlanningMetricsPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    PlanningMetricsPanel .metric-good {
        color: $success;
    }

    PlanningMetricsPanel .metric-warn {
        color: $warning;
    }

    PlanningMetricsPanel .metric-bad {
        color: $error;
    }

    PlanningMetricsPanel .metric-neutral {
        color: $text;
    }
    """

    def __init__(self, planning: PlanningInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._planning = planning

    def compose(self) -> ComposeResult:
        yield Label("Planning Metrics", classes="title")

        p = self._planning

        if p.total_tasks == 0:
            yield Label("No tasks defined yet", classes="metric-neutral")
            return

        yield Label(f"Tasks: {p.total_tasks}", classes="metric-neutral")
        yield Label(f"Behaviors: {p.total_behaviors}", classes="metric-neutral")

        # Color-code behaviors per task (2-5 is ideal)
        avg = p.avg_behaviors_per_task
        if 2 <= avg <= 5:
            avg_class = "metric-good"
        elif 1 <= avg < 2 or 5 < avg <= 7:
            avg_class = "metric-warn"
        else:
            avg_class = "metric-bad"
        yield Label(f"Avg/task: {avg:.1f}", classes=avg_class)

        if p.steel_thread_count > 0:
            yield Label(f"Steel thread: {p.steel_thread_count} tasks", classes="metric-neutral")

        if p.phase_count > 0:
            yield Label(f"Phases: {p.phase_count}", classes="metric-neutral")


class ValidationPanel(Static):
    """Panel showing task validation status."""

    DEFAULT_CSS = """
    ValidationPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    ValidationPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    ValidationPanel .verdict-ready {
        color: $success;
        text-style: bold;
    }

    ValidationPanel .verdict-notes {
        color: $warning;
        text-style: bold;
    }

    ValidationPanel .verdict-blocked {
        color: $error;
        text-style: bold;
    }

    ValidationPanel .verdict-pending {
        color: $text-muted;
    }

    ValidationPanel .issue {
        color: $text-muted;
        margin-left: 1;
    }
    """

    def __init__(self, planning: PlanningInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._planning = planning

    def compose(self) -> ComposeResult:
        yield Label("Validation", classes="title")

        verdict = self._planning.validation_verdict
        if verdict is None:
            yield Label("Not validated yet", classes="verdict-pending")
            return

        verdict_classes = {
            "READY": "verdict-ready",
            "READY_WITH_NOTES": "verdict-notes",
            "BLOCKED": "verdict-blocked",
        }
        verdict_icons = {
            "READY": "✓",
            "READY_WITH_NOTES": "⚠",
            "BLOCKED": "✗",
        }

        icon = verdict_icons.get(verdict, "?")
        css_class = verdict_classes.get(verdict, "verdict-pending")
        yield Label(f"{icon} {verdict}", classes=css_class)

        issues = self._planning.validation_issues
        if issues:
            yield Label(f"{len(issues)} issue(s):", classes="issue")
            for issue in issues[:3]:  # Limit to first 3
                truncated = issue[:40] + "..." if len(issue) > 40 else issue
                yield Label(f"  • {truncated}", classes="issue")
            if len(issues) > 3:
                yield Label(f"  ... and {len(issues) - 3} more", classes="issue")


class SpecCoveragePanel(Static):
    """Panel showing spec requirement coverage."""

    DEFAULT_CSS = """
    SpecCoveragePanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    SpecCoveragePanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    SpecCoveragePanel .coverage-good {
        color: $success;
    }

    SpecCoveragePanel .coverage-warn {
        color: $warning;
    }

    SpecCoveragePanel .coverage-bad {
        color: $error;
    }

    SpecCoveragePanel .uncovered {
        color: $text-muted;
        margin-left: 1;
    }
    """

    def __init__(self, planning: PlanningInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._planning = planning

    def compose(self) -> ComposeResult:
        yield Label("Spec Coverage", classes="title")

        pct = self._planning.spec_coverage_pct
        if pct is None:
            yield Label("Not computed", classes="coverage-warn")
            return

        if pct >= 90:
            css_class = "coverage-good"
        elif pct >= 70:
            css_class = "coverage-warn"
        else:
            css_class = "coverage-bad"

        yield Label(f"{pct:.0f}% covered", classes=css_class)

        uncovered = self._planning.uncovered_requirements
        if uncovered:
            yield Label(f"Uncovered: {', '.join(uncovered[:5])}", classes="uncovered")
            if len(uncovered) > 5:
                yield Label(f"  ... and {len(uncovered) - 5} more", classes="uncovered")
