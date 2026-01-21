#!/usr/bin/env bash
# Initialize all planning directories required by the orchestrator workflow.
# This script MUST be run at the start of any /plan or /execute session.
#
# Usage: ./scripts/init-planning-dirs.sh [base_dir]
#   base_dir: Optional. Defaults to current working directory.
#
# Creates:
#   .tasker/artifacts/  - For capability-map.json, physical-map.json
#   .tasker/inputs/     - For spec.md
#   .tasker/tasks/      - For T001.json, T002.json, etc.
#   .tasker/reports/    - For task-validation-report.md
#   .tasker/bundles/    - For execution bundles
#   .claude/logs/                - For activity logging

set -euo pipefail

BASE_DIR="${1:-$(pwd)}"
TASKER_DIR="$BASE_DIR/.tasker"

echo "Initializing planning directories..."
echo "Base directory: $BASE_DIR"
echo "Planning directory: $TASKER_DIR"

# Create all required directories
mkdir -p "$TASKER_DIR/artifacts"
mkdir -p "$TASKER_DIR/inputs"
mkdir -p "$TASKER_DIR/tasks"
mkdir -p "$TASKER_DIR/reports"
mkdir -p "$TASKER_DIR/bundles"
mkdir -p "$BASE_DIR/.claude/logs"

# Verify creation
echo ""
echo "Directory structure initialized:"
ls -la "$TASKER_DIR"

echo ""
echo "TASKER_DIR=$TASKER_DIR"
