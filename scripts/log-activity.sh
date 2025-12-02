#!/usr/bin/env bash
# Log activity to .claude/logs/activity.log
#
# Usage: ./scripts/log-activity.sh <level> <agent> <event> <message>
#   level:   INFO, WARN, ERROR
#   agent:   orchestrator, logic-architect, physical-architect, task-author, etc.
#   event:   start, decision, tool, complete, spawn, spawn-complete, phase-transition, validation
#   message: Description of the activity
#
# Examples:
#   ./scripts/log-activity.sh INFO orchestrator spawn "launching logic-architect for capability extraction"
#   ./scripts/log-activity.sh INFO logic-architect start "Extracting capabilities from spec"
#   ./scripts/log-activity.sh INFO logic-architect complete "Wrote capability-map.json with 5 capabilities"
#   ./scripts/log-activity.sh ERROR task-executor "Failed to write file: permission denied"

set -euo pipefail

LEVEL="${1:-INFO}"
AGENT="${2:-unknown}"
EVENT="${3:-message}"
MESSAGE="${4:-}"

LOG_FILE=".claude/logs/activity.log"

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Format: [ISO-timestamp] [LEVEL] [agent] event: message
TIMESTAMP=$(date -Iseconds)
echo "[$TIMESTAMP] [$LEVEL] [$AGENT] $EVENT: $MESSAGE" >> "$LOG_FILE"
