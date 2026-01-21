package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// TaskerDirName is the default name for the tasker working directory.
// This is created inside the target project directory (e.g., /myproject/.tasker/).
const TaskerDirName = ".tasker"

// PlanningSubdirs defines the required subdirectories within a planning directory.
var PlanningSubdirs = []string{
	"bundles",
	"artifacts",
}

// DirectoryError represents an error related to directory operations.
type DirectoryError struct {
	Path    string
	Op      string
	Message string
}

func (e *DirectoryError) Error() string {
	return fmt.Sprintf("directory %s failed for %s: %s", e.Op, e.Path, e.Message)
}

// InitDirectories creates the planning directory structure at the specified root.
// If planningDir is empty, it defaults to ".tasker" under the root.
// Returns the absolute path to the planning directory.
func InitDirectories(root string, planningDir string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", &DirectoryError{Path: root, Op: "resolve", Message: err.Error()}
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &DirectoryError{Path: absRoot, Op: "stat", Message: "root directory does not exist"}
		}
		return "", &DirectoryError{Path: absRoot, Op: "stat", Message: err.Error()}
	}
	if !info.IsDir() {
		return "", &DirectoryError{Path: absRoot, Op: "validate", Message: "root path is not a directory"}
	}

	if planningDir == "" {
		planningDir = TaskerDirName
	}

	planningPath := filepath.Join(absRoot, planningDir)
	if err := os.MkdirAll(planningPath, 0755); err != nil {
		return "", &DirectoryError{Path: planningPath, Op: "create", Message: err.Error()}
	}

	for _, subdir := range PlanningSubdirs {
		subdirPath := filepath.Join(planningPath, subdir)
		if err := os.MkdirAll(subdirPath, 0755); err != nil {
			return "", &DirectoryError{Path: subdirPath, Op: "create", Message: err.Error()}
		}
	}

	return planningPath, nil
}

// EnsurePlanningDir verifies that a planning directory exists and has the required structure.
// Returns an error if the directory or any required subdirectory is missing.
func EnsurePlanningDir(planningPath string) error {
	absPath, err := filepath.Abs(planningPath)
	if err != nil {
		return &DirectoryError{Path: planningPath, Op: "resolve", Message: err.Error()}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DirectoryError{Path: absPath, Op: "stat", Message: "planning directory does not exist"}
		}
		return &DirectoryError{Path: absPath, Op: "stat", Message: err.Error()}
	}
	if !info.IsDir() {
		return &DirectoryError{Path: absPath, Op: "validate", Message: "path is not a directory"}
	}

	for _, subdir := range PlanningSubdirs {
		subdirPath := filepath.Join(absPath, subdir)
		info, err := os.Stat(subdirPath)
		if err != nil {
			if os.IsNotExist(err) {
				return &DirectoryError{Path: subdirPath, Op: "stat", Message: fmt.Sprintf("required subdirectory %s does not exist", subdir)}
			}
			return &DirectoryError{Path: subdirPath, Op: "stat", Message: err.Error()}
		}
		if !info.IsDir() {
			return &DirectoryError{Path: subdirPath, Op: "validate", Message: fmt.Sprintf("%s is not a directory", subdir)}
		}
	}

	return nil
}
