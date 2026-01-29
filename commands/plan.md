# Plan

Decompose a specification into an executable task DAG.

**IMPORTANT:** Follow the instructions in the `tasker:plan` skill directly. Do NOT invoke any other skill.

The plan workflow guides you through:
1. Asking for the target project directory
2. Detecting specs from `/specify` workflow
3. Running planning agents (physical-architect, task-author, task-plan-verifier, plan-auditor)
4. Producing a task DAG ready for `/execute`
