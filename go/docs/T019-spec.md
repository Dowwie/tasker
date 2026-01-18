# T019: Git and directory utilities

## Summary

Implemented utility functions for git repository verification and planning directory initialization. These utilities support the core tasker workflow by ensuring the execution environment is properly configured.

## Components

- `go/internal/util/git.go` - Git repository verification utilities
- `go/internal/util/git_test.go` - Unit tests for git utilities
- `go/internal/util/dirs.go` - Planning directory initialization utilities
- `go/internal/util/dirs_test.go` - Unit tests for directory utilities

## API / Interface

### Git Utilities (`git.go`)

```go
// EnsureGitRepo verifies that the specified path is within a valid git repository.
// Returns the root directory of the git repository if found.
func EnsureGitRepo(path string) (string, error)

// IsGitRepo checks if the specified path is inside a git repository.
func IsGitRepo(path string) bool

// GitRepoError represents an error related to git repository operations.
type GitRepoError struct {
    Path    string
    Message string
}
```

### Directory Utilities (`dirs.go`)

```go
// InitDirectories creates the planning directory structure at the specified root.
// If planningDir is empty, it defaults to "project-planning" under the root.
// Returns the absolute path to the planning directory.
func InitDirectories(root string, planningDir string) (string, error)

// EnsurePlanningDir verifies that a planning directory exists and has the required structure.
func EnsurePlanningDir(planningPath string) error

// DirectoryError represents an error related to directory operations.
type DirectoryError struct {
    Path    string
    Op      string
    Message string
}

// PlanningDirName is the default name for the planning directory.
const PlanningDirName = "project-planning"

// PlanningSubdirs defines the required subdirectories within a planning directory.
var PlanningSubdirs = []string{"bundles", "artifacts"}
```

## Dependencies

- Standard library only (os, os/exec, path/filepath, fmt)

## Testing

```bash
go test ./go/internal/util/... -v
```

Specific test commands:
```bash
go test ./go/internal/util/... -run TestEnsureGitRepo -v
go test ./go/internal/util/... -run TestInitDirectories -v
```
