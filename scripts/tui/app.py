"""
Tasker TUI Application.

Main entry point for the terminal user interface.
"""

from __future__ import annotations

import sys
from pathlib import Path

# Ensure scripts directory is in path
SCRIPT_DIR = Path(__file__).resolve().parent.parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from textual.app import App  # noqa: E402
from textual.binding import Binding  # noqa: E402

from tui.providers import WorkflowState  # noqa: E402
from tui.state_provider import FileStateProvider  # noqa: E402
from tui.views.dashboard import DashboardScreen  # noqa: E402
from tui.views.task_detail import TaskDetailScreen  # noqa: E402

# Auto-refresh interval in seconds
AUTO_REFRESH_INTERVAL = 5.0


class TaskerApp(App):
    """Main Tasker TUI application."""

    TITLE = "Tasker"
    SUB_TITLE = "Workflow Monitor"

    CSS = """
    Screen {
        background: $surface;
    }
    """

    BINDINGS = [
        Binding("q", "quit", "Quit", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("a", "toggle_auto_refresh", "Auto-Refresh", show=True),
        Binding("d", "toggle_dark", "Dark/Light", show=True),
    ]

    def __init__(
        self,
        state_file: Path | None = None,
        auto_refresh: bool = True,
        **kwargs,
    ) -> None:
        super().__init__(**kwargs)
        self._provider = FileStateProvider(state_file)
        self._state: WorkflowState | None = None
        self._auto_refresh = auto_refresh
        self._refresh_timer = None
        self._last_updated_at: str | None = None

    def on_mount(self) -> None:
        """Called when app is mounted."""
        self._state = self._provider.load()
        if self._state:
            self._last_updated_at = self._state.updated_at.isoformat()
        self.push_screen(DashboardScreen(self._state))

        if self._auto_refresh:
            self._start_auto_refresh()

    def _start_auto_refresh(self) -> None:
        """Start the auto-refresh timer."""
        self._refresh_timer = self.set_interval(
            AUTO_REFRESH_INTERVAL,
            self._check_for_updates,
        )

    def _stop_auto_refresh(self) -> None:
        """Stop the auto-refresh timer."""
        if self._refresh_timer:
            self._refresh_timer.stop()
            self._refresh_timer = None

    def _check_for_updates(self) -> None:
        """Check if state has changed and refresh if so."""
        new_state = self._provider.load()
        if not new_state:
            return

        new_updated_at = new_state.updated_at.isoformat()
        if new_updated_at != self._last_updated_at:
            self._last_updated_at = new_updated_at
            self._state = new_state
            # Only refresh if we're on the dashboard
            if isinstance(self.screen, DashboardScreen):
                self.pop_screen()
                self.push_screen(DashboardScreen(self._state))

    def refresh_state(self) -> None:
        """Reload state and refresh the dashboard."""
        self._state = self._provider.load()
        if self._state:
            self._last_updated_at = self._state.updated_at.isoformat()
        # Pop current screen and push new one with fresh state
        self.pop_screen()
        self.push_screen(DashboardScreen(self._state))

    def show_task_detail(self, task_id: str) -> None:
        """Show detail screen for a specific task."""
        if not self._state:
            return
        task = self._state.tasks.get(task_id)
        if task:
            self.push_screen(TaskDetailScreen(task))

    def action_toggle_dark(self) -> None:
        """Toggle dark mode."""
        self.dark = not self.dark

    def action_toggle_auto_refresh(self) -> None:
        """Toggle auto-refresh on/off."""
        self._auto_refresh = not self._auto_refresh
        if self._auto_refresh:
            self._start_auto_refresh()
            self.notify("Auto-refresh enabled")
        else:
            self._stop_auto_refresh()
            self.notify("Auto-refresh disabled")

    def action_refresh(self) -> None:
        """Refresh the current view."""
        self.refresh_state()


def run(state_file: Path | None = None, auto_refresh: bool = True) -> None:
    """Run the TUI application."""
    app = TaskerApp(state_file=state_file, auto_refresh=auto_refresh)
    app.run()


if __name__ == "__main__":
    run()
