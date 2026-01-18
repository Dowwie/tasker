// Package shim provides utilities for translating Python script arguments to Go subcommand format.
// This enables backward compatibility by allowing existing Python scripts to forward calls to the Go binary.
package shim

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TranslationResult holds the translated command and arguments.
type TranslationResult struct {
	Subcommand string
	Args       []string
}

// ScriptMapping defines how a Python script maps to Go subcommands.
type ScriptMapping struct {
	ScriptName  string
	GoCommand   string
	SubCommands map[string]SubCommandMapping
}

// SubCommandMapping defines how a Python subcommand maps to Go.
type SubCommandMapping struct {
	GoSubCommand string
	FlagMapping  map[string]string
}

var (
	// ErrBinaryNotFound is returned when the Go binary cannot be located.
	ErrBinaryNotFound = errors.New("tasker binary not found")

	// ErrUnknownScript is returned when an unknown script is provided.
	ErrUnknownScript = errors.New("unknown script name")

	// ErrUnknownCommand is returned when an unknown command is provided.
	ErrUnknownCommand = errors.New("unknown command")
)

// DefaultMappings returns the default script-to-command mappings.
func DefaultMappings() map[string]ScriptMapping {
	return map[string]ScriptMapping{
		"state.py": {
			ScriptName: "state.py",
			GoCommand:  "state",
			SubCommands: map[string]SubCommandMapping{
				"init":          {GoSubCommand: "init"},
				"status":        {GoSubCommand: "status"},
				"start-task":    {GoSubCommand: "task start"},
				"complete-task": {GoSubCommand: "task complete", FlagMapping: map[string]string{"--created": "--created", "--modified": "--modified"}},
				"fail-task":     {GoSubCommand: "task fail", FlagMapping: map[string]string{"--category": "--category", "--no-retry": "--retryable=false", "--retryable": "--retryable"}},
				"retry-task":    {GoSubCommand: "task retry"},
				"skip-task":     {GoSubCommand: "task skip", FlagMapping: map[string]string{"--reason": "--reason"}},
				"ready-tasks":   {GoSubCommand: "ready"},
			},
		},
		"validate.py": {
			ScriptName: "validate.py",
			GoCommand:  "validate",
			SubCommands: map[string]SubCommandMapping{
				"dag":          {GoSubCommand: "dag"},
				"steel-thread": {GoSubCommand: "steel-thread"},
				"gates":        {GoSubCommand: "gates"},
			},
		},
		"bundle.py": {
			ScriptName: "bundle.py",
			GoCommand:  "bundle",
			SubCommands: map[string]SubCommandMapping{
				"generate":           {GoSubCommand: "generate"},
				"generate-ready":     {GoSubCommand: "generate-ready"},
				"validate":           {GoSubCommand: "validate"},
				"validate-integrity": {GoSubCommand: "validate-integrity"},
				"list":               {GoSubCommand: "list"},
				"clean":              {GoSubCommand: "clean"},
			},
		},
	}
}

// TranslateArgs converts Python script arguments to Go subcommand format.
func TranslateArgs(scriptName string, args []string, mappings map[string]ScriptMapping) (*TranslationResult, error) {
	mapping, ok := mappings[scriptName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownScript, scriptName)
	}

	if len(args) == 0 {
		return &TranslationResult{
			Subcommand: mapping.GoCommand,
			Args:       []string{},
		}, nil
	}

	cmdName := args[0]
	subCmdMapping, ok := mapping.SubCommands[cmdName]
	if !ok {
		return &TranslationResult{
			Subcommand: mapping.GoCommand,
			Args:       args,
		}, nil
	}

	result := &TranslationResult{
		Subcommand: mapping.GoCommand,
		Args:       []string{},
	}

	subCmdParts := strings.Fields(subCmdMapping.GoSubCommand)
	result.Args = append(result.Args, subCmdParts...)

	remainingArgs := args[1:]
	result.Args = append(result.Args, translateFlags(remainingArgs, subCmdMapping.FlagMapping)...)

	return result, nil
}

// translateFlags converts Python-style flags to Go-style flags.
func translateFlags(args []string, flagMapping map[string]string) []string {
	if flagMapping == nil {
		return args
	}

	result := make([]string, 0, len(args))
	i := 0
	for i < len(args) {
		arg := args[i]

		if goFlag, ok := flagMapping[arg]; ok {
			if strings.Contains(goFlag, "=") {
				result = append(result, goFlag)
			} else {
				result = append(result, goFlag)
			}
		} else {
			result = append(result, arg)
		}
		i++
	}

	return result
}

// FindBinary locates the tasker Go binary.
// It searches in the following order:
// 1. TASKER_BINARY environment variable
// 2. Same directory as the calling script (go/bin/tasker)
// 3. System PATH
func FindBinary() (string, error) {
	if envBinary := os.Getenv("TASKER_BINARY"); envBinary != "" {
		if _, err := os.Stat(envBinary); err == nil {
			return envBinary, nil
		}
	}

	execPath, err := os.Executable()
	if err == nil {
		possiblePaths := []string{
			filepath.Join(filepath.Dir(execPath), "tasker"),
			filepath.Join(filepath.Dir(execPath), "..", "go", "bin", "tasker"),
			filepath.Join(filepath.Dir(execPath), "..", "bin", "tasker"),
		}
		for _, p := range possiblePaths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	if path, err := exec.LookPath("tasker"); err == nil {
		return path, nil
	}

	return "", ErrBinaryNotFound
}

// ExecBinary executes the Go binary with the translated arguments.
// It returns the combined stdout/stderr output and any error.
func ExecBinary(binaryPath string, result *TranslationResult) ([]byte, error) {
	args := []string{result.Subcommand}
	args = append(args, result.Args...)

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin

	output, err := cmd.CombinedOutput()
	return output, err
}

// RunShim is a convenience function that performs the full shim workflow:
// 1. Finds the Go binary
// 2. Translates arguments
// 3. Executes the binary
// Returns the output, exit code, and any error.
func RunShim(scriptName string, args []string) (output []byte, exitCode int, err error) {
	binaryPath, err := FindBinary()
	if err != nil {
		return nil, 1, fmt.Errorf("binary not found: %w", err)
	}

	mappings := DefaultMappings()
	result, err := TranslateArgs(scriptName, args, mappings)
	if err != nil {
		return nil, 1, fmt.Errorf("translation failed: %w", err)
	}

	output, err = ExecBinary(binaryPath, result)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return output, exitErr.ExitCode(), nil
		}
		return output, 1, err
	}

	return output, 0, nil
}

// HandleError formats an error message for when the binary is not found.
func HandleError(err error) string {
	if errors.Is(err, ErrBinaryNotFound) {
		return `Error: tasker binary not found.

The Go binary 'tasker' could not be located. Please ensure:
1. The binary has been built: cd go && go build -o bin/tasker ./cmd/tasker
2. The binary is in your PATH, or
3. Set TASKER_BINARY environment variable to the binary path

Falling back to Python implementation.`
	}

	return fmt.Sprintf("Error: %v", err)
}

// ShouldUseGoBinary determines if the Go binary should be used.
// Returns true if the binary exists and USE_PYTHON_IMPL is not set.
func ShouldUseGoBinary() bool {
	if os.Getenv("USE_PYTHON_IMPL") == "1" {
		return false
	}

	_, err := FindBinary()
	return err == nil
}
