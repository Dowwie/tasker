# T013: Spec review and analysis

## Summary

Implements spec weakness detection and review status tracking for the Go tasker CLI. The AnalyzeSpec function scans specification documents for common weakness patterns (non-behavioral requirements, implicit assumptions, missing acceptance criteria, etc.) and generates a structured review report. GetReviewStatus provides visibility into the current review state and blocking issues.

## Components

- `go/internal/spec/review.go` - Core spec analysis and review management functions
- `go/internal/spec/review_test.go` - Unit tests covering weakness detection, status tracking, and review lifecycle
- `go/internal/command/spec/spec.go` - CLI subcommands for spec review, status, session management, and resolution

## API / Interface

### Spec Analysis

```go
// Analyze a spec file for weaknesses
result, err := spec.AnalyzeSpec("/path/to/spec.md")
if err != nil {
    return err
}

fmt.Printf("Found %d weaknesses\n", result.NewFindings)
if result.Blocking {
    fmt.Println("Critical issues must be resolved")
}

// Analyze spec content directly
result, err := spec.AnalyzeSpecContent(content, "source-name")
```

### Review Status

```go
// Get current review status
status, err := spec.GetReviewStatus(planningDir)
if err != nil {
    return err
}

fmt.Printf("Status: %s\n", status.Status)
fmt.Printf("Critical: %d\n", status.Critical)
fmt.Printf("Blocking: %v\n", status.Blocking)

// Load full review
review, err := spec.LoadReview(planningDir)

// Save review
err := spec.SaveReview(planningDir, review)

// Resolve a weakness
err := spec.ResolveWeakness(review, "W1-001", "Added measurable SLA")
```

### CLI Commands

```bash
# Analyze a spec file
tasker spec review /path/to/spec.md
tasker spec review /path/to/spec.md --json
tasker spec review /path/to/spec.md --save

# Check review status
tasker spec status
tasker spec status --json

# Show session details
tasker spec session show

# Resolve a weakness
tasker spec resolve W1-001 "Added measurable performance SLA"
```

## Weakness Categories

| Category | Description | Severity |
|----------|-------------|----------|
| non_behavioral | Quality attributes without measurable criteria | warning |
| implicit | Assumptions using words like "obviously", "clearly" | warning |
| missing_ac | Error handling without acceptance criteria | critical |
| fragmented | References to other sections | info |
| cross_cutting | Logging, auth, security concerns | info |
| contradiction | Conflicting requirements | critical |

## Testing

```bash
go test ./go/internal/spec/... -run TestAnalyzeSpec -v
go test ./go/internal/spec/... -run TestGetReviewStatus -v
go test ./go/internal/spec/... -v
```

## Schema Compatibility

The spec review output conforms to `schemas/spec-review.schema.json`, enabling validation and interoperability with the Python implementation.
