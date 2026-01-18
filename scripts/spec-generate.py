#!/usr/bin/env python3
"""
Spec and ADR Generator

Generates spec packets and ADR files from session state
and discovery content.
"""

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Optional

DISCOVERY_FILE = ".claude/clarify-session.md"
SESSION_FILE = ".claude/spec-session.json"

# Default to current directory, but should be overridden with TARGET_DIR
# Specs and ADRs go in target project's docs/ directory
def get_specs_dir(target_dir: str = None) -> Path:
    base = Path(target_dir) if target_dir else Path.cwd()
    return base / "docs" / "specs"

def get_adrs_dir(target_dir: str = None) -> Path:
    base = Path(target_dir) if target_dir else Path.cwd()
    return base / "docs" / "adrs"

# Legacy constants for backwards compatibility
SPECS_DIR = "docs/specs"
ADRS_DIR = "docs/adrs"


def slugify(text: str) -> str:
    """Convert text to slug format."""
    slug = text.lower()
    slug = re.sub(r'[^\w\s-]', '', slug)
    slug = re.sub(r'[\s_]+', '-', slug)
    slug = re.sub(r'-+', '-', slug)
    return slug.strip('-')


def load_session() -> Optional[dict]:
    """Load session state."""
    session_path = Path(SESSION_FILE)
    if not session_path.exists():
        return None
    return json.loads(session_path.read_text())


def load_discovery() -> Optional[str]:
    """Load discovery file content."""
    discovery_path = Path(DISCOVERY_FILE)
    if not discovery_path.exists():
        return None
    return discovery_path.read_text()


def generate_spec_packet(
    title: str,
    slug: str,
    goal: str,
    non_goals: list[str],
    done_means: list[str],
    workflows: str,
    invariants: list[str],
    interfaces: str,
    architecture: str,
    decisions: list[dict],
    open_questions: dict,
    handoff: dict,
    adrs: list[str]
) -> str:
    """Generate spec packet markdown."""

    # Related ADRs section
    if adrs:
        adr_links = []
        for adr_id in adrs:
            adr_files = list(Path(ADRS_DIR).glob(f"ADR-{adr_id}*.md"))
            if adr_files:
                adr_file = adr_files[0]
                adr_links.append(f"- [{adr_file.stem}](../adrs/{adr_file.name})")
            else:
                adr_links.append(f"- ADR-{adr_id} (file not found)")
        related_adrs_md = "\n".join(adr_links)
    else:
        related_adrs_md = "(none)"

    non_goals_md = "\n".join(f"- {ng}" for ng in non_goals) if non_goals else "- (none specified)"
    done_means_md = "\n".join(f"- {dm}" for dm in done_means) if done_means else "- (none specified)"
    invariants_md = "\n".join(f"- {inv}" for inv in invariants) if invariants else "- (none specified)"

    # Decisions table
    decisions_rows = []
    for d in decisions:
        decision_text = d['decision']
        adr_link = f"[ADR-{d['adr_id']}](../adrs/ADR-{d['adr_id']}.md)" if d.get('adr_id') else "(inline)"
        decisions_rows.append(f"| {decision_text} | {adr_link} |")

    if decisions_rows:
        decisions_md = "| Decision | ADR |\n|----------|-----|\n" + "\n".join(decisions_rows)
    else:
        decisions_md = "(no decisions recorded)"

    blocking_qs = open_questions.get("blocking", [])
    non_blocking_qs = open_questions.get("non_blocking", [])

    blocking_md = "\n".join(f"- {q}" for q in blocking_qs) if blocking_qs else "(none)"
    non_blocking_md = "\n".join(f"- {q}" for q in non_blocking_qs) if non_blocking_qs else "(none)"

    handoff_md = ""
    if handoff:
        handoff_md = f"""- **What to build:** {handoff.get('what_to_build', 'See workflows above')}
- **Must preserve:** {handoff.get('must_preserve', 'See invariants above')}
- **Blocking conditions:** {handoff.get('blocking_conditions', 'None' if not blocking_qs else 'See blocking open questions')}"""
    else:
        handoff_md = """- **What to build:** See workflows above
- **Must preserve:** See invariants above
- **Blocking conditions:** None"""

    return f"""# Spec: {title}

## Related ADRs
{related_adrs_md}

## Goal
{goal}

## Non-goals
{non_goals_md}

## Done means
{done_means_md}

## Workflows
{workflows if workflows else '(to be defined)'}

## Invariants
{invariants_md}

## Interfaces
{interfaces if interfaces else 'No new/changed interfaces'}

## Architecture sketch
{architecture if architecture else '(to be defined)'}

## Decisions
{decisions_md}

## Open Questions

### Blocking
{blocking_md}

### Non-blocking
{non_blocking_md}

## Agent Handoff
{handoff_md}

## Artifacts
- **Capability Map:** [{slug}.capabilities.json](./{slug}.capabilities.json)
- **Discovery Log:** [clarify-session.md](../.claude/clarify-session.md)
"""


def generate_adr(
    number: int,
    title: str,
    context: str,
    decision: str,
    alternatives: list[dict],
    consequences: list[str],
    applies_to: list[dict] = None,
    supersedes: str = None,
    related_adrs: list[str] = None
) -> str:
    """Generate ADR markdown.

    Args:
        applies_to: List of {"slug": "feature-a", "title": "Feature A"} dicts
    """

    # Alternatives table
    if alternatives:
        alt_rows = []
        for alt in alternatives:
            alt_rows.append(f"| {alt['name']} | {alt.get('reason', 'N/A')} |")
        alternatives_md = "| Alternative | Why Not Chosen |\n|-------------|----------------|\n" + "\n".join(alt_rows)
    else:
        alternatives_md = "(no alternatives recorded)"

    consequences_md = "\n".join(f"- {c}" for c in consequences) if consequences else "- (none specified)"

    # Applies to (many-to-many with specs)
    if applies_to:
        applies_md = "\n".join(
            f"- [{s.get('title', s['slug'])}](../specs/{s['slug']}.md)"
            for s in applies_to
        )
    else:
        applies_md = "- (none specified)"

    # Related ADRs
    supersedes_md = f"ADR-{supersedes}" if supersedes else "(none)"
    related_md = ", ".join(f"ADR-{r}" for r in related_adrs) if related_adrs else "(none)"

    return f"""# ADR-{number:04d}: {title}

**Status:** Accepted
**Date:** {datetime.now().strftime('%Y-%m-%d')}

## Applies To
{applies_md}

## Context
{context}

## Decision
{decision}

## Alternatives Considered
{alternatives_md}

## Consequences
{consequences_md}

## Related
- Supersedes: {supersedes_md}
- Related ADRs: {related_md}
"""


def cmd_spec(args):
    """Generate spec packet from session."""
    session = load_session()
    if not session:
        print("Error: No active session. Run `python3 scripts/spec-session.py init <topic>` first.")
        sys.exit(1)

    topic = session.get("topic", "Untitled")
    slug = slugify(topic)

    scope = session.get("scope", {})
    goal = scope.get("goal", "(goal not defined)")
    non_goals = scope.get("non_goals", [])
    done_means = scope.get("done_means", [])

    workflows = session.get("workflows", "")
    invariants = session.get("invariants", [])
    interfaces = session.get("interfaces", "")
    architecture = session.get("architecture", "")
    decisions = session.get("decisions", [])
    open_questions = session.get("open_questions", {"blocking": [], "non_blocking": []})
    handoff = session.get("handoff", {})
    adrs = session.get("adrs", [])

    spec_content = generate_spec_packet(
        title=topic,
        slug=slug,
        goal=goal,
        non_goals=non_goals,
        done_means=done_means,
        workflows=workflows,
        invariants=invariants,
        interfaces=interfaces,
        architecture=architecture,
        decisions=decisions,
        open_questions=open_questions,
        handoff=handoff,
        adrs=adrs
    )

    # Determine output directory
    target_dir = getattr(args, 'target_dir', None) or session.get('target_dir')
    specs_dir = get_specs_dir(target_dir)
    specs_dir.mkdir(parents=True, exist_ok=True)

    output_path = specs_dir / f"{slug}.md"

    if output_path.exists() and not args.force:
        print(f"Error: {output_path} already exists. Use --force to overwrite.")
        sys.exit(1)

    output_path.write_text(spec_content)
    print(f"Generated: {output_path}")
    return str(output_path)


def cmd_adr(args):
    """Generate ADR from arguments."""
    session = load_session()

    number = args.number
    if number is None:
        adrs_dir = Path(ADRS_DIR)
        if adrs_dir.exists():
            existing = list(adrs_dir.glob("ADR-*.md"))
            numbers = []
            for f in existing:
                match = re.match(r"ADR-(\d+)", f.name)
                if match:
                    numbers.append(int(match.group(1)))
            number = max(numbers) + 1 if numbers else 1
        else:
            number = 1

    alternatives = []
    if args.alternatives:
        for alt in args.alternatives:
            if ":" in alt:
                name, reason = alt.split(":", 1)
                alternatives.append({"name": name.strip(), "reason": reason.strip()})
            else:
                alternatives.append({"name": alt, "reason": "Not selected"})

    consequences = args.consequences if args.consequences else []

    # Build applies_to list (many-to-many with specs)
    applies_to = []
    if args.applies_to:
        for spec_ref in args.applies_to:
            if ":" in spec_ref:
                slug, title = spec_ref.split(":", 1)
                applies_to.append({"slug": slug.strip(), "title": title.strip()})
            else:
                applies_to.append({"slug": spec_ref, "title": spec_ref})
    elif session:
        # Default to current session's spec
        spec_slug = slugify(session.get("topic", "unknown"))
        spec_title = session.get("topic", "Unknown")
        applies_to.append({"slug": spec_slug, "title": spec_title})

    adr_content = generate_adr(
        number=number,
        title=args.title,
        context=args.context,
        decision=args.decision,
        alternatives=alternatives,
        consequences=consequences,
        applies_to=applies_to,
        supersedes=args.supersedes,
        related_adrs=args.related
    )

    # Determine output directory
    target_dir = getattr(args, 'target_dir', None) or (session.get('target_dir') if session else None)
    adrs_dir = get_adrs_dir(target_dir)
    adrs_dir.mkdir(parents=True, exist_ok=True)

    adr_slug = slugify(args.title)
    output_path = adrs_dir / f"ADR-{number:04d}-{adr_slug}.md"

    if output_path.exists() and not args.force:
        print(f"Error: {output_path} already exists. Use --force to overwrite.")
        sys.exit(1)

    output_path.write_text(adr_content)
    print(f"Generated: {output_path}")

    if session:
        if "adrs" not in session:
            session["adrs"] = []
        session["adrs"].append(f"{number:04d}")
        Path(SESSION_FILE).write_text(json.dumps(session, indent=2))

    return str(output_path)


def cmd_update_session(args):
    """Update session with spec content fields."""
    session = load_session()
    if not session:
        print("Error: No active session.")
        sys.exit(1)

    field = args.field
    value = args.value

    if field == "goal":
        if "scope" not in session:
            session["scope"] = {}
        session["scope"]["goal"] = value
    elif field == "non_goals":
        if "scope" not in session:
            session["scope"] = {}
        if "non_goals" not in session["scope"]:
            session["scope"]["non_goals"] = []
        session["scope"]["non_goals"].append(value)
    elif field == "done_means":
        if "scope" not in session:
            session["scope"] = {}
        if "done_means" not in session["scope"]:
            session["scope"]["done_means"] = []
        session["scope"]["done_means"].append(value)
    elif field == "workflows":
        session["workflows"] = value
    elif field == "invariants":
        if "invariants" not in session:
            session["invariants"] = []
        session["invariants"].append(value)
    elif field == "interfaces":
        session["interfaces"] = value
    elif field == "architecture":
        session["architecture"] = value
    else:
        print(f"Unknown field: {field}")
        sys.exit(1)

    Path(SESSION_FILE).write_text(json.dumps(session, indent=2))
    print(f"Updated {field}")


def main():
    parser = argparse.ArgumentParser(description="Spec and ADR Generator")
    subparsers = parser.add_subparsers(dest="command", help="Commands")

    spec_parser = subparsers.add_parser("spec", help="Generate spec packet")
    spec_parser.add_argument("--target-dir", dest="target_dir", help="Target project directory (outputs to docs/specs/)")
    spec_parser.add_argument("--force", action="store_true", help="Overwrite existing file")

    adr_parser = subparsers.add_parser("adr", help="Generate ADR")
    adr_parser.add_argument("--target-dir", dest="target_dir", help="Target project directory (outputs to docs/adrs/)")
    adr_parser.add_argument("--number", "-n", type=int, help="ADR number (auto-increments if not specified)")
    adr_parser.add_argument("--title", "-t", required=True, help="ADR title")
    adr_parser.add_argument("--context", "-c", required=True, help="Context for the decision")
    adr_parser.add_argument("--decision", "-d", required=True, help="The decision made")
    adr_parser.add_argument("--alternatives", "-a", nargs="*", help="Alternatives (format: 'Name: reason')")
    adr_parser.add_argument("--consequences", nargs="*", help="Consequences of the decision")
    adr_parser.add_argument("--applies-to", nargs="*", dest="applies_to", help="Specs this ADR applies to (format: 'slug:Title' or just 'slug')")
    adr_parser.add_argument("--supersedes", help="ADR number this supersedes (e.g., '0001')")
    adr_parser.add_argument("--related", nargs="*", help="Related ADR numbers (e.g., '0002' '0003')")
    adr_parser.add_argument("--force", action="store_true", help="Overwrite existing file")

    update_parser = subparsers.add_parser("update", help="Update session field")
    update_parser.add_argument("field", choices=["goal", "non_goals", "done_means", "workflows", "invariants", "interfaces", "architecture"])
    update_parser.add_argument("value", help="Value to set/append")

    args = parser.parse_args()

    if args.command == "spec":
        cmd_spec(args)
    elif args.command == "adr":
        cmd_adr(args)
    elif args.command == "update":
        cmd_update_session(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
