package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/dgordon/tasker/internal/logging"
)

const (
	EnvPlanningDir = "TASKER_PLANNING_DIR"
	EnvTargetDir   = "TASKER_TARGET_DIR"
	EnvLogLevel    = "TASKER_LOG_LEVEL"
	EnvSchemaDir   = "TASKER_SCHEMA_DIR"
	EnvDebug       = "TASKER_DEBUG"
)

const (
	DefaultPlanningDirName = "project-planning"
	DefaultSchemaDirName   = "schemas"
)

type Config struct {
	PlanningDir string
	TargetDir   string
	SchemaDir   string
	LogLevel    logging.Level
	Debug       bool
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

func Get() *Config {
	configOnce.Do(func() {
		globalConfig = loadFromEnvironment()
	})
	return globalConfig
}

func Reset() {
	globalConfig = nil
	configOnce = sync.Once{}
}

func loadFromEnvironment() *Config {
	cfg := &Config{
		LogLevel: logging.LevelInfo,
		Debug:    false,
	}

	if planningDir := os.Getenv(EnvPlanningDir); planningDir != "" {
		cfg.PlanningDir = planningDir
	}

	if targetDir := os.Getenv(EnvTargetDir); targetDir != "" {
		cfg.TargetDir = targetDir
	}

	if schemaDir := os.Getenv(EnvSchemaDir); schemaDir != "" {
		cfg.SchemaDir = schemaDir
	}

	if logLevel := os.Getenv(EnvLogLevel); logLevel != "" {
		if level, err := logging.ParseLevel(logLevel); err == nil {
			cfg.LogLevel = level
		}
	}

	if debug := os.Getenv(EnvDebug); debug != "" {
		cfg.Debug = parseBool(debug)
		if cfg.Debug {
			cfg.LogLevel = logging.LevelDebug
		}
	}

	return cfg
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (c *Config) ResolvePlanningDir() string {
	if c.PlanningDir != "" {
		return c.PlanningDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	defaultPath := filepath.Join(cwd, DefaultPlanningDirName)
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	return ""
}

func (c *Config) ResolveSchemaDir() string {
	if c.SchemaDir != "" {
		return c.SchemaDir
	}

	planningDir := c.ResolvePlanningDir()
	if planningDir != "" {
		parentDir := filepath.Dir(planningDir)
		schemaPath := filepath.Join(parentDir, DefaultSchemaDirName)
		if _, err := os.Stat(schemaPath); err == nil {
			return schemaPath
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	defaultPath := filepath.Join(cwd, DefaultSchemaDirName)
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	return ""
}

func (c *Config) Validate() []string {
	var issues []string
	return issues
}

func (c *Config) ValidateRequired(required ...string) []string {
	var issues []string
	for _, req := range required {
		switch req {
		case "planning_dir":
			if c.ResolvePlanningDir() == "" {
				issues = append(issues, "planning directory not found; set TASKER_PLANNING_DIR or run from project root")
			}
		case "schema_dir":
			if c.ResolveSchemaDir() == "" {
				issues = append(issues, "schema directory not found; set TASKER_SCHEMA_DIR")
			}
		}
	}
	return issues
}

func GetEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func GetEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return parseBool(val)
	}
	return defaultVal
}

func GetEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
