# T001: Go module setup and core state types

## Summary

Establishes the Go module foundation for the tasker CLI, implementing core state management types that match the Python scripts/state.py JSON schema. Provides file-locked read/write operations for concurrent-safe state manipulation.

## Components

- `go/go.mod` - Go module definition with cobra dependency
- `go/cmd/tasker/main.go` - CLI entry point with cobra root command
- `go/internal/state/state.go` - State types and LoadState/SaveState/ValidateState functions
- `go/internal/state/lock.go` - File locking using flock for atomic operations
- `go/internal/state/state_test.go` - Unit tests for all state operations

## API / Interface

```go
// Core state loading/saving
func LoadState(path string) (*State, error)
func SaveState(path string, state *State) error
func ValidateState(state *State) []error

// StateManager provides a higher-level interface
type StateManager struct { ... }
func NewStateManager(planningDir string) *StateManager
func (sm *StateManager) Load() (*State, error)
func (sm *StateManager) Save(state *State) error
func (sm *StateManager) Validate(state *State) []error
func (sm *StateManager) Path() string

// File locking for concurrent access
func AcquireReadLock(path string) (*FileLock, error)
func AcquireWriteLock(path string) (*FileLock, error)
func (fl *FileLock) Release() error
```

## State Types

The `State` struct mirrors the state.schema.json:
- `Version` - must be "2.0"
- `Phase` - current phase and completed phases
- `TargetDir` - project target directory
- `Tasks` - map of task ID to Task struct
- `Artifacts` - capability map, physical map, validation results
- `Execution` - runtime state (current phase, active tasks, counts)
- `Events` - audit log of state changes

## Dependencies

- `github.com/spf13/cobra` - CLI framework

## Testing

```bash
go test ./go/internal/state/... -v
```

All tests verify:
- LoadState reads and parses state.json correctly
- SaveState writes atomically with file locking
- ValidateState enforces schema constraints
- File locking prevents concurrent write corruption
