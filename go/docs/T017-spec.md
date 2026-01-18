# T017: Archive and display utilities

## Summary

Implements archive and display utilities for the tasker CLI. The archive module enables archiving planning artifacts with timestamped directories and manifest generation. The display module provides dashboard and status output functions for visualizing task progress.

## Components

- `go/internal/util/archive.go` - ArchivePlanning function for archiving planning artifacts
- `go/internal/util/archive_test.go` - Unit tests for archive operations
- `go/internal/util/display.go` - ShowDashboard and ShowStatus functions for display output
- `go/internal/util/display_test.go` - Unit tests for display utilities

## API / Interface

### Archive

```go
// ArchivePlanning archives planning artifacts from a planning directory.
// Creates a timestamped copy under archiveRoot/projectName/planning/timestamp/.
func ArchivePlanning(planningDir, archiveRoot, projectName string) (*ArchiveResult, error)

type ArchiveResult struct {
    ArchivePath string  // Full path to the created archive
    ArchiveID   string  // Timestamp-based ID (YYYYMMDD_HHMMSS)
    ArchivedAt  string  // ISO timestamp of archive creation
    ItemsCount  int     // Number of items archived
}

type ArchiveManifest struct {
    Version        string              // "1.0"
    ArchiveType    string              // "planning"
    ProjectName    string
    ArchiveID      string
    ArchivedAt     string
    SourceDir      string
    PhaseAtArchive string
    Contents       map[string][]string
    TaskSummary    *ArchiveTaskSummary
}
```

### Display

```go
// ShowDashboard writes a dashboard display of task progress with status breakdown.
func ShowDashboard(w io.Writer, data DashboardData)

// ShowStatus writes a compact status summary suitable for scripts.
// Format: PHASE complete/total running=N failed=N [HALTED: reason]
func ShowStatus(w io.Writer, data StatusData)

// CountTasksByStatus counts tasks from a task map.
func CountTasksByStatus(tasks map[string]interface{}) StatusCounts

// ExtractActiveTasks returns task IDs with "running" status.
func ExtractActiveTasks(tasks map[string]interface{}) []string

// ExtractFailedTasks returns task IDs with "failed" status.
func ExtractFailedTasks(tasks map[string]interface{}) []string
```

## Dependencies

- Standard library only (encoding/json, fmt, io, os, path/filepath, sort, strings, time)

## Testing

```bash
cd go && go test ./internal/util/... -run TestArchive -v
cd go && go test ./internal/util/... -run TestShowDashboard -v
cd go && go test ./internal/util/... -run TestShowStatus -v
```

## Archive Directory Structure

```
archive/
└── {project_name}/
    └── planning/
        └── {timestamp}/
            ├── inputs/
            ├── artifacts/
            ├── tasks/
            ├── reports/
            ├── state.json
            └── archive-manifest.json
```

## Dashboard Output Example

```
============================================================
                    TASK DASHBOARD
============================================================

Phase:      executing
Target:     /home/user/project

------------------------------------------------------------
PROGRESS
------------------------------------------------------------
[####################--------------------] 50.0%

Status Breakdown:
  Complete:    5
  Running:     1
  Ready:       1
  Pending:     2
  Failed:      1
  Blocked:     0
  Skipped:     0
  ---------
  Total:      10

------------------------------------------------------------
ACTIVE TASKS
------------------------------------------------------------
  - T006

------------------------------------------------------------
RECENT FAILURES
------------------------------------------------------------
  - T003

============================================================
```

## Status Output Example

```
EXECUTING 5/10 running=2 failed=1
EXECUTING 3/10 running=0 failed=2 [HALTED: max failures exceeded]
```
