# T018: Evaluation Utilities

## Summary

This task implements the evaluation utilities for the Go tasker port. These utilities port the functionality from the Python `evaluate.py` script, providing metrics computation from execution state.

## Components

- `go/internal/util/evaluate.go` - RunEvaluation for evaluation operations, computes performance metrics from task execution state
- `go/internal/util/evaluate_test.go` - Unit tests for evaluation functionality

## API / Interface

### Evaluation

```go
// RunEvaluation runs a full evaluation on the state file in the planning directory
// and returns the evaluation result with computed metrics.
func RunEvaluation(planningDir string) (*EvaluationResult, error)

// EvaluationResult contains the full evaluation report data.
type EvaluationResult struct {
    PlanningVerdict      string
    PlanningIssuesCount  int
    Metrics              *EvaluationMetrics
    FailedTasks          []FailedTask
    ImprovementPatterns  []ImprovementPattern
    TotalTasks           int
    BlockedCount         int
    SkippedCount         int
}

// EvaluationMetrics contains computed performance metrics.
type EvaluationMetrics struct {
    TaskSuccessRate         float64
    FirstAttemptSuccessRate float64
    AvgAttempts             float64
    TokensPerTask           int
    CostPerTask             float64
    QualityPassRate         float64
    FunctionalPassRate      float64
    TestEdgeCaseRate        float64
    // ... additional fields for counts and breakdowns
}

// FormatEvaluationReport formats an EvaluationResult as a human-readable string.
func FormatEvaluationReport(result *EvaluationResult) string
```

## Dependencies

- Standard library only (no external dependencies beyond existing go.mod)

## Testing

```bash
# Run evaluation tests
cd go && go test ./internal/util/... -run TestRunEvaluation -v

# Run all util tests
cd go && go test ./internal/util/... -v
```
