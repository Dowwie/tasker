# T016: Bundle generation and operations

## Summary
Implements bundle generation and management operations for task execution. Bundles are self-contained JSON packages that include everything a task-executor needs to implement a task: behaviors, file mappings, dependencies, acceptance criteria, and constraints.

## Components
- `go/internal/bundle/bundle.go` - Core bundle generation and operations
- `go/internal/bundle/bundle_test.go` - Unit tests for bundle operations
- `go/internal/command/bundle/bundle.go` - CLI subcommands for bundle management

## API / Interface

### Generator
```go
type Generator struct {
    planningDir string
}

func NewGenerator(planningDir string) *Generator

func (g *Generator) GenerateBundle(taskID string) (*Bundle, error)
func (g *Generator) GenerateReadyBundles() ([]*Bundle, []error)
func (g *Generator) ValidateBundle(taskID string) (*schema.ValidationResult, error)
func (g *Generator) ValidateIntegrity(taskID string) (*IntegrityResult, error)
func (g *Generator) ListBundles() ([]BundleInfo, error)
func (g *Generator) CleanBundles() (int, error)
func (g *Generator) LoadBundle(taskID string) (*Bundle, error)
```

### Bundle Structure
```go
type Bundle struct {
    Version            string
    BundleCreatedAt    string
    TaskID             string
    Name               string
    Phase              int
    TargetDir          string
    Context            Context
    Behaviors          []Behavior
    Files              []FileMapping
    Dependencies       Dependencies
    AcceptanceCriteria []AcceptanceCriterion
    Constraints        Constraints
    StateMachine       *StateMachine
    Checksums          Checksums
}
```

### CLI Commands
```bash
tasker bundle generate <task-id>     # Generate bundle for single task
tasker bundle generate-ready         # Generate bundles for all ready tasks
tasker bundle validate <task-id>     # Validate bundle against schema
tasker bundle validate-integrity <task-id>  # Validate dependencies + checksums
tasker bundle list                   # List existing bundles
tasker bundle clean                  # Remove all bundles
```

## Behaviors Implemented
- B49: GenerateBundle - Generate bundle for task
- B50: GenerateReadyBundles - Generate bundles for ready tasks
- B51: ValidateBundle - Validate bundle against schema
- B52: ValidateIntegrity - Validate bundle integrity
- B53: ListBundles - List existing bundles
- B54: CleanBundles - Remove all bundles

## Key Design Decisions
1. Bundles are self-contained with expanded behavior details (not just IDs)
2. Checksums are computed for artifacts and dependency files to detect drift
3. Constraints are parsed from constraints.md into structured format
4. State machine context is preserved if present in task definition

## Dependencies
- `internal/state` - State management
- `internal/schema` - Schema validation
- `internal/errors` - Error handling

## Testing
```bash
go test ./go/internal/bundle/... -v
```
