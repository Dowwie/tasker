# T012: FSM Validator

## Summary

Implements FSM validation functions that check I1-I5 invariants from the FSM spec. Provides completeness checking for dead-end states, transition coverage calculation, and task coverage verification for planning gates.

## Components

- `go/internal/fsm/validate.go` - Core FSM validation with ValidateInvariants, CheckCompleteness, ComputeCoverage, CheckTaskCoverage
- `go/internal/fsm/validate_test.go` - Comprehensive unit tests for all validation behaviors

## API / Interface

### Validation Types

```go
type ValidationIssue struct {
    Invariant string            `json:"invariant"`
    Message   string            `json:"message"`
    Context   map[string]string `json:"context,omitempty"`
}

type ValidationResult struct {
    Passed   bool                `json:"passed"`
    Issues   []ValidationIssue   `json:"issues"`
    Warnings []ValidationWarning `json:"warnings"`
}

type CoverageResult struct {
    TotalTransitions   int      `json:"total_transitions"`
    CoveredTransitions int      `json:"covered_transitions"`
    CoveragePercent    float64  `json:"coverage_percent"`
    UncoveredIDs       []string `json:"uncovered_ids,omitempty"`
}
```

### Functions

```go
result, err := fsm.ValidateInvariants(fsmDir)

result := fsm.CheckCompleteness(states, transitions)

coverage := fsm.ComputeCoverage(transitions)

taskResult, err := fsm.CheckTaskCoverage(
    index,
    fsmDir,
    tasksCoverage,
    steelThreadThreshold,  // default 1.0 (100%)
    nonSteelThreadThreshold, // default 0.9 (90%)
)
```

## Invariants Checked

| Invariant | Description | Check |
|-----------|-------------|-------|
| I1 | Steel Thread FSM mandatory | `primary_machine` exists with `level: steel_thread` |
| I3 | Completeness | Initial state, terminals, no dead ends, reachability |
| I4 | Guard-Invariant linkage | Guards should reference invariant IDs |
| I2, I5 | Behavior-first, No silent ambiguity | Enforced during compilation |

## Validation Behavior

### ValidateInvariants
- Loads index.json from FSM directory
- Checks I1: primary_machine exists and has steel_thread level
- For each machine, loads states/transitions and validates I3, I4

### CheckCompleteness
- Validates initial_state is defined and exists in states
- Validates terminal_states are defined and exist
- Checks non-terminal states have outgoing transitions
- Validates all transition references point to existing states
- Computes reachability from initial state

### ComputeCoverage
- Calculates percentage of transitions with linked behaviors
- Returns list of uncovered transition IDs

### CheckTaskCoverage
- Validates tasks cover required FSM transitions
- Steel thread transitions require threshold (default 100%)
- Non-steel thread transitions require threshold (default 90%)
- Returns coverage metrics and uncovered transition IDs

## Dependencies

- `github.com/dgordon/tasker/internal/fsm` - FSM types from compiler
- `github.com/dgordon/tasker/internal/errors` - Error types

## Testing

```bash
cd go && go test ./internal/fsm/... -run TestValidateInvariants -v
cd go && go test ./internal/fsm/... -run TestCheckCompleteness -v
cd go && go test ./internal/fsm/... -run TestComputeCoverage -v
cd go && go test ./internal/fsm/... -run TestCheckTaskCoverage -v
```

## Behavior Coverage

| Behavior | Implementation |
|----------|----------------|
| B41: ValidateInvariants | `ValidateInvariants()` - checks I1-I5 invariants |
| B42: CheckCompleteness | `CheckCompleteness()` - identifies dead-end states |
| B43: ComputeCoverage | `ComputeCoverage()` - calculates transition coverage |
| B44: CheckTaskCoverage | `CheckTaskCoverage()` - verifies task FSM coverage |
