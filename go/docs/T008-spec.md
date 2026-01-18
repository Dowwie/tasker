# T008: Planning gates validation

## Summary

Implements planning validation gates for the tasker CLI, enabling verification of spec coverage, detection of phase leakage, and validation of acceptance criteria quality. Provides a unified `RunAllGates` function that executes all validation gates and aggregates results.

## Components

- `go/internal/validate/gates.go` - Core planning gate functions: CheckSpecCoverage, DetectPhaseLeakage, ValidateAcceptanceCriteria, RunAllGates
- `go/internal/validate/gates_test.go` - Unit tests for all planning gate operations

## API / Interface

### Gate Functions

```go
func CheckSpecCoverage(tasks []TaskDefinition, capabilityMap *CapabilityMap, threshold float64) *SpecCoverageResult
func DetectPhaseLeakage(tasks []TaskDefinition, currentPhase int, phaseKeywords map[int][]string) *PhaseLeakageResult
func ValidateAcceptanceCriteria(tasks []TaskDefinition) *ACValidationResult
func RunAllGates(planningDir string, currentPhase int, coverageThreshold float64) (*AllGatesResult, error)
```

### Result Types

```go
type SpecCoverageResult struct {
    Passed            bool
    Ratio             float64
    Threshold         float64
    TotalBehaviors    int
    CoveredBehaviors  int
    UncoveredBehaviors []string
    CoverageByDomain  map[string]float64
}

type PhaseLeakageResult struct {
    Passed     bool
    Violations []PhaseLeakageViolation
}

type ACValidationResult struct {
    Passed bool
    Issues []ACQualityIssue
}

type AllGatesResult struct {
    Passed       bool
    Gates        []GateResult
    SpecCoverage *SpecCoverageResult
    PhaseLeakage *PhaseLeakageResult
    ACValidation *ACValidationResult
}
```

### Helper Functions

```go
func LoadTaskDefinitions(planningDir string) ([]TaskDefinition, error)
func LoadCapabilityMap(planningDir string) (*CapabilityMap, error)
```

## Behaviors Implemented

- **B27 (CheckSpecCoverage)**: Validates that all behaviors in the capability map are covered by task definitions. Computes coverage ratio and compares against configurable threshold. Reports uncovered behaviors and per-domain coverage.

- **B28 (DetectPhaseLeakage)**: Scans Phase 1 tasks for Phase 2+ keywords (deployment, production, scale, performance optimization, migration, deprecation, backward compatibility). Supports custom keyword configuration.

- **B29 (ValidateAcceptanceCriteria)**: Validates AC quality by checking:
  - Tasks have at least one AC
  - Criteria are non-empty and meaningful (>10 chars)
  - Verification commands are present and follow standard patterns (go test, pytest, npm test, make test, cargo test, bash, ./script)

- **B30 (RunAllGates)**: Orchestrates execution of all gates, loads tasks and capability map, aggregates results, and reports overall pass/fail status.

## Validation Logic

### Spec Coverage
1. Extract all behaviors from capability map domains
2. Collect behaviors referenced by tasks
3. Compute coverage ratio = covered / total
4. Compare against threshold (default 1.0 = 100%)
5. Track per-domain coverage for detailed reporting

### Phase Leakage Detection
1. Filter to tasks matching current phase
2. Concatenate task name and all AC text
3. Search for future-phase keywords (case-insensitive)
4. Report violations with task ID and evidence

### AC Quality Validation
1. Check each task has acceptance criteria
2. Validate criterion text is non-empty and >10 chars
3. Validate verification command is present
4. Check verification command matches known executable patterns

## Dependencies

- T007: DAG validation (Task struct, validate package)

## Testing

```bash
cd go && go test ./internal/validate/... -run TestCheckSpecCoverage -v
cd go && go test ./internal/validate/... -run TestDetectPhaseLeakage -v
cd go && go test ./internal/validate/... -run TestValidateAcceptanceCriteria -v
cd go && go test ./internal/validate/... -run TestRunAllGates -v
```

Tests verify:
- Full and partial spec coverage scenarios
- Coverage threshold comparisons
- Multi-domain coverage tracking
- Phase leakage detection with default and custom keywords
- Phase filtering (only current phase tasks checked)
- AC validation for missing, empty, and short criteria
- Verification command pattern matching
- RunAllGates integration with all sub-gates
- Error handling for missing tasks/capability map
