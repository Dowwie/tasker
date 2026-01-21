# Tasker to Beads

Transform Tasker task definitions into rich, self-contained Beads issues.

Use the `tasker-to-beads` skill: `Skill("tasker-to-beads")`

## What This Does

1. **Asks for target directory** — Where development will happen (may differ from planning project)
2. **Initializes beads** in the target directory if not already done
3. Extracts structural data from Tasker task files
4. Uses LLM comprehension to enrich with spec context, architecture narrative, and human-readable descriptions
5. Creates Beads issues with proper labels, priorities, and dependencies in the target project

## Interactive Workflow

When invoked, this command will:

1. **First**, ask: "Where would you like to create the Beads issues? This should be the directory where the actual development will take place."
   - Options: Current directory, specify path, or common locations
2. **Check** if beads is initialized in that directory
3. **Initialize beads** if needed (with option to customize issue prefix)
4. **Process tasks** according to mode selected

## Usage

- `/tasker-to-beads` — Interactive mode, asks for target directory and processes all tasks
- `/tasker-to-beads T001` — Process single task by ID (still asks for target)
- `/tasker-to-beads --batch` — Process all tasks and create manifest for batch import
- `/tasker-to-beads --target /path/to/project` — Skip target prompt, use specified directory

## CLI Commands

The tasker binary includes transform commands for mechanical transformations:

```bash
# Check status of source and target
tasker transform status -t /path/to/target

# Initialize beads in target with custom prefix
# Runs: bd init <PREFIX> && bd onboard
tasker transform init-target /path/to/target FATHOM

# Prepare context for all tasks
tasker transform context --all

# Create issue in target directory
tasker transform create T001 .tasker/beads-export/T001-enriched.json -t /path/to/target
```

## Example Session

```
User: /tasker-to-beads

Claude: I'll help you transform Tasker tasks into Beads issues.

First, where would you like to create the Beads issues?
- [ ] Current directory (/Users/dev/tasker)
- [ ] Specify a different path

User: /Users/dev/fathom

Claude: Checking /Users/dev/fathom...
  - Directory exists: Yes
  - Beads initialized: No

I'll initialize Beads with prefix "FATHOM". Continue? [Y/n]

...
```
