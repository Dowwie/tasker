"""Tests for bundle.py - execution bundle generator."""

import json
import sys
from pathlib import Path

import pytest

# Add scripts to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))

from bundle import (
    clean_bundles,
    find_behavior_by_id,
    find_dependencies_files,
    find_files_for_behavior,
    generate_bundle,
    list_bundles,
    load_json,
    parse_constraints,
    validate_bundle,
    validate_bundle_checksums,
    validate_bundle_dependencies,
    validate_verification_commands,
)


@pytest.fixture
def temp_planning_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Create temporary planning directory structure."""
    planning_dir = tmp_path / ".tasker"
    planning_dir.mkdir()
    (planning_dir / "tasks").mkdir()
    (planning_dir / "artifacts").mkdir()
    (planning_dir / "inputs").mkdir()
    (planning_dir / "bundles").mkdir()

    # Patch module constants
    import bundle

    monkeypatch.setattr(bundle, "PLANNING_DIR", planning_dir)
    monkeypatch.setattr(bundle, "BUNDLES_DIR", planning_dir / "bundles")
    monkeypatch.setattr(bundle, "TASKS_DIR", planning_dir / "tasks")
    monkeypatch.setattr(bundle, "ARTIFACTS_DIR", planning_dir / "artifacts")
    monkeypatch.setattr(bundle, "INPUTS_DIR", planning_dir / "inputs")
    monkeypatch.setattr(bundle, "SCHEMAS_DIR", tmp_path / "schemas")

    # Create schemas dir with bundle schema
    schemas_dir = tmp_path / "schemas"
    schemas_dir.mkdir()
    schema = {
        "$schema": "http://json-schema.org/draft-07/schema#",
        "required": ["version", "task_id", "name", "target_dir", "behaviors", "files", "acceptance_criteria"],
    }
    (schemas_dir / "execution-bundle.schema.json").write_text(json.dumps(schema))

    return planning_dir


@pytest.fixture
def sample_capability_map() -> dict:
    """Sample capability map for testing."""
    return {
        "version": "1.0",
        "domains": [
            {
                "id": "D001",
                "name": "Authentication",
                "capabilities": [
                    {
                        "id": "C001",
                        "name": "User Login",
                        "spec_ref": "REQ-001",
                        "behaviors": [
                            {
                                "id": "B001",
                                "name": "validate_credentials",
                                "type": "process",
                                "description": "Validate user email and password",
                            },
                            {
                                "id": "B002",
                                "name": "CredentialError",
                                "type": "output",
                                "description": "Error for invalid credentials",
                            },
                        ],
                    }
                ],
            },
            {
                "id": "D002",
                "name": "Data",
                "capabilities": [
                    {
                        "id": "C002",
                        "name": "User Storage",
                        "behaviors": [
                            {
                                "id": "B003",
                                "name": "UserRepository",
                                "type": "state",
                                "description": "User data access layer",
                            }
                        ],
                    }
                ],
            },
        ],
        "flows": [],
    }


@pytest.fixture
def sample_physical_map() -> dict:
    """Sample physical map for testing."""
    return {
        "version": "1.0",
        "target_dir": "/tmp/target",
        "file_mapping": [
            {
                "behavior_id": "B001",
                "behavior_name": "validate_credentials",
                "files": [
                    {
                        "path": "src/auth/validator.py",
                        "action": "create",
                        "layer": "domain",
                        "purpose": "Credential validation logic",
                    }
                ],
                "tests": [
                    {
                        "path": "tests/auth/test_validator.py",
                        "action": "create",
                    }
                ],
            },
            {
                "behavior_id": "B002",
                "behavior_name": "CredentialError",
                "files": [
                    {
                        "path": "src/auth/errors.py",
                        "action": "create",
                        "layer": "domain",
                        "purpose": "Authentication error types",
                    }
                ],
                "tests": [],
            },
        ],
    }


@pytest.fixture
def sample_task() -> dict:
    """Sample task definition for testing."""
    return {
        "id": "T001",
        "name": "Implement credential validation",
        "phase": 1,
        "context": {
            "steel_thread": True,
        },
        "behaviors": ["B001", "B002"],
        "files": [
            {
                "path": "src/auth/validator.py",
                "action": "create",
                "purpose": "Main validation module",
            }
        ],
        "dependencies": {
            "tasks": [],
            "external": ["pydantic"],
        },
        "acceptance_criteria": [
            {
                "criterion": "Valid credentials return True",
                "verification": "pytest tests/auth/test_validator.py::test_valid",
            },
            {
                "criterion": "Invalid email raises ValidationError",
                "verification": "pytest tests/auth/test_validator.py::test_invalid_email",
            },
        ],
    }


@pytest.fixture
def sample_state() -> dict:
    """Sample state for testing."""
    return {
        "version": "2.0",
        "phase": {"current": "ready", "completed": ["ingestion", "logical", "physical"]},
        "target_dir": "/tmp/target-project",
        "tasks": {
            "T001": {
                "id": "T001",
                "name": "Implement credential validation",
                "status": "pending",
                "phase": 1,
                "depends_on": [],
            },
            "T002": {
                "id": "T002",
                "name": "Implement user repository",
                "status": "complete",
                "phase": 1,
                "depends_on": [],
                "files_created": ["src/data/user_repo.py", "tests/data/test_user_repo.py"],
            },
        },
        "execution": {},
    }


class TestLoadJson:
    """Tests for load_json function."""

    def test_load_existing_file(self, tmp_path: Path) -> None:
        """Test loading an existing JSON file."""
        data = {"key": "value", "number": 42}
        file_path = tmp_path / "test.json"
        file_path.write_text(json.dumps(data))

        result = load_json(file_path)

        assert result == data

    def test_load_nonexistent_file(self, tmp_path: Path) -> None:
        """Test loading a file that doesn't exist."""
        result = load_json(tmp_path / "nonexistent.json")

        assert result is None


class TestFindBehaviorById:
    """Tests for find_behavior_by_id function."""

    def test_find_existing_behavior(self, sample_capability_map: dict) -> None:
        """Test finding a behavior that exists."""
        result = find_behavior_by_id(sample_capability_map, "B001")

        assert result is not None
        assert result["id"] == "B001"
        assert result["name"] == "validate_credentials"
        assert result["type"] == "process"
        assert result["domain"] == "Authentication"
        assert result["capability"] == "User Login"
        assert result["capability_id"] == "C001"
        assert result["spec_ref"] == "REQ-001"

    def test_find_behavior_in_different_domain(self, sample_capability_map: dict) -> None:
        """Test finding a behavior in a different domain."""
        result = find_behavior_by_id(sample_capability_map, "B003")

        assert result is not None
        assert result["id"] == "B003"
        assert result["domain"] == "Data"
        assert result["capability"] == "User Storage"

    def test_find_nonexistent_behavior(self, sample_capability_map: dict) -> None:
        """Test finding a behavior that doesn't exist."""
        result = find_behavior_by_id(sample_capability_map, "B999")

        assert result is None

    def test_find_behavior_empty_map(self) -> None:
        """Test finding behavior in empty capability map."""
        result = find_behavior_by_id({"domains": []}, "B001")

        assert result is None


class TestFindFilesForBehavior:
    """Tests for find_files_for_behavior function."""

    def test_find_files_with_tests(self, sample_physical_map: dict) -> None:
        """Test finding files including test files."""
        result = find_files_for_behavior(sample_physical_map, "B001")

        assert len(result) == 2
        paths = [f["path"] for f in result]
        assert "src/auth/validator.py" in paths
        assert "tests/auth/test_validator.py" in paths

    def test_find_files_without_tests(self, sample_physical_map: dict) -> None:
        """Test finding files when no tests defined."""
        result = find_files_for_behavior(sample_physical_map, "B002")

        assert len(result) == 1
        assert result[0]["path"] == "src/auth/errors.py"

    def test_find_files_nonexistent_behavior(self, sample_physical_map: dict) -> None:
        """Test finding files for behavior not in physical map."""
        result = find_files_for_behavior(sample_physical_map, "B999")

        assert result == []

    def test_files_have_behavior_reference(self, sample_physical_map: dict) -> None:
        """Test that returned files include behavior ID reference."""
        result = find_files_for_behavior(sample_physical_map, "B001")

        for file_info in result:
            assert "behaviors" in file_info
            assert "B001" in file_info["behaviors"]


class TestFindDependenciesFiles:
    """Tests for find_dependencies_files function."""

    def test_find_files_from_completed_task(self, sample_state: dict) -> None:
        """Test finding files from a completed dependency task."""
        result = find_dependencies_files(sample_state, ["T002"])

        assert len(result) == 2
        assert "src/data/user_repo.py" in result
        assert "tests/data/test_user_repo.py" in result

    def test_find_files_from_pending_task(self, sample_state: dict) -> None:
        """Test that pending tasks have no files."""
        result = find_dependencies_files(sample_state, ["T001"])

        assert result == []

    def test_find_files_nonexistent_task(self, sample_state: dict) -> None:
        """Test finding files from nonexistent task."""
        result = find_dependencies_files(sample_state, ["T999"])

        assert result == []

    def test_find_files_multiple_tasks(self, sample_state: dict) -> None:
        """Test finding files from multiple tasks."""
        # Add another completed task
        sample_state["tasks"]["T003"] = {
            "id": "T003",
            "status": "complete",
            "files_created": ["src/core/config.py"],
        }

        result = find_dependencies_files(sample_state, ["T002", "T003"])

        assert len(result) == 3
        assert "src/data/user_repo.py" in result
        assert "src/core/config.py" in result


class TestParseConstraints:
    """Tests for parse_constraints function."""

    def test_parse_python_constraints(self) -> None:
        """Test parsing Python-specific constraints."""
        raw = """
        # Tech Stack
        - Python 3.12
        - FastAPI framework
        - pytest for testing
        - Use Protocol for interfaces
        - Use dataclass for DTOs
        """

        result = parse_constraints(raw)

        assert result["language"] == "Python"
        assert result["framework"] == "FastAPI"
        assert result["testing"] == "pytest"
        assert "Use Protocol for interfaces" in result["patterns"]
        assert "Use dataclass for data structures" in result["patterns"]
        assert result["raw"] == raw

    def test_parse_typescript_constraints(self) -> None:
        """Test parsing TypeScript constraints."""
        raw = "Use TypeScript with strict mode"

        result = parse_constraints(raw)

        assert result["language"] == "TypeScript"

    def test_parse_empty_constraints(self) -> None:
        """Test parsing None constraints."""
        result = parse_constraints(None)

        assert result == {}

    def test_parse_django_framework(self) -> None:
        """Test parsing Django framework."""
        raw = "Build with Django REST framework"

        result = parse_constraints(raw)

        assert result["framework"] == "Django"

    def test_parse_factory_pattern(self) -> None:
        """Test parsing factory pattern constraint."""
        raw = "Use factory functions for object creation"

        result = parse_constraints(raw)

        assert "Use factory functions for construction" in result["patterns"]


class TestGenerateBundle:
    """Tests for generate_bundle function."""

    def test_generate_bundle_success(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test successful bundle generation."""
        # Write required files
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        success, msg, bundle = generate_bundle("T001")

        assert success is True
        assert "Bundle generated" in msg
        assert bundle is not None

        # Verify bundle contents
        assert bundle["version"] == "1.2"
        assert bundle["task_id"] == "T001"
        assert bundle["name"] == "Implement credential validation"
        assert bundle["phase"] == 1
        assert bundle["target_dir"] == "/tmp/target-project"

        # Verify behaviors were expanded
        assert len(bundle["behaviors"]) == 2
        behavior_ids = [b["id"] for b in bundle["behaviors"]]
        assert "B001" in behavior_ids
        assert "B002" in behavior_ids

        # Verify acceptance criteria preserved
        assert len(bundle["acceptance_criteria"]) == 2

    def test_generate_bundle_with_constraints(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test bundle generation with constraints file."""
        # Write required files
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))
        (temp_planning_dir / "inputs" / "constraints.md").write_text(
            "Python 3.12, FastAPI, pytest, Protocol"
        )

        success, _, bundle = generate_bundle("T001")

        assert success is True
        assert bundle["constraints"]["language"] == "Python"
        assert bundle["constraints"]["framework"] == "FastAPI"

    def test_generate_bundle_task_not_found(self, temp_planning_dir: Path) -> None:
        """Test bundle generation when task doesn't exist."""
        success, msg, bundle = generate_bundle("T999")

        assert success is False
        assert "not found" in msg.lower()
        assert bundle is None

    def test_generate_bundle_missing_capability_map(
        self, temp_planning_dir: Path, sample_task: dict
    ) -> None:
        """Test bundle generation when capability map missing."""
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))

        success, msg, bundle = generate_bundle("T001")

        assert success is False
        assert "capability-map" in msg.lower()
        assert bundle is None

    def test_generate_bundle_writes_file(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test that bundle is written to bundles directory."""
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        bundle_path = temp_planning_dir / "bundles" / "T001-bundle.json"
        assert bundle_path.exists()

        written_bundle = json.loads(bundle_path.read_text())
        assert written_bundle["task_id"] == "T001"


class TestValidateBundle:
    """Tests for validate_bundle function."""

    def test_validate_valid_bundle(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test validating a valid bundle."""
        # Generate a bundle first
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))
        generate_bundle("T001")

        valid, msg = validate_bundle("T001")

        assert valid is True
        assert "valid" in msg.lower()

    def test_validate_nonexistent_bundle(self, temp_planning_dir: Path) -> None:
        """Test validating bundle that doesn't exist."""
        valid, msg = validate_bundle("T999")

        assert valid is False
        assert "not found" in msg.lower()

    def test_validate_bundle_missing_field(self, temp_planning_dir: Path) -> None:
        """Test validating bundle with missing required field."""
        # Write invalid bundle
        invalid_bundle = {"version": "1.0", "task_id": "T001"}
        (temp_planning_dir / "bundles" / "T001-bundle.json").write_text(
            json.dumps(invalid_bundle)
        )

        valid, msg = validate_bundle("T001")

        assert valid is False
        # Error message may say "missing" or "required" depending on validator
        assert "missing" in msg.lower() or "required" in msg.lower()


class TestListBundles:
    """Tests for list_bundles function."""

    def test_list_bundles_empty(self, temp_planning_dir: Path) -> None:
        """Test listing when no bundles exist."""
        result = list_bundles()

        assert result == []

    def test_list_bundles_multiple(self, temp_planning_dir: Path) -> None:
        """Test listing multiple bundles."""
        (temp_planning_dir / "bundles" / "T001-bundle.json").write_text("{}")
        (temp_planning_dir / "bundles" / "T002-bundle.json").write_text("{}")
        (temp_planning_dir / "bundles" / "T003-bundle.json").write_text("{}")

        result = list_bundles()

        assert len(result) == 3
        assert "T001" in result
        assert "T002" in result
        assert "T003" in result


class TestCleanBundles:
    """Tests for clean_bundles function."""

    def test_clean_bundles_removes_all(self, temp_planning_dir: Path) -> None:
        """Test that clean removes all bundles."""
        (temp_planning_dir / "bundles" / "T001-bundle.json").write_text("{}")
        (temp_planning_dir / "bundles" / "T002-bundle.json").write_text("{}")

        count = clean_bundles()

        assert count == 2
        assert not (temp_planning_dir / "bundles" / "T001-bundle.json").exists()
        assert not (temp_planning_dir / "bundles" / "T002-bundle.json").exists()

    def test_clean_bundles_empty_dir(self, temp_planning_dir: Path) -> None:
        """Test cleaning when no bundles exist."""
        count = clean_bundles()

        assert count == 0


class TestBundleIntegration:
    """Integration tests for complete bundle workflow."""

    def test_full_workflow(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test complete workflow: generate, list, validate, clean."""
        # Setup
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        # Generate
        success, _, bundle = generate_bundle("T001")
        assert success is True

        # List
        bundles = list_bundles()
        assert "T001" in bundles

        # Validate
        valid, _ = validate_bundle("T001")
        assert valid is True

        # Clean
        count = clean_bundles()
        assert count == 1
        assert list_bundles() == []

    def test_bundle_with_dependencies(
        self,
        temp_planning_dir: Path,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test bundle generation with task dependencies."""
        # Task with dependencies
        task_with_deps = {
            "id": "T003",
            "name": "Implement login endpoint",
            "phase": 2,
            "behaviors": ["B001"],
            "files": [{"path": "src/api/login.py", "action": "create"}],
            "dependencies": {"tasks": ["T002"], "external": []},
            "acceptance_criteria": [
                {"criterion": "Login works", "verification": "pytest tests/api/test_login.py"}
            ],
        }

        (temp_planning_dir / "tasks" / "T003.json").write_text(json.dumps(task_with_deps))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        success, _, bundle = generate_bundle("T003")

        assert success is True
        assert bundle["dependencies"]["tasks"] == ["T002"]
        # T002 was completed with files
        assert "src/data/user_repo.py" in bundle["dependencies"]["files"]


class TestValidateBundleDependencies:
    """Tests for validate_bundle_dependencies function."""

    def test_all_dependencies_exist(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        tmp_path: Path,
    ) -> None:
        """Test when all dependency files exist."""
        # Create target directory with dependency files
        target_dir = tmp_path / "target-project"
        target_dir.mkdir()
        (target_dir / "src" / "data").mkdir(parents=True)
        (target_dir / "src" / "data" / "user_repo.py").write_text("# repo")

        # Update sample_state with target_dir
        sample_state = {
            "version": "2.0",
            "target_dir": str(target_dir),
            "tasks": {
                "T001": {"id": "T001", "status": "pending", "phase": 1},
                "T002": {
                    "id": "T002",
                    "status": "complete",
                    "files_created": ["src/data/user_repo.py"],
                },
            },
            "execution": {},
        }

        # Task that depends on T002
        task_with_deps = {
            "id": "T003",
            "name": "Use user repo",
            "phase": 2,
            "behaviors": ["B001"],
            "files": [{"path": "src/api/users.py", "action": "create"}],
            "dependencies": {"tasks": ["T002"], "external": []},
            "acceptance_criteria": [{"criterion": "Works", "verification": "pytest"}],
        }

        (temp_planning_dir / "tasks" / "T003.json").write_text(json.dumps(task_with_deps))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        # Generate bundle
        generate_bundle("T003")

        # Validate dependencies
        valid, missing = validate_bundle_dependencies("T003")

        assert valid is True
        assert len(missing) == 0

    def test_missing_dependencies(
        self,
        temp_planning_dir: Path,
        sample_capability_map: dict,
        sample_physical_map: dict,
        tmp_path: Path,
    ) -> None:
        """Test when dependency files are missing."""
        target_dir = tmp_path / "target-project"
        target_dir.mkdir()
        # NOT creating the dependency file

        sample_state = {
            "version": "2.0",
            "target_dir": str(target_dir),
            "tasks": {
                "T002": {
                    "id": "T002",
                    "status": "complete",
                    "files_created": ["src/data/missing.py"],
                },
            },
            "execution": {},
        }

        task_with_deps = {
            "id": "T003",
            "name": "Use missing file",
            "phase": 2,
            "behaviors": ["B001"],
            "files": [{"path": "src/api/users.py", "action": "create"}],
            "dependencies": {"tasks": ["T002"], "external": []},
            "acceptance_criteria": [{"criterion": "Works", "verification": "pytest"}],
        }

        (temp_planning_dir / "tasks" / "T003.json").write_text(json.dumps(task_with_deps))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T003")

        valid, missing = validate_bundle_dependencies("T003")

        assert valid is False
        assert "src/data/missing.py" in missing

    def test_nonexistent_bundle(self, temp_planning_dir: Path) -> None:
        """Test validation of nonexistent bundle."""
        valid, missing = validate_bundle_dependencies("T999")

        assert valid is False
        assert any("not found" in m.lower() for m in missing)


class TestValidateVerificationCommands:
    """Tests for validate_verification_commands function."""

    def test_valid_commands(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test bundle with valid verification commands."""
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        valid, invalid = validate_verification_commands("T001")

        assert valid is True
        assert len(invalid) == 0

    def test_empty_verification_command(
        self,
        temp_planning_dir: Path,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test detection of empty verification command."""
        task_empty_cmd = {
            "id": "T001",
            "name": "Task with empty command",
            "phase": 1,
            "behaviors": ["B001"],
            "files": [{"path": "src/file.py", "action": "create"}],
            "dependencies": {"tasks": []},
            "acceptance_criteria": [
                {"criterion": "Something works", "verification": ""},
            ],
        }

        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(task_empty_cmd))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        valid, invalid = validate_verification_commands("T001")

        assert valid is False
        assert any("empty" in i.lower() for i in invalid)

    def test_invalid_command_syntax(
        self,
        temp_planning_dir: Path,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test detection of syntactically invalid command."""
        task_bad_cmd = {
            "id": "T001",
            "name": "Task with bad command",
            "phase": 1,
            "behaviors": ["B001"],
            "files": [{"path": "src/file.py", "action": "create"}],
            "dependencies": {"tasks": []},
            "acceptance_criteria": [
                {"criterion": "Something", "verification": "echo 'unclosed quote"},
            ],
        }

        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(task_bad_cmd))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        valid, invalid = validate_verification_commands("T001")

        assert valid is False
        assert len(invalid) > 0

    def test_complex_valid_command(
        self,
        temp_planning_dir: Path,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test complex but valid verification command."""
        task_complex = {
            "id": "T001",
            "name": "Task with complex command",
            "phase": 1,
            "behaviors": ["B001"],
            "files": [{"path": "src/file.py", "action": "create"}],
            "dependencies": {"tasks": []},
            "acceptance_criteria": [
                {
                    "criterion": "All tests pass",
                    "verification": "pytest tests/ -v --tb=short -x --cov=src --cov-report=term-missing",
                },
            ],
        }

        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(task_complex))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        valid, invalid = validate_verification_commands("T001")

        assert valid is True
        assert len(invalid) == 0

    def test_nonexistent_bundle(self, temp_planning_dir: Path) -> None:
        """Test validation of nonexistent bundle."""
        valid, invalid = validate_verification_commands("T999")

        assert valid is False
        assert any("not found" in i.lower() for i in invalid)


class TestValidateBundleChecksums:
    """Tests for validate_bundle_checksums function."""

    def test_checksums_valid(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test when all checksums match."""
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        valid, changed = validate_bundle_checksums("T001")

        assert valid is True
        assert len(changed) == 0

    def test_checksums_artifact_changed(
        self,
        temp_planning_dir: Path,
        sample_task: dict,
        sample_capability_map: dict,
        sample_physical_map: dict,
        sample_state: dict,
    ) -> None:
        """Test when artifact changes after bundle generation."""
        (temp_planning_dir / "tasks" / "T001.json").write_text(json.dumps(sample_task))
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )
        (temp_planning_dir / "artifacts" / "physical-map.json").write_text(
            json.dumps(sample_physical_map)
        )
        (temp_planning_dir / "state.json").write_text(json.dumps(sample_state))

        generate_bundle("T001")

        # Modify capability map after bundle generation
        sample_capability_map["version"] = "2.0"
        (temp_planning_dir / "artifacts" / "capability-map.json").write_text(
            json.dumps(sample_capability_map)
        )

        valid, changed = validate_bundle_checksums("T001")

        assert valid is False
        assert any("capability_map" in c for c in changed)

    def test_checksums_old_bundle_format(self, temp_planning_dir: Path) -> None:
        """Test validation of bundle without checksums (old format)."""
        # Create old-format bundle without checksums
        old_bundle = {
            "version": "1.0",
            "task_id": "T001",
            "name": "Old bundle",
            "phase": 1,
            "target_dir": "/tmp/target",
            "behaviors": [],
            "files": [],
            "dependencies": {"tasks": [], "files": [], "external": []},
            "acceptance_criteria": [],
            "constraints": {},
            # No checksums field
        }
        (temp_planning_dir / "bundles" / "T001-bundle.json").write_text(
            json.dumps(old_bundle)
        )

        valid, changed = validate_bundle_checksums("T001")

        # Should pass since no checksums to validate
        assert valid is True
        assert len(changed) == 0

    def test_checksums_nonexistent_bundle(self, temp_planning_dir: Path) -> None:
        """Test checksum validation of nonexistent bundle."""
        valid, changed = validate_bundle_checksums("T999")

        assert valid is False
        assert any("not found" in c.lower() for c in changed)
