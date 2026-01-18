# T002: Task lifecycle operations

## Summary

Implements task lifecycle management operations for the tasker CLI, enabling decomposition initialization, task loading from files, and full task state transitions (start, complete, fail, retry, skip). This builds on T001's core state types to provide the behavioral layer for task execution workflows.

## Components

- `go/internal/state/task.go` - Task lifecycle functions: InitDecomposition, LoadTasks, GetReadyTasks, StartTask, CompleteTask, FailTask, RetryTask, SkipTask
- `go/internal/state/task_test.go` - Unit tests for all task lifecycle operations
- `go/internal/command/root.go` - Cobra root command definition with planning-dir flag
- `go/internal/command/state/state.go` - State subcommands (init, status, ready, task start/complete/fail/retry/skip)

## API / Interface

### State Library Functions

```go
func InitDecomposition(planningDir, targetDir string) (*State, error)
func LoadTasks(planningDir string) (map[string]Task, error)
func GetReadyTasks(state *State) []Task
func StartTask(sm *StateManager, taskID string) error
func CompleteTask(sm *StateManager, taskID string, filesCreated, filesModified []string) error
func FailTask(sm *StateManager, taskID, errorMsg, category string, retryable bool) error
func RetryTask(sm *StateManager, taskID string) error
func SkipTask(sm *StateManager, taskID, reason string) error
```

### CLI Commands

```bash
tasker state init <target-dir>           # Initialize decomposition
tasker state status                      # Show current state
tasker state ready                       # List ready tasks
tasker state task start <task-id>        # Start a task
tasker state task complete <task-id>     # Complete a task
tasker state task fail <task-id> <msg>   # Fail a task
tasker state task retry <task-id>        # Retry failed task
tasker state task skip <task-id>         # Skip a task
```

## State Transitions

Tasks follow this state machine:

- `pending` -> `running` (StartTask)
- `running` -> `complete` (CompleteTask)
- `running` -> `failed` (FailTask)
- `failed` -> `pending` (RetryTask, if retryable)
- `pending|blocked` -> `skipped` (SkipTask)

Skipped tasks do not block dependents, enabling dependency graph traversal to continue.

## Dependencies

- T001: Core state types (State, Task, StateManager)
- github.com/spf13/cobra - CLI framework

## Testing

```bash
cd go && go test ./internal/state/... -v
```

Tests verify:
- InitDecomposition creates valid state.json
- LoadTasks reads task files from tasks/ directory
- GetReadyTasks computes ready tasks based on dependency satisfaction
- StartTask marks pending tasks as running with timestamp
- CompleteTask marks running tasks as complete with file tracking
- FailTask marks running tasks as failed with classification
- RetryTask resets failed tasks to pending
- SkipTask marks tasks as skipped without blocking dependents
