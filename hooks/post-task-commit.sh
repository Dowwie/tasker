#!/usr/bin/env bash
#
# post-task-commit.sh - Auto-commit task files after successful execution
#
# Two modes:
#   Hook mode:   Called by Claude Code PostToolUse, receives JSON on stdin
#   Manual mode: ./post-task-commit.sh <task_id> <target_dir> <planning_dir>
#
# Exit codes:
#   0 - Success (committed or nothing to commit)
#   1 - Error (missing args, invalid paths, git failure)
#

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

# Get tasker binary path (fast if already installed)
if [[ -z "${TASKER_BINARY:-}" ]]; then
    TASKER_BIN=$("$PLUGIN_ROOT/scripts/ensure-tasker.sh" 2>/dev/null) || exit 0
else
    TASKER_BIN="$TASKER_BINARY"
fi

# Determine mode based on arguments
if [[ $# -ge 3 ]]; then
    # Manual mode: args provided
    TASK_ID="$1"
    TARGET_DIR="$2"
    PLANNING_DIR="$3"
elif [[ $# -eq 0 ]]; then
    # Hook mode: parse from stdin JSON
    INPUT=$(cat)

    # Extract output from hook JSON using Go CLI
    OUTPUT=$(echo "$INPUT" | "$TASKER_BIN" hook parse-output 2>/dev/null || echo "")

    # Parse task ID from output (e.g., "T001: SUCCESS")
    TASK_ID=$(echo "$OUTPUT" | grep -oE 'T[0-9]+: SUCCESS' | head -1 | cut -d: -f1)

    if [[ -z "$TASK_ID" ]]; then
        # Not a successful task-executor output, skip silently
        exit 0
    fi

    # Get paths from state.json using Go CLI
    TARGET_DIR=$("$TASKER_BIN" state get-field target_dir 2>/dev/null || echo "")
    PLANNING_DIR="$(pwd)/project-planning"
else
    echo "Usage: $0 <task_id> <target_dir> <planning_dir>" >&2
    echo "   or: Called as hook with JSON on stdin" >&2
    exit 1
fi

# Validate inputs
if [[ -z "$TASK_ID" || -z "$TARGET_DIR" || -z "$PLANNING_DIR" ]]; then
    exit 0  # Silent exit for hook mode, missing context
fi

RESULT_FILE="$PLANNING_DIR/bundles/${TASK_ID}-result.json"

if [[ ! -f "$RESULT_FILE" ]]; then
    [[ $# -ge 3 ]] && echo "ERROR: Result file not found: $RESULT_FILE" >&2
    exit $([[ $# -ge 3 ]] && echo 1 || echo 0)
fi

if [[ ! -d "$TARGET_DIR/.git" ]]; then
    [[ $# -ge 3 ]] && echo "ERROR: Not a git repository: $TARGET_DIR" >&2
    exit $([[ $# -ge 3 ]] && echo 1 || echo 0)
fi

cd "$TARGET_DIR"

# Extract task name and status from result JSON using Go CLI
RESULT_INFO=$("$TASKER_BIN" -p "$PLANNING_DIR" bundle result-info "$TASK_ID" 2>/dev/null || echo "$TASK_ID	unknown")
TASK_NAME=$(echo "$RESULT_INFO" | cut -f1)
STATUS=$(echo "$RESULT_INFO" | cut -f2)

if [[ "$STATUS" != "success" ]]; then
    [[ $# -ge 3 ]] && echo "SKIP: Task $TASK_ID status is '$STATUS', not committing"
    exit 0
fi

# Get files from result (created + modified) using Go CLI
FILES_JSON=$("$TASKER_BIN" -p "$PLANNING_DIR" bundle result-files "$TASK_ID" 2>/dev/null || echo "")

if [[ -z "$FILES_JSON" ]]; then
    [[ $# -ge 3 ]] && echo "SKIP: No files recorded in result for $TASK_ID"
    exit 0
fi

# Check which files have uncommitted changes
UNCOMMITTED_FILES=()
while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    if [[ -f "$file" ]]; then
        if ! git diff --quiet -- "$file" 2>/dev/null || \
           ! git diff --cached --quiet -- "$file" 2>/dev/null || \
           [[ -n "$(git ls-files --others --exclude-standard -- "$file" 2>/dev/null)" ]]; then
            UNCOMMITTED_FILES+=("$file")
        fi
    fi
done <<< "$FILES_JSON"

if [[ ${#UNCOMMITTED_FILES[@]} -eq 0 ]]; then
    [[ $# -ge 3 ]] && echo "OK: All files for $TASK_ID already committed"
    exit 0
fi

[[ $# -ge 3 ]] && echo "COMMIT: Found ${#UNCOMMITTED_FILES[@]} uncommitted files for $TASK_ID"

# Stage and commit
for file in "${UNCOMMITTED_FILES[@]}"; do
    [[ $# -ge 3 ]] && echo "  + $file"
    git add "$file"
done

COMMIT_MSG="${TASK_ID}: ${TASK_NAME}"
git commit -m "$COMMIT_MSG" --no-verify

COMMIT_SHA=$(git rev-parse HEAD)
[[ $# -ge 3 ]] && echo "COMMITTED: $COMMIT_SHA - $COMMIT_MSG"

# Update result file with commit info using Go CLI
"$TASKER_BIN" -p "$PLANNING_DIR" bundle update-git "$TASK_ID" --sha="$COMMIT_SHA" --msg="$COMMIT_MSG" 2>/dev/null || true

[[ $# -ge 3 ]] && echo "OK: Updated $RESULT_FILE with commit info"
exit 0
