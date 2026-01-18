# T010: FSM Compiler

## Summary

Implements a Go FSM compiler that compiles finite state machines from spec JSON or capability maps. The compiler extracts states from workflow steps, derives transitions from variants/failures, and links guards to invariants. Outputs FSM artifacts in the standard format (index.json, states.json, transitions.json).

## Components

- `go/internal/fsm/compiler.go` - Core FSM compiler with CompileFromSpec, CompileFromCapabilityMap, WriteStates, WriteTransitions
- `go/internal/fsm/compiler_test.go` - Comprehensive unit tests for all compiler behaviors
- `go/internal/command/fsm/fsm.go` - CLI commands: compile, from-capability-map, validate, mermaid

## API / Interface

### Compiler

```go
compiler := fsm.NewCompiler()

machine, err := compiler.CompileFromSpec(&fsm.SpecData{
    Workflows: []fsm.Workflow{...},
    Invariants: []map[string]interface{}{...},
})

machine, err := compiler.CompileFromCapabilityMap(&fsm.CapabilityMap{
    Domains: []fsm.CapabilityMapDomain{...},
    Flows: []fsm.CapabilityMapFlow{...},
}, specText)

err := compiler.WriteStates(machine, "output/states.json")
err := compiler.WriteTransitions(machine, "output/transitions.json")

index, err := compiler.Export(outputDir, specSlug, specChecksum)
```

### CLI Commands

```bash
tasker fsm compile <spec.json> --output-dir <dir> --slug <name>
tasker fsm from-capability-map <capability-map.json> [spec.md] --output-dir <dir>
tasker fsm validate <fsm-dir>
tasker fsm mermaid <fsm-dir>
```

## Output Format

### states.json
```json
{
  "version": "1.0",
  "machine_id": "M1",
  "initial_state": "S1",
  "terminal_states": ["S4", "S5"],
  "states": [
    {
      "id": "S1",
      "name": "Awaiting start",
      "type": "initial",
      "spec_ref": {"quote": "...", "location": "..."},
      "behaviors": ["B001"]
    }
  ]
}
```

### transitions.json
```json
{
  "version": "1.0",
  "machine_id": "M1",
  "transitions": [
    {
      "id": "TR1",
      "from_state": "S1",
      "to_state": "S2",
      "trigger": "start",
      "guards": [{"condition": "...", "invariant_id": "INV-001"}],
      "behaviors": ["B001"],
      "is_failure_path": false
    }
  ],
  "guards_index": {
    "INV-001": ["TR1"]
  }
}
```

## Dependencies

- `github.com/dgordon/tasker/internal/errors` - Error types
- `github.com/dgordon/tasker/internal/schema` - Schema validation for validate command
- `github.com/spf13/cobra` - CLI framework

## Testing

```bash
go test ./go/internal/fsm/... -run TestCompileFromSpec -v
go test ./go/internal/fsm/... -run TestCompileFromCapabilityMap -v
go test ./go/internal/fsm/... -run TestWriteStates -v
go test ./go/internal/fsm/... -run TestWriteTransitions -v
```

## Behavior Coverage

| Behavior | Implementation |
|----------|----------------|
| B35: CompileFromSpec | `compiler.CompileFromSpec()` - parses spec JSON workflows and invariants |
| B36: CompileFromCapabilityMap | `compiler.CompileFromCapabilityMap()` - converts flows to workflows |
| B37: WriteStates | `compiler.WriteStates()` - outputs states.json with initial/terminal states |
| B38: WriteTransitions | `compiler.WriteTransitions()` - outputs transitions.json with guards index |
