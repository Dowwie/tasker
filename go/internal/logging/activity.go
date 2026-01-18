package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ActivityLogger struct {
	mu       sync.Mutex
	logPath  string
	minLevel Level
}

var defaultActivityLogger *ActivityLogger

// NewActivityLogger creates an ActivityLogger that writes to the specified log file.
func NewActivityLogger(logPath string, minLevel Level) *ActivityLogger {
	return &ActivityLogger{
		logPath:  logPath,
		minLevel: minLevel,
	}
}

// DefaultActivityLogger returns the singleton activity logger.
// Initializes with .claude/logs/activity.log if not already set.
func DefaultActivityLogger() *ActivityLogger {
	if defaultActivityLogger == nil {
		defaultActivityLogger = NewActivityLogger(".claude/logs/activity.log", LevelInfo)
	}
	return defaultActivityLogger
}

// SetDefaultActivityLogger sets the global activity logger.
func SetDefaultActivityLogger(logger *ActivityLogger) {
	defaultActivityLogger = logger
}

// LogActivity writes a structured activity log entry matching the shell script format:
// [ISO-timestamp] [LEVEL] [agent] event: message
func (a *ActivityLogger) LogActivity(level Level, agent, event, message string) error {
	if level < a.minLevel {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(a.logPath), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(a.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open activity log: %w", err)
	}
	defer f.Close()

	timestamp := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] [%s] [%s] %s: %s\n", timestamp, level.String(), agent, event, message)

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("failed to write activity log: %w", err)
	}

	return nil
}

// SetLevel sets the minimum level for activity logging.
func (a *ActivityLogger) SetLevel(level Level) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.minLevel = level
}

// SetPath updates the log file path.
func (a *ActivityLogger) SetPath(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logPath = path
}

// LogPath returns the current log file path.
func (a *ActivityLogger) LogPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.logPath
}

// Package-level functions for convenient access

// LogActivity writes an activity log entry using the default activity logger.
func LogActivity(level Level, agent, event, message string) error {
	return DefaultActivityLogger().LogActivity(level, agent, event, message)
}

// LogActivityInfo is a convenience function for INFO level activity logs.
func LogActivityInfo(agent, event, message string) error {
	return LogActivity(LevelInfo, agent, event, message)
}

// LogActivityWarn is a convenience function for WARN level activity logs.
func LogActivityWarn(agent, event, message string) error {
	return LogActivity(LevelWarn, agent, event, message)
}

// LogActivityError is a convenience function for ERROR level activity logs.
func LogActivityError(agent, event, message string) error {
	return LogActivity(LevelError, agent, event, message)
}
