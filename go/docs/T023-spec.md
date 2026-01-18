# T023: TUI Task Detail View

## Summary

Implemented the task detail view for the TUI, providing comprehensive display of task information including name, status, dependencies, acceptance criteria with verification status, and files with action types.

## Components

- `go/internal/tui/task_detail.go` - RenderTaskDetail and supporting functions for task detail display
- `go/internal/tui/task_detail_test.go` - Unit tests for task detail view functionality

## API / Interface

```go
// AcceptanceCriterion represents a single acceptance criterion
type AcceptanceCriterion struct {
    Criterion    string `json:"criterion"`
    Verification string `json:"verification"`
}

// TaskFile represents a file associated with a task
type TaskFile struct {
    Path    string `json:"path"`
    Action  string `json:"action"`
    Purpose string `json:"purpose"`
}

// TaskDefinition represents parsed task definition JSON
type TaskDefinition struct {
    ID                 string                `json:"id"`
    Name               string                `json:"name"`
    AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
    Files              []TaskFile            `json:"files"`
}

// TaskDetail combines state Task with definition data
type TaskDetail struct {
    Task               state.Task
    AcceptanceCriteria []AcceptanceCriterion
    Files              []TaskFile
}

// LoadTaskDefinition reads and parses a task definition file
func LoadTaskDefinition(filePath string) (*TaskDefinition, error)

// BuildTaskDetail constructs a TaskDetail from a state Task
func BuildTaskDetail(task state.Task) TaskDetail

// RenderTaskDetail renders the task detail view (behavior B64)
func RenderTaskDetail(detail TaskDetail, width int) string
```

## Behaviors Implemented

### B64: RenderTaskDetail

Renders a detailed view of a single task showing:
- Task ID and name (header)
- Status with color coding
- Phase number
- Dependencies (DependsOn tasks)
- Blocked tasks (tasks this blocks)
- Acceptance criteria with verification status indicators
- Files with action type badges (create/modify/delete)
- Error message if present

## Features

- Verification status display: Shows PASS/FAIL/PARTIAL indicators based on task verification results
- Color-coded action types: create (green), modify (orange), delete (red)
- Status-aware styling: Different colors for complete, running, failed, pending, etc.
- Graceful handling: Works with partial data when task definition file is missing

## Dependencies

- github.com/charmbracelet/lipgloss - Styling for terminal output
- github.com/dgordon/tasker/internal/state - State management types

## Testing

```bash
# Run all task detail tests
go test ./internal/tui/... -run TaskDetail -v

# Run specific acceptance tests
go test ./internal/tui/... -run TestRenderTaskDetail -v
go test ./internal/tui/... -run TestTaskDetailCriteria -v
go test ./internal/tui/... -run TestTaskDetailFiles -v
```
