"""Tests for state.py - state management for task decomposition."""

import json
import sys
from datetime import datetime
from pathlib import Path

import pytest

# Add scripts to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))

from state import (
    add_event,
    advance_phase,
    can_advance_phase,
    commit_task_changes,
    complete_task,
    fail_task,
    file_checksum,
    get_next_phase,
    get_phase_order,
    get_ready_tasks,
    init_state,
    load_state,
    load_tasks_from_dir,
    log_tokens,
    now_iso,
    register_artifact,
    register_task_validation,
    save_state,
    start_task,
    validate_json,
)


@pytest.fixture
def temp_state_env(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Set up temporary state environment."""
    import state

    planning_dir = tmp_path / "project-planning"
    planning_dir.mkdir()
    (planning_dir / "tasks").mkdir()
    (planning_dir / "artifacts").mkdir()
    (planning_dir / "inputs").mkdir()

    schemas_dir = tmp_path / "schemas"
    schemas_dir.mkdir()

    # Create minimal schemas
    capability_schema = {
        "$schema": "http://json-schema.org/draft-07/schema#",
        "required": ["version", "domains", "flows"],
    }
    (schemas_dir / "capability-map.schema.json").write_text(json.dumps(capability_schema))

    physical_schema = {
        "$schema": "http://json-schema.org/draft-07/schema#",
        "required": ["version", "file_mapping"],
    }
    (schemas_dir / "physical-map.schema.json").write_text(json.dumps(physical_schema))

    monkeypatch.setattr(state, "PROJECT_ROOT", tmp_path)
    monkeypatch.setattr(state, "PLANNING_DIR", planning_dir)
    monkeypatch.setattr(state, "STATE_FILE", planning_dir / "state.json")
    monkeypatch.setattr(state, "SCHEMAS_DIR", schemas_dir)

    return tmp_path


class TestNowIso:
    """Tests for now_iso function."""

    def test_returns_iso_format(self) -> None:
        """Test that now_iso returns ISO format string."""
        result = now_iso()

        # Should be parseable as ISO datetime
        datetime.fromisoformat(result.replace("Z", "+00:00"))

    def test_returns_utc_time(self) -> None:
        """Test that now_iso returns UTC time."""
        result = now_iso()

        # Should contain timezone info
        assert "+" in result or "Z" in result


class TestFileChecksum:
    """Tests for file_checksum function."""

    def test_checksum_existing_file(self, tmp_path: Path) -> None:
        """Test checksum of existing file."""
        file_path = tmp_path / "test.txt"
        file_path.write_text("hello world")

        result = file_checksum(file_path)

        assert len(result) == 16  # Truncated SHA256
        assert result.isalnum()

    def test_checksum_nonexistent_file(self, tmp_path: Path) -> None:
        """Test checksum of nonexistent file."""
        result = file_checksum(tmp_path / "nonexistent.txt")

        assert result == ""

    def test_checksum_is_deterministic(self, tmp_path: Path) -> None:
        """Test that same content produces same checksum."""
        file1 = tmp_path / "file1.txt"
        file2 = tmp_path / "file2.txt"
        file1.write_text("same content")
        file2.write_text("same content")

        assert file_checksum(file1) == file_checksum(file2)

    def test_different_content_different_checksum(self, tmp_path: Path) -> None:
        """Test that different content produces different checksum."""
        file1 = tmp_path / "file1.txt"
        file2 = tmp_path / "file2.txt"
        file1.write_text("content one")
        file2.write_text("content two")

        assert file_checksum(file1) != file_checksum(file2)


class TestLoadSaveState:
    """Tests for load_state and save_state functions."""

    def test_load_nonexistent_state(self, temp_state_env: Path) -> None:
        """Test loading state when file doesn't exist."""
        result = load_state()

        assert result is None

    def test_save_and_load_state(self, temp_state_env: Path) -> None:
        """Test saving and loading state."""
        target = str(temp_state_env / "target")
        state = init_state(target)
        save_state(state)

        loaded = load_state()

        assert loaded is not None
        assert loaded["target_dir"] == target
        assert loaded["version"] == "2.0"

    def test_save_updates_timestamp(self, temp_state_env: Path) -> None:
        """Test that save_state updates the timestamp."""
        state = init_state("/tmp/target")
        original_time = state["updated_at"]

        # Small delay to ensure different timestamp
        import time

        time.sleep(0.01)

        save_state(state)

        assert state["updated_at"] != original_time


class TestAddEvent:
    """Tests for add_event function."""

    def test_add_event_basic(self) -> None:
        """Test adding a basic event."""
        state = {"events": []}

        add_event(state, "test_event")

        assert len(state["events"]) == 1
        assert state["events"][0]["type"] == "test_event"
        assert "timestamp" in state["events"][0]

    def test_add_event_with_task_id(self) -> None:
        """Test adding event with task ID."""
        state = {"events": []}

        add_event(state, "task_started", task_id="T001")

        assert state["events"][0]["task_id"] == "T001"

    def test_add_event_with_details(self) -> None:
        """Test adding event with details."""
        state = {"events": []}

        add_event(state, "custom", details={"key": "value"})

        assert state["events"][0]["details"]["key"] == "value"

    def test_add_event_creates_events_list(self) -> None:
        """Test that events list is created if missing."""
        state = {}

        add_event(state, "test")

        assert "events" in state
        assert len(state["events"]) == 1


class TestValidateJson:
    """Tests for validate_json function."""

    def test_validate_valid_data(self, temp_state_env: Path) -> None:
        """Test validating data with all required fields."""
        data = {"version": "1.0", "domains": [], "flows": []}

        valid, error = validate_json(data, "capability-map")

        assert valid is True
        assert error == ""

    def test_validate_missing_field(self, temp_state_env: Path) -> None:
        """Test validating data missing required field."""
        data = {"version": "1.0", "domains": []}  # Missing 'flows'

        valid, error = validate_json(data, "capability-map")

        assert valid is False
        assert "flows" in error.lower()

    def test_validate_nonexistent_schema(self, temp_state_env: Path) -> None:
        """Test validating against nonexistent schema."""
        valid, error = validate_json({}, "nonexistent-schema")

        assert valid is False
        assert "not found" in error.lower()


class TestInitState:
    """Tests for init_state function."""

    def test_init_state_structure(self, tmp_path: Path) -> None:
        """Test that init_state creates correct structure."""
        target = str(tmp_path / "target")
        state = init_state(target)

        assert state["version"] == "2.0"
        assert state["target_dir"] == target
        assert state["phase"]["current"] == "ingestion"
        assert state["phase"]["completed"] == []
        assert state["tasks"] == {}
        assert state["artifacts"] == {}
        assert state["execution"]["current_wave"] == 0

    def test_init_state_resolves_path(self) -> None:
        """Test that init_state resolves relative paths."""
        state = init_state("./relative/path")

        assert state["target_dir"].startswith("/")

    def test_init_state_logs_event(self) -> None:
        """Test that init_state logs initialization event."""
        state = init_state("/tmp/target")

        assert len(state["events"]) == 1
        assert state["events"][0]["type"] == "initialized"


class TestPhaseManagement:
    """Tests for phase management functions."""

    def test_get_phase_order(self) -> None:
        """Test phase order is correct."""
        order = get_phase_order()

        assert order[0] == "ingestion"
        assert order[-1] == "complete"
        assert "executing" in order
        assert "validation" in order
        assert len(order) == 9

    def test_get_next_phase(self) -> None:
        """Test getting next phase."""
        assert get_next_phase("ingestion") == "logical"
        assert get_next_phase("logical") == "physical"
        assert get_next_phase("ready") == "executing"

    def test_get_next_phase_from_complete(self) -> None:
        """Test that complete has no next phase."""
        assert get_next_phase("complete") is None

    def test_get_next_phase_invalid(self) -> None:
        """Test getting next phase from invalid phase."""
        assert get_next_phase("invalid") is None


class TestCanAdvancePhase:
    """Tests for can_advance_phase function."""

    def test_can_advance_from_ingestion(self, temp_state_env: Path) -> None:
        """Test advancing from ingestion requires spec.md."""
        import state as state_module

        state = init_state("/tmp/target")

        # Without spec.md
        can, reason = can_advance_phase(state)
        assert can is False
        assert "spec.md" in reason.lower()

        # With spec.md
        (state_module.PLANNING_DIR / "inputs" / "spec.md").write_text("# Spec")
        can, reason = can_advance_phase(state)
        assert can is True

    def test_can_advance_from_logical(self, temp_state_env: Path) -> None:
        """Test advancing from logical requires validated capability map."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "logical"

        # Without validated artifact
        can, reason = can_advance_phase(state)
        assert can is False

        # With validated artifact
        state["artifacts"]["capability_map"] = {"valid": True}
        can, reason = can_advance_phase(state)
        assert can is True

    def test_can_advance_from_definition(self, temp_state_env: Path) -> None:
        """Test advancing from definition requires tasks."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "definition"

        # Without tasks
        can, reason = can_advance_phase(state)
        assert can is False

        # With tasks
        state["tasks"]["T001"] = {"id": "T001", "wave": 1}
        can, reason = can_advance_phase(state)
        assert can is True


class TestAdvancePhase:
    """Tests for advance_phase function."""

    def test_advance_phase_success(self, temp_state_env: Path) -> None:
        """Test successful phase advancement."""
        import state as state_module

        state = init_state("/tmp/target")
        (state_module.PLANNING_DIR / "inputs" / "spec.md").write_text("# Spec")

        success, msg = advance_phase(state)

        assert success is True
        assert state["phase"]["current"] == "logical"
        assert "ingestion" in state["phase"]["completed"]

    def test_advance_phase_failure(self, temp_state_env: Path) -> None:
        """Test failed phase advancement."""
        state = init_state("/tmp/target")

        success, msg = advance_phase(state)

        assert success is False
        assert state["phase"]["current"] == "ingestion"

    def test_advance_phase_from_complete(self, temp_state_env: Path) -> None:
        """Test cannot advance from complete phase."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "complete"

        success, msg = advance_phase(state)

        assert success is False
        # Message can be "final" or "unknown" depending on implementation
        assert "final" in msg.lower() or "unknown" in msg.lower()


class TestRegisterArtifact:
    """Tests for register_artifact function."""

    def test_register_valid_artifact(self, temp_state_env: Path) -> None:
        """Test registering a valid artifact."""
        import state as state_module

        state = init_state("/tmp/target")

        # Create valid artifact
        artifact = {"version": "1.0", "domains": [], "flows": []}
        artifact_path = state_module.PLANNING_DIR / "artifacts" / "capability-map.json"
        artifact_path.write_text(json.dumps(artifact))

        success, msg = register_artifact(state, "capability_map", str(artifact_path))

        assert success is True
        assert state["artifacts"]["capability_map"]["valid"] is True
        assert state["artifacts"]["capability_map"]["checksum"] != ""

    def test_register_invalid_artifact(self, temp_state_env: Path) -> None:
        """Test registering an invalid artifact."""
        import state as state_module

        state = init_state("/tmp/target")

        # Create invalid artifact (missing required field)
        artifact = {"version": "1.0"}
        artifact_path = state_module.PLANNING_DIR / "artifacts" / "capability-map.json"
        artifact_path.write_text(json.dumps(artifact))

        success, msg = register_artifact(state, "capability_map", str(artifact_path))

        assert success is False
        assert state["artifacts"]["capability_map"]["valid"] is False

    def test_register_nonexistent_artifact(self, temp_state_env: Path) -> None:
        """Test registering nonexistent artifact."""
        state = init_state("/tmp/target")

        success, msg = register_artifact(state, "capability_map", "/nonexistent.json")

        assert success is False
        assert "not found" in msg.lower()


class TestLoadTasksFromDir:
    """Tests for load_tasks_from_dir function."""

    def test_load_tasks_from_empty_dir(self, temp_state_env: Path) -> None:
        """Test loading from empty tasks directory."""
        state = init_state("/tmp/target")

        count = load_tasks_from_dir(state)

        assert count == 0
        assert state["tasks"] == {}

    def test_load_tasks_success(self, temp_state_env: Path) -> None:
        """Test loading tasks successfully."""
        import state as state_module

        state = init_state("/tmp/target")

        # Create task files
        task1 = {"id": "T001", "name": "Task 1", "wave": 1, "dependencies": {"tasks": []}}
        task2 = {"id": "T002", "name": "Task 2", "wave": 2, "dependencies": {"tasks": ["T001"]}}

        (state_module.PLANNING_DIR / "tasks" / "T001.json").write_text(json.dumps(task1))
        (state_module.PLANNING_DIR / "tasks" / "T002.json").write_text(json.dumps(task2))

        count = load_tasks_from_dir(state)

        assert count == 2
        assert "T001" in state["tasks"]
        assert "T002" in state["tasks"]
        assert state["tasks"]["T002"]["depends_on"] == ["T001"]

    def test_load_tasks_computes_blocks(self, temp_state_env: Path) -> None:
        """Test that reverse dependencies (blocks) are computed."""
        import state as state_module

        state = init_state("/tmp/target")

        task1 = {"id": "T001", "name": "Task 1", "wave": 1, "dependencies": {"tasks": []}}
        task2 = {"id": "T002", "name": "Task 2", "wave": 2, "dependencies": {"tasks": ["T001"]}}

        (state_module.PLANNING_DIR / "tasks" / "T001.json").write_text(json.dumps(task1))
        (state_module.PLANNING_DIR / "tasks" / "T002.json").write_text(json.dumps(task2))

        load_tasks_from_dir(state)

        # T001 blocks T002
        assert "T002" in state["tasks"]["T001"]["blocks"]


class TestGetReadyTasks:
    """Tests for get_ready_tasks function."""

    def test_get_ready_no_dependencies(self, temp_state_env: Path) -> None:
        """Test getting ready tasks with no dependencies."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "pending", "depends_on": []},
            "T002": {"id": "T002", "status": "pending", "depends_on": []},
        }

        ready = get_ready_tasks(state)

        assert len(ready) == 2
        assert "T001" in ready
        assert "T002" in ready

    def test_get_ready_with_dependencies(self, temp_state_env: Path) -> None:
        """Test getting ready tasks respects dependencies."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "pending", "depends_on": []},
            "T002": {"id": "T002", "status": "pending", "depends_on": ["T001"]},
        }

        ready = get_ready_tasks(state)

        # Only T001 is ready (T002 depends on incomplete T001)
        assert ready == ["T001"]

    def test_get_ready_after_completion(self, temp_state_env: Path) -> None:
        """Test that task becomes ready after dependency completes."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "complete", "depends_on": []},
            "T002": {"id": "T002", "status": "pending", "depends_on": ["T001"]},
        }

        ready = get_ready_tasks(state)

        assert "T002" in ready

    def test_running_tasks_not_ready(self, temp_state_env: Path) -> None:
        """Test that running tasks are not listed as ready."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "running", "depends_on": []},
        }

        ready = get_ready_tasks(state)

        assert ready == []


class TestStartTask:
    """Tests for start_task function."""

    def test_start_task_success(self, temp_state_env: Path) -> None:
        """Test starting a pending task."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "pending", "depends_on": []},
        }

        success, msg = start_task(state, "T001")

        assert success is True
        assert state["tasks"]["T001"]["status"] == "running"
        assert "T001" in state["execution"]["active_tasks"]
        assert "started_at" in state["tasks"]["T001"]

    def test_start_task_not_found(self, temp_state_env: Path) -> None:
        """Test starting nonexistent task."""
        state = init_state("/tmp/target")
        state["tasks"] = {}

        success, msg = start_task(state, "T999")

        assert success is False
        assert "not found" in msg.lower()

    def test_start_task_not_pending(self, temp_state_env: Path) -> None:
        """Test starting a task that's not pending."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "running", "depends_on": []},
        }

        success, msg = start_task(state, "T001")

        assert success is False
        assert "running" in msg.lower()

    def test_start_task_dependency_not_met(self, temp_state_env: Path) -> None:
        """Test starting task with incomplete dependency."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "pending", "depends_on": []},
            "T002": {"id": "T002", "status": "pending", "depends_on": ["T001"]},
        }

        success, msg = start_task(state, "T002")

        assert success is False
        assert "T001" in msg


class TestCompleteTask:
    """Tests for complete_task function."""

    def test_complete_task_success(self, temp_state_env: Path) -> None:
        """Test completing a running task."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "running", "depends_on": []},
        }
        state["execution"]["active_tasks"] = ["T001"]

        success, msg = complete_task(state, "T001", files_created=["src/file.py"])

        assert success is True
        assert state["tasks"]["T001"]["status"] == "complete"
        assert state["tasks"]["T001"]["files_created"] == ["src/file.py"]
        assert "T001" not in state["execution"]["active_tasks"]
        assert state["execution"]["completed_count"] == 1

    def test_complete_task_not_running(self, temp_state_env: Path) -> None:
        """Test completing a task that's not running."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "pending", "depends_on": []},
        }

        success, msg = complete_task(state, "T001")

        assert success is False
        assert "pending" in msg.lower()


class TestFailTask:
    """Tests for fail_task function."""

    def test_fail_task_success(self, temp_state_env: Path) -> None:
        """Test failing a task."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "running", "depends_on": [], "blocks": ["T002"]},
            "T002": {"id": "T002", "status": "pending", "depends_on": ["T001"], "blocks": []},
        }
        state["execution"]["active_tasks"] = ["T001"]

        success, msg = fail_task(state, "T001", "Test failed")

        assert success is True
        assert state["tasks"]["T001"]["status"] == "failed"
        assert state["tasks"]["T001"]["error"] == "Test failed"
        assert state["execution"]["failed_count"] == 1

    def test_fail_task_blocks_dependents(self, temp_state_env: Path) -> None:
        """Test that failing a task blocks dependent tasks."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "running", "depends_on": [], "blocks": ["T002"]},
            "T002": {"id": "T002", "status": "pending", "depends_on": ["T001"], "blocks": []},
        }
        state["execution"]["active_tasks"] = ["T001"]

        fail_task(state, "T001", "Test failed")

        assert state["tasks"]["T002"]["status"] == "blocked"
        assert "T001" in state["tasks"]["T002"]["error"]


class TestLogTokens:
    """Tests for log_tokens function."""

    def test_log_tokens(self, temp_state_env: Path) -> None:
        """Test logging token usage."""
        state = init_state("/tmp/target")

        log_tokens(state, "session-123", 1000, 500, 0.05)

        assert state["execution"]["total_tokens"] == 1500
        assert state["execution"]["total_cost_usd"] == 0.05

    def test_log_tokens_accumulates(self, temp_state_env: Path) -> None:
        """Test that token logging accumulates."""
        state = init_state("/tmp/target")

        log_tokens(state, "session-1", 1000, 500, 0.05)
        log_tokens(state, "session-2", 2000, 1000, 0.10)

        assert state["execution"]["total_tokens"] == 4500
        assert abs(state["execution"]["total_cost_usd"] - 0.15) < 0.0001

    def test_log_tokens_adds_event(self, temp_state_env: Path) -> None:
        """Test that token logging adds event."""
        state = init_state("/tmp/target")

        log_tokens(state, "session-123", 1000, 500, 0.05)

        # Find tokens_logged event
        token_events = [e for e in state["events"] if e["type"] == "tokens_logged"]
        assert len(token_events) == 1
        assert token_events[0]["details"]["session_id"] == "session-123"


class TestCommitTaskChanges:
    """Tests for commit_task_changes function."""

    def test_commit_task_not_found(self, temp_state_env: Path) -> None:
        """Test committing nonexistent task."""
        state = init_state("/tmp/target")
        state["tasks"] = {}

        success, msg = commit_task_changes(state, "T999")

        assert success is False
        assert "not found" in msg.lower()

    def test_commit_task_not_complete(self, temp_state_env: Path) -> None:
        """Test committing a task that's not complete."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {"id": "T001", "status": "running", "depends_on": []},
        }

        success, msg = commit_task_changes(state, "T001")

        assert success is False
        assert "running" in msg.lower()

    def test_commit_task_no_files(self, temp_state_env: Path) -> None:
        """Test committing a task with no files tracked."""
        state = init_state("/tmp/target")
        state["tasks"] = {
            "T001": {
                "id": "T001",
                "name": "Test task",
                "status": "complete",
                "depends_on": [],
                "files_created": [],
                "files_modified": [],
            },
        }

        success, msg = commit_task_changes(state, "T001")

        assert success is False
        assert "no files" in msg.lower()

    def test_commit_task_success(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        """Test successful commit of task files."""
        import subprocess
        import state as state_module

        # Create a git repo in tmp_path
        target_dir = tmp_path / "target"
        target_dir.mkdir()
        subprocess.run(["git", "init"], cwd=target_dir, capture_output=True)
        subprocess.run(
            ["git", "config", "user.email", "test@test.com"],
            cwd=target_dir,
            capture_output=True,
        )
        subprocess.run(
            ["git", "config", "user.name", "Test"],
            cwd=target_dir,
            capture_output=True,
        )

        # Create a file to commit
        test_file = target_dir / "src" / "file.py"
        test_file.parent.mkdir(parents=True)
        test_file.write_text("# test")

        # Setup state environment
        planning_dir = tmp_path / "project-planning"
        planning_dir.mkdir()
        monkeypatch.setattr(state_module, "PLANNING_DIR", planning_dir)
        monkeypatch.setattr(state_module, "STATE_FILE", planning_dir / "state.json")

        state = init_state(str(target_dir))
        state["tasks"] = {
            "T001": {
                "id": "T001",
                "name": "Implement feature",
                "status": "complete",
                "depends_on": [],
                "files_created": ["src/file.py"],
                "files_modified": [],
            },
        }

        success, msg = commit_task_changes(state, "T001")

        assert success is True
        assert "1 file(s)" in msg

        # Verify commit was made
        result = subprocess.run(
            ["git", "log", "--oneline", "-1"],
            cwd=target_dir,
            capture_output=True,
            text=True,
        )
        assert "T001: Implement feature" in result.stdout

    def test_commit_task_adds_event(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        """Test that committing adds event to state."""
        import subprocess
        import state as state_module

        # Create a git repo
        target_dir = tmp_path / "target"
        target_dir.mkdir()
        subprocess.run(["git", "init"], cwd=target_dir, capture_output=True)
        subprocess.run(
            ["git", "config", "user.email", "test@test.com"],
            cwd=target_dir,
            capture_output=True,
        )
        subprocess.run(
            ["git", "config", "user.name", "Test"],
            cwd=target_dir,
            capture_output=True,
        )

        test_file = target_dir / "file.py"
        test_file.write_text("# test")

        planning_dir = tmp_path / "project-planning"
        planning_dir.mkdir()
        monkeypatch.setattr(state_module, "PLANNING_DIR", planning_dir)
        monkeypatch.setattr(state_module, "STATE_FILE", planning_dir / "state.json")

        state = init_state(str(target_dir))
        state["tasks"] = {
            "T001": {
                "id": "T001",
                "name": "Test",
                "status": "complete",
                "depends_on": [],
                "files_created": ["file.py"],
                "files_modified": [],
            },
        }

        commit_task_changes(state, "T001")

        commit_events = [e for e in state["events"] if e["type"] == "task_committed"]
        assert len(commit_events) == 1
        assert commit_events[0]["task_id"] == "T001"
        assert commit_events[0]["details"]["files"] == ["file.py"]


class TestRegisterTaskValidation:
    """Tests for register_task_validation function."""

    def test_register_ready_verdict(self) -> None:
        """Test registering READY verdict."""
        state = init_state("/tmp/target")

        success, msg = register_task_validation(state, "READY", "All tasks aligned")

        assert success is True
        assert "READY" in msg
        assert state["artifacts"]["task_validation"]["verdict"] == "READY"
        assert state["artifacts"]["task_validation"]["valid"] is True
        assert state["artifacts"]["task_validation"]["summary"] == "All tasks aligned"

    def test_register_ready_with_notes_verdict(self) -> None:
        """Test registering READY_WITH_NOTES verdict."""
        state = init_state("/tmp/target")
        issues = ["T002: missing constraints", "T005: unclear deps"]

        success, msg = register_task_validation(
            state, "READY_WITH_NOTES", "Minor issues found", issues
        )

        assert success is True
        assert state["artifacts"]["task_validation"]["verdict"] == "READY_WITH_NOTES"
        assert state["artifacts"]["task_validation"]["valid"] is True
        assert state["artifacts"]["task_validation"]["issues"] == issues

    def test_register_blocked_verdict(self) -> None:
        """Test registering BLOCKED verdict."""
        state = init_state("/tmp/target")
        issues = ["T005: not in spec"]

        success, msg = register_task_validation(
            state, "BLOCKED", "Critical issues found", issues
        )

        assert success is False
        assert "blocked" in msg.lower()
        assert state["artifacts"]["task_validation"]["verdict"] == "BLOCKED"
        assert state["artifacts"]["task_validation"]["valid"] is False

    def test_register_invalid_verdict(self) -> None:
        """Test registering invalid verdict fails."""
        state = init_state("/tmp/target")

        success, msg = register_task_validation(state, "INVALID")

        assert success is False
        assert "Invalid verdict" in msg

    def test_register_adds_event(self) -> None:
        """Test that registration adds event."""
        state = init_state("/tmp/target")

        register_task_validation(state, "READY", "All good")

        events = [e for e in state["events"] if e["type"] == "task_validation_complete"]
        assert len(events) == 1
        assert events[0]["details"]["verdict"] == "READY"
        assert events[0]["details"]["valid"] is True


class TestCanAdvanceFromValidation:
    """Tests for can_advance_phase from validation phase."""

    def test_can_advance_without_validation(self) -> None:
        """Test cannot advance without task validation."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "validation"

        can, reason = can_advance_phase(state)

        assert can is False
        assert "validation" in reason.lower()

    def test_can_advance_with_ready_verdict(self) -> None:
        """Test can advance with READY verdict."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "validation"
        state["artifacts"]["task_validation"] = {
            "verdict": "READY",
            "valid": True,
        }

        can, reason = can_advance_phase(state)

        assert can is True

    def test_can_advance_with_ready_with_notes_verdict(self) -> None:
        """Test can advance with READY_WITH_NOTES verdict."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "validation"
        state["artifacts"]["task_validation"] = {
            "verdict": "READY_WITH_NOTES",
            "valid": True,
        }

        can, reason = can_advance_phase(state)

        assert can is True

    def test_cannot_advance_with_blocked_verdict(self) -> None:
        """Test cannot advance with BLOCKED verdict."""
        state = init_state("/tmp/target")
        state["phase"]["current"] = "validation"
        state["artifacts"]["task_validation"] = {
            "verdict": "BLOCKED",
            "valid": False,
            "error": "T005 not in spec",
        }

        can, reason = can_advance_phase(state)

        assert can is False
        assert "blocking" in reason.lower() or "validation" in reason.lower()
