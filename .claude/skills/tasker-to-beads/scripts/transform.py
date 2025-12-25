#!/usr/bin/env python3
"""
Tasker to Beads Transformer

Prepares task context for LLM enrichment and handles beads issue creation.
This script does the mechanical work; the LLM does the comprehension work.

Usage:
    transform.py context <task_id> [-t TARGET_DIR]
                                              Prepare context for single task
    transform.py context --all [-t TARGET_DIR]
                                              Prepare context for all tasks
    transform.py create <task_id> <description_file> [-t TARGET_DIR]
                                              Create beads issue from enriched description
    transform.py batch-create <manifest_file> [-t TARGET_DIR]
                                              Create multiple issues from manifest
    transform.py status [-t TARGET_DIR]       Show transformation status
    transform.py init-target <target_dir>     Initialize beads in target directory

Options:
    -t, --target-dir DIR    Target directory for beads management (where issues
                            will be created). If not specified, uses current
                            project. Beads will be initialized if needed.
"""

import json
import os
import re
import subprocess
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
SKILL_DIR = SCRIPT_DIR.parent  # .claude/skills/tasker-to-beads/


def find_project_root() -> Path:
    """Find project root by walking up to find project-planning/ or .git/"""
    current = SKILL_DIR
    for _ in range(10):  # Max 10 levels up
        if (current / "project-planning").is_dir():
            return current
        if (current / ".git").is_dir():
            return current
        parent = current.parent
        if parent == current:
            break
        current = parent
    raise RuntimeError("Could not find project root (no project-planning/ or .git/ found)")


PROJECT_ROOT = find_project_root()
PLANNING_DIR = PROJECT_ROOT / "project-planning"
TASKS_DIR = PLANNING_DIR / "tasks"
ARTIFACTS_DIR = PLANNING_DIR / "artifacts"
INPUTS_DIR = PLANNING_DIR / "inputs"
BEADS_OUTPUT_DIR = PLANNING_DIR / "beads-export"

# Target directory for beads operations (can be different from project root)
TARGET_DIR: Path | None = None


def parse_target_dir(args: list[str]) -> tuple[Path | None, list[str]]:
    """Extract -t/--target-dir from args, return (target_path, remaining_args)."""
    remaining = []
    target = None
    i = 0
    while i < len(args):
        if args[i] in ("-t", "--target-dir"):
            if i + 1 < len(args):
                target = Path(args[i + 1]).resolve()
                i += 2
                continue
        remaining.append(args[i])
        i += 1
    return target, remaining


def get_target_dir() -> Path:
    """Get the target directory for beads operations."""
    return TARGET_DIR if TARGET_DIR else PROJECT_ROOT


def is_beads_initialized(directory: Path) -> bool:
    """Check if beads is initialized in the given directory."""
    return (directory / ".beads").is_dir()


def init_beads_in_target(target_dir: Path, prefix: str = "TASK") -> tuple[bool, str]:
    """Initialize beads in the target directory if not already done.

    Runs two commands:
    1. bd init <prefix> - creates .beads directory structure
    2. bd onboard - sets up project configuration
    """
    if is_beads_initialized(target_dir):
        return True, f"Beads already initialized in {target_dir}"

    env = {**os.environ, "PWD": str(target_dir)}

    # Step 1: Run bd init
    init_cmd = ["bd", "init", prefix]
    init_result = subprocess.run(
        init_cmd,
        capture_output=True,
        text=True,
        cwd=target_dir,
        env=env
    )

    if init_result.returncode != 0:
        return False, f"Failed to initialize beads: {init_result.stderr}"

    # Step 2: Run bd onboard
    onboard_cmd = ["bd", "onboard"]
    onboard_result = subprocess.run(
        onboard_cmd,
        capture_output=True,
        text=True,
        cwd=target_dir,
        env=env
    )

    if onboard_result.returncode != 0:
        return False, f"Beads initialized but onboarding failed: {onboard_result.stderr}"

    return True, f"Beads initialized and onboarded in {target_dir} with prefix '{prefix}'"


def load_json(path: Path) -> dict | None:
    if not path.exists():
        return None
    return json.loads(path.read_text())


def load_spec() -> str:
    spec_path = INPUTS_DIR / "spec.md"
    if spec_path.exists():
        return spec_path.read_text()
    return ""


def load_capability_map() -> dict:
    return load_json(ARTIFACTS_DIR / "capability-map.json") or {}


def load_physical_map() -> dict:
    return load_json(ARTIFACTS_DIR / "physical-map.json") or {}


def load_state() -> dict:
    return load_json(PLANNING_DIR / "state.json") or {}


def load_task(task_id: str) -> dict | None:
    return load_json(TASKS_DIR / f"{task_id}.json")


def get_all_task_ids() -> list[str]:
    if not TASKS_DIR.exists():
        return []
    return sorted([p.stem for p in TASKS_DIR.glob("T*.json")])


def find_capability_context(capability_map: dict, task: dict) -> dict:
    """Find the capability and domain context for a task."""
    context = task.get("context", {})
    domain_name = context.get("domain", "")
    capability_name = context.get("capability", "")

    result = {
        "domain": None,
        "capability": None,
        "behaviors": [],
    }

    for domain in capability_map.get("domains", []):
        if domain.get("name") == domain_name or domain.get("id") == context.get("domain_id"):
            result["domain"] = {
                "name": domain.get("name"),
                "description": domain.get("description", ""),
            }
            for capability in domain.get("capabilities", []):
                if capability.get("name") == capability_name or capability.get("id") == context.get("capability_id"):
                    result["capability"] = {
                        "name": capability.get("name"),
                        "description": capability.get("description", ""),
                        "spec_ref": capability.get("spec_ref", ""),
                    }
                    # Get behaviors referenced by task
                    task_behavior_ids = task.get("behaviors", [])
                    for behavior in capability.get("behaviors", []):
                        if behavior.get("id") in task_behavior_ids:
                            result["behaviors"].append({
                                "id": behavior.get("id"),
                                "name": behavior.get("name"),
                                "description": behavior.get("description", ""),
                                "type": behavior.get("type", "process"),
                            })
                    break
            break

    return result


def extract_relevant_spec_sections(spec: str, task: dict, capability_context: dict) -> list[str]:
    """Extract spec sections likely relevant to this task using keyword matching."""
    if not spec:
        return []

    # Build keyword list from task context
    keywords = set()

    # From task name
    keywords.update(task.get("name", "").lower().split())

    # From context
    context = task.get("context", {})
    if context.get("domain"):
        keywords.update(context["domain"].lower().split())
    if context.get("capability"):
        keywords.update(context["capability"].lower().split())

    # From behaviors
    for behavior in capability_context.get("behaviors", []):
        keywords.update(behavior.get("name", "").lower().split())

    # From files
    for file_info in task.get("files", []):
        # Extract meaningful parts from path
        path = file_info.get("path", "")
        parts = re.split(r"[/_.]", path)
        keywords.update(p.lower() for p in parts if len(p) > 2)

    # Remove common words
    stopwords = {"the", "and", "for", "with", "from", "that", "this", "are", "will", "can", "should"}
    keywords -= stopwords

    # Split spec into sections (by markdown headers)
    sections = re.split(r"\n(?=#{1,3}\s)", spec)

    relevant = []
    for section in sections:
        section_lower = section.lower()
        # Score by keyword matches
        score = sum(1 for kw in keywords if kw in section_lower and len(kw) > 3)
        if score >= 2:  # At least 2 keyword matches
            # Truncate long sections
            if len(section) > 1500:
                section = section[:1500] + "\n[...truncated...]"
            relevant.append(section.strip())

    return relevant[:5]  # Max 5 sections


def get_dependency_context(state: dict, task: dict) -> list[dict]:
    """Get context about task dependencies."""
    deps = []
    task_deps = task.get("dependencies", {}).get("tasks", [])

    for dep_id in task_deps:
        dep_task = load_task(dep_id)
        if dep_task:
            deps.append({
                "id": dep_id,
                "name": dep_task.get("name", ""),
                "files_created": [f.get("path") for f in dep_task.get("files", [])],
            })

    return deps


def phase_to_priority(phase: int) -> str:
    """Map task phase to beads priority."""
    if phase == 1:
        return "critical"
    elif phase == 2:
        return "high"
    elif phase in (3, 4):
        return "medium"
    else:
        return "low"


def prepare_task_context(task_id: str) -> dict | None:
    """Prepare full context for a task for LLM enrichment."""
    task = load_task(task_id)
    if not task:
        return None

    spec = load_spec()
    capability_map = load_capability_map()
    state = load_state()

    capability_context = find_capability_context(capability_map, task)
    relevant_spec = extract_relevant_spec_sections(spec, task, capability_context)
    dependency_context = get_dependency_context(state, task)

    state_task = state.get("tasks", {}).get(task_id, {})

    return {
        "task_id": task_id,
        "task": task,
        "state": {
            "status": state_task.get("status", "pending"),
            "phase": state_task.get("phase", task.get("phase", 1)),
            "blocks": state_task.get("blocks", []),
        },
        "capability_context": capability_context,
        "relevant_spec_sections": relevant_spec,
        "dependency_context": dependency_context,
        "suggested_priority": phase_to_priority(task.get("phase", 1)),
        "suggested_labels": build_labels(task, capability_context),
    }


def build_labels(task: dict, capability_context: dict) -> list[str]:
    """Build suggested labels for the beads issue."""
    labels = []

    context = task.get("context", {})
    if context.get("domain"):
        labels.append(f"domain:{context['domain'].lower().replace(' ', '-')}")
    if context.get("capability"):
        labels.append(f"capability:{context['capability'].lower().replace(' ', '-')}")
    if context.get("steel_thread"):
        labels.append("steel-thread")

    phase = task.get("phase", 0)
    if phase > 0:
        labels.append(f"phase:{phase}")

    return labels


def prepare_all_contexts() -> list[dict]:
    """Prepare context for all tasks."""
    task_ids = get_all_task_ids()
    contexts = []
    for task_id in task_ids:
        ctx = prepare_task_context(task_id)
        if ctx:
            contexts.append(ctx)
    return contexts


def priority_to_bd_priority(priority: str) -> str:
    """Convert priority string to bd priority (0-4, 0=highest)."""
    mapping = {
        "critical": "0",
        "high": "1",
        "medium": "2",
        "low": "3",
    }
    return mapping.get(priority.lower(), "2")


def create_beads_issue(
    task_id: str,
    title: str,
    description: str,
    priority: str,
    labels: list[str],
    dependencies: list[str],
) -> tuple[bool, str]:
    """Create a beads issue using the bd CLI in the target directory.

    Note: Dependencies are Tasker IDs (T001, T002) which must be mapped to
    beads IDs after creation using link_dependencies().
    """
    target = get_target_dir()
    env = {**os.environ, "PWD": str(target)}

    # Ensure beads is initialized in target
    if not is_beads_initialized(target):
        success, msg = init_beads_in_target(target)
        if not success:
            return False, msg
        print(f"  {msg}")

    # Build the create command (without dependencies - added later via bd dep add)
    cmd = [
        "bd", "create", title,
        "-t", "task",
        "-p", priority_to_bd_priority(priority),
        "--silent",  # Output only the issue ID
    ]

    # Add labels if present (include task_id as label for mapping)
    all_labels = list(labels) + [f"tasker:{task_id}"]
    cmd.extend(["-l", ",".join(all_labels)])

    # Add description if present
    if description:
        cmd.extend(["-d", description])

    result = subprocess.run(cmd, capture_output=True, text=True, cwd=target, env=env)

    if result.returncode != 0:
        return False, f"Failed to create issue: {result.stderr}"

    # With --silent, output is just the issue ID
    issue_id = result.stdout.strip()
    if not issue_id:
        return False, f"No issue ID returned from bd create"

    return True, issue_id


def link_dependencies(
    task_id_to_beads_id: dict[str, str],
    task_dependencies: dict[str, list[str]],
) -> tuple[int, int]:
    """Link dependencies between beads issues after all are created.

    Args:
        task_id_to_beads_id: Mapping of Tasker IDs to beads issue IDs
        task_dependencies: Mapping of Tasker IDs to their dependency Tasker IDs

    Returns:
        Tuple of (successful links, failed links)
    """
    target = get_target_dir()
    env = {**os.environ, "PWD": str(target)}
    success_count = 0
    fail_count = 0

    for task_id, dep_task_ids in task_dependencies.items():
        beads_id = task_id_to_beads_id.get(task_id)
        if not beads_id:
            continue

        for dep_task_id in dep_task_ids:
            dep_beads_id = task_id_to_beads_id.get(dep_task_id)
            if not dep_beads_id:
                print(f"  Warning: Dependency {dep_task_id} not found for {task_id}")
                fail_count += 1
                continue

            # bd dep add <issue> <depends-on> --type blocks
            cmd = ["bd", "dep", "add", beads_id, dep_beads_id, "-t", "blocks"]
            result = subprocess.run(cmd, capture_output=True, text=True, cwd=target, env=env)

            if result.returncode != 0:
                print(f"  Failed to link {task_id} -> {dep_task_id}: {result.stderr}")
                fail_count += 1
            else:
                success_count += 1

    return success_count, fail_count


def save_context_for_enrichment(task_id: str, context: dict) -> Path:
    """Save context to file for LLM processing."""
    BEADS_OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    output_path = BEADS_OUTPUT_DIR / f"{task_id}-context.json"
    output_path.write_text(json.dumps(context, indent=2))
    return output_path


def print_context_summary(context: dict) -> None:
    """Print a summary of the prepared context."""
    print(f"Task: {context['task_id']} - {context['task'].get('name', '')}")
    print(f"  Phase: {context['state']['phase']}")
    print(f"  Priority: {context['suggested_priority']}")
    print(f"  Labels: {', '.join(context['suggested_labels'])}")
    print(f"  Spec sections found: {len(context['relevant_spec_sections'])}")
    print(f"  Dependencies: {len(context['dependency_context'])}")
    print(f"  Behaviors: {len(context['capability_context'].get('behaviors', []))}")


def main() -> None:
    global TARGET_DIR

    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    # Parse target directory from args
    TARGET_DIR, args = parse_target_dir(sys.argv[1:])
    if not args:
        print(__doc__)
        sys.exit(1)

    cmd = args[0]

    if cmd == "context":
        if len(args) < 2:
            print("Usage: transform.py context <task_id> | --all [-t TARGET_DIR]")
            sys.exit(1)

        if args[1] == "--all":
            contexts = prepare_all_contexts()
            print(f"Prepared context for {len(contexts)} tasks\n")
            for ctx in contexts:
                print_context_summary(ctx)
                save_context_for_enrichment(ctx["task_id"], ctx)
                print()
            print(f"\nContext files saved to: {BEADS_OUTPUT_DIR}/")
            if TARGET_DIR:
                print(f"Target directory for beads: {TARGET_DIR}")
                print(f"  Beads initialized: {is_beads_initialized(TARGET_DIR)}")
        else:
            task_id = args[1]
            context = prepare_task_context(task_id)
            if not context:
                print(f"Task not found: {task_id}")
                sys.exit(1)
            print_context_summary(context)
            output_path = save_context_for_enrichment(task_id, context)
            print(f"\nFull context saved to: {output_path}")

            # Also output JSON to stdout for piping
            if "--json" in args:
                print("\n---JSON---")
                print(json.dumps(context, indent=2))

    elif cmd == "status":
        task_ids = get_all_task_ids()
        state = load_state()
        target = get_target_dir()

        print(f"Source Project: {PROJECT_ROOT}")
        print(f"Tasker Tasks: {len(task_ids)}")
        print(f"State phase: {state.get('phase', {}).get('current', 'unknown')}")

        # Check for existing beads export
        if BEADS_OUTPUT_DIR.exists():
            exported = list(BEADS_OUTPUT_DIR.glob("*-context.json"))
            enriched = list(BEADS_OUTPUT_DIR.glob("*-enriched.json"))
            print(f"\nBeads export directory: {BEADS_OUTPUT_DIR}")
            print(f"  Context files: {len(exported)}")
            print(f"  Enriched files: {len(enriched)}")

        # Target directory status
        print(f"\nTarget Directory: {target}")
        if is_beads_initialized(target):
            print("  Beads: initialized")
            # Count issues in target
            beads_dir = target / ".beads" / "issues"
            if beads_dir.exists():
                issue_count = len(list(beads_dir.glob("*.md")))
                print(f"  Issues: {issue_count}")
        else:
            print("  Beads: not initialized")
            print("  Run 'transform.py init-target <dir>' to initialize")

    elif cmd == "init-target":
        if len(args) < 2:
            print("Usage: transform.py init-target <target_dir> [PREFIX]")
            sys.exit(1)

        target = Path(args[1]).resolve()
        prefix = args[2] if len(args) > 2 else "TASK"

        if not target.exists():
            print(f"Target directory does not exist: {target}")
            sys.exit(1)

        success, msg = init_beads_in_target(target, prefix)
        print(msg)
        sys.exit(0 if success else 1)

    elif cmd == "create":
        if len(args) < 3:
            print("Usage: transform.py create <task_id> <enriched_file> [-t TARGET_DIR]")
            sys.exit(1)
        task_id = args[1]
        desc_file = Path(args[2])

        if not desc_file.exists():
            print(f"Description file not found: {desc_file}")
            sys.exit(1)

        target = get_target_dir()
        print(f"Creating issue in: {target}")

        enriched = json.loads(desc_file.read_text())

        success, result = create_beads_issue(
            task_id=task_id,
            title=enriched.get("title", ""),
            description=enriched.get("description", ""),
            priority=enriched.get("priority", "medium"),
            labels=enriched.get("labels", []),
            dependencies=enriched.get("dependencies", []),
        )

        if success:
            print(f"Created beads issue: {result}")
        else:
            print(f"Error: {result}")
            sys.exit(1)

    elif cmd == "batch-create":
        if len(args) < 2:
            print("Usage: transform.py batch-create <manifest_file> [-t TARGET_DIR]")
            sys.exit(1)

        manifest_path = Path(args[1])
        if not manifest_path.exists():
            print(f"Manifest not found: {manifest_path}")
            sys.exit(1)

        target = get_target_dir()
        print(f"Creating issues in: {target}")

        manifest = json.loads(manifest_path.read_text())

        # Phase 1: Create all issues (without dependencies)
        print("\n--- Phase 1: Creating issues ---")
        task_id_to_beads_id: dict[str, str] = {}
        task_dependencies: dict[str, list[str]] = {}
        created = 0
        failed = 0

        for entry in manifest.get("issues", []):
            task_id = entry.get("task_id", "")
            success, result = create_beads_issue(
                task_id=task_id,
                title=entry.get("title", ""),
                description=entry.get("description", ""),
                priority=entry.get("priority", "medium"),
                labels=entry.get("labels", []),
                dependencies=[],  # Dependencies added in phase 2
            )
            if success:
                print(f"  Created: {task_id} -> {result}")
                task_id_to_beads_id[task_id] = result
                created += 1

                # Get dependencies from Tasker task file (the DAG source of truth)
                task = load_task(task_id)
                if task:
                    deps = task.get("dependencies", {}).get("tasks", [])
                    if deps:
                        task_dependencies[task_id] = deps
            else:
                print(f"  Failed {task_id}: {result}")
                failed += 1

        print(f"\nPhase 1 complete: {created} created, {failed} failed")

        # Phase 2: Link dependencies using the Tasker DAG
        if task_dependencies:
            print("\n--- Phase 2: Linking dependencies ---")
            dep_success, dep_failed = link_dependencies(task_id_to_beads_id, task_dependencies)
            print(f"\nPhase 2 complete: {dep_success} links created, {dep_failed} failed")
        else:
            print("\nNo dependencies to link.")

        # Save the mapping for reference
        mapping_file = BEADS_OUTPUT_DIR / "task-to-beads-mapping.json"
        mapping_file.write_text(json.dumps(task_id_to_beads_id, indent=2))
        print(f"\nMapping saved to: {mapping_file}")

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
