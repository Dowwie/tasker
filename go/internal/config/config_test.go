package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgordon/tasker/internal/logging"
)

func TestConfig(t *testing.T) {
	clearEnv := func() {
		os.Unsetenv(EnvPlanningDir)
		os.Unsetenv(EnvTargetDir)
		os.Unsetenv(EnvLogLevel)
		os.Unsetenv(EnvSchemaDir)
		os.Unsetenv(EnvDebug)
		Reset()
	}

	t.Run("loads from environment", func(t *testing.T) {
		clearEnv()

		os.Setenv(EnvPlanningDir, "/custom/planning")
		os.Setenv(EnvTargetDir, "/custom/target")
		os.Setenv(EnvLogLevel, "DEBUG")
		os.Setenv(EnvSchemaDir, "/custom/schemas")
		defer clearEnv()

		cfg := Get()

		if cfg.PlanningDir != "/custom/planning" {
			t.Errorf("expected /custom/planning, got %s", cfg.PlanningDir)
		}
		if cfg.TargetDir != "/custom/target" {
			t.Errorf("expected /custom/target, got %s", cfg.TargetDir)
		}
		if cfg.LogLevel != logging.LevelDebug {
			t.Errorf("expected DEBUG level, got %v", cfg.LogLevel)
		}
		if cfg.SchemaDir != "/custom/schemas" {
			t.Errorf("expected /custom/schemas, got %s", cfg.SchemaDir)
		}
	})

	t.Run("defaults without env vars", func(t *testing.T) {
		clearEnv()

		cfg := Get()

		if cfg.LogLevel != logging.LevelInfo {
			t.Errorf("expected INFO level by default, got %v", cfg.LogLevel)
		}
		if cfg.Debug {
			t.Error("expected debug false by default")
		}
	})

	t.Run("debug enables debug log level", func(t *testing.T) {
		clearEnv()

		os.Setenv(EnvDebug, "true")
		defer clearEnv()

		cfg := Get()

		if !cfg.Debug {
			t.Error("expected debug true")
		}
		if cfg.LogLevel != logging.LevelDebug {
			t.Errorf("expected DEBUG level when debug is true, got %v", cfg.LogLevel)
		}
	})

	t.Run("resolve planning dir from cwd", func(t *testing.T) {
		clearEnv()

		tmpDir := t.TempDir()
		planningDir := filepath.Join(tmpDir, DefaultPlanningDirName)
		if err := os.MkdirAll(planningDir, 0755); err != nil {
			t.Fatalf("failed to create planning dir: %v", err)
		}

		origWd, _ := os.Getwd()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}
		defer os.Chdir(origWd)

		cfg := Get()
		resolved := cfg.ResolvePlanningDir()

		resolvedReal, _ := filepath.EvalSymlinks(resolved)
		planningDirReal, _ := filepath.EvalSymlinks(planningDir)
		if resolvedReal != planningDirReal {
			t.Errorf("expected %s, got %s", planningDir, resolved)
		}
	})

	t.Run("explicit planning dir takes precedence", func(t *testing.T) {
		clearEnv()

		os.Setenv(EnvPlanningDir, "/explicit/path")
		defer clearEnv()

		cfg := Get()

		if cfg.ResolvePlanningDir() != "/explicit/path" {
			t.Errorf("expected /explicit/path, got %s", cfg.ResolvePlanningDir())
		}
	})

	t.Run("resolve schema dir relative to planning", func(t *testing.T) {
		clearEnv()

		tmpDir := t.TempDir()
		planningDir := filepath.Join(tmpDir, DefaultPlanningDirName)
		schemaDir := filepath.Join(tmpDir, DefaultSchemaDirName)
		os.MkdirAll(planningDir, 0755)
		os.MkdirAll(schemaDir, 0755)

		os.Setenv(EnvPlanningDir, planningDir)
		defer clearEnv()

		cfg := Get()
		resolved := cfg.ResolveSchemaDir()

		if resolved != schemaDir {
			t.Errorf("expected %s, got %s", schemaDir, resolved)
		}
	})

	t.Run("singleton behavior", func(t *testing.T) {
		clearEnv()

		cfg1 := Get()
		cfg2 := Get()

		if cfg1 != cfg2 {
			t.Error("expected Get() to return same instance")
		}
	})

	t.Run("reset clears singleton", func(t *testing.T) {
		clearEnv()

		cfg1 := Get()
		Reset()
		os.Setenv(EnvPlanningDir, "/new/path")
		defer clearEnv()

		cfg2 := Get()

		if cfg1 == cfg2 {
			t.Error("expected different instance after Reset()")
		}
		if cfg2.PlanningDir != "/new/path" {
			t.Errorf("expected new config value, got %s", cfg2.PlanningDir)
		}
	})

	t.Run("validate required", func(t *testing.T) {
		clearEnv()

		tmpDir := t.TempDir()
		os.Setenv(EnvPlanningDir, tmpDir)
		defer clearEnv()

		cfg := Get()
		issues := cfg.ValidateRequired("planning_dir")

		if len(issues) > 0 {
			t.Errorf("expected no issues with valid planning dir, got: %v", issues)
		}
	})

	t.Run("validate required missing", func(t *testing.T) {
		clearEnv()

		cfg := Get()
		issues := cfg.ValidateRequired("planning_dir")

		if len(issues) == 0 {
			t.Error("expected issue for missing planning dir")
		}
	})
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"invalid", false},
		{"  true  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetEnvHelpers(t *testing.T) {
	t.Run("GetEnvInt", func(t *testing.T) {
		os.Setenv("TEST_INT", "42")
		defer os.Unsetenv("TEST_INT")

		if GetEnvInt("TEST_INT", 0) != 42 {
			t.Error("expected 42")
		}
		if GetEnvInt("MISSING_INT", 99) != 99 {
			t.Error("expected default 99")
		}
	})

	t.Run("GetEnvInt invalid", func(t *testing.T) {
		os.Setenv("TEST_INT", "not a number")
		defer os.Unsetenv("TEST_INT")

		if GetEnvInt("TEST_INT", 99) != 99 {
			t.Error("expected default for invalid int")
		}
	})

	t.Run("GetEnvBool", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "true")
		defer os.Unsetenv("TEST_BOOL")

		if !GetEnvBool("TEST_BOOL", false) {
			t.Error("expected true")
		}
		if GetEnvBool("MISSING_BOOL", true) != true {
			t.Error("expected default true")
		}
	})

	t.Run("GetEnvString", func(t *testing.T) {
		os.Setenv("TEST_STR", "value")
		defer os.Unsetenv("TEST_STR")

		if GetEnvString("TEST_STR", "default") != "value" {
			t.Error("expected value")
		}
		if GetEnvString("MISSING_STR", "default") != "default" {
			t.Error("expected default")
		}
	})
}

func TestValidate(t *testing.T) {
	Reset()
	cfg := Get()
	issues := cfg.Validate()

	if len(issues) != 0 {
		t.Errorf("expected empty slice, got %v", issues)
	}
}

func TestValidateRequiredSchemaDir(t *testing.T) {
	clearEnv := func() {
		os.Unsetenv(EnvPlanningDir)
		os.Unsetenv(EnvTargetDir)
		os.Unsetenv(EnvLogLevel)
		os.Unsetenv(EnvSchemaDir)
		os.Unsetenv(EnvDebug)
		Reset()
	}

	t.Run("schema_dir missing", func(t *testing.T) {
		clearEnv()
		cfg := Get()
		issues := cfg.ValidateRequired("schema_dir")

		if len(issues) == 0 {
			t.Error("expected issue for missing schema dir")
		}
	})

	t.Run("schema_dir present", func(t *testing.T) {
		clearEnv()
		tmpDir := t.TempDir()
		os.Setenv(EnvSchemaDir, tmpDir)
		defer clearEnv()

		cfg := Get()
		issues := cfg.ValidateRequired("schema_dir")

		if len(issues) != 0 {
			t.Errorf("expected no issues with valid schema dir, got: %v", issues)
		}
	})
}

func TestResolveSchemaDirFromCwd(t *testing.T) {
	clearEnv := func() {
		os.Unsetenv(EnvPlanningDir)
		os.Unsetenv(EnvTargetDir)
		os.Unsetenv(EnvLogLevel)
		os.Unsetenv(EnvSchemaDir)
		os.Unsetenv(EnvDebug)
		Reset()
	}

	clearEnv()

	tmpDir := t.TempDir()
	schemaDir := filepath.Join(tmpDir, DefaultSchemaDirName)
	os.MkdirAll(schemaDir, 0755)

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origWd)
	defer clearEnv()

	cfg := Get()
	resolved := cfg.ResolveSchemaDir()

	resolvedReal, _ := filepath.EvalSymlinks(resolved)
	schemaDirReal, _ := filepath.EvalSymlinks(schemaDir)
	if resolvedReal != schemaDirReal {
		t.Errorf("expected %s, got %s", schemaDir, resolved)
	}
}

func TestResolveSchemaDirNotFound(t *testing.T) {
	clearEnv := func() {
		os.Unsetenv(EnvPlanningDir)
		os.Unsetenv(EnvTargetDir)
		os.Unsetenv(EnvLogLevel)
		os.Unsetenv(EnvSchemaDir)
		os.Unsetenv(EnvDebug)
		Reset()
	}

	clearEnv()

	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origWd)
	defer clearEnv()

	cfg := Get()
	resolved := cfg.ResolveSchemaDir()

	if resolved != "" {
		t.Errorf("expected empty string for missing schema dir, got %s", resolved)
	}
}

func TestResolvePlanningDirNotFound(t *testing.T) {
	clearEnv := func() {
		os.Unsetenv(EnvPlanningDir)
		os.Unsetenv(EnvTargetDir)
		os.Unsetenv(EnvLogLevel)
		os.Unsetenv(EnvSchemaDir)
		os.Unsetenv(EnvDebug)
		Reset()
	}

	clearEnv()

	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origWd)
	defer clearEnv()

	cfg := Get()
	resolved := cfg.ResolvePlanningDir()

	if resolved != "" {
		t.Errorf("expected empty string for missing planning dir, got %s", resolved)
	}
}

func TestExplicitSchemaDir(t *testing.T) {
	clearEnv := func() {
		os.Unsetenv(EnvPlanningDir)
		os.Unsetenv(EnvTargetDir)
		os.Unsetenv(EnvLogLevel)
		os.Unsetenv(EnvSchemaDir)
		os.Unsetenv(EnvDebug)
		Reset()
	}

	clearEnv()
	os.Setenv(EnvSchemaDir, "/explicit/schemas")
	defer clearEnv()

	cfg := Get()
	if cfg.ResolveSchemaDir() != "/explicit/schemas" {
		t.Errorf("expected /explicit/schemas, got %s", cfg.ResolveSchemaDir())
	}
}
