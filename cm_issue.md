# Issue: logic-architect Agent Fails to Write capability-map.json

## Problem Summary

The `logic-architect` sub-agent performs its analysis correctly and produces valid JSON output, but **fails to invoke the Write tool** to save the file to disk. The orchestrator is then forced to manually extract the JSON content from the agent's conversation output and write it to file directly.

## Observed Behavior

1. Orchestrator spawns `logic-architect` with proper context including `PLANNING_DIR` absolute path
2. Agent performs capability extraction from spec **correctly**
3. Agent produces valid JSON output **in the conversation**
4. Agent does NOT call the Write tool to save the file
5. Agent reports completion with "SUCCESS" message
6. **File does not exist at `{PLANNING_DIR}/artifacts/capability-map.json`**
7. Orchestrator must manually extract content and write:
   > "The agent didn't write the file. I have the content from the agent's output - let me write it directly and proceed"

## Evidence from Activity Log

```
[2025-12-02T08:24:02-05:00] [INFO] [orchestrator] spawn: launching logic-architect for capability extraction
[2025-12-02T08:27:23-05:00] [INFO] [orchestrator] spawn-complete: logic-architect finished with SUCCESS
[2025-12-02T08:27:28-05:00] [WARN] [orchestrator] validation: capability_map - FILE NOT FOUND - respawning agent
[2025-12-02T08:29:03-05:00] [INFO] [orchestrator] validation: capability_map - PASSED
```

The agent required re-spawning before the file was actually written (or the orchestrator had to write it manually).

## Root Cause Analysis

**Primary Cause**: Agent outputs JSON to the conversation instead of invoking the Write tool.

The agent correctly:
- Reads the spec
- Extracts capabilities
- Produces valid JSON

But fails to:
- Call the Write tool with the content
- Verify the file was written

**Why This Happens**:
1. Agent treats "showing the JSON" as equivalent to "writing the file"
2. Instructions to use Write tool are buried in multi-step checklist
3. Agent may run out of context or terminate before reaching the Write step
4. No enforcement mechanism - agent can "complete" without file existing

## Current Mitigations in Agent Prompt

The `.claude/agents/logic-architect.md` already includes:

```markdown
**CRITICAL - YOUR TASK IS NOT COMPLETE UNTIL YOU DO ALL OF THESE:
1. You MUST use the Write tool to save the file. Do NOT just output JSON to the conversation.
2. You MUST use the PLANNING_DIR absolute path provided in the spawn context. Do NOT use relative paths like `project-planning/`.
3. You MUST verify the file exists after writing by running: `ls -la {PLANNING_DIR}/artifacts/capability-map.json`
4. You MUST run validation: `cd {PLANNING_DIR}/.. && python3 scripts/state.py validate capability_map`
```

## Current Mitigations in Orchestrator

The orchestrator skill (`.claude/skills/orchestrator/SKILL.md`) includes:

1. Output verification step before validation
2. Recovery procedure with explicit re-spawn instructions
3. Warning to never log `spawn-complete: SUCCESS` until file verified

## Files Involved

- `.claude/agents/logic-architect.md` - Sub-agent instructions
- `.claude/skills/orchestrator/SKILL.md` - Orchestrator workflow
- `schemas/capability-map.schema.json` - Schema for validation
- `scripts/state.py` - State management and validation
- `project-planning/artifacts/capability-map.json` - Target output file

## Potential Solutions to Try

### 1. Restructure Agent Prompt - "Write First" Pattern
Move the Write tool instruction to the very beginning of the agent's task list, not as a final step.

**Current pattern**:
```
1. Read spec
2. Extract capabilities
3. Build JSON
4. Write file (often skipped)
5. Verify
```

**Proposed pattern**:
```
YOUR FIRST ACTION must be to set up a placeholder file:
1. Write a minimal valid JSON to {file} immediately
2. Read spec
3. Extract capabilities
4. Build JSON incrementally, OVERWRITING the file after each section
5. Verify final file
```

### 2. Explicit Tool Call Requirement in Prompt
Add a more forceful instruction at the top:

```
## MANDATORY TOOL USAGE
You MUST call the Write tool. Outputting JSON to this conversation does NOT write it to disk.
The Write tool is how files get created. No Write call = No file = FAILURE.
```

### 3. Two-Phase Approach
Split the agent into two calls:
- Phase 1: Return JSON content (no file write expected)
- Phase 2: Orchestrator writes the file

This accepts the reality that agents often fail to write files.

### 4. Add File Existence Check to Agent Definition
Modify the agent type definition to require file existence as part of completion criteria.

### 5. Use Bash for File Writing
Instead of Write tool, have agent use:
```bash
cat > {file} << 'EOF'
{json content}
EOF
```

Bash commands may be more reliably executed.

---

# Activity Log

Format: `[DATE] [ATTEMPT #] - Description of what was tried and result`

---

## 2025-12-02: Issue Documented

- **Action**: Created this issue document to track the persistent bug
- **Symptom**: Agent produces correct JSON but outputs to conversation instead of calling Write tool
- **Evidence**: Orchestrator says "The agent didn't write the file. I have the content from the agent's output - let me write it directly"
- **Status**: Issue persists despite existing prompt mitigations
- **Next Steps**: Try one of the proposed solutions above

---

## 2025-12-02: Applied "Write First" Pattern Fix

- **Attempt #**: 1
- **Change Made**: Restructured agent prompt to enforce "Write First" pattern
- **File(s) Changed**: `.claude/agents/logic-architect.md`
- **Result**: PENDING VERIFICATION

### Root Cause Analysis

The issue stems from **prompt structure**, not tool availability. The agent has access to the Write tool, but the instructions were structured as:

1. Read spec → 2. Analyze (200+ lines of detailed instructions) → 3. Build JSON → 4. Write file (often skipped)

By the time the agent completes the intellectually demanding analysis work (I.P.S.O. decomposition, phase filtering, capability extraction), it has:
- Exhausted attention on the analysis
- Conflated "showing JSON in conversation" with "writing JSON to file"
- Reached a natural stopping point before the Write step

### Changes Applied

1. **Added "MANDATORY FIRST ACTION" section** at the very top of the prompt
   - Agent must write a placeholder file BEFORE any analysis
   - Includes verification step (`ls -la`) before proceeding
   - Explains WHY: "Outputting JSON to conversation does NOT create a file"

2. **Restructured workflow** to emphasize Write-first:
   - Step 1: WRITE PLACEHOLDER
   - Step 2: READ spec
   - Step 3: ANALYZE
   - Step 4: OVERWRITE with complete analysis
   - Step 5: VALIDATE

3. **Reorganized final checklist** to prioritize file existence:
   - File Existence checks come FIRST
   - Explicit reminder: "If `ls` shows 'No such file', use Write tool NOW"

### Why This Should Work

The "Write First" pattern creates a **behavioral anchor**:
- Agent's first action is Write, establishing the tool usage pattern
- File exists before analysis begins, eliminating "did I write it?" ambiguity
- Verification step confirms success before proceeding
- Final Write is an OVERWRITE, not a new action to remember

### Next Steps

- Test with a new `/plan` session
- Monitor activity log for:
  - Placeholder file creation at start
  - Final validation passing without respawn
- If issue persists, consider Solution #3 (Two-Phase Approach) where orchestrator handles all file writing

---

## 2025-12-02: ROOT CAUSE FOUND - Wrong Tool Names in Frontmatter

- **Attempt #**: 2
- **Result**: ✅ **ROOT CAUSE IDENTIFIED AND FIXED**

### The Real Problem

The agent prompt changes (Attempt #1) were **irrelevant** because the agent had **NO ACCESS TO ANY TOOLS**.

Evidence from spawn log:
```
⏺ logic-architect(Extract capabilities from spec)
  ⎿  Done (0 tool uses · 11.6k tokens · 1m 10s)
```

**0 tool uses** - the agent could not use Read, Write, or Bash because it had no tools available.

### Root Cause

The agent frontmatter used **incorrect tool names**:

```yaml
# WRONG (what we had):
tools:
  - bash
  - file_read
  - file_write

# CORRECT (Claude Code expects):
tools: Read, Write, Bash, Glob, Grep
```

Claude Code requires:
1. **PascalCase** tool names: `Read`, `Write`, `Bash` (not `file_read`, `file_write`, `bash`)
2. **Comma-separated list** format (not YAML array)

When tool names don't match, Claude Code silently assigns **zero tools** to the agent, which is why the system prompt showed `(Tools: )` for logic-architect.

### Files Fixed

All 6 agent files updated:
- `.claude/agents/logic-architect.md` → `tools: Read, Write, Bash, Glob, Grep`
- `.claude/agents/physical-architect.md` → `tools: Read, Write, Bash, Glob, Grep`
- `.claude/agents/task-author.md` → `tools: Read, Write, Bash, Glob, Grep`
- `.claude/agents/plan-auditor.md` → `tools: Read, Write, Bash, Glob, Grep`
- `.claude/agents/task-executor.md` → `tools: Read, Write, Edit, Bash, Glob, Grep`
- `.claude/agents/task-verifier.md` → `tools: Read, Bash, Glob, Grep`
- `.claude/agents/task-plan-verifier.md` → `tools: Read, Bash, Glob, Grep`

### Why Previous Fixes Failed

All previous attempts focused on **prompt engineering** to make the agent "remember" to write files. But the agent literally **could not** write files because it had no Write tool. No amount of prompt restructuring could fix a tool access problem.

### Verification

After fix, agents should show tools in system prompt:
```
- logic-architect: ... (Tools: Read, Write, Bash, Glob, Grep)
```

Instead of:
```
- logic-architect: ... (Tools: )
```

### Status

**FIXED** - Pending verification with next `/plan` session.

