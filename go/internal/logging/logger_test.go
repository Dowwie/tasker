package logging

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	t.Run("level filtering", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, LevelWarn)

		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")

		output := buf.String()
		if strings.Contains(output, "debug message") {
			t.Error("debug message should be filtered")
		}
		if strings.Contains(output, "info message") {
			t.Error("info message should be filtered")
		}
		if !strings.Contains(output, "warn message") {
			t.Error("warn message should be present")
		}
		if !strings.Contains(output, "error message") {
			t.Error("error message should be present")
		}
	})

	t.Run("output to stderr", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, LevelInfo)
		logger.Info("test message")

		if buf.Len() == 0 {
			t.Error("expected output")
		}
	})

	t.Run("structured format", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, LevelInfo)
		logger.Info("test %s", "message")

		output := buf.String()
		if !strings.Contains(output, "[INFO]") {
			t.Error("expected [INFO] level marker")
		}
		if !strings.Contains(output, "test message") {
			t.Error("expected formatted message")
		}
		if !strings.HasSuffix(output, "\n") {
			t.Error("expected newline at end")
		}
	})

	t.Run("level change", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, LevelError)

		logger.Info("should not appear")
		if buf.Len() > 0 {
			t.Error("message should be filtered at ERROR level")
		}

		logger.SetLevel(LevelInfo)
		logger.Info("should appear")
		if buf.Len() == 0 {
			t.Error("message should appear after level change")
		}
	})

	t.Run("prefix support", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, LevelInfo)
		logger.SetPrefix("mycomponent")
		logger.Info("test")

		output := buf.String()
		if !strings.Contains(output, "[mycomponent]") {
			t.Errorf("expected prefix in output, got: %s", output)
		}
	})
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"DEBUG", LevelDebug, false},
		{"debug", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"info", LevelInfo, false},
		{"WARN", LevelWarn, false},
		{"WARNING", LevelWarn, false},
		{"ERROR", LevelError, false},
		{"error", LevelError, false},
		{"invalid", LevelInfo, true},
		{"  INFO  ", LevelInfo, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if level != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.level.String())
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Error("Default() should not return nil")
	}

	if logger.GetLevel() != LevelInfo {
		t.Errorf("expected default level INFO, got %v", logger.GetLevel())
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetLevel(LevelDebug)
	defer func() {
		SetOutput(os.Stderr)
		SetLevel(LevelInfo)
	}()

	Debug("debug")
	Info("info")
	Warn("warn")
	Error("error")

	output := buf.String()
	if !strings.Contains(output, "debug") {
		t.Error("expected debug message")
	}
	if !strings.Contains(output, "info") {
		t.Error("expected info message")
	}
	if !strings.Contains(output, "warn") {
		t.Error("expected warn message")
	}
	if !strings.Contains(output, "error") {
		t.Error("expected error message")
	}
}

func TestInitFromEnv(t *testing.T) {
	origLevel := defaultLogger.GetLevel()
	defer func() {
		defaultLogger.SetLevel(origLevel)
		os.Unsetenv("TASKER_LOG_LEVEL")
	}()

	os.Setenv("TASKER_LOG_LEVEL", "DEBUG")
	InitFromEnv()

	if defaultLogger.GetLevel() != LevelDebug {
		t.Errorf("expected DEBUG level after InitFromEnv, got %v", defaultLogger.GetLevel())
	}
}
