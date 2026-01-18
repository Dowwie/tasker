package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitRepoError represents an error related to git repository operations.
type GitRepoError struct {
	Path    string
	Message string
}

func (e *GitRepoError) Error() string {
	return fmt.Sprintf("git repository error at %s: %s", e.Path, e.Message)
}

// EnsureGitRepo verifies that the specified path is within a valid git repository.
// It returns the root directory of the git repository if found, or an error if not.
func EnsureGitRepo(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", &GitRepoError{Path: path, Message: fmt.Sprintf("failed to resolve path: %v", err)}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &GitRepoError{Path: absPath, Message: "path does not exist"}
		}
		return "", &GitRepoError{Path: absPath, Message: fmt.Sprintf("failed to stat path: %v", err)}
	}

	checkDir := absPath
	if !info.IsDir() {
		checkDir = filepath.Dir(absPath)
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = checkDir
	output, err := cmd.Output()
	if err != nil {
		return "", &GitRepoError{Path: absPath, Message: "not inside a git repository"}
	}

	repoRoot := filepath.Clean(string(output[:len(output)-1]))
	return repoRoot, nil
}

// IsGitRepo checks if the specified path is inside a git repository.
func IsGitRepo(path string) bool {
	_, err := EnsureGitRepo(path)
	return err == nil
}
