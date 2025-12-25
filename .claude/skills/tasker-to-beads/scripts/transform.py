#!/usr/bin/env python3
"""
Tasker to Beads Transformer

Prepares task context for LLM enrichment and handles beads issue creation.
This script does the mechanical work; the LLM does the comprehension work.

Usage:
    tasker_to_beads.py context <task_id>      Prepare context for single task
    tasker_to_beads.py context --all          Prepare context for all tasks
    tasker_to_beads.py create <task_id> <description_file>
                                              Create beads issue from enriched description
    tasker_to_beads.py batch-create <manifest_file>
                                              Create multiple issues from manifest
    tasker_to_beads.py status                 Show transformation status
"""

import json
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


def create_beads_issue(
    task_id: str,
    title: str,
    description: str,
    priority: str,
    labels: list[str],
    dependencies: list[str],
) -> tuple[bool, str]:
    """Create a beads issue using the beads CLI."""
    # Create the issue
    cmd = ["claude", "-p", f"/beads:create \"{title}\" task {priority}"]
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=PROJECT_ROOT)

    if result.returncode != 0:
        return False, f"Failed to create issue: {result.stderr}"

    # Extract issue ID from output (beads typically outputs the created issue ID)
    output = result.stdout
    issue_id_match = re.search(r"([A-Z]+-\d+)", output)
    if not issue_id_match:
        return False, f"Could not extract issue ID from output: {output}"

    issue_id = issue_id_match.group(1)

    # Add labels
    for label in labels:
        subprocess.run(
            ["claude", "-p", f"/beads:label add {issue_id} {label}"],
            capture_output=True,
            cwd=PROJECT_ROOT,
        )

    # Add dependencies
    for dep in dependencies:
        subprocess.run(
            ["claude", "-p", f"/beads:dep add {issue_id} {dep}"],
            capture_output=True,
            cwd=PROJECT_ROOT,
        )

    return True, issue_id


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
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "context":
        if len(sys.argv) < 3:
            print("Usage: tasker_to_beads.py context <task_id> | --all")
            sys.exit(1)

        if sys.argv[2] == "--all":
            contexts = prepare_all_contexts()
            print(f"Prepared context for {len(contexts)} tasks\n")
            for ctx in contexts:
                print_context_summary(ctx)
                save_context_for_enrichment(ctx["task_id"], ctx)
                print()
            print(f"\nContext files saved to: {BEADS_OUTPUT_DIR}/")
        else:
            task_id = sys.argv[2]
            context = prepare_task_context(task_id)
            if not context:
                print(f"Task not found: {task_id}")
                sys.exit(1)
            print_context_summary(context)
            output_path = save_context_for_enrichment(task_id, context)
            print(f"\nFull context saved to: {output_path}")

            # Also output JSON to stdout for piping
            if "--json" in sys.argv:
                print("\n---JSON---")
                print(json.dumps(context, indent=2))

    elif cmd == "status":
        task_ids = get_all_task_ids()
        state = load_state()

        print(f"Tasker Tasks: {len(task_ids)}")
        print(f"State phase: {state.get('phase', {}).get('current', 'unknown')}")

        # Check for existing beads export
        if BEADS_OUTPUT_DIR.exists():
            exported = list(BEADS_OUTPUT_DIR.glob("*-context.json"))
            enriched = list(BEADS_OUTPUT_DIR.glob("*-enriched.json"))
            print(f"\nBeads export directory: {BEADS_OUTPUT_DIR}")
            print(f"  Context files: {len(exported)}")
            print(f"  Enriched files: {len(enriched)}")

    elif cmd == "create":
        if len(sys.argv) < 4:
            print("Usage: tasker_to_beads.py create <task_id> <enriched_description_file>")
            sys.exit(1)
        task_id = sys.argv[2]
        desc_file = Path(sys.argv[3])

        if not desc_file.exists():
            print(f"Description file not found: {desc_file}")
            sys.exit(1)

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
        if len(sys.argv) < 3:
            print("Usage: tasker_to_beads.py batch-create <manifest_file>")
            sys.exit(1)

        manifest_path = Path(sys.argv[2])
        if not manifest_path.exists():
            print(f"Manifest not found: {manifest_path}")
            sys.exit(1)

        manifest = json.loads(manifest_path.read_text())
        created = 0
        failed = 0

        for entry in manifest.get("issues", []):
            success, result = create_beads_issue(
                task_id=entry.get("task_id", ""),
                title=entry.get("title", ""),
                description=entry.get("description", ""),
                priority=entry.get("priority", "medium"),
                labels=entry.get("labels", []),
                dependencies=entry.get("dependencies", []),
            )
            if success:
                print(f"Created: {result}")
                created += 1
            else:
                print(f"Failed {entry.get('task_id')}: {result}")
                failed += 1

        print(f"\nCreated: {created}, Failed: {failed}")

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
