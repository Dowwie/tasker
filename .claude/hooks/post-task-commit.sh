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

# Determine mode based on arguments
if [[ $# -ge 3 ]]; then
    # Manual mode: args provided
    TASK_ID="$1"
    TARGET_DIR="$2"
    PLANNING_DIR="$3"
elif [[ $# -eq 0 ]]; then
    # Hook mode: parse from stdin JSON
    INPUT=$(cat)

    # Extract output from hook JSON
    OUTPUT=$(echo "$INPUT" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    out = d.get('output', d.get('tool_output', ''))
    if isinstance(out, dict):
        out = out.get('output', '')
    print(out)
except:
    print('')
" 2>/dev/null || echo "")

    # Parse task ID from output (e.g., "T001: SUCCESS")
    TASK_ID=$(echo "$OUTPUT" | grep -oE 'T[0-9]+: SUCCESS' | head -1 | cut -d: -f1)

    if [[ -z "$TASK_ID" ]]; then
        # Not a successful task-executor output, skip silently
        exit 0
    fi

    # Get paths from state.json
    STATE_FILE="./project-planning/state.json"
    if [[ ! -f "$STATE_FILE" ]]; then
        exit 0
    fi

    TARGET_DIR=$(python3 -c "import json; print(json.load(open('$STATE_FILE')).get('target_dir', ''))" 2>/dev/null || echo "")
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

# Extract task name and status from result JSON
TASK_NAME=$(python3 -c "import json; print(json.load(open('$RESULT_FILE')).get('name', '$TASK_ID'))" 2>/dev/null || echo "$TASK_ID")
STATUS=$(python3 -c "import json; print(json.load(open('$RESULT_FILE')).get('status', 'unknown'))" 2>/dev/null || echo "unknown")

if [[ "$STATUS" != "success" ]]; then
    [[ $# -ge 3 ]] && echo "SKIP: Task $TASK_ID status is '$STATUS', not committing"
    exit 0
fi

# Get files from result (created + modified)
FILES_JSON=$(python3 -c "
import json
result = json.load(open('$RESULT_FILE'))
files = result.get('files', {})
created = files.get('created', []) or []
modified = files.get('modified', []) or []
all_files = [f for f in created + modified if f]
print('\n'.join(all_files))
" 2>/dev/null || echo "")

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

# Update result file with commit info
python3 -c "
import json
with open('$RESULT_FILE', 'r') as f:
    result = json.load(f)
result['git'] = {
    'committed': True,
    'commit_sha': '$COMMIT_SHA',
    'commit_message': '$COMMIT_MSG',
    'committed_by': 'hook'
}
with open('$RESULT_FILE', 'w') as f:
    json.dump(result, f, indent=2)
" 2>/dev/null || true

[[ $# -ge 3 ]] && echo "OK: Updated $RESULT_FILE with commit info"
exit 0
