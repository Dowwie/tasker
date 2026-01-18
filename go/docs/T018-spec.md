# T018: Evaluation and Compliance Utilities

## Summary

This task implements the evaluation and compliance checking utilities for the Go tasker port. These utilities port the functionality from the Python `evaluate.py` and `compliance-check.py` scripts, providing metrics computation from execution state and spec compliance verification.

## Components

- `go/internal/util/evaluate.go` - RunEvaluation for evaluation operations, computes performance metrics from task execution state
- `go/internal/util/evaluate_test.go` - Unit tests for evaluation functionality
- `go/internal/util/compliance.go` - CheckCompliance for compliance checks, verifies spec requirements against implementation
- `go/internal/util/compliance_test.go` - Unit tests for compliance checks

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

### Compliance

```go
// CheckCompliance runs all configured compliance checks and returns a report.
func CheckCompliance(opts ComplianceCheckOptions) *ComplianceReport

// ComplianceCheckOptions configures which compliance checks to run.
type ComplianceCheckOptions struct {
    SpecContent        string
    SpecPath           string
    MigrationsDir      string
    SettingsPath       string
    RoutesPath         string
    CodePath           string
    CheckSchema        bool
    CheckConfig        bool
    CheckAPI           bool
    CheckObservability bool
}

// ComplianceReport contains the complete compliance check result.
type ComplianceReport struct {
    Version    string
    SpecPath   string
    TargetPath string
    CheckedAt  string
    Gaps       []ComplianceGap
    Summary    *ComplianceSummary
}

// Extraction functions for parsing spec content
func ExtractDDLElements(specContent string) ([]TableDef, []ConstraintDef, []IndexDef)
func ExtractConfigRequirements(specContent string) []ConfigVar
func ExtractAPIRequirements(specContent string) []EndpointDef
func ExtractObservabilityRequirements(specContent string) ([]MetricDef, []SpanDef)

// FormatComplianceReport formats a ComplianceReport as a human-readable string.
func FormatComplianceReport(report *ComplianceReport) string
```

## Compliance Check Categories

The compliance checker supports four verification categories:

- **V1: Schema Compliance** - Verifies DDL elements (tables, constraints, indexes) exist in migrations
- **V2: Configuration Compliance** - Verifies environment variables are wired in settings
- **V3: API Compliance** - Verifies endpoints exist with correct methods
- **V4: Observability Compliance** - Verifies OTel spans and metrics are registered

## Dependencies

- Standard library only (no external dependencies beyond existing go.mod)

## Testing

```bash
# Run evaluation tests
cd go && go test ./internal/util/... -run TestRunEvaluation -v

# Run compliance tests
cd go && go test ./internal/util/... -run TestCheckCompliance -v

# Run all util tests
cd go && go test ./internal/util/... -v
```
