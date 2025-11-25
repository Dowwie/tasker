#!/usr/bin/env python3
"""
Install /work command in target project.

This script generates a customized /work command for the target project
that references the tasker planning directory.

Usage:
    install_work_command.py <target_dir>
    install_work_command.py <target_dir> --dry-run
"""

import json
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
TEMPLATE_PATH = PROJECT_ROOT / "templates" / "commands" / "work.md"


def install_work_command(target_dir: str, dry_run: bool = False) -> tuple[bool, str]:
    """
    Install /work command in target project.

    Args:
        target_dir: Path to target project
        dry_run: If True, only show what would be done

    Returns:
        (success, message)
    """
    target_path = Path(target_dir).resolve()

    if not target_path.exists():
        return False, f"Target directory does not exist: {target_path}"

    if not TEMPLATE_PATH.exists():
        return False, f"Template not found: {TEMPLATE_PATH}"

    # Read template
    template = TEMPLATE_PATH.read_text()

    # Replace placeholders
    content = template.replace("{{PLANNING_DIR}}", str(PROJECT_ROOT))
    content = content.replace("{{TARGET_DIR}}", str(target_path))

    # Determine output path
    commands_dir = target_path / ".claude" / "commands"
    work_path = commands_dir / "work.md"

    if dry_run:
        print("DRY RUN - Would perform the following actions:")
        print(f"  Create directory: {commands_dir}")
        print(f"  Write file: {work_path}")
        print(f"  Planning directory: {PROJECT_ROOT}")
        print(f"  Target directory: {target_path}")
        print("\nGenerated content preview (first 500 chars):")
        print("-" * 40)
        print(content[:500])
        print("-" * 40)
        return True, "Dry run completed"

    # Create directory and write file
    commands_dir.mkdir(parents=True, exist_ok=True)
    work_path.write_text(content)

    # Also update/create settings.local.json with permissions
    settings_path = target_path / ".claude" / "settings.local.json"
    settings = {}

    if settings_path.exists():
        try:
            settings = json.loads(settings_path.read_text())
        except json.JSONDecodeError:
            pass

    # Ensure permissions section exists
    if "permissions" not in settings:
        settings["permissions"] = {}
    if "allow" not in settings["permissions"]:
        settings["permissions"]["allow"] = []

    # Add required permissions if not present
    planning_read = f"Read({PROJECT_ROOT}/**)"
    planning_edit = f"Edit({PROJECT_ROOT}/project-planning/**)"
    bash_state = f"Bash(python3 {PROJECT_ROOT}/scripts/state.py:*)"
    bash_bundle = f"Bash(python3 {PROJECT_ROOT}/scripts/bundle.py:*)"

    for perm in [planning_read, planning_edit, bash_state, bash_bundle]:
        if perm not in settings["permissions"]["allow"]:
            settings["permissions"]["allow"].append(perm)

    settings_path.write_text(json.dumps(settings, indent=2))

    return True, f"""
Work command installed successfully!

Files created/updated:
  - {work_path}
  - {settings_path}

Permissions added:
  - Read({PROJECT_ROOT}/**)
  - Edit({PROJECT_ROOT}/project-planning/**)
  - Bash(python3 {PROJECT_ROOT}/scripts/state.py:*)
  - Bash(python3 {PROJECT_ROOT}/scripts/bundle.py:*)

Usage:
  cd {target_path}
  claude
  > /work
"""


def main() -> None:
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    target_dir = sys.argv[1]
    dry_run = "--dry-run" in sys.argv

    success, msg = install_work_command(target_dir, dry_run)
    print(msg)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
