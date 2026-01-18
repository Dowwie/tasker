# T004: Halt control operations

## Summary

Implements graceful halt control for the tasker execution engine. Allows external signals to pause task execution, check halt status, and resume when ready. The halt state persists in state.json and includes audit events for traceability.

## Components

- `go/internal/state/halt.go` - Halt control operations (RequestHalt, CheckHalt, ResumeExecution, GetHaltStatus)
- `go/internal/state/halt_test.go` - Comprehensive unit tests for halt functionality

## API / Interface

```go
type HaltInfo struct {
    Requested   bool   `json:"requested"`
    Reason      string `json:"reason,omitempty"`
    RequestedAt string `json:"requested_at,omitempty"`
    RequestedBy string `json:"requested_by,omitempty"`
}

type HaltStatus struct {
    Halted      bool   `json:"halted"`
    Reason      string `json:"reason,omitempty"`
    RequestedAt string `json:"requested_at,omitempty"`
    RequestedBy string `json:"requested_by,omitempty"`
}

func RequestHalt(path string, reason string, requestedBy string) error
func CheckHalt(path string) (bool, error)
func ResumeExecution(path string) error
func GetHaltStatus(path string) (*HaltStatus, error)

func (sm *StateManager) RequestHalt(reason string, requestedBy string) error
func (sm *StateManager) CheckHalt() (bool, error)
func (sm *StateManager) ResumeExecution() error
func (sm *StateManager) GetHaltStatus() (*HaltStatus, error)
```

## Behaviors Implemented

| ID | Name | Type | Description |
|----|------|------|-------------|
| B16 | RequestHalt | state | Sets halt flag with reason and timestamp in state file |
| B17 | CheckHalt | process | Returns true if halt has been requested |
| B18 | ResumeExecution | state | Clears halt flag and allows task execution |
| B19 | GetHaltStatus | output | Returns current halt status with details |

## State Changes

The `State` struct now includes:
```go
Halt *HaltInfo `json:"halt,omitempty"`
```

Events are recorded for:
- `halt_requested` - When halt is requested (includes reason, requested_by)
- `execution_resumed` - When execution resumes (includes previous_reason)

## Dependencies

- T001 (Go module setup and core state types)

## Testing

```bash
go test ./go/internal/state/... -run TestRequestHalt -v
go test ./go/internal/state/... -run TestCheckHalt -v
go test ./go/internal/state/... -run TestResumeExecution -v
go test ./go/internal/state/... -run TestGetHaltStatus -v
```

## Usage Example

```go
sm := NewStateManager("/path/to/planning")

// Request halt
err := sm.RequestHalt("User requested pause", "orchestrator")

// Check if halted before starting task
halted, err := sm.CheckHalt()
if halted {
    status, _ := sm.GetHaltStatus()
    fmt.Printf("Halted: %s (by %s)\n", status.Reason, status.RequestedBy)
}

// Resume after review
err = sm.ResumeExecution()
```
