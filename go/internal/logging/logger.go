package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN", "WARNING":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	prefix string
}

var defaultLogger = &Logger{
	level:  LevelInfo,
	output: os.Stderr,
}

func Default() *Logger {
	return defaultLogger
}

func New(w io.Writer, level Level) *Logger {
	return &Logger{
		level:  level,
		output: w,
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	msg := fmt.Sprintf(format, args...)

	var line string
	if l.prefix != "" {
		line = fmt.Sprintf("%s [%s] [%s] %s\n", timestamp, level.String(), l.prefix, msg)
	} else {
		line = fmt.Sprintf("%s [%s] %s\n", timestamp, level.String(), msg)
	}

	fmt.Fprint(l.output, line)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

func SetOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

func SetPrefix(prefix string) {
	defaultLogger.SetPrefix(prefix)
}

func InitFromEnv() {
	if levelStr := os.Getenv("TASKER_LOG_LEVEL"); levelStr != "" {
		if level, err := ParseLevel(levelStr); err == nil {
			defaultLogger.SetLevel(level)
		}
	}
}
