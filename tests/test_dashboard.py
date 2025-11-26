"""Unit tests for the dashboard module."""

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest.mock import patch

import pytest

# Import dashboard module
import sys
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
from dashboard import (
    format_duration,
    format_time_ago,
    progress_bar,
    box_line,
    box_text,
    render_compact,
    render_dashboard,
    load_state,
    BOX_H,
    BOX_TL,
    BOX_TR,
    BOX_V,
)


class TestFormatDuration:
    """Tests for format_duration function."""

    def test_seconds_only(self) -> None:
        now = datetime.now(timezone.utc)
        start = (now - timedelta(seconds=45)).isoformat()
        end = now.isoformat()

        result = format_duration(start, end)
        assert result == "45s"

    def test_minutes_and_seconds(self) -> None:
        now = datetime.now(timezone.utc)
        start = (now - timedelta(minutes=5, seconds=30)).isoformat()
        end = now.isoformat()

        result = format_duration(start, end)
        assert result == "5m 30s"

    def test_hours_and_minutes(self) -> None:
        now = datetime.now(timezone.utc)
        start = (now - timedelta(hours=2, minutes=15)).isoformat()
        end = now.isoformat()

        result = format_duration(start, end)
        assert result == "2h 15m"

    def test_no_end_time_uses_now(self) -> None:
        start = (datetime.now(timezone.utc) - timedelta(seconds=30)).isoformat()

        result = format_duration(start)
        # Should be around 30 seconds, allow some margin
        assert "s" in result

    def test_invalid_timestamp_returns_dash(self) -> None:
        result = format_duration("invalid", "also-invalid")
        assert result == "—"

    def test_handles_z_suffix(self) -> None:
        now = datetime.now(timezone.utc)
        start = (now - timedelta(minutes=10)).strftime("%Y-%m-%dT%H:%M:%SZ")
        end = now.strftime("%Y-%m-%dT%H:%M:%SZ")

        result = format_duration(start, end)
        assert result == "10m 0s"


class TestFormatTimeAgo:
    """Tests for format_time_ago function."""

    def test_seconds_ago(self) -> None:
        ts = (datetime.now(timezone.utc) - timedelta(seconds=30)).isoformat()
        result = format_time_ago(ts)
        assert "s ago" in result

    def test_minutes_ago(self) -> None:
        ts = (datetime.now(timezone.utc) - timedelta(minutes=15)).isoformat()
        result = format_time_ago(ts)
        assert "m ago" in result

    def test_hours_ago(self) -> None:
        ts = (datetime.now(timezone.utc) - timedelta(hours=5)).isoformat()
        result = format_time_ago(ts)
        assert "h ago" in result

    def test_days_ago(self) -> None:
        ts = (datetime.now(timezone.utc) - timedelta(days=3)).isoformat()
        result = format_time_ago(ts)
        assert "d ago" in result

    def test_invalid_timestamp_returns_dash(self) -> None:
        result = format_time_ago("not-a-timestamp")
        assert result == "—"


class TestProgressBar:
    """Tests for progress_bar function."""

    def test_zero_percent(self) -> None:
        result = progress_bar(0, 10, width=10)
        assert "0%" in result
        assert "░" * 10 in result

    def test_fifty_percent(self) -> None:
        result = progress_bar(5, 10, width=10)
        assert "50%" in result
        assert "█" * 5 in result
        assert "░" * 5 in result

    def test_hundred_percent(self) -> None:
        result = progress_bar(10, 10, width=10)
        assert "100%" in result
        assert "█" * 10 in result

    def test_zero_total(self) -> None:
        result = progress_bar(0, 0, width=10)
        assert "0%" in result

    def test_custom_width(self) -> None:
        result = progress_bar(5, 10, width=20)
        # Should have 10 filled and 10 empty
        assert result.count("█") == 10
        assert result.count("░") == 10


class TestBoxDrawing:
    """Tests for box drawing helper functions."""

    def test_box_line(self) -> None:
        result = box_line(BOX_TL, BOX_H, BOX_TR, 10)
        assert result.startswith(BOX_TL)
        assert result.endswith(BOX_TR)
        assert len(result) == 10

    def test_box_text_left_align(self) -> None:
        result = box_text("test", 20, "left")
        assert result.startswith(BOX_V)
        assert result.endswith(BOX_V)
        assert "test" in result

    def test_box_text_center_align(self) -> None:
        result = box_text("test", 20, "center")
        # Text should be centered
        content = result[2:-2]  # Remove borders and padding
        stripped = content.strip()
        assert stripped == "test"

    def test_box_text_truncates_long_text(self) -> None:
        long_text = "a" * 100
        result = box_text(long_text, 20)
        assert "…" in result
        assert len(result) == 20


class TestRenderCompact:
    """Tests for render_compact function."""

    def test_basic_compact_output(self) -> None:
        state = {
            "phase": {"current": "executing"},
            "tasks": {
                "T001": {"id": "T001", "status": "complete"},
                "T002": {"id": "T002", "status": "running"},
                "T003": {"id": "T003", "status": "pending"},
            },
        }

        result = render_compact(state)

        assert "EXECUTING" in result
        assert "1/3 done" in result
        assert "1 running" in result

    def test_compact_with_failures(self) -> None:
        state = {
            "phase": {"current": "executing"},
            "tasks": {
                "T001": {"id": "T001", "status": "complete"},
                "T002": {"id": "T002", "status": "failed"},
            },
        }

        result = render_compact(state)

        assert "1 failed" in result

    def test_compact_empty_tasks(self) -> None:
        state = {
            "phase": {"current": "ready"},
            "tasks": {},
        }

        result = render_compact(state)

        assert "READY" in result
        assert "0/0" in result


class TestRenderDashboard:
    """Tests for render_dashboard function."""

    @pytest.fixture
    def sample_state(self) -> dict:
        """Create a sample state for testing."""
        now = datetime.now(timezone.utc)
        return {
            "version": "2.0",
            "phase": {"current": "executing", "completed": ["ready"]},
            "target_dir": "/path/to/project",
            "updated_at": now.isoformat(),
            "artifacts": {},
            "tasks": {
                "T001": {
                    "id": "T001",
                    "name": "First task",
                    "status": "complete",
                    "wave": 1,
                    "depends_on": [],
                    "blocks": ["T002"],
                    "completed_at": (now - timedelta(hours=1)).isoformat(),
                },
                "T002": {
                    "id": "T002",
                    "name": "Second task",
                    "status": "running",
                    "wave": 2,
                    "depends_on": ["T001"],
                    "blocks": [],
                    "started_at": (now - timedelta(minutes=30)).isoformat(),
                },
                "T003": {
                    "id": "T003",
                    "name": "Third task",
                    "status": "pending",
                    "wave": 2,
                    "depends_on": ["T001"],
                    "blocks": [],
                },
            },
            "execution": {
                "current_wave": 2,
                "active_tasks": ["T002"],
                "completed_count": 1,
                "failed_count": 0,
                "total_tokens": 50000,
                "total_cost_usd": 0.25,
            },
            "events": [
                {
                    "timestamp": (now - timedelta(hours=1)).isoformat(),
                    "type": "task_completed",
                    "task_id": "T001",
                },
                {
                    "timestamp": (now - timedelta(minutes=30)).isoformat(),
                    "type": "task_started",
                    "task_id": "T002",
                },
            ],
        }

    def test_dashboard_contains_header(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "EXECUTOR STATUS DASHBOARD" in result

    def test_dashboard_contains_phase(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "EXECUTING" in result

    def test_dashboard_contains_progress(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "1/3 tasks complete" in result
        assert "33%" in result

    def test_dashboard_contains_running_tasks(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "RUNNING" in result
        assert "T002" in result

    def test_dashboard_contains_ready_tasks(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "READY TO EXECUTE" in result
        assert "T003" in result

    def test_dashboard_contains_resource_usage(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "RESOURCE USAGE" in result
        assert "50,000" in result
        assert "$0.2500" in result

    def test_dashboard_contains_wave_progress(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "WAVE PROGRESS" in result
        assert "Wave 1" in result
        assert "Wave 2" in result

    def test_dashboard_shows_failed_tasks(self, sample_state: dict) -> None:
        sample_state["tasks"]["T002"]["status"] = "failed"
        sample_state["tasks"]["T002"]["error"] = "Something went wrong"

        result = render_dashboard(sample_state, use_color=False)

        assert "FAILED" in result
        assert "Something went wrong" in result

    def test_dashboard_shows_blocked_tasks(self, sample_state: dict) -> None:
        sample_state["tasks"]["T003"]["status"] = "blocked"
        sample_state["tasks"]["T003"]["error"] = "Blocked by T002"

        result = render_dashboard(sample_state, use_color=False)

        assert "BLOCKED" in result
        assert "T003" in result

    def test_dashboard_shows_recent_activity(self, sample_state: dict) -> None:
        result = render_dashboard(sample_state, use_color=False)

        assert "RECENT ACTIVITY" in result
        assert "completed" in result
        assert "started" in result


class TestLoadState:
    """Tests for load_state function."""

    def test_returns_none_when_no_file(self, tmp_path: Path) -> None:
        with patch("dashboard.STATE_FILE", tmp_path / "nonexistent.json"):
            result = load_state()
            assert result is None

    def test_loads_valid_state_file(self, tmp_path: Path) -> None:
        state_file = tmp_path / "state.json"
        state_file.write_text(json.dumps({"version": "2.0", "phase": {"current": "ready"}}))

        with patch("dashboard.STATE_FILE", state_file):
            result = load_state()

            assert result is not None
            assert result["version"] == "2.0"
            assert result["phase"]["current"] == "ready"
