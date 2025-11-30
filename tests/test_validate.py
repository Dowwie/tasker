"""Tests for validate.py - validation module for task decomposition."""

import json
import sys
from pathlib import Path

import pytest

# Add scripts to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))

from validate import (
    build_dependency_graph,
    compute_calibration_metrics,
    detect_cycles,
    prepare_rollback_checksums,
    validate_all_verification_commands,
    validate_dag,
    validate_steel_thread,
    verify_rollback_integrity,
)


@pytest.fixture
def temp_validate_env(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Set up temporary validation environment."""
    import validate

    planning_dir = tmp_path / "project-planning"
    planning_dir.mkdir()
    (planning_dir / "tasks").mkdir()

    monkeypatch.setattr(validate, "PROJECT_ROOT", tmp_path)
    monkeypatch.setattr(validate, "PLANNING_DIR", planning_dir)
    monkeypatch.setattr(validate, "STATE_FILE", planning_dir / "state.json")
    monkeypatch.setattr(validate, "TASKS_DIR", planning_dir / "tasks")

    return tmp_path


class TestBuildDependencyGraph:
    """Tests for build_dependency_graph function."""

    def test_empty_tasks(self) -> None:
        """Test with no tasks."""
        state = {"tasks": {}}
        graph = build_dependency_graph(state)
        assert graph == {}

    def test_single_task_no_deps(self) -> None:
        """Test single task with no dependencies."""
        state = {"tasks": {"T001": {"id": "T001", "depends_on": []}}}
        graph = build_dependency_graph(state)
        assert "T001" in graph
        assert graph["T001"] == []

    def test_linear_dependency_chain(self) -> None:
        """Test linear A -> B -> C dependency chain."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": []},
                "T002": {"id": "T002", "depends_on": ["T001"]},
                "T003": {"id": "T003", "depends_on": ["T002"]},
            }
        }
        graph = build_dependency_graph(state)

        # T001 blocks T002
        assert "T002" in graph["T001"]
        # T002 blocks T003
        assert "T003" in graph["T002"]

    def test_diamond_dependency(self) -> None:
        """Test diamond pattern A -> B,C -> D."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": []},
                "T002": {"id": "T002", "depends_on": ["T001"]},
                "T003": {"id": "T003", "depends_on": ["T001"]},
                "T004": {"id": "T004", "depends_on": ["T002", "T003"]},
            }
        }
        graph = build_dependency_graph(state)

        # T001 blocks both T002 and T003
        assert set(graph["T001"]) == {"T002", "T003"}
        # Both T002 and T003 block T004
        assert "T004" in graph["T002"]
        assert "T004" in graph["T003"]


class TestDetectCycles:
    """Tests for detect_cycles function."""

    def test_no_cycles_linear(self) -> None:
        """Test linear chain has no cycles."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": []},
                "T002": {"id": "T002", "depends_on": ["T001"]},
                "T003": {"id": "T003", "depends_on": ["T002"]},
            }
        }
        has_cycles, cycles = detect_cycles(state)
        assert has_cycles is False
        assert cycles == []

    def test_no_cycles_diamond(self) -> None:
        """Test diamond pattern has no cycles."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": []},
                "T002": {"id": "T002", "depends_on": ["T001"]},
                "T003": {"id": "T003", "depends_on": ["T001"]},
                "T004": {"id": "T004", "depends_on": ["T002", "T003"]},
            }
        }
        has_cycles, cycles = detect_cycles(state)
        assert has_cycles is False

    def test_simple_cycle(self) -> None:
        """Test detection of simple A -> B -> A cycle."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": ["T002"]},
                "T002": {"id": "T002", "depends_on": ["T001"]},
            }
        }
        has_cycles, cycles = detect_cycles(state)
        assert has_cycles is True
        assert len(cycles) > 0

    def test_self_cycle(self) -> None:
        """Test detection of self-referencing cycle."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": ["T001"]},
            }
        }
        has_cycles, cycles = detect_cycles(state)
        assert has_cycles is True

    def test_longer_cycle(self) -> None:
        """Test detection of A -> B -> C -> A cycle."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": ["T003"]},
                "T002": {"id": "T002", "depends_on": ["T001"]},
                "T003": {"id": "T003", "depends_on": ["T002"]},
            }
        }
        has_cycles, cycles = detect_cycles(state)
        assert has_cycles is True

    def test_empty_tasks(self) -> None:
        """Test with no tasks."""
        state = {"tasks": {}}
        has_cycles, cycles = detect_cycles(state)
        assert has_cycles is False
        assert cycles == []


class TestValidateDag:
    """Tests for validate_dag function."""

    def test_valid_dag(self) -> None:
        """Test valid DAG returns success."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": []},
                "T002": {"id": "T002", "depends_on": ["T001"]},
            }
        }
        valid, msg = validate_dag(state)
        assert valid is True
        assert "valid" in msg.lower()

    def test_invalid_dag_with_cycle(self) -> None:
        """Test invalid DAG with cycle returns failure."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": ["T002"]},
                "T002": {"id": "T002", "depends_on": ["T001"]},
            }
        }
        valid, msg = validate_dag(state)
        assert valid is False
        assert "cycle" in msg.lower()


class TestValidateSteelThread:
    """Tests for validate_steel_thread function."""

    def test_no_steel_thread_tasks(self, temp_validate_env: Path) -> None:
        """Test when no steel thread tasks defined."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": [], "wave": 1},
            }
        }
        # Create task file without steel_thread
        import validate

        task = {"id": "T001", "context": {"steel_thread": False}}
        (validate.TASKS_DIR / "T001.json").write_text(json.dumps(task))

        valid, issues = validate_steel_thread(state)
        assert valid is False
        assert any("no steel thread" in i.lower() for i in issues)

    def test_steel_thread_in_late_wave(self, temp_validate_env: Path) -> None:
        """Test steel thread task in late wave."""
        state = {
            "tasks": {
                "T001": {"id": "T001", "depends_on": [], "wave": 1},
                "T002": {"id": "T002", "depends_on": [], "wave": 5},
            }
        }
        import validate

        # T001 is steel thread in wave 1 (good)
        task1 = {"id": "T001", "context": {"steel_thread": True}}
        (validate.TASKS_DIR / "T001.json").write_text(json.dumps(task1))

        # T002 is steel thread in wave 5 (bad, given max wave is 5)
        task2 = {"id": "T002", "context": {"steel_thread": True}}
        (validate.TASKS_DIR / "T002.json").write_text(json.dumps(task2))

        valid, issues = validate_steel_thread(state)
        # Should flag T002 as being in a late wave
        assert any("wave" in i.lower() for i in issues)


class TestComputeCalibrationMetrics:
    """Tests for compute_calibration_metrics function."""

    def test_empty_state(self) -> None:
        """Test with no verified tasks."""
        state = {"tasks": {}}
        metrics = compute_calibration_metrics(state)

        assert metrics["total_verified"] == 0
        assert metrics["calibration_score"] == 0.0

    def test_all_pass(self) -> None:
        """Test with all tasks passing verification."""
        state = {
            "tasks": {
                "T001": {
                    "status": "complete",
                    "verification": {"verdict": "PASS", "recommendation": "PROCEED"},
                },
                "T002": {
                    "status": "complete",
                    "verification": {"verdict": "PASS", "recommendation": "PROCEED"},
                },
            }
        }
        metrics = compute_calibration_metrics(state)

        assert metrics["total_verified"] == 2
        assert metrics["verdict_distribution"]["PASS"] == 2
        assert metrics["calibration_score"] == 1.0
        assert len(metrics["false_positives"]) == 0

    def test_false_positive_detection(self) -> None:
        """Test detection of false positives (PROCEED but failed)."""
        state = {
            "tasks": {
                "T001": {
                    "status": "failed",
                    "error": "Test failed",
                    "verification": {"verdict": "PASS", "recommendation": "PROCEED"},
                },
            }
        }
        metrics = compute_calibration_metrics(state)

        assert len(metrics["false_positives"]) == 1
        assert metrics["false_positives"][0]["task_id"] == "T001"
        assert metrics["calibration_score"] == 0.0

    def test_false_negative_detection(self) -> None:
        """Test detection of potential false negatives (BLOCK but succeeded)."""
        state = {
            "tasks": {
                "T001": {
                    "status": "complete",
                    "verification": {"verdict": "FAIL", "recommendation": "BLOCK"},
                },
            }
        }
        metrics = compute_calibration_metrics(state)

        assert len(metrics["false_negatives"]) == 1
        assert metrics["false_negatives"][0]["task_id"] == "T001"

    def test_mixed_results(self) -> None:
        """Test with mixed verification results."""
        state = {
            "tasks": {
                "T001": {
                    "status": "complete",
                    "verification": {"verdict": "PASS", "recommendation": "PROCEED"},
                },
                "T002": {
                    "status": "complete",
                    "verification": {"verdict": "CONDITIONAL", "recommendation": "PROCEED"},
                },
                "T003": {
                    "status": "failed",
                    "verification": {"verdict": "FAIL", "recommendation": "BLOCK"},
                },
            }
        }
        metrics = compute_calibration_metrics(state)

        assert metrics["total_verified"] == 3
        assert metrics["verdict_distribution"]["PASS"] == 1
        assert metrics["verdict_distribution"]["CONDITIONAL"] == 1
        assert metrics["verdict_distribution"]["FAIL"] == 1


class TestRollbackIntegrity:
    """Tests for rollback integrity functions."""

    def test_prepare_checksums(self, tmp_path: Path) -> None:
        """Test preparing checksums before modification."""
        # Create test files
        file1 = tmp_path / "src" / "file1.py"
        file1.parent.mkdir(parents=True)
        file1.write_text("original content 1")

        file2 = tmp_path / "src" / "file2.py"
        file2.write_text("original content 2")

        checksums = prepare_rollback_checksums(
            tmp_path, ["src/file1.py", "src/file2.py", "src/new.py"]
        )

        assert "src/file1.py" in checksums
        assert "src/file2.py" in checksums
        assert len(checksums["src/file1.py"]) == 64  # SHA256 hex
        assert checksums["src/new.py"] == ""  # New file doesn't exist

    def test_verify_rollback_success(self, tmp_path: Path) -> None:
        """Test successful rollback verification."""
        # Create original file
        file1 = tmp_path / "src" / "file1.py"
        file1.parent.mkdir(parents=True)
        file1.write_text("original content")

        # Get original checksum
        original_checksums = prepare_rollback_checksums(tmp_path, ["src/file1.py"])

        # Simulate task execution that creates a new file
        new_file = tmp_path / "src" / "new.py"
        new_file.write_text("new content")

        # Simulate rollback: delete created file
        new_file.unlink()

        # Verify rollback
        success, issues = verify_rollback_integrity(
            tmp_path,
            original_checksums,
            files_created=["src/new.py"],
            files_modified=["src/file1.py"],
        )

        assert success is True
        assert len(issues) == 0

    def test_verify_rollback_file_not_deleted(self, tmp_path: Path) -> None:
        """Test rollback failure when created file not deleted."""
        file1 = tmp_path / "src" / "file1.py"
        file1.parent.mkdir(parents=True)
        file1.write_text("original")

        original_checksums = prepare_rollback_checksums(tmp_path, ["src/file1.py"])

        # Create new file and don't delete it
        new_file = tmp_path / "src" / "new.py"
        new_file.write_text("new content")

        success, issues = verify_rollback_integrity(
            tmp_path,
            original_checksums,
            files_created=["src/new.py"],
            files_modified=[],
        )

        assert success is False
        assert any("not deleted" in i for i in issues)

    def test_verify_rollback_file_not_restored(self, tmp_path: Path) -> None:
        """Test rollback failure when modified file not restored."""
        file1 = tmp_path / "src" / "file1.py"
        file1.parent.mkdir(parents=True)
        file1.write_text("original content")

        original_checksums = prepare_rollback_checksums(tmp_path, ["src/file1.py"])

        # Modify the file and don't restore it
        file1.write_text("modified content")

        success, issues = verify_rollback_integrity(
            tmp_path,
            original_checksums,
            files_created=[],
            files_modified=["src/file1.py"],
        )

        assert success is False
        assert any("not restored" in i for i in issues)


class TestValidateAllVerificationCommands:
    """Tests for validate_all_verification_commands function."""

    def test_valid_commands(self, temp_validate_env: Path) -> None:
        """Test with valid verification commands."""
        import validate

        state = {"tasks": {"T001": {"id": "T001"}}}

        task = {
            "id": "T001",
            "acceptance_criteria": [
                {"criterion": "Tests pass", "verification": "pytest tests/"},
                {"criterion": "Lint passes", "verification": "ruff check src/"},
            ],
        }
        (validate.TASKS_DIR / "T001.json").write_text(json.dumps(task))

        valid, issues = validate_all_verification_commands(state)

        assert valid is True
        assert len(issues) == 0

    def test_empty_command(self, temp_validate_env: Path) -> None:
        """Test detection of empty verification command."""
        import validate

        state = {"tasks": {"T001": {"id": "T001"}}}

        task = {
            "id": "T001",
            "acceptance_criteria": [
                {"criterion": "Something", "verification": ""},
            ],
        }
        (validate.TASKS_DIR / "T001.json").write_text(json.dumps(task))

        valid, issues = validate_all_verification_commands(state)

        assert valid is False
        assert "T001" in issues

    def test_invalid_command_syntax(self, temp_validate_env: Path) -> None:
        """Test detection of syntactically invalid command."""
        import validate

        state = {"tasks": {"T001": {"id": "T001"}}}

        task = {
            "id": "T001",
            "acceptance_criteria": [
                {"criterion": "Bad command", "verification": "echo 'unclosed quote"},
            ],
        }
        (validate.TASKS_DIR / "T001.json").write_text(json.dumps(task))

        valid, issues = validate_all_verification_commands(state)

        assert valid is False
        assert "T001" in issues

    def test_complex_valid_command(self, temp_validate_env: Path) -> None:
        """Test with complex but valid command."""
        import validate

        state = {"tasks": {"T001": {"id": "T001"}}}

        task = {
            "id": "T001",
            "acceptance_criteria": [
                {
                    "criterion": "Integration test",
                    "verification": "pytest tests/integration/ -v --tb=short -x",
                },
            ],
        }
        (validate.TASKS_DIR / "T001.json").write_text(json.dumps(task))

        valid, issues = validate_all_verification_commands(state)

        assert valid is True
