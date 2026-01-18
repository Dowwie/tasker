# T020: Activity Logging

## Summary

Implements activity logging functionality that writes structured log entries to a file, matching the format of the shell script `log-activity.sh`.

## Components

- `go/internal/logging/activity.go` - ActivityLogger type and LogActivity functions
- `go/internal/logging/activity_test.go` - Comprehensive unit tests

## API / Interface

```go
// Create a new activity logger
logger := NewActivityLogger("/path/to/activity.log", LevelInfo)

// Log an activity entry
err := logger.LogActivity(level Level, agent string, event string, message string) error

// Package-level convenience functions
err := LogActivity(level, agent, event, message)
err := LogActivityInfo(agent, event, message)
err := LogActivityWarn(agent, event, message)
err := LogActivityError(agent, event, message)

// Configure the default logger
SetDefaultActivityLogger(logger)
DefaultActivityLogger() *ActivityLogger

// Configure instance
logger.SetLevel(level Level)
logger.SetPath(path string)
logger.LogPath() string
```

## Log Format

Matches shell script output format:
```
[ISO-timestamp] [LEVEL] [agent] event: message
```

Example:
```
[2026-01-18T19:49:34+00:00] [INFO] [orchestrator] spawn: launching logic-architect
```

## Dependencies

- Uses existing `Level` type from `logger.go`
- Thread-safe with mutex protection

## Testing

```bash
cd go && go test ./internal/logging/... -run TestLogActivity -v
cd go && go test ./internal/logging/... -run TestLogActivityFormat -v
```
