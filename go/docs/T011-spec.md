# T011: FSM Mermaid Generator

## Summary

Implements Mermaid diagram and markdown notes generation from FSM state/transition files. Provides `GenerateMermaid` for visualization and `GenerateNotes` for documentation.

## Components

- `go/internal/fsm/mermaid.go` - GenerateMermaid and GenerateNotes functions
- `go/internal/fsm/mermaid_test.go` - Unit tests for Mermaid generation

## API / Interface

### GenerateMermaid

```go
mermaid := fsm.GenerateMermaid(machineName, statesFile, transitionsFile)
```

Generates a Mermaid stateDiagram-v2 diagram:
- Initial states shown with `[*] --> StateID`
- Success states shown with `StateID --> [*]`
- Failure states marked with `[FAILURE]`
- Transitions show trigger labels (truncated at 30 chars)
- Double quotes escaped to single quotes

### GenerateNotes

```go
notes := fsm.GenerateNotes(machineName, statesFile, transitionsFile)
```

Generates markdown documentation including:
- Machine name header
- States section with initial/terminal callouts
- States table (ID, Name, Type, Description)
- Transitions table (ID, From, To, Trigger, Guards)
- Guards index mapping invariants to transitions
- Embedded Mermaid diagram

## Dependencies

- `go/internal/fsm/compiler.go` - StatesFile, TransitionsFile, State, Transition types

## Testing

```bash
cd go && go test ./internal/fsm/... -run TestGenerateMermaid -v
cd go && go test ./internal/fsm/... -run TestGenerateNotes -v
```

## Behavior Coverage

| Behavior | Implementation |
|----------|----------------|
| B39: GenerateMermaid | `GenerateMermaid()` - outputs valid Mermaid stateDiagram-v2 syntax |
| B40: GenerateNotes | `GenerateNotes()` - creates markdown documentation for FSM states |
