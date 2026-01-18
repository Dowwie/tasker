package logging

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLogActivity(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test-activity.log")

	logger := NewActivityLogger(logPath, LevelInfo)

	err := logger.LogActivity(LevelInfo, "test-agent", "test-event", "test message")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "[INFO]") {
		t.Errorf("expected log to contain [INFO], got: %s", logContent)
	}
	if !strings.Contains(logContent, "[test-agent]") {
		t.Errorf("expected log to contain [test-agent], got: %s", logContent)
	}
	if !strings.Contains(logContent, "test-event:") {
		t.Errorf("expected log to contain test-event:, got: %s", logContent)
	}
	if !strings.Contains(logContent, "test message") {
		t.Errorf("expected log to contain test message, got: %s", logContent)
	}
}

func TestLogActivityFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "format-test.log")

	logger := NewActivityLogger(logPath, LevelInfo)
	err := logger.LogActivity(LevelInfo, "orchestrator", "spawn", "launching logic-architect")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logLine := strings.TrimSpace(string(content))

	// Shell script format: [ISO-timestamp] [LEVEL] [agent] event: message
	// ISO-8601 with timezone: 2026-01-18T19:49:34+00:00
	pattern := `^\[\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}\] \[INFO\] \[orchestrator\] spawn: launching logic-architect$`
	matched, err := regexp.MatchString(pattern, logLine)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("log format does not match shell script output format.\nExpected pattern: %s\nGot: %s", pattern, logLine)
	}
}

func TestLogActivityLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "filter-test.log")

	logger := NewActivityLogger(logPath, LevelWarn)

	err := logger.LogActivity(LevelInfo, "agent", "event", "should be filtered")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	err = logger.LogActivity(LevelWarn, "agent", "event", "should appear")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	if strings.Contains(logContent, "should be filtered") {
		t.Errorf("INFO message should be filtered when level is WARN")
	}
	if !strings.Contains(logContent, "should appear") {
		t.Errorf("WARN message should appear when level is WARN")
	}
}

func TestLogActivityAllLevels(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "levels-test.log")

	logger := NewActivityLogger(logPath, LevelDebug)

	tests := []struct {
		level Level
		tag   string
	}{
		{LevelDebug, "[DEBUG]"},
		{LevelInfo, "[INFO]"},
		{LevelWarn, "[WARN]"},
		{LevelError, "[ERROR]"},
	}

	for _, tt := range tests {
		err := logger.LogActivity(tt.level, "agent", "event", "message for "+tt.tag)
		if err != nil {
			t.Fatalf("LogActivity failed for %s: %v", tt.tag, err)
		}
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	for _, tt := range tests {
		if !strings.Contains(logContent, tt.tag) {
			t.Errorf("expected log to contain %s", tt.tag)
		}
	}
}

func TestLogActivityCreatesDirIfNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nested", "dir", "activity.log")

	logger := NewActivityLogger(logPath, LevelInfo)
	err := logger.LogActivity(LevelInfo, "agent", "event", "test")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected log file to be created in nested directory")
	}
}

func TestLogActivityAppends(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "append-test.log")

	logger := NewActivityLogger(logPath, LevelInfo)

	err := logger.LogActivity(LevelInfo, "agent", "event", "first message")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	err = logger.LogActivity(LevelInfo, "agent", "event", "second message")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "first message") {
		t.Error("expected log to contain first message")
	}
	if !strings.Contains(logContent, "second message") {
		t.Error("expected log to contain second message")
	}

	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}
}

func TestActivityLoggerSetLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "set-level-test.log")

	logger := NewActivityLogger(logPath, LevelError)

	err := logger.LogActivity(LevelInfo, "agent", "event", "should not appear")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	logger.SetLevel(LevelInfo)

	err = logger.LogActivity(LevelInfo, "agent", "event", "should appear")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	if strings.Contains(logContent, "should not appear") {
		t.Error("message before SetLevel should be filtered")
	}
	if !strings.Contains(logContent, "should appear") {
		t.Error("message after SetLevel should appear")
	}
}

func TestActivityLoggerSetPath(t *testing.T) {
	tmpDir := t.TempDir()
	logPath1 := filepath.Join(tmpDir, "path1.log")
	logPath2 := filepath.Join(tmpDir, "path2.log")

	logger := NewActivityLogger(logPath1, LevelInfo)

	err := logger.LogActivity(LevelInfo, "agent", "event", "message1")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	logger.SetPath(logPath2)

	err = logger.LogActivity(LevelInfo, "agent", "event", "message2")
	if err != nil {
		t.Fatalf("LogActivity failed: %v", err)
	}

	content1, _ := os.ReadFile(logPath1)
	content2, _ := os.ReadFile(logPath2)

	if !strings.Contains(string(content1), "message1") {
		t.Error("expected message1 in first log file")
	}
	if strings.Contains(string(content1), "message2") {
		t.Error("message2 should not be in first log file")
	}
	if !strings.Contains(string(content2), "message2") {
		t.Error("expected message2 in second log file")
	}
}

func TestActivityLoggerLogPath(t *testing.T) {
	logger := NewActivityLogger("/test/path/activity.log", LevelInfo)

	if logger.LogPath() != "/test/path/activity.log" {
		t.Errorf("expected /test/path/activity.log, got %s", logger.LogPath())
	}
}

func TestPackageLevelActivityFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "pkg-level-test.log")

	oldLogger := defaultActivityLogger
	defer func() { defaultActivityLogger = oldLogger }()

	SetDefaultActivityLogger(NewActivityLogger(logPath, LevelInfo))

	err := LogActivityInfo("agent", "event", "info message")
	if err != nil {
		t.Fatalf("LogActivityInfo failed: %v", err)
	}

	err = LogActivityWarn("agent", "event", "warn message")
	if err != nil {
		t.Fatalf("LogActivityWarn failed: %v", err)
	}

	err = LogActivityError("agent", "event", "error message")
	if err != nil {
		t.Fatalf("LogActivityError failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "[INFO]") || !strings.Contains(logContent, "info message") {
		t.Error("expected INFO log entry")
	}
	if !strings.Contains(logContent, "[WARN]") || !strings.Contains(logContent, "warn message") {
		t.Error("expected WARN log entry")
	}
	if !strings.Contains(logContent, "[ERROR]") || !strings.Contains(logContent, "error message") {
		t.Error("expected ERROR log entry")
	}
}

func TestDefaultActivityLogger(t *testing.T) {
	oldLogger := defaultActivityLogger
	defaultActivityLogger = nil
	defer func() { defaultActivityLogger = oldLogger }()

	logger := DefaultActivityLogger()
	if logger == nil {
		t.Error("DefaultActivityLogger should not return nil")
	}
	if logger.LogPath() != ".claude/logs/activity.log" {
		t.Errorf("expected default path .claude/logs/activity.log, got %s", logger.LogPath())
	}
}
