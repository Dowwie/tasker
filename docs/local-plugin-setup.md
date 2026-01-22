# Local Plugin Development Setup

Guide for setting up Claude Code plugins from local directory sources with working skills.

## Problem

Local directory marketplace plugins have different path resolution than git-sourced plugins. The skill system has two separate mechanisms that must both be satisfied:

1. **Skill Discovery** (startup) - Uses plugin cache, populates available skills list
2. **Skill Content Loading** (invocation) - Looks at `~/.claude/skills/{plugin}/{skill}.md`

Without proper setup, skills appear in the available list but fail to load when invoked.

## Required Setup

### 1. Register the Marketplace

Add to `~/.claude/plugins/known_marketplaces.json`:

```json
{
  "your-marketplace": {
    "source": {
      "source": "directory",
      "path": "/path/to/your/plugin"
    },
    "installLocation": "/path/to/your/plugin",
    "lastUpdated": "2026-01-22T00:00:00.000Z"
  }
}
```

### 2. Symlink Marketplace to Marketplaces Directory

Git-sourced marketplaces are cloned to `~/.claude/plugins/marketplaces/`. Local directory sources must be symlinked there:

```bash
ln -s /path/to/your/plugin ~/.claude/plugins/marketplaces/{marketplace-name}
```

### 3. Plugin Structure

```
your-plugin/
├── .claude-plugin/
│   ├── marketplace.json
│   └── plugin.json
└── skills/
    └── {skill-name}/
        └── SKILL.md
```

### 4. marketplace.json

**Do NOT use explicit `skills` array** - auto-discovery from `skills/` directory works; explicit paths fail.

```json
{
  "name": "your-marketplace",
  "owner": { "name": "Your Name" },
  "plugins": [
    {
      "name": "your-plugin",
      "source": "./",
      "description": "Plugin description"
    }
  ]
}
```

**Wrong:**
```json
{
  "plugins": [
    {
      "name": "your-plugin",
      "skills": ["./skills/my-skill"]  // DO NOT DO THIS
    }
  ]
}
```

### 5. SKILL.md Frontmatter

Only use recognized fields. The `tools:` field is NOT recognized and causes skill loading to fail.

**Correct:**
```yaml
---
name: skill-name
description: What the skill does
---
```

**Wrong:**
```yaml
---
name: skill-name
description: What the skill does
tools:           # NOT RECOGNIZED - causes load failure
  - SomeTool
---
```

### 6. Create Skill Content Symlinks

The skill content loader looks at `~/.claude/skills/{plugin}/{skill}.md`, not the plugin cache. Create symlinks:

```bash
mkdir -p ~/.claude/skills/{plugin-name}
ln -s /path/to/plugin/skills/{skill-name}/SKILL.md \
      ~/.claude/skills/{plugin-name}/{skill-name}.md
```

**Note:** The symlink target is `SKILL.md` but the link name is `{skill-name}.md`.

## Complete Example

For a plugin called `tasker` with a skill called `specify`:

```bash
# 1. Symlink marketplace
ln -s /Users/me/projects/tasker ~/.claude/plugins/marketplaces/tasker-marketplace

# 2. Create skill content symlink
mkdir -p ~/.claude/skills/tasker
ln -s /Users/me/projects/tasker/skills/specify/SKILL.md \
      ~/.claude/skills/tasker/specify.md
```

## Verification

After setup, start a new Claude Code session and:

1. Check skill appears in `/help` or system prompt's available skills
2. Invoke with `/{plugin}:{skill}` (e.g., `/tasker:specify`)
3. Skill should load without errors or manual file searching

## Debugging

Check debug logs for skill loading:

```bash
grep -E "(Loading|Loaded).*skills" ~/.claude/debug/*.txt | tail -20
```

**Success indicators:**
- `Loaded N skills from plugin {name} default directory`

**Failure indicators:**
- `Loaded 0 skills from plugin {name} custom path`
- Agent manually searching for skill files after invocation

## Root Causes

| Symptom | Cause |
|---------|-------|
| "Loaded 0 skills from custom path" | Explicit `skills` array in marketplace.json |
| Skill not found at invocation | Missing `~/.claude/skills/{plugin}/{skill}.md` symlink |
| Skill discovery fails | Missing marketplace symlink in `~/.claude/plugins/marketplaces/` |
| Skill load fails silently | Unrecognized field in SKILL.md frontmatter (e.g., `tools:`) |

## Hook Errors

If you see "SessionStart:startup hook error" or "UserPromptSubmit hook error":

### Common Causes

1. **Script not executable**
   ```bash
   chmod +x scripts/*.sh hooks/*.sh
   ```

2. **Script fails silently** - Check what the hook script does:
   ```bash
   # Test the hook manually
   CLAUDE_PLUGIN_ROOT=/path/to/plugin ./scripts/your-hook.sh
   ```

3. **Missing dependencies** - Hook scripts may depend on external binaries or network resources

### For tasker specifically

The `ensure-tasker.sh` script downloads a binary from GitHub releases. If releases aren't published or network fails:

```bash
# Check if binary exists
ls -la ~/.local/bin/tasker/tasker

# If missing and releases not published, create a placeholder or disable the hook
```

To disable hooks during development, comment them out in `hooks/hooks.json`:

```json
{
  "hooks": {
    "SessionStart": []
  }
}
```
