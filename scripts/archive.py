#!/usr/bin/env python3
"""
Archive Manager for Task Decomposition Protocol v2

Archives planning and execution artifacts after workflow completion.
Preserves full context for post-hoc analysis and auditing.

Usage:
    archive.py planning <project_name>      Archive planning artifacts
    archive.py execution <project_name>     Archive execution artifacts
    archive.py list [--project <name>]      List archived sessions
    archive.py restore <archive_id>         Restore archived session (planning only)

Archive Structure:
    archive/
    └── {project_name}/
        ├── planning/
        │   └── {timestamp}/
        │       ├── inputs/
        │       ├── artifacts/
        │       ├── tasks/
        │       ├── reports/
        │       └── archive-manifest.json
        └── execution/
            └── {timestamp}/
                ├── bundles/
                ├── results/
                ├── logs/
                └── archive-manifest.json
"""

import json
import shutil
import sys
from datetime import datetime, timezone
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
TASKER_DIR = PROJECT_ROOT / ".tasker"
ARCHIVE_DIR = PROJECT_ROOT / "archive"


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def timestamp_id() -> str:
    """Generate timestamp-based archive ID."""
    return datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")


def archive_planning(project_name: str) -> Path:
    """Archive planning artifacts after workflow completion."""
    if not TASKER_DIR.exists():
        print(f"Error: Planning directory not found: {TASKER_DIR}", file=sys.stderr)
        sys.exit(1)

    state_file = TASKER_DIR / "state.json"
    if not state_file.exists():
        print("Error: No state.json found - nothing to archive", file=sys.stderr)
        sys.exit(1)

    state = json.loads(state_file.read_text())
    current_phase = state.get("phase", {}).get("current", "unknown")

    # Create archive directory
    archive_id = timestamp_id()
    archive_path = ARCHIVE_DIR / project_name / "planning" / archive_id
    archive_path.mkdir(parents=True, exist_ok=True)

    # Directories to archive
    dirs_to_archive = ["inputs", "artifacts", "tasks", "reports"]

    for dir_name in dirs_to_archive:
        src_dir = TASKER_DIR / dir_name
        if src_dir.exists() and any(src_dir.iterdir()):
            dst_dir = archive_path / dir_name
            shutil.copytree(src_dir, dst_dir)
            print(f"  Archived: {dir_name}/")

    # Copy state.json
    shutil.copy2(state_file, archive_path / "state.json")
    print("  Archived: state.json")

    # Create manifest
    manifest = {
        "version": "1.0",
        "archive_type": "planning",
        "project_name": project_name,
        "archive_id": archive_id,
        "archived_at": now_iso(),
        "source_dir": str(TASKER_DIR),
        "phase_at_archive": current_phase,
        "contents": {
            "inputs": list((archive_path / "inputs").glob("*")) if (archive_path / "inputs").exists() else [],
            "artifacts": list((archive_path / "artifacts").glob("*")) if (archive_path / "artifacts").exists() else [],
            "tasks": list((archive_path / "tasks").glob("*.json")) if (archive_path / "tasks").exists() else [],
            "reports": list((archive_path / "reports").glob("*")) if (archive_path / "reports").exists() else [],
        },
        "task_summary": {
            "total": len(state.get("tasks", {})),
            "by_status": _count_by_status(state.get("tasks", {})),
        },
    }

    # Convert Path objects to strings for JSON serialization
    for key in manifest["contents"]:
        manifest["contents"][key] = [str(p.name) for p in manifest["contents"][key]]

    manifest_path = archive_path / "archive-manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2))
    print("  Created: archive-manifest.json")

    print(f"\nArchive created: {archive_path}")
    return archive_path


def archive_execution(project_name: str) -> Path:
    """Archive execution artifacts after workflow completion."""
    if not TASKER_DIR.exists():
        print(f"Error: Planning directory not found: {TASKER_DIR}", file=sys.stderr)
        sys.exit(1)

    state_file = TASKER_DIR / "state.json"
    if not state_file.exists():
        print("Error: No state.json found - nothing to archive", file=sys.stderr)
        sys.exit(1)

    state = json.loads(state_file.read_text())

    # Check for execution artifacts
    bundles_dir = TASKER_DIR / "bundles"
    logs_dir = PROJECT_ROOT / ".claude" / "logs"

    has_bundles = bundles_dir.exists() and any(bundles_dir.glob("*-result.json"))
    has_logs = logs_dir.exists() and any(logs_dir.glob("*.log"))

    if not has_bundles and not has_logs:
        print("No execution artifacts to archive")
        return None

    # Create archive directory
    archive_id = timestamp_id()
    archive_path = ARCHIVE_DIR / project_name / "execution" / archive_id
    archive_path.mkdir(parents=True, exist_ok=True)

    archived_items = []

    # Archive bundles and results
    if has_bundles:
        dst_bundles = archive_path / "bundles"
        shutil.copytree(bundles_dir, dst_bundles)
        archived_items.append("bundles/")
        print("  Archived: bundles/")

    # Archive logs
    if has_logs:
        dst_logs = archive_path / "logs"
        dst_logs.mkdir(exist_ok=True)
        for log_file in logs_dir.glob("*.log"):
            shutil.copy2(log_file, dst_logs / log_file.name)
        archived_items.append("logs/")
        print("  Archived: logs/")

    # Copy state.json snapshot
    shutil.copy2(state_file, archive_path / "state.json")
    print("  Archived: state.json")

    # Create manifest
    results = list(bundles_dir.glob("*-result.json")) if has_bundles else []
    task_results = {}
    for result_file in results:
        result_data = json.loads(result_file.read_text())
        task_id = result_data.get("task_id", result_file.stem.replace("-result", ""))
        task_results[task_id] = {
            "status": result_data.get("status"),
            "completed_at": result_data.get("completed_at"),
        }

    manifest = {
        "version": "1.0",
        "archive_type": "execution",
        "project_name": project_name,
        "archive_id": archive_id,
        "archived_at": now_iso(),
        "source_dir": str(TASKER_DIR),
        "target_dir": state.get("target_dir", ""),
        "contents": archived_items,
        "execution_summary": {
            "total_tasks": len(state.get("tasks", {})),
            "by_status": _count_by_status(state.get("tasks", {})),
            "task_results": task_results,
        },
    }

    manifest_path = archive_path / "archive-manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2))
    print("  Created: archive-manifest.json")

    print(f"\nArchive created: {archive_path}")
    return archive_path


def _count_by_status(tasks: dict) -> dict:
    """Count tasks by status."""
    counts = {}
    for task in tasks.values():
        status = task.get("status", "unknown")
        counts[status] = counts.get(status, 0) + 1
    return counts


def list_archives(project_name: str = None) -> None:
    """List all archived sessions."""
    if not ARCHIVE_DIR.exists():
        print("No archives found")
        return

    if project_name:
        projects = [ARCHIVE_DIR / project_name]
    else:
        projects = list(ARCHIVE_DIR.iterdir())

    for project_path in sorted(projects):
        if not project_path.is_dir():
            continue

        print(f"\n{project_path.name}/")

        for archive_type in ["planning", "execution"]:
            type_dir = project_path / archive_type
            if not type_dir.exists():
                continue

            print(f"  {archive_type}/")
            for archive_dir in sorted(type_dir.iterdir(), reverse=True):
                if not archive_dir.is_dir():
                    continue

                manifest_path = archive_dir / "archive-manifest.json"
                if manifest_path.exists():
                    manifest = json.loads(manifest_path.read_text())
                    archived_at = manifest.get("archived_at", "unknown")
                    phase = manifest.get("phase_at_archive", "")
                    summary = manifest.get("task_summary", manifest.get("execution_summary", {}))
                    total = summary.get("total_tasks", summary.get("total", 0))
                    by_status = summary.get("by_status", {})

                    status_str = ", ".join(f"{k}: {v}" for k, v in by_status.items())
                    print(f"    {archive_dir.name}  [{archived_at[:10]}]  tasks: {total} ({status_str})")
                else:
                    print(f"    {archive_dir.name}  (no manifest)")


def restore_planning(archive_id: str, project_name: str = None) -> None:
    """Restore planning artifacts from archive."""
    # Find the archive
    if project_name:
        search_dirs = [ARCHIVE_DIR / project_name / "planning"]
    else:
        search_dirs = list(ARCHIVE_DIR.glob("*/planning"))

    archive_path = None
    for search_dir in search_dirs:
        candidate = search_dir / archive_id
        if candidate.exists():
            archive_path = candidate
            break

    if not archive_path:
        print(f"Error: Archive not found: {archive_id}", file=sys.stderr)
        sys.exit(1)

    # Confirm restore
    print(f"Restoring from: {archive_path}")
    print(f"This will OVERWRITE: {TASKER_DIR}")
    response = input("Continue? (yes/no): ")
    if response.lower() != "yes":
        print("Aborted")
        return

    # Clear existing planning directory
    if TASKER_DIR.exists():
        for item in TASKER_DIR.iterdir():
            if item.is_dir():
                shutil.rmtree(item)
            else:
                item.unlink()

    # Restore directories
    for dir_name in ["inputs", "artifacts", "tasks", "reports"]:
        src_dir = archive_path / dir_name
        if src_dir.exists():
            dst_dir = TASKER_DIR / dir_name
            shutil.copytree(src_dir, dst_dir)
            print(f"  Restored: {dir_name}/")

    # Restore state.json
    state_src = archive_path / "state.json"
    if state_src.exists():
        shutil.copy2(state_src, TASKER_DIR / "state.json")
        print("  Restored: state.json")

    print(f"\nRestored planning artifacts from archive: {archive_id}")


def clean_planning_dir() -> None:
    """Clean planning directory after successful archive."""
    if not TASKER_DIR.exists():
        return

    # Keep the directory structure but clear contents
    for subdir in ["inputs", "artifacts", "tasks", "reports", "bundles"]:
        dir_path = TASKER_DIR / subdir
        if dir_path.exists():
            for item in dir_path.iterdir():
                if item.is_file():
                    item.unlink()

    # Remove state.json
    state_file = TASKER_DIR / "state.json"
    if state_file.exists():
        state_file.unlink()

    print("Cleaned planning directory")


def main() -> None:
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "planning":
        if len(sys.argv) < 3:
            print("Usage: archive.py planning <project_name>")
            sys.exit(1)
        project_name = sys.argv[2]
        archive_planning(project_name)

        # Ask about cleanup
        if "--clean" in sys.argv:
            clean_planning_dir()

    elif cmd == "execution":
        if len(sys.argv) < 3:
            print("Usage: archive.py execution <project_name>")
            sys.exit(1)
        project_name = sys.argv[2]
        archive_execution(project_name)

    elif cmd == "list":
        project_name = None
        if "--project" in sys.argv:
            idx = sys.argv.index("--project")
            if idx + 1 < len(sys.argv):
                project_name = sys.argv[idx + 1]
        list_archives(project_name)

    elif cmd == "restore":
        if len(sys.argv) < 3:
            print("Usage: archive.py restore <archive_id> [--project <name>]")
            sys.exit(1)
        archive_id = sys.argv[2]
        project_name = None
        if "--project" in sys.argv:
            idx = sys.argv.index("--project")
            if idx + 1 < len(sys.argv):
                project_name = sys.argv[idx + 1]
        restore_planning(archive_id, project_name)

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
