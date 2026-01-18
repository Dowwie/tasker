# T003: Phase management and state recovery

## Summary

Implements phase management operations (GetStatus, AdvancePhase, ValidateArtifact) and state recovery functionality (RecoverState) for the tasker CLI. Phase management enables tracking workflow progress and transitioning between decomposition phases. State recovery provides automatic recovery from corrupted state files without data loss.

## Components

- `go/internal/state/phase.go` - Phase management functions: GetStatus, AdvancePhase, ValidateArtifact
- `go/internal/state/recovery.go` - State recovery: RecoverState with partial data preservation
- `go/internal/state/phase_test.go` - Unit tests for phase management operations
- `go/internal/state/recovery_test.go` - Unit tests for state recovery

## API / Interface

### Phase Management

```go
type StatusSummary struct {
    Phase         string         `json:"phase"`
    TotalTasks    int            `json:"total_tasks"`
    ByStatus      map[string]int `json:"by_status"`
    ActiveTasks   []string       `json:"active_tasks"`
    ReadyTasks    []string       `json:"ready_tasks"`
    FailedTasks   []string       `json:"failed_tasks"`
    CurrentPhase  int            `json:"current_phase"`
    PhaseProgress string         `json:"phase_progress"`
}

func GetStatus(state *State) StatusSummary
func AdvancePhase(sm *StateManager) (string, error)
func ValidateArtifact(planningDir, artifactPath, schemaPath string) (*ArtifactValidation, error)
```

### State Recovery

```go
type RecoveryResult struct {
    Recovered      bool     `json:"recovered"`
    TasksRecovered int      `json:"tasks_recovered"`
    DataLost       []string `json:"data_lost,omitempty"`
    BackupPath     string   `json:"backup_path,omitempty"`
    Error          string   `json:"error,omitempty"`
}

func RecoverState(planningDir string) (*State, *RecoveryResult, error)
```

## Phase Order

The workflow progresses through these phases in order:

1. `ingestion` - Initial specification intake
2. `spec_review` - Specification review and clarification
3. `logical` - Logical capability mapping
4. `physical` - Physical file mapping
5. `definition` - Task definition
6. `validation` - Task validation
7. `sequencing` - Dependency sequencing
8. `ready` - Ready for execution
9. `executing` - Task execution in progress
10. `complete` - All tasks finished

## Recovery Behavior

RecoverState handles corrupted state files by:

1. Creating a backup of the corrupted file (`.corrupted.<timestamp>`)
2. Attempting partial JSON parsing to recover valid fields
3. Preserving task data where possible
4. Loading task definitions from `tasks/` directory as fallback
5. Recalculating execution statistics from recovered task states
6. Adding a `state_recovered` event to the audit log
7. Tracking data loss for reporting

## Dependencies

- T001: Core state types (State, Task, StateManager)
- T002: Task lifecycle (LoadTasks, GetReadyTasks)

## Testing

```bash
cd go && go test ./internal/state/... -run "TestGetStatus|TestAdvancePhase|TestValidateArtifact|TestRecoverState" -v
```

Tests verify:
- GetStatus returns accurate phase and task status summaries
- AdvancePhase transitions through phases correctly
- AdvancePhase blocks transition from executing when tasks incomplete
- ValidateArtifact validates JSON files against schemas
- RecoverState preserves valid data from corrupted files
- RecoverState creates backups before modification
- RecoverState tracks data loss for transparency
