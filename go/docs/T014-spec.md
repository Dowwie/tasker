# T014: Spec generation

## Summary

Implements spec document generation from session state for the Go tasker CLI. The GenerateSpec function takes a Session object and produces a structured markdown specification document that includes all key elements: goals, non-goals, done means, workflows, invariants, interfaces, architecture, decisions, open questions, and agent handoff information. Also includes ADR generation capabilities.

## Components

- `go/internal/spec/generate.go` - Core spec and ADR generation functions
- `go/internal/spec/generate_test.go` - Unit tests for generation functionality

## API / Interface

### Spec Generation

```go
// Generate spec from session and write to file
result, err := spec.GenerateSpec(session, spec.GenerateOptions{
    TargetDir: "/path/to/project",
    Force:     false,
})
if err != nil {
    return err
}
fmt.Printf("Generated: %s\n", result.OutputPath)

// Generate spec content only (without writing to file)
content, err := spec.GenerateSpecContent(session)
if err != nil {
    return err
}
fmt.Println(content)
```

### ADR Generation

```go
// Generate ADR from input
input := spec.ADRInput{
    Number:   1,
    Title:    "Use bcrypt for password hashing",
    Context:  "We need secure password storage.",
    Decision: "Use bcrypt with cost factor 12.",
    Alternatives: []spec.Alternative{
        {Name: "Argon2", Reason: "More complex to configure"},
    },
    Consequences: []string{"Passwords are secure against rainbow tables"},
    AppliesTo: []spec.SpecReference{
        {Slug: "user-auth", Title: "User Authentication"},
    },
}

result, err := spec.GenerateADR(input, spec.GenerateOptions{
    TargetDir: "/path/to/project",
})
```

### Utility Functions

```go
// Convert text to slug format
slug := spec.Slugify("My Feature Name") // "my-feature-name"
```

## Generated Spec Structure

The generated spec document follows this format:

```markdown
# Spec: {Topic}

## Related ADRs
- [ADR-0001](../adrs/ADR-0001.md)

## Goal
{goal or "(goal not defined)"}

## Non-goals
- {non-goal items}

## Done means
- {done means items}

## Workflows
{workflow content}

## Invariants
- {invariant items}

## Interfaces
{interface description}

## Architecture sketch
{architecture description}

## Decisions
| Decision | ADR |
|----------|-----|
| {decision} | [ADR-0001](../adrs/ADR-0001.md) |

## Open Questions

### Blocking
- {blocking questions}

### Non-blocking
- {non-blocking questions}

## Agent Handoff
- **What to build:** {description}
- **Must preserve:** {invariants}
- **Blocking conditions:** {conditions}

## Artifacts
- **Capability Map:** [{slug}.capabilities.json](./{slug}.capabilities.json)
- **Discovery Log:** [clarify-session.md](../.claude/clarify-session.md)
```

## Testing

```bash
go test ./go/internal/spec/... -run TestGenerateSpec -v
go test ./go/internal/spec/... -run TestGenerateSpecFormat -v
go test ./go/internal/spec/... -run TestGenerateADR -v
```

## Dependencies

- Reuses types from session.go: Session, Scope, OpenQuestions, Decision
- Uses internal/errors package for consistent error handling
