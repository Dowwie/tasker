#!/usr/bin/env python3
"""
Bundle Generator for Task Decomposition Protocol v2

Generates self-contained execution bundles that include everything
a task-executor needs to implement a task.

Usage:
    bundle.py generate <task_id>     Generate bundle for single task
    bundle.py generate-ready         Generate bundles for all ready tasks
    bundle.py validate <task_id>     Validate existing bundle
    bundle.py list                   List existing bundles
    bundle.py clean                  Remove all bundles
"""

import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
PLANNING_DIR = PROJECT_ROOT / "project-planning"
BUNDLES_DIR = PLANNING_DIR / "bundles"
TASKS_DIR = PLANNING_DIR / "tasks"
ARTIFACTS_DIR = PLANNING_DIR / "artifacts"
INPUTS_DIR = PLANNING_DIR / "inputs"
SCHEMAS_DIR = PROJECT_ROOT / "schemas"


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def load_json(path: Path) -> dict | None:
    """Load JSON file, return None if not found."""
    if not path.exists():
        return None
    return json.loads(path.read_text())


def load_task(task_id: str) -> dict | None:
    """Load task definition from tasks directory."""
    task_path = TASKS_DIR / f"{task_id}.json"
    return load_json(task_path)


def load_capability_map() -> dict | None:
    """Load capability map artifact."""
    return load_json(ARTIFACTS_DIR / "capability-map.json")


def load_physical_map() -> dict | None:
    """Load physical map artifact."""
    return load_json(ARTIFACTS_DIR / "physical-map.json")


def load_state() -> dict | None:
    """Load current state."""
    return load_json(PLANNING_DIR / "state.json")


def load_constraints() -> str | None:
    """Load constraints.md if exists."""
    constraints_path = INPUTS_DIR / "constraints.md"
    if constraints_path.exists():
        return constraints_path.read_text()
    return None


def find_atom_by_id(capability_map: dict, atom_id: str) -> dict | None:
    """Find atom details from capability map by ID."""
    for domain in capability_map.get("domains", []):
        for capability in domain.get("capabilities", []):
            for atom in capability.get("atoms", []):
                if atom.get("id") == atom_id:
                    return {
                        **atom,
                        "domain": domain.get("name"),
                        "domain_id": domain.get("id"),
                        "capability": capability.get("name"),
                        "capability_id": capability.get("id"),
                        "spec_ref": capability.get("spec_ref"),
                    }
    return None


def find_files_for_atom(physical_map: dict, atom_id: str) -> list[dict]:
    """Find file mappings for an atom from physical map."""
    files = []
    for mapping in physical_map.get("file_mapping", []):
        if mapping.get("atom_id") == atom_id:
            for file_info in mapping.get("files", []):
                files.append({
                    **file_info,
                    "atoms": [atom_id],
                })
            for test_info in mapping.get("tests", []):
                files.append({
                    **test_info,
                    "layer": "test",
                    "atoms": [atom_id],
                })
    return files


def find_dependencies_files(state: dict, task_deps: list[str]) -> list[str]:
    """Get files created by dependency tasks."""
    files = []
    for dep_id in task_deps:
        dep_task = state.get("tasks", {}).get(dep_id, {})
        files.extend(dep_task.get("files_created", []))
    return files


def parse_constraints(raw_constraints: str | None) -> dict:
    """Parse constraints.md into structured format."""
    if not raw_constraints:
        return {}

    result: dict[str, Any] = {"raw": raw_constraints}
    lines = raw_constraints.lower()

    if "python" in lines:
        result["language"] = "Python"
    elif "typescript" in lines:
        result["language"] = "TypeScript"
    elif "rust" in lines:
        result["language"] = "Rust"

    if "fastapi" in lines:
        result["framework"] = "FastAPI"
    elif "django" in lines:
        result["framework"] = "Django"
    elif "flask" in lines:
        result["framework"] = "Flask"

    if "pytest" in lines:
        result["testing"] = "pytest"

    patterns = []
    if "protocol" in lines:
        patterns.append("Use Protocol for interfaces")
    if "dataclass" in lines:
        patterns.append("Use dataclass for data structures")
    if "factory" in lines:
        patterns.append("Use factory functions for construction")
    if patterns:
        result["patterns"] = patterns

    return result


def generate_bundle(task_id: str) -> tuple[bool, str, dict | None]:
    """
    Generate execution bundle for a task.

    Returns: (success, message, bundle_dict)
    """
    task = load_task(task_id)
    if not task:
        return False, f"Task not found: {task_id}", None

    capability_map = load_capability_map()
    if not capability_map:
        return False, "capability-map.json not found", None

    physical_map = load_physical_map()
    if not physical_map:
        return False, "physical-map.json not found", None

    state = load_state()
    if not state:
        return False, "state.json not found", None

    raw_constraints = load_constraints()

    expanded_atoms = []
    context = task.get("context", {})

    for atom_id in task.get("atoms", []):
        atom_details = find_atom_by_id(capability_map, atom_id)
        if atom_details:
            expanded_atoms.append({
                "id": atom_details["id"],
                "name": atom_details["name"],
                "type": atom_details.get("type", "process"),
                "description": atom_details.get("description", ""),
            })
            if not context.get("domain"):
                context["domain"] = atom_details.get("domain")
            if not context.get("capability"):
                context["capability"] = atom_details.get("capability")
            if not context.get("capability_id"):
                context["capability_id"] = atom_details.get("capability_id")
            if not context.get("spec_ref"):
                context["spec_ref"] = atom_details.get("spec_ref")
        else:
            expanded_atoms.append({
                "id": atom_id,
                "name": f"Unknown atom {atom_id}",
                "type": "process",
                "description": "",
            })

    files = []
    seen_paths: set[str] = set()

    for file_info in task.get("files", []):
        path = file_info.get("path")
        if path and path not in seen_paths:
            files.append(file_info)
            seen_paths.add(path)

    for atom_id in task.get("atoms", []):
        atom_files = find_files_for_atom(physical_map, atom_id)
        for file_info in atom_files:
            path = file_info.get("path")
            if path and path not in seen_paths:
                files.append(file_info)
                seen_paths.add(path)

    task_deps = task.get("dependencies", {}).get("tasks", [])
    dep_files = find_dependencies_files(state, task_deps)

    bundle = {
        "version": "1.0",
        "bundle_created_at": now_iso(),
        "task_id": task_id,
        "name": task.get("name", ""),
        "wave": task.get("wave", 1),
        "target_dir": state.get("target_dir", ""),
        "context": context,
        "atoms": expanded_atoms,
        "files": files,
        "dependencies": {
            "tasks": task_deps,
            "files": dep_files,
            "external": task.get("dependencies", {}).get("external", []),
        },
        "acceptance_criteria": task.get("acceptance_criteria", []),
        "constraints": parse_constraints(raw_constraints),
    }

    BUNDLES_DIR.mkdir(parents=True, exist_ok=True)
    bundle_path = BUNDLES_DIR / f"{task_id}-bundle.json"
    bundle_path.write_text(json.dumps(bundle, indent=2))

    return True, f"Bundle generated: {bundle_path}", bundle


def generate_ready_bundles() -> tuple[int, int]:
    """Generate bundles for all ready tasks. Returns (success_count, fail_count)."""
    state = load_state()
    if not state:
        print("No state file found", file=sys.stderr)
        return 0, 0

    from state import get_ready_tasks
    ready = get_ready_tasks(state)

    success = 0
    fail = 0

    for task_id in ready:
        ok, msg, _ = generate_bundle(task_id)
        print(f"{task_id}: {msg}")
        if ok:
            success += 1
        else:
            fail += 1

    return success, fail


def validate_bundle(task_id: str) -> tuple[bool, str]:
    """Validate existing bundle against schema."""
    bundle_path = BUNDLES_DIR / f"{task_id}-bundle.json"
    if not bundle_path.exists():
        return False, f"Bundle not found: {bundle_path}"

    schema_path = SCHEMAS_DIR / "execution-bundle.schema.json"
    if not schema_path.exists():
        return False, f"Schema not found: {schema_path}"

    bundle = json.loads(bundle_path.read_text())
    schema = json.loads(schema_path.read_text())

    required = schema.get("required", [])
    for field in required:
        if field not in bundle:
            return False, f"Missing required field: {field}"

    return True, "Bundle is valid"


def list_bundles() -> list[str]:
    """List all existing bundles."""
    if not BUNDLES_DIR.exists():
        return []
    return [p.stem.replace("-bundle", "") for p in BUNDLES_DIR.glob("*-bundle.json")]


def clean_bundles() -> int:
    """Remove all bundles. Returns count removed."""
    if not BUNDLES_DIR.exists():
        return 0

    count = 0
    for bundle_file in BUNDLES_DIR.glob("*-bundle.json"):
        bundle_file.unlink()
        count += 1

    return count


def main() -> None:
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "generate":
        if len(sys.argv) < 3:
            print("Usage: bundle.py generate <task_id>")
            sys.exit(1)
        task_id = sys.argv[2]
        success, msg, _ = generate_bundle(task_id)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "generate-ready":
        success, fail = generate_ready_bundles()
        print(f"Generated {success} bundles, {fail} failed")
        sys.exit(0 if fail == 0 else 1)

    elif cmd == "validate":
        if len(sys.argv) < 3:
            print("Usage: bundle.py validate <task_id>")
            sys.exit(1)
        task_id = sys.argv[2]
        valid, msg = validate_bundle(task_id)
        print(msg)
        sys.exit(0 if valid else 1)

    elif cmd == "list":
        bundles = list_bundles()
        if bundles:
            for b in bundles:
                print(b)
        else:
            print("No bundles found")

    elif cmd == "clean":
        count = clean_bundles()
        print(f"Removed {count} bundles")

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
