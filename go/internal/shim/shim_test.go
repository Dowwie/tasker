package shim

import (
	"os"
	"testing"
)

func TestTranslateArgs_StateCommands(t *testing.T) {
	mappings := DefaultMappings()

	tests := []struct {
		name           string
		scriptName     string
		args           []string
		wantSubcommand string
		wantArgs       []string
		wantErr        bool
	}{
		{
			name:           "state status",
			scriptName:     "state.py",
			args:           []string{"status"},
			wantSubcommand: "state",
			wantArgs:       []string{"status"},
			wantErr:        false,
		},
		{
			name:           "state init with target",
			scriptName:     "state.py",
			args:           []string{"init", "/path/to/project"},
			wantSubcommand: "state",
			wantArgs:       []string{"init", "/path/to/project"},
			wantErr:        false,
		},
		{
			name:           "start-task translates to task start",
			scriptName:     "state.py",
			args:           []string{"start-task", "T001"},
			wantSubcommand: "state",
			wantArgs:       []string{"task", "start", "T001"},
			wantErr:        false,
		},
		{
			name:           "complete-task with flags",
			scriptName:     "state.py",
			args:           []string{"complete-task", "T001", "--created", "file1.go", "file2.go", "--modified", "file3.go"},
			wantSubcommand: "state",
			wantArgs:       []string{"task", "complete", "T001", "--created", "file1.go", "file2.go", "--modified", "file3.go"},
			wantErr:        false,
		},
		{
			name:           "fail-task with category",
			scriptName:     "state.py",
			args:           []string{"fail-task", "T001", "error message", "--category", "test"},
			wantSubcommand: "state",
			wantArgs:       []string{"task", "fail", "T001", "error message", "--category", "test"},
			wantErr:        false,
		},
		{
			name:           "fail-task with no-retry translates to retryable=false",
			scriptName:     "state.py",
			args:           []string{"fail-task", "T001", "error message", "--no-retry"},
			wantSubcommand: "state",
			wantArgs:       []string{"task", "fail", "T001", "error message", "--retryable=false"},
			wantErr:        false,
		},
		{
			name:           "retry-task",
			scriptName:     "state.py",
			args:           []string{"retry-task", "T001"},
			wantSubcommand: "state",
			wantArgs:       []string{"task", "retry", "T001"},
			wantErr:        false,
		},
		{
			name:           "skip-task with reason",
			scriptName:     "state.py",
			args:           []string{"skip-task", "T001", "--reason", "not needed"},
			wantSubcommand: "state",
			wantArgs:       []string{"task", "skip", "T001", "--reason", "not needed"},
			wantErr:        false,
		},
		{
			name:           "ready-tasks",
			scriptName:     "state.py",
			args:           []string{"ready-tasks"},
			wantSubcommand: "state",
			wantArgs:       []string{"ready"},
			wantErr:        false,
		},
		{
			name:           "no args returns base command",
			scriptName:     "state.py",
			args:           []string{},
			wantSubcommand: "state",
			wantArgs:       []string{},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TranslateArgs(tt.scriptName, tt.args, mappings)

			if (err != nil) != tt.wantErr {
				t.Errorf("TranslateArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if result.Subcommand != tt.wantSubcommand {
				t.Errorf("TranslateArgs() Subcommand = %v, want %v", result.Subcommand, tt.wantSubcommand)
			}

			if len(result.Args) != len(tt.wantArgs) {
				t.Errorf("TranslateArgs() Args length = %v, want %v", len(result.Args), len(tt.wantArgs))
				t.Errorf("Got: %v, Want: %v", result.Args, tt.wantArgs)
				return
			}

			for i, arg := range result.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("TranslateArgs() Args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestTranslateArgs_ValidateCommands(t *testing.T) {
	mappings := DefaultMappings()

	tests := []struct {
		name           string
		args           []string
		wantSubcommand string
		wantArgs       []string
	}{
		{
			name:           "validate dag",
			args:           []string{"dag"},
			wantSubcommand: "validate",
			wantArgs:       []string{"dag"},
		},
		{
			name:           "validate steel-thread",
			args:           []string{"steel-thread"},
			wantSubcommand: "validate",
			wantArgs:       []string{"steel-thread"},
		},
		{
			name:           "validate gates",
			args:           []string{"gates"},
			wantSubcommand: "validate",
			wantArgs:       []string{"gates"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TranslateArgs("validate.py", tt.args, mappings)

			if err != nil {
				t.Errorf("TranslateArgs() unexpected error: %v", err)
				return
			}

			if result.Subcommand != tt.wantSubcommand {
				t.Errorf("TranslateArgs() Subcommand = %v, want %v", result.Subcommand, tt.wantSubcommand)
			}

			if len(result.Args) != len(tt.wantArgs) {
				t.Errorf("TranslateArgs() Args length = %v, want %v", len(result.Args), len(tt.wantArgs))
				return
			}

			for i, arg := range result.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("TranslateArgs() Args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestTranslateArgs_BundleCommands(t *testing.T) {
	mappings := DefaultMappings()

	tests := []struct {
		name           string
		args           []string
		wantSubcommand string
		wantArgs       []string
	}{
		{
			name:           "bundle generate",
			args:           []string{"generate", "T001"},
			wantSubcommand: "bundle",
			wantArgs:       []string{"generate", "T001"},
		},
		{
			name:           "bundle generate-ready",
			args:           []string{"generate-ready"},
			wantSubcommand: "bundle",
			wantArgs:       []string{"generate-ready"},
		},
		{
			name:           "bundle validate",
			args:           []string{"validate", "T001"},
			wantSubcommand: "bundle",
			wantArgs:       []string{"validate", "T001"},
		},
		{
			name:           "bundle validate-integrity",
			args:           []string{"validate-integrity", "T001"},
			wantSubcommand: "bundle",
			wantArgs:       []string{"validate-integrity", "T001"},
		},
		{
			name:           "bundle list",
			args:           []string{"list"},
			wantSubcommand: "bundle",
			wantArgs:       []string{"list"},
		},
		{
			name:           "bundle clean",
			args:           []string{"clean"},
			wantSubcommand: "bundle",
			wantArgs:       []string{"clean"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TranslateArgs("bundle.py", tt.args, mappings)

			if err != nil {
				t.Errorf("TranslateArgs() unexpected error: %v", err)
				return
			}

			if result.Subcommand != tt.wantSubcommand {
				t.Errorf("TranslateArgs() Subcommand = %v, want %v", result.Subcommand, tt.wantSubcommand)
			}

			if len(result.Args) != len(tt.wantArgs) {
				t.Errorf("TranslateArgs() Args length = %v, want %v", len(result.Args), len(tt.wantArgs))
				return
			}

			for i, arg := range result.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("TranslateArgs() Args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestTranslateArgs_UnknownScript(t *testing.T) {
	mappings := DefaultMappings()

	_, err := TranslateArgs("unknown.py", []string{"command"}, mappings)
	if err == nil {
		t.Errorf("TranslateArgs() expected error for unknown script")
	}

	if !isUnknownScriptError(err) {
		t.Errorf("TranslateArgs() expected ErrUnknownScript, got %v", err)
	}
}

func TestTranslateArgs_UnknownCommand(t *testing.T) {
	mappings := DefaultMappings()

	result, err := TranslateArgs("state.py", []string{"unknown-command", "arg1"}, mappings)
	if err != nil {
		t.Errorf("TranslateArgs() unexpected error: %v", err)
		return
	}

	if result.Subcommand != "state" {
		t.Errorf("TranslateArgs() Subcommand = %v, want state", result.Subcommand)
	}
	if len(result.Args) != 2 || result.Args[0] != "unknown-command" {
		t.Errorf("TranslateArgs() Args = %v, want [unknown-command arg1]", result.Args)
	}
}

func isUnknownScriptError(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "unknown script name")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExecBinary_NotFound(t *testing.T) {
	result := &TranslationResult{
		Subcommand: "state",
		Args:       []string{"status"},
	}

	_, err := ExecBinary("/nonexistent/binary/path", result)
	if err == nil {
		t.Errorf("ExecBinary() expected error for nonexistent binary")
	}
}

func TestHandleError_BinaryNotFound(t *testing.T) {
	msg := HandleError(ErrBinaryNotFound)

	if !contains(msg, "tasker binary not found") {
		t.Errorf("HandleError() message should contain 'tasker binary not found', got: %s", msg)
	}

	if !contains(msg, "TASKER_BINARY") {
		t.Errorf("HandleError() message should mention TASKER_BINARY env var, got: %s", msg)
	}
}

func TestHandleError_OtherError(t *testing.T) {
	err := ErrUnknownScript
	msg := HandleError(err)

	if !contains(msg, "Error:") {
		t.Errorf("HandleError() message should start with 'Error:', got: %s", msg)
	}
}

func TestShouldUseGoBinary_WithEnvVar(t *testing.T) {
	originalValue := os.Getenv("USE_PYTHON_IMPL")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("USE_PYTHON_IMPL")
		} else {
			os.Setenv("USE_PYTHON_IMPL", originalValue)
		}
	}()

	os.Setenv("USE_PYTHON_IMPL", "1")

	if ShouldUseGoBinary() {
		t.Errorf("ShouldUseGoBinary() should return false when USE_PYTHON_IMPL=1")
	}
}

func TestFindBinary_WithEnvVar(t *testing.T) {
	originalValue := os.Getenv("TASKER_BINARY")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalValue)
		}
	}()

	os.Setenv("TASKER_BINARY", "/nonexistent/path/to/tasker")

	_, err := FindBinary()
	if err == nil {
		t.Logf("FindBinary() with invalid TASKER_BINARY should fall through to PATH check")
	}
}

func TestDefaultMappings(t *testing.T) {
	mappings := DefaultMappings()

	expectedScripts := []string{"state.py", "validate.py", "bundle.py"}
	for _, script := range expectedScripts {
		if _, ok := mappings[script]; !ok {
			t.Errorf("DefaultMappings() missing mapping for %s", script)
		}
	}

	stateMapping := mappings["state.py"]
	if stateMapping.GoCommand != "state" {
		t.Errorf("state.py GoCommand = %v, want 'state'", stateMapping.GoCommand)
	}

	expectedStateSubcommands := []string{"init", "status", "start-task", "complete-task", "fail-task", "retry-task", "skip-task", "ready-tasks"}
	for _, cmd := range expectedStateSubcommands {
		if _, ok := stateMapping.SubCommands[cmd]; !ok {
			t.Errorf("state.py missing subcommand mapping for %s", cmd)
		}
	}

	validateMapping := mappings["validate.py"]
	if validateMapping.GoCommand != "validate" {
		t.Errorf("validate.py GoCommand = %v, want 'validate'", validateMapping.GoCommand)
	}

	bundleMapping := mappings["bundle.py"]
	if bundleMapping.GoCommand != "bundle" {
		t.Errorf("bundle.py GoCommand = %v, want 'bundle'", bundleMapping.GoCommand)
	}
}

func TestRunShim_BinaryNotFound(t *testing.T) {
	originalValue := os.Getenv("TASKER_BINARY")
	originalPath := os.Getenv("PATH")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalValue)
		}
		os.Setenv("PATH", originalPath)
	}()

	os.Unsetenv("TASKER_BINARY")
	os.Setenv("PATH", "/nonexistent")

	_, exitCode, err := RunShim("state.py", []string{"status"})
	if err == nil {
		t.Error("RunShim() expected error when binary not found")
	}
	if exitCode != 1 {
		t.Errorf("RunShim() exitCode = %d, want 1", exitCode)
	}
}

func TestRunShim_UnknownScript(t *testing.T) {
	_, exitCode, err := RunShim("unknown.py", []string{"command"})
	if err == nil {
		t.Error("RunShim() expected error for unknown script")
	}
	if exitCode != 1 {
		t.Errorf("RunShim() exitCode = %d, want 1", exitCode)
	}
}

func TestFindBinary_FromEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := tmpDir + "/tasker"
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	originalValue := os.Getenv("TASKER_BINARY")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalValue)
		}
	}()

	os.Setenv("TASKER_BINARY", binaryPath)

	found, err := FindBinary()
	if err != nil {
		t.Errorf("FindBinary() unexpected error: %v", err)
	}
	if found != binaryPath {
		t.Errorf("FindBinary() = %s, want %s", found, binaryPath)
	}
}

func TestFindBinary_NotFound(t *testing.T) {
	originalBinary := os.Getenv("TASKER_BINARY")
	originalPath := os.Getenv("PATH")
	defer func() {
		if originalBinary == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalBinary)
		}
		os.Setenv("PATH", originalPath)
	}()

	os.Unsetenv("TASKER_BINARY")
	os.Setenv("PATH", "/nonexistent/path/only")

	_, err := FindBinary()
	if err == nil {
		t.Error("FindBinary() expected error when binary not found")
	}
	if err != ErrBinaryNotFound {
		t.Errorf("FindBinary() error = %v, want ErrBinaryNotFound", err)
	}
}

func TestShouldUseGoBinary_WhenBinaryExists(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := tmpDir + "/tasker"
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	originalBinary := os.Getenv("TASKER_BINARY")
	originalPython := os.Getenv("USE_PYTHON_IMPL")
	defer func() {
		if originalBinary == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalBinary)
		}
		if originalPython == "" {
			os.Unsetenv("USE_PYTHON_IMPL")
		} else {
			os.Setenv("USE_PYTHON_IMPL", originalPython)
		}
	}()

	os.Setenv("TASKER_BINARY", binaryPath)
	os.Unsetenv("USE_PYTHON_IMPL")

	if !ShouldUseGoBinary() {
		t.Error("ShouldUseGoBinary() should return true when binary exists and USE_PYTHON_IMPL is not set")
	}
}

func TestShouldUseGoBinary_WhenBinaryNotExists(t *testing.T) {
	originalBinary := os.Getenv("TASKER_BINARY")
	originalPython := os.Getenv("USE_PYTHON_IMPL")
	originalPath := os.Getenv("PATH")
	defer func() {
		if originalBinary == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalBinary)
		}
		if originalPython == "" {
			os.Unsetenv("USE_PYTHON_IMPL")
		} else {
			os.Setenv("USE_PYTHON_IMPL", originalPython)
		}
		os.Setenv("PATH", originalPath)
	}()

	os.Unsetenv("TASKER_BINARY")
	os.Unsetenv("USE_PYTHON_IMPL")
	os.Setenv("PATH", "/nonexistent/path/only")

	if ShouldUseGoBinary() {
		t.Error("ShouldUseGoBinary() should return false when binary does not exist")
	}
}

func TestRunShim_SuccessfulExecution(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := tmpDir + "/tasker"

	script := `#!/bin/sh
echo "success"
exit 0
`
	if err := os.WriteFile(binaryPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	originalValue := os.Getenv("TASKER_BINARY")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalValue)
		}
	}()

	os.Setenv("TASKER_BINARY", binaryPath)

	output, exitCode, err := RunShim("state.py", []string{"status"})
	if err != nil {
		t.Errorf("RunShim() unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("RunShim() exitCode = %d, want 0", exitCode)
	}
	if !contains(string(output), "success") {
		t.Errorf("RunShim() output = %s, want to contain 'success'", string(output))
	}
}

func TestRunShim_NonZeroExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := tmpDir + "/tasker"

	script := `#!/bin/sh
echo "error output"
exit 42
`
	if err := os.WriteFile(binaryPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	originalValue := os.Getenv("TASKER_BINARY")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalValue)
		}
	}()

	os.Setenv("TASKER_BINARY", binaryPath)

	output, exitCode, err := RunShim("state.py", []string{"status"})
	if err != nil {
		t.Errorf("RunShim() unexpected error for non-zero exit: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("RunShim() exitCode = %d, want 42", exitCode)
	}
	if !contains(string(output), "error output") {
		t.Errorf("RunShim() output = %s, want to contain 'error output'", string(output))
	}
}

func TestFindBinary_FromPath(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := tmpDir + "/tasker"

	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	originalBinary := os.Getenv("TASKER_BINARY")
	originalPath := os.Getenv("PATH")
	defer func() {
		if originalBinary == "" {
			os.Unsetenv("TASKER_BINARY")
		} else {
			os.Setenv("TASKER_BINARY", originalBinary)
		}
		os.Setenv("PATH", originalPath)
	}()

	os.Unsetenv("TASKER_BINARY")
	os.Setenv("PATH", tmpDir+":"+originalPath)

	found, err := FindBinary()
	if err != nil {
		t.Errorf("FindBinary() unexpected error: %v", err)
	}
	if found != binaryPath {
		t.Errorf("FindBinary() = %s, want %s", found, binaryPath)
	}
}
