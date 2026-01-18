# T021: TUI Application Core and Keyboard Handling

## Summary

Implemented the interactive terminal user interface (TUI) for tasker using the bubbletea framework. The TUI provides keyboard-driven navigation through tasks and displays current state information.

## Components

- `go/internal/tui/app.go` - Core TUI application using bubbletea Model-View-Update pattern
- `go/internal/tui/app_test.go` - Unit tests for TUI functionality
- `go/internal/command/tui/tui.go` - Cobra subcommand to launch the TUI

## API / Interface

```go
// StateProvider interface for testability
type StateProvider interface {
    Load() (*state.State, error)
}

// Model represents the TUI application state
type Model struct {
    planningDir   string
    stateProvider StateProvider
    state         *state.State
    tasks         []state.Task
    cursor        int
    err           error
    width         int
    height        int
    quitting      bool
}

// NewModel creates a new TUI model for the given planning directory
func NewModel(planningDir string) Model

// FetchState retrieves the current state for display (behavior B66)
func FetchState(provider StateProvider) (*state.State, error)

// Run launches the TUI application
func Run(planningDir string) error
```

## Behaviors Implemented

### B65: HandleKeyboard
Processes keyboard input for navigation:
- `j` / `down` - Move cursor down
- `k` / `up` - Move cursor up
- `enter` - Select task (placeholder for future detail view)
- `q` / `Ctrl+C` - Quit application
- `r` - Refresh state

### B66: FetchState
Retrieves the current state from the state provider for display in the TUI.

## Dependencies

- github.com/charmbracelet/bubbletea - TUI framework
- github.com/charmbracelet/lipgloss - Styling for terminal output
- github.com/dgordon/tasker/internal/state - State management

## Testing

```bash
# Run all TUI tests
go test ./internal/tui/... -v

# Run specific acceptance tests
go test ./internal/tui/... -run TestAppStart -v
go test ./internal/tui/... -run TestHandleKeyboard -v
go test ./internal/tui/... -run TestFetchState -v
```

## Usage

```bash
# Launch the TUI
tasker tui

# With custom planning directory
tasker -p /path/to/planning tui
```
