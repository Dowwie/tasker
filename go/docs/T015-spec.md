# T015: Spec session management

## Summary

Implements session management for the spec development workflow in the Go tasker CLI. The `Session` type and related functions manage discovery session state, tracking phases, open questions, clarifications, decisions, and gate requirements. This enables the `/specify` workflow to track progress through the spec development lifecycle.

## Components

- `go/internal/spec/session.go` - Core session management types and functions
- `go/internal/spec/session_test.go` - Unit tests covering session lifecycle, tracking, and gate validation

## API / Interface

### Session Management

```go
// Initialize a new spec session
session, err := spec.InitSession(baseDir, "Feature Name", targetDir)
if err != nil {
    return err
}

// Load an existing session
session, err := spec.LoadSessionFromBaseDir(baseDir)
if session == nil {
    fmt.Println("No active session")
}

// Save session after modifications
err := spec.SaveSessionToDir(baseDir, session)

// Get session status summary
status, err := spec.GetSessionStatus(baseDir)
fmt.Printf("Phase: %s (%d/%d)\n", status.Phase, status.PhaseIndex+1, status.TotalPhases)
```

### Phase Management

```go
// Advance to next phase
nextPhase, err := spec.AdvancePhase(session)
if err != nil {
    fmt.Println("Already at final phase")
}

// Check gate requirements
result := spec.CheckGate(session, baseDir)
if !result.Passed {
    for _, issue := range result.Issues {
        fmt.Printf("  - %s\n", issue)
    }
}
```

### Tracking

```go
// Track rounds
spec.IncrementRound(session)

// Track open questions
spec.AddOpenQuestion(session, "What about security?", true)  // blocking
spec.AddOpenQuestion(session, "Color preference?", false)    // non-blocking

// Resolve questions
resolved := spec.ResolveOpenQuestion(session, "What about security?", "Using OAuth2")

// Track decisions
spec.AddDecision(session, "Use PostgreSQL", "ADR-001")

// Set scope
spec.SetScope(session, "Build CLI tool", []string{"GUI"}, []string{"Tests pass"})
```

### ADR Numbering

```go
// Get next available ADR number
num, err := spec.GetNextADRNumber(targetDir)
// Returns 1 if no ADRs exist, or max+1
```

## Phase Workflow

The session progresses through these phases:

| Phase | Description |
|-------|-------------|
| scope | Define goal, non-goals, done means |
| clarify | Gather requirements through discovery rounds |
| synthesis | Consolidate findings |
| architecture | Design system structure |
| decisions | Record decisions and ADRs |
| gate | Validate handoff readiness |
| spec_review | Review spec for weaknesses |
| export | Generate spec artifacts |
| complete | Session finished |

## Gate Requirements

Before export, the gate check validates:
- Goal is defined
- Done means are specified
- No blocking open questions remain
- Discovery file exists

## Testing

```bash
go test ./go/internal/spec/... -run TestManageSession -v
go test ./go/internal/spec/... -run TestSessionTracking -v
go test ./go/internal/spec/... -v  # all spec tests
```

## Compatibility

The `Session` type is compatible with the existing `GenerateSpecFromSession` function in `generate.go`, enabling direct spec generation from session state.
