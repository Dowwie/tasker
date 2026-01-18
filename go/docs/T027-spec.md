# T027: Shim scripts for backward compatibility

## Summary

This task implements shim logic to allow existing Python scripts (`state.py`, `validate.py`, `bundle.py`) to forward calls to the Go binary when available, enabling a smooth migration path and backward compatibility.

## Components

- `go/internal/shim/shim.go` - Core shim logic including argument translation and binary execution
- `go/internal/shim/shim_test.go` - Comprehensive unit tests for shim functionality
- `scripts/state.py` - Modified to forward supported commands to Go binary
- `scripts/validate.py` - Modified to forward supported commands to Go binary
- `scripts/bundle.py` - Modified to forward supported commands to Go binary

## API / Interface

### Go Shim Package

```go
// TranslateArgs converts Python script arguments to Go subcommand format
func TranslateArgs(scriptName string, args []string, mappings map[string]ScriptMapping) (*TranslationResult, error)

// FindBinary locates the tasker Go binary
func FindBinary() (string, error)

// ExecBinary executes the Go binary with translated arguments
func ExecBinary(binaryPath string, result *TranslationResult) ([]byte, error)

// HandleError formats an error message for when the binary is not found
func HandleError(err error) string

// ShouldUseGoBinary determines if the Go binary should be used
func ShouldUseGoBinary() bool
```

### Python Script Behavior

Each modified Python script:
1. Checks if `USE_PYTHON_IMPL=1` environment variable is set (skip shim if true)
2. Checks if the command is supported by the Go binary
3. Locates the Go binary via `TASKER_BINARY` env var, relative paths, or PATH
4. Translates arguments to Go subcommand format
5. Executes the Go binary and exits with its return code
6. Falls back to Python implementation if binary not found or command not supported

### Supported Commands by Script

| Script | Go-supported Commands |
|--------|----------------------|
| state.py | init, status, start-task, complete-task, fail-task, retry-task, skip-task, ready-tasks |
| validate.py | dag, steel-thread, gates |
| bundle.py | generate, generate-ready, validate, validate-integrity, list, clean |

## Environment Variables

- `TASKER_BINARY` - Override path to Go binary
- `USE_PYTHON_IMPL=1` - Force Python implementation (skip shim)

## Testing

```bash
# Run Go shim unit tests
cd go && go test ./internal/shim/... -v

# Verify shim forwards correctly
python3 scripts/state.py status
tasker state status
```

## Binary Search Order

1. `TASKER_BINARY` environment variable
2. `go/bin/tasker` relative to script location
3. `bin/tasker` relative to script location
4. System PATH
