"""Task detail view for drilling into individual tasks."""

from textual.app import ComposeResult
from textual.containers import ScrollableContainer
from textual.screen import Screen
from textual.widgets import Footer, Header, Label, Static

from tui.providers import TaskInfo


class VerificationPanel(Static):
    """Panel showing verification results for a task."""

    DEFAULT_CSS = """
    VerificationPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    VerificationPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    VerificationPanel .verdict-pass {
        color: $success;
    }

    VerificationPanel .verdict-fail {
        color: $error;
    }

    VerificationPanel .verdict-conditional {
        color: $warning;
    }

    VerificationPanel .criterion {
        margin-left: 2;
    }

    VerificationPanel .score-pass {
        color: $success;
    }

    VerificationPanel .score-partial {
        color: $warning;
    }

    VerificationPanel .score-fail {
        color: $error;
    }
    """

    def __init__(self, verification: dict, **kwargs) -> None:
        super().__init__(**kwargs)
        self._verification = verification

    def compose(self) -> ComposeResult:
        yield Label("Verification", classes="title")

        if not self._verification:
            yield Label("No verification data")
            return

        verdict = self._verification.get("verdict", "N/A")
        recommendation = self._verification.get("recommendation", "N/A")
        verdict_class = f"verdict-{verdict.lower()}"

        yield Label(f"Verdict: {verdict} ({recommendation})", classes=verdict_class)

        # Criteria
        criteria = self._verification.get("criteria", [])
        if criteria:
            yield Label("")
            yield Label("Criteria:")
            for c in criteria:
                score = c.get("score", "N/A")
                score_class = f"score-{score.lower()}" if score else ""
                yield Label(
                    f"  {c.get('name', 'Unknown')}: {score}",
                    classes=f"criterion {score_class}",
                )
                if c.get("evidence"):
                    yield Label(f"    {c['evidence']}", classes="criterion")

        # Quality
        quality = self._verification.get("quality", {})
        if quality:
            yield Label("")
            yield Label("Quality:")
            for dim, score in quality.items():
                score_class = f"score-{score.lower()}" if score else ""
                yield Label(f"  {dim}: {score}", classes=f"criterion {score_class}")

        # Tests
        tests = self._verification.get("tests", {})
        if tests:
            yield Label("")
            yield Label("Tests:")
            for dim, score in tests.items():
                score_class = f"score-{score.lower()}" if score else ""
                yield Label(f"  {dim}: {score}", classes=f"criterion {score_class}")


class FilesPanel(Static):
    """Panel showing files created/modified by a task."""

    DEFAULT_CSS = """
    FilesPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    FilesPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }

    FilesPanel .file-created {
        color: $success;
    }

    FilesPanel .file-modified {
        color: $warning;
    }
    """

    def __init__(
        self, files_created: tuple[str, ...], files_modified: tuple[str, ...], **kwargs
    ) -> None:
        super().__init__(**kwargs)
        self._created = files_created
        self._modified = files_modified

    def compose(self) -> ComposeResult:
        yield Label("Files", classes="title")

        if not self._created and not self._modified:
            yield Label("No file changes recorded")
            return

        if self._created:
            yield Label("Created:")
            for f in self._created:
                yield Label(f"  + {f}", classes="file-created")

        if self._modified:
            yield Label("Modified:")
            for f in self._modified:
                yield Label(f"  ~ {f}", classes="file-modified")


class DependenciesPanel(Static):
    """Panel showing task dependencies."""

    DEFAULT_CSS = """
    DependenciesPanel {
        height: auto;
        border: solid $primary;
        padding: 1;
        margin-bottom: 1;
    }

    DependenciesPanel .title {
        text-style: bold;
        margin-bottom: 1;
    }
    """

    def __init__(
        self, depends_on: tuple[str, ...], blocks: tuple[str, ...], **kwargs
    ) -> None:
        super().__init__(**kwargs)
        self._depends_on = depends_on
        self._blocks = blocks

    def compose(self) -> ComposeResult:
        yield Label("Dependencies", classes="title")

        if self._depends_on:
            yield Label(f"Depends on: {', '.join(self._depends_on)}")
        else:
            yield Label("Depends on: (none)")

        if self._blocks:
            yield Label(f"Blocks: {', '.join(self._blocks)}")
        else:
            yield Label("Blocks: (none)")


class TaskDetailScreen(Screen):
    """Screen showing detailed task information."""

    BINDINGS = [
        ("q", "quit", "Quit"),
        ("escape", "back", "Back"),
        ("b", "back", "Back"),
    ]

    DEFAULT_CSS = """
    TaskDetailScreen {
        padding: 1;
    }

    TaskDetailScreen .task-header {
        text-style: bold;
        margin-bottom: 1;
    }

    TaskDetailScreen .task-status {
        margin-bottom: 1;
    }

    TaskDetailScreen .status-complete {
        color: $success;
    }

    TaskDetailScreen .status-failed {
        color: $error;
    }

    TaskDetailScreen .status-running {
        color: $warning;
    }

    TaskDetailScreen .status-pending {
        color: $text-muted;
    }

    TaskDetailScreen .error-message {
        color: $error;
        margin: 1 0;
        padding: 1;
        border: solid $error;
    }
    """

    def __init__(self, task: TaskInfo, **kwargs) -> None:
        super().__init__(**kwargs)
        self._task = task

    def compose(self) -> ComposeResult:
        yield Header()

        with ScrollableContainer():
            yield Label(f"{self._task.id}: {self._task.name}", classes="task-header")

            status_class = f"status-{self._task.status}"
            status_parts = [f"Status: {self._task.status.upper()}"]
            status_parts.append(f"Wave: {self._task.wave}")
            status_parts.append(f"Attempts: {self._task.attempts}")

            if self._task.duration_seconds:
                status_parts.append(f"Duration: {self._task.duration_seconds:.1f}s")

            yield Label(" | ".join(status_parts), classes=f"task-status {status_class}")

            if self._task.error:
                yield Label(f"Error: {self._task.error}", classes="error-message")

            yield DependenciesPanel(self._task.depends_on, self._task.blocks)
            yield FilesPanel(self._task.files_created, self._task.files_modified)
            yield VerificationPanel(self._task.verification)

        yield Footer()

    def action_back(self) -> None:
        """Go back to the dashboard."""
        self.app.pop_screen()

    def action_quit(self) -> None:
        """Quit the application."""
        self.app.exit()
