# Execute

Execute tasks from a completed plan using isolated subagents.

**IMPORTANT:** Follow the instructions in the `tasker:execute` skill directly. Do NOT invoke any other skill.

The execute workflow:
1. Asks for the target project directory
2. Verifies planning is complete
3. Runs task-executor subagents for each ready task
4. Handles checkpointing and crash recovery
