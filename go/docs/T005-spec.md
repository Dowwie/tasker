# T005: Metrics and logging operations

## Summary
Implements metrics and logging operations for the state management system, including token usage tracking, performance metrics computation, planning quality metrics, and failure breakdown analysis.

## Components
- `internal/state/metrics.go` - Metrics and logging operations
- `internal/state/metrics_test.go` - Unit tests for metrics operations

## API / Interface

### Types

```go
type TokenEntry struct {
    Timestamp    string  `json:"timestamp"`
    TaskID       string  `json:"task_id,omitempty"`
    InputTokens  int     `json:"input_tokens"`
    OutputTokens int     `json:"output_tokens"`
    TotalTokens  int     `json:"total_tokens"`
    CostUSD      float64 `json:"cost_usd,omitempty"`
    Model        string  `json:"model,omitempty"`
}

type PerformanceMetrics struct {
    TotalTasks        int     `json:"total_tasks"`
    CompletedTasks    int     `json:"completed_tasks"`
    FailedTasks       int     `json:"failed_tasks"`
    SkippedTasks      int     `json:"skipped_tasks"`
    PendingTasks      int     `json:"pending_tasks"`
    RunningTasks      int     `json:"running_tasks"`
    CompletionRate    float64 `json:"completion_rate"`
    SuccessRate       float64 `json:"success_rate"`
    AvgDurationSecs   float64 `json:"avg_duration_seconds"`
    TotalDurationSecs float64 `json:"total_duration_seconds"`
    TotalTokens       int     `json:"total_tokens"`
    TotalCostUSD      float64 `json:"total_cost_usd"`
}

type PlanningMetrics struct {
    TotalPhases        int             `json:"total_phases"`
    CurrentPhase       int             `json:"current_phase"`
    TasksPerPhase      map[int]int     `json:"tasks_per_phase"`
    CompletedPerPhase  map[int]int     `json:"completed_per_phase"`
    PhaseProgress      map[int]float64 `json:"phase_progress"`
    OverallProgress    float64         `json:"overall_progress"`
    EstimatedRemaining int             `json:"estimated_remaining_tasks"`
    BlockedTasks       int             `json:"blocked_tasks"`
    ReadyTasks         int             `json:"ready_tasks"`
}

type FailureBreakdown struct {
    ByCategory        map[string]int   `json:"by_category"`
    ByRetryable       map[bool]int     `json:"by_retryable"`
    TotalFailures     int              `json:"total_failures"`
    RetryableCount    int              `json:"retryable_count"`
    NonRetryableCount int              `json:"non_retryable_count"`
    FailedTasks       []FailedTaskInfo `json:"failed_tasks,omitempty"`
}
```

### Functions

```go
func LogTokens(sm *StateManager, taskID string, inputTokens, outputTokens int, costUSD float64, model string) error
func GetMetrics(state *State) *PerformanceMetrics
func GetPlanningMetrics(state *State) *PlanningMetrics
func GetFailureMetrics(state *State) *FailureBreakdown
```

### StateManager Methods

```go
func (sm *StateManager) LogTokens(taskID string, inputTokens, outputTokens int, costUSD float64, model string) error
func (sm *StateManager) GetMetrics() (*PerformanceMetrics, error)
func (sm *StateManager) GetPlanningMetrics() (*PlanningMetrics, error)
func (sm *StateManager) GetFailureMetrics() (*FailureBreakdown, error)
```

## Behaviors Implemented

- **B20 LogTokens**: Records token usage with timestamp and task context, updates cumulative totals in Execution, creates tokens_logged event
- **B21 GetMetrics**: Computes performance metrics including completion rate, success rate, average duration, and token usage
- **B22 GetPlanningMetrics**: Computes planning quality metrics including phase progress, ready/blocked task counts
- **B23 GetFailureMetrics**: Shows failure breakdown by category and retryability

## Testing

```bash
cd go && go test ./internal/state/... -run TestLogTokens -v
cd go && go test ./internal/state/... -run TestGetMetrics -v
cd go && go test ./internal/state/... -run TestGetPlanningMetrics -v
cd go && go test ./internal/state/... -run TestGetFailureMetrics -v
```

## Dependencies
- T001: Core state management (State, Task, Execution types)
- T002: Task lifecycle operations (TaskFailure type)
