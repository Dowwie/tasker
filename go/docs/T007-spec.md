# T007: DAG validation and cycle detection

## Summary

Implements DAG validation capabilities for the tasker CLI, enabling detection of dependency cycles, verification that all referenced dependencies exist, and validation of steel thread paths. Uses depth-first search for cycle detection and Kahn's algorithm for topological sorting.

## Components

- `go/internal/validate/dag.go` - Core DAG validation functions: DetectCycles, CheckDependencyExistence, ValidateSteelThread, ValidateDAG, TopologicalSort
- `go/internal/validate/dag_test.go` - Unit tests for all validation operations
- `go/internal/command/validate/validate.go` - CLI subcommands for validation (dag, gates, steel-thread)

## API / Interface

### Validation Library Functions

```go
type Task struct {
    ID          string
    DependsOn   []string
    Blocks      []string
    SteelThread bool
}

func DetectCycles(tasks map[string]Task) *CycleError
func CheckDependencyExistence(tasks map[string]Task) []MissingDependencyError
func ValidateSteelThread(tasks map[string]Task) *SteelThreadError
func ValidateDAG(tasks map[string]Task) ValidationResult
func TopologicalSort(tasks map[string]Task) ([]string, error)
```

### Error Types

```go
type CycleError struct {
    Cycle []string
}

type MissingDependencyError struct {
    TaskID      string
    MissingDeps []string
}

type SteelThreadError struct {
    Message string
}
```

### CLI Commands

```bash
tasker validate dag             # Validate DAG for cycles and missing deps
tasker validate gates           # Validate phase gate completion
tasker validate steel-thread    # Validate steel thread path integrity
```

## Algorithms

### Cycle Detection

Uses depth-first search with recursion stack tracking:
1. Visit each unvisited node
2. Add to recursion stack (current DFS path)
3. If neighbor is in recursion stack, cycle found
4. Extract cycle path from recursion stack
5. Deterministic: sorts task IDs before iteration

### Steel Thread Validation

Validates that:
1. All steel thread task dependencies exist
2. All steel thread task dependencies are also steel thread tasks
3. No cycles exist within steel thread subgraph

### Topological Sort

Uses Kahn's algorithm:
1. Compute in-degree for each task
2. Initialize queue with zero in-degree tasks
3. Process queue, decrementing dependent in-degrees
4. Result is valid execution order

## Dependencies

- T002: Task lifecycle operations (state types and task loading)
- github.com/spf13/cobra - CLI framework

## Testing

```bash
cd go && go test ./internal/validate/... -v
```

Tests verify:
- DetectCycles finds simple cycles, self-cycles, and complex cycles
- DetectCycles returns nil for valid DAGs and disconnected components
- CheckDependencyExistence identifies missing dependencies
- ValidateSteelThread validates path integrity and dependency constraints
- TopologicalSort produces valid execution order
- Error type formatting is correct
