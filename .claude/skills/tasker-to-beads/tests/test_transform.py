#!/usr/bin/env python3
"""
Unit tests for the tasker-to-beads transform script.

Run with: python3 -m pytest .claude/skills/tasker-to-beads/tests/ -v
"""

import json
import tempfile
from pathlib import Path
from unittest.mock import patch

import pytest

# Import the module under test
import sys
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
import transform


class TestPhaseToPriority:
    """Tests for phase_to_priority mapping."""

    def test_phase_1_is_critical(self):
        assert transform.phase_to_priority(1) == "critical"

    def test_phase_2_is_high(self):
        assert transform.phase_to_priority(2) == "high"

    def test_phase_3_is_medium(self):
        assert transform.phase_to_priority(3) == "medium"

    def test_phase_4_is_medium(self):
        assert transform.phase_to_priority(4) == "medium"

    def test_phase_5_is_low(self):
        assert transform.phase_to_priority(5) == "low"

    def test_phase_6_is_low(self):
        assert transform.phase_to_priority(6) == "low"

    def test_phase_0_is_low(self):
        assert transform.phase_to_priority(0) == "low"


class TestBuildLabels:
    """Tests for build_labels function."""

    def test_builds_domain_label(self):
        task = {"context": {"domain": "Infrastructure"}, "phase": 1}
        labels = transform.build_labels(task, {})
        assert "domain:infrastructure" in labels

    def test_builds_capability_label(self):
        task = {"context": {"capability": "Project Setup"}, "phase": 1}
        labels = transform.build_labels(task, {})
        assert "capability:project-setup" in labels

    def test_builds_steel_thread_label(self):
        task = {"context": {"steel_thread": True}, "phase": 1}
        labels = transform.build_labels(task, {})
        assert "steel-thread" in labels

    def test_builds_phase_label(self):
        task = {"context": {}, "phase": 2}
        labels = transform.build_labels(task, {})
        assert "phase:2" in labels

    def test_no_phase_label_for_phase_0(self):
        task = {"context": {}, "phase": 0}
        labels = transform.build_labels(task, {})
        assert not any(l.startswith("phase:") for l in labels)

    def test_handles_spaces_in_domain(self):
        task = {"context": {"domain": "State Management"}, "phase": 1}
        labels = transform.build_labels(task, {})
        assert "domain:state-management" in labels

    def test_empty_context(self):
        task = {"context": {}, "phase": 1}
        labels = transform.build_labels(task, {})
        assert labels == ["phase:1"]


class TestExtractRelevantSpecSections:
    """Tests for extract_relevant_spec_sections function."""

    def test_returns_empty_for_empty_spec(self):
        task = {"name": "Test task", "context": {}, "files": []}
        result = transform.extract_relevant_spec_sections("", task, {})
        assert result == []

    def test_finds_sections_matching_task_name(self):
        spec = """# Introduction
Some intro text.

## Session Management
This section discusses session handling and state.

## Other Section
Unrelated content here.
"""
        task = {"name": "Session state handler", "context": {}, "files": []}
        result = transform.extract_relevant_spec_sections(spec, task, {})
        assert len(result) >= 1
        assert any("session" in s.lower() for s in result)

    def test_finds_sections_matching_domain(self):
        spec = """# Overview

## Database Integration
PostgreSQL setup and configuration.

## Frontend
React components.
"""
        task = {
            "name": "Setup",
            "context": {"domain": "Database"},
            "files": [],
        }
        result = transform.extract_relevant_spec_sections(spec, task, {})
        assert any("database" in s.lower() for s in result)

    def test_truncates_long_sections(self):
        long_section = "## Test Section\n" + "x" * 2000
        spec = long_section
        task = {"name": "Test section handler", "context": {}, "files": []}
        result = transform.extract_relevant_spec_sections(spec, task, {})
        if result:
            assert len(result[0]) < 1600
            assert "[...truncated...]" in result[0]

    def test_limits_to_5_sections(self):
        # Create a spec with many matching sections
        sections = [f"## Section {i}\ntest keyword content here" for i in range(10)]
        spec = "\n\n".join(sections)
        task = {"name": "Test keyword", "context": {}, "files": []}
        result = transform.extract_relevant_spec_sections(spec, task, {})
        assert len(result) <= 5

    def test_uses_file_paths_for_keywords(self):
        spec = """# Overview

## Authentication Module
Login and auth handling.

## Database
Data storage.
"""
        task = {
            "name": "Setup",
            "context": {},
            "files": [{"path": "lib/auth/login.ex"}],
        }
        result = transform.extract_relevant_spec_sections(spec, task, {})
        assert any("auth" in s.lower() for s in result)


class TestFindCapabilityContext:
    """Tests for find_capability_context function."""

    def test_finds_matching_domain_and_capability(self):
        capability_map = {
            "domains": [
                {
                    "name": "Infrastructure",
                    "id": "D1",
                    "description": "Core infrastructure",
                    "capabilities": [
                        {
                            "name": "Project Setup",
                            "id": "C1",
                            "description": "Initial project setup",
                            "spec_ref": "Section 1.2",
                            "behaviors": [
                                {"id": "B1", "name": "Init mix", "type": "process"}
                            ],
                        }
                    ],
                }
            ]
        }
        task = {
            "context": {"domain": "Infrastructure", "capability": "Project Setup"},
            "behaviors": ["B1"],
        }
        result = transform.find_capability_context(capability_map, task)

        assert result["domain"]["name"] == "Infrastructure"
        assert result["capability"]["name"] == "Project Setup"
        assert len(result["behaviors"]) == 1
        assert result["behaviors"][0]["id"] == "B1"

    def test_returns_nulls_for_no_match(self):
        capability_map = {"domains": []}
        task = {"context": {"domain": "Unknown"}, "behaviors": []}
        result = transform.find_capability_context(capability_map, task)

        assert result["domain"] is None
        assert result["capability"] is None
        assert result["behaviors"] == []

    def test_handles_empty_capability_map(self):
        result = transform.find_capability_context({}, {"context": {}, "behaviors": []})
        assert result["domain"] is None
        assert result["capability"] is None

    def test_filters_behaviors_to_task_behaviors(self):
        capability_map = {
            "domains": [
                {
                    "name": "Test",
                    "capabilities": [
                        {
                            "name": "Cap",
                            "behaviors": [
                                {"id": "B1", "name": "First"},
                                {"id": "B2", "name": "Second"},
                                {"id": "B3", "name": "Third"},
                            ],
                        }
                    ],
                }
            ]
        }
        task = {
            "context": {"domain": "Test", "capability": "Cap"},
            "behaviors": ["B1", "B3"],  # Only B1 and B3, not B2
        }
        result = transform.find_capability_context(capability_map, task)

        behavior_ids = [b["id"] for b in result["behaviors"]]
        assert "B1" in behavior_ids
        assert "B3" in behavior_ids
        assert "B2" not in behavior_ids


class TestGetDependencyContext:
    """Tests for get_dependency_context function."""

    def test_returns_empty_for_no_dependencies(self):
        task = {"dependencies": {"tasks": []}}
        result = transform.get_dependency_context({}, task)
        assert result == []

    def test_returns_dependency_info(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create a mock task file
            tasks_dir = Path(tmpdir) / "project-planning" / "tasks"
            tasks_dir.mkdir(parents=True)

            dep_task = {
                "id": "T001",
                "name": "Dependency task",
                "files": [
                    {"path": "lib/foo.ex"},
                    {"path": "lib/bar.ex"},
                ],
            }
            (tasks_dir / "T001.json").write_text(json.dumps(dep_task))

            # Patch the TASKS_DIR
            with patch.object(transform, "TASKS_DIR", tasks_dir):
                task = {"dependencies": {"tasks": ["T001"]}}
                result = transform.get_dependency_context({}, task)

            assert len(result) == 1
            assert result[0]["id"] == "T001"
            assert result[0]["name"] == "Dependency task"
            assert "lib/foo.ex" in result[0]["files_created"]

    def test_handles_missing_dependency_task(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tasks_dir = Path(tmpdir) / "project-planning" / "tasks"
            tasks_dir.mkdir(parents=True)

            with patch.object(transform, "TASKS_DIR", tasks_dir):
                task = {"dependencies": {"tasks": ["T999"]}}  # Doesn't exist
                result = transform.get_dependency_context({}, task)

            assert result == []


class TestLoadJson:
    """Tests for load_json function."""

    def test_loads_valid_json(self):
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump({"key": "value"}, f)
            f.flush()
            result = transform.load_json(Path(f.name))

        assert result == {"key": "value"}
        Path(f.name).unlink()

    def test_returns_none_for_missing_file(self):
        result = transform.load_json(Path("/nonexistent/file.json"))
        assert result is None


class TestFindProjectRoot:
    """Tests for find_project_root function."""

    def test_finds_project_with_project_planning(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create project-planning directory
            (Path(tmpdir) / "project-planning").mkdir()

            # Create skill directory structure
            skill_dir = Path(tmpdir) / ".claude" / "skills" / "test-skill"
            skill_dir.mkdir(parents=True)

            with patch.object(transform, "SKILL_DIR", skill_dir):
                result = transform.find_project_root()

            assert result == Path(tmpdir)

    def test_finds_project_with_git(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create .git directory
            (Path(tmpdir) / ".git").mkdir()

            # Create skill directory structure
            skill_dir = Path(tmpdir) / ".claude" / "skills" / "test-skill"
            skill_dir.mkdir(parents=True)

            with patch.object(transform, "SKILL_DIR", skill_dir):
                result = transform.find_project_root()

            assert result == Path(tmpdir)


class TestPrepareTaskContext:
    """Integration tests for prepare_task_context function."""

    def test_prepares_complete_context(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tmppath = Path(tmpdir)

            # Create directory structure
            planning_dir = tmppath / "project-planning"
            tasks_dir = planning_dir / "tasks"
            artifacts_dir = planning_dir / "artifacts"
            inputs_dir = planning_dir / "inputs"

            for d in [tasks_dir, artifacts_dir, inputs_dir]:
                d.mkdir(parents=True)

            # Create task file
            task = {
                "id": "T001",
                "name": "Test task",
                "phase": 2,
                "context": {"domain": "Test", "capability": "Testing"},
                "behaviors": [],
                "files": [],
                "dependencies": {"tasks": []},
                "acceptance_criteria": [],
            }
            (tasks_dir / "T001.json").write_text(json.dumps(task))

            # Create state file
            state = {
                "tasks": {
                    "T001": {
                        "status": "pending",
                        "phase": 2,
                        "blocks": ["T002"],
                    }
                }
            }
            (planning_dir / "state.json").write_text(json.dumps(state))

            # Create minimal artifacts
            (artifacts_dir / "capability-map.json").write_text(json.dumps({"domains": []}))
            (artifacts_dir / "physical-map.json").write_text(json.dumps({}))
            (inputs_dir / "spec.md").write_text("# Spec\n\nTest content.")

            # Patch module paths
            with patch.object(transform, "PLANNING_DIR", planning_dir), \
                 patch.object(transform, "TASKS_DIR", tasks_dir), \
                 patch.object(transform, "ARTIFACTS_DIR", artifacts_dir), \
                 patch.object(transform, "INPUTS_DIR", inputs_dir):

                result = transform.prepare_task_context("T001")

            assert result is not None
            assert result["task_id"] == "T001"
            assert result["task"]["name"] == "Test task"
            assert result["state"]["status"] == "pending"
            assert result["state"]["blocks"] == ["T002"]
            assert result["suggested_priority"] == "high"
            assert "domain:test" in result["suggested_labels"]

    def test_returns_none_for_missing_task(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tasks_dir = Path(tmpdir) / "tasks"
            tasks_dir.mkdir()

            with patch.object(transform, "TASKS_DIR", tasks_dir):
                result = transform.prepare_task_context("T999")

            assert result is None


class TestGetAllTaskIds:
    """Tests for get_all_task_ids function."""

    def test_returns_sorted_task_ids(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tasks_dir = Path(tmpdir)

            # Create task files in random order
            for tid in ["T003", "T001", "T010", "T002"]:
                (tasks_dir / f"{tid}.json").write_text("{}")

            with patch.object(transform, "TASKS_DIR", tasks_dir):
                result = transform.get_all_task_ids()

            assert result == ["T001", "T002", "T003", "T010"]

    def test_returns_empty_for_missing_directory(self):
        with patch.object(transform, "TASKS_DIR", Path("/nonexistent")):
            result = transform.get_all_task_ids()
        assert result == []

    def test_ignores_non_task_files(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tasks_dir = Path(tmpdir)

            (tasks_dir / "T001.json").write_text("{}")
            (tasks_dir / "other.json").write_text("{}")
            (tasks_dir / "T002.txt").write_text("")

            with patch.object(transform, "TASKS_DIR", tasks_dir):
                result = transform.get_all_task_ids()

            assert result == ["T001"]


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
