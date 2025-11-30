"""Main dashboard view combining all panels."""

from textual.app import ComposeResult
from textual.containers import Container, ScrollableContainer, Vertical
from textual.screen import Screen
from textual.widgets import Footer, Header, Label, Static

from tui.providers import TaskInfo, WorkflowState
from tui.views.widgets import (
    CalibrationPanel,
    CostPanel,
    CurrentTaskPanel,
    HealthPanel,
    ProgressPanel,
    TaskRow,
)


class PhaseIndicator(Static):
    """Shows current workflow phase."""

    DEFAULT_CSS = """
    PhaseIndicator {
        height: 3;
        border: solid $primary;
        padding: 0 1;
        content-align: center middle;
    }

    PhaseIndicator .phase-name {
        text-style: bold;
        color: $accent;
    }
    """

    PHASE_DISPLAY = {
        "ingestion": "📥 INGESTION",
        "logical": "🧠 LOGICAL DESIGN",
        "physical": "📁 PHYSICAL MAPPING",
        "definition": "📋 TASK DEFINITION",
        "validation": "✔️ VALIDATION",
        "sequencing": "🔢 SEQUENCING",
        "ready": "🚀 READY",
        "executing": "⚡ EXECUTING",
        "complete": "✅ COMPLETE",
    }

    def __init__(self, phase: str, **kwargs) -> None:
        super().__init__(**kwargs)
        self._phase = phase

    def compose(self) -> ComposeResult:
        display = self.PHASE_DISPLAY.get(self._phase, self._phase.upper())
        yield Label(f"Phase: {display}", classes="phase-name")


class TaskListPanel(Static):
    """Scrollable list of tasks."""

    DEFAULT_CSS = """
    TaskListPanel {
        height: 100%;
        border: solid $primary;
        padding: 1;
    }

    TaskListPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    TaskListPanel .task-list {
        height: 1fr;
        overflow-y: auto;
    }
    """

    def __init__(self, tasks: dict[str, TaskInfo], **kwargs) -> None:
        super().__init__(**kwargs)
        self._tasks = tasks

    def compose(self) -> ComposeResult:
        yield Label("Tasks", classes="title")

        # Sort by wave, then by ID
        sorted_tasks = sorted(
            self._tasks.values(), key=lambda t: (t.wave, t.id)
        )

        with ScrollableContainer(classes="task-list"):
            current_wave = None
            for task in sorted_tasks:
                if task.wave != current_wave:
                    current_wave = task.wave
                    yield Label(f"─── Wave {current_wave} ───")
                yield TaskRow(task)


class RecentActivityPanel(Static):
    """Shows recent events/activity."""

    DEFAULT_CSS = """
    RecentActivityPanel {
        height: auto;
        max-height: 12;
        border: solid $primary;
        padding: 1;
    }

    RecentActivityPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    RecentActivityPanel .activity-complete {
        color: $success;
    }

    RecentActivityPanel .activity-failed {
        color: $error;
    }
    """

    def __init__(self, tasks: dict[str, TaskInfo], **kwargs) -> None:
        super().__init__(**kwargs)
        self._tasks = tasks

    def compose(self) -> ComposeResult:
        yield Label("Recent Activity", classes="title")

        # Get recently completed/failed tasks
        finished = [
            t for t in self._tasks.values()
            if t.status in ("complete", "failed", "skipped") and t.completed_at
        ]
        finished.sort(key=lambda t: t.completed_at or t.started_at, reverse=True)

        for task in finished[:5]:
            if task.status == "complete":
                icon = "✓"
                css_class = "activity-complete"
                duration = f"{task.duration_seconds:.0f}s" if task.duration_seconds else ""
                attempts = f"({task.attempts} attempt{'s' if task.attempts > 1 else ''})" if task.attempts > 1 else ""
                yield Label(f"{icon} {task.id} completed {duration} {attempts}", classes=css_class)
            elif task.status == "failed":
                icon = "✗"
                css_class = "activity-failed"
                yield Label(f"{icon} {task.id} failed: {task.error or 'Unknown'}", classes=css_class)
            elif task.status == "skipped":
                yield Label(f"⊖ {task.id} skipped")

        if not finished:
            yield Label("No completed tasks yet")


class DashboardScreen(Screen):
    """Main dashboard screen."""

    BINDINGS = [
        ("q", "quit", "Quit"),
        ("r", "refresh", "Refresh"),
        ("t", "tasks", "Task List"),
        ("d", "details", "Details"),
    ]

    DEFAULT_CSS = """
    DashboardScreen {
        layout: grid;
        grid-size: 3 3;
        grid-columns: 1fr 2fr 1fr;
        grid-rows: auto 1fr auto;
    }

    #header-row {
        column-span: 3;
        height: 3;
    }

    #left-column {
        row-span: 1;
        padding: 1;
    }

    #center-column {
        row-span: 1;
        padding: 1;
    }

    #right-column {
        row-span: 1;
        padding: 1;
    }

    #footer-row {
        column-span: 3;
        height: auto;
    }

    .no-state {
        text-align: center;
        margin: 2;
        color: $warning;
    }
    """

    def __init__(self, state: WorkflowState | None = None, **kwargs) -> None:
        super().__init__(**kwargs)
        self._state = state

    def compose(self) -> ComposeResult:
        yield Header()

        if not self._state:
            yield Label(
                "No workflow state found.\n\n"
                "Run 'python3 scripts/state.py init <target_dir>' to start.",
                classes="no-state",
            )
            yield Footer()
            return

        # Calculate max wave
        max_wave = max((t.wave for t in self._state.tasks.values()), default=1)

        # Get active tasks
        active_task_infos = [
            self._state.tasks[tid]
            for tid in self._state.execution.active_tasks
            if tid in self._state.tasks
        ]

        # Header row
        with Container(id="header-row"):
            yield PhaseIndicator(self._state.phase.current)

        # Left column - Health & Calibration
        with Vertical(id="left-column"):
            yield HealthPanel(self._state.health_checks)
            yield CalibrationPanel(self._state.calibration)
            yield CostPanel(self._state.execution)

        # Center column - Progress & Tasks
        with Vertical(id="center-column"):
            yield ProgressPanel(
                self._state.execution, self._state.tasks, max_wave
            )
            yield CurrentTaskPanel(active_task_infos)
            yield RecentActivityPanel(self._state.tasks)

        # Right column - Task List
        with Vertical(id="right-column"):
            yield TaskListPanel(self._state.tasks)

        yield Footer()

    def action_refresh(self) -> None:
        """Refresh the dashboard."""
        # This will be implemented to reload state and re-render
        self.app.refresh_state()

    def action_quit(self) -> None:
        """Quit the application."""
        self.app.exit()
