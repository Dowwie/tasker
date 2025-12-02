#!/usr/bin/env bash
# Initialize all planning directories required by the orchestrator workflow.
# This script MUST be run at the start of any /plan or /execute session.
#
# Usage: ./scripts/init-planning-dirs.sh [base_dir]
#   base_dir: Optional. Defaults to current working directory.
#
# Creates:
#   project-planning/artifacts/  - For capability-map.json, physical-map.json
#   project-planning/inputs/     - For spec.md
#   project-planning/tasks/      - For T001.json, T002.json, etc.
#   project-planning/reports/    - For task-validation-report.md
#   project-planning/bundles/    - For execution bundles
#   .claude/logs/                - For activity logging

set -euo pipefail

BASE_DIR="${1:-$(pwd)}"
PLANNING_DIR="$BASE_DIR/project-planning"

echo "Initializing planning directories..."
echo "Base directory: $BASE_DIR"
echo "Planning directory: $PLANNING_DIR"

# Create all required directories
mkdir -p "$PLANNING_DIR/artifacts"
mkdir -p "$PLANNING_DIR/inputs"
mkdir -p "$PLANNING_DIR/tasks"
mkdir -p "$PLANNING_DIR/reports"
mkdir -p "$PLANNING_DIR/bundles"
mkdir -p "$BASE_DIR/.claude/logs"

# Verify creation
echo ""
echo "Directory structure initialized:"
ls -la "$PLANNING_DIR"

echo ""
echo "PLANNING_DIR=$PLANNING_DIR"
