package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGitRepo(t *testing.T) {
	t.Run("valid git repository", func(t *testing.T) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}

		repoRoot, err := EnsureGitRepo(cwd)
		if err != nil {
			t.Fatalf("EnsureGitRepo failed for valid repo: %v", err)
		}

		if repoRoot == "" {
			t.Error("EnsureGitRepo returned empty repo root")
		}

		gitDir := filepath.Join(repoRoot, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			t.Errorf("returned repo root %s does not contain .git directory", repoRoot)
		}
	})

	t.Run("file inside git repository", func(t *testing.T) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}

		testFile := filepath.Join(cwd, "git.go")
		repoRoot, err := EnsureGitRepo(testFile)
		if err != nil {
			t.Fatalf("EnsureGitRepo failed for file in repo: %v", err)
		}

		if repoRoot == "" {
			t.Error("EnsureGitRepo returned empty repo root for file")
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		_, err := EnsureGitRepo("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("EnsureGitRepo should fail for non-existent path")
		}

		gitErr, ok := err.(*GitRepoError)
		if !ok {
			t.Errorf("expected GitRepoError, got %T", err)
		}
		if gitErr.Message != "path does not exist" {
			t.Errorf("expected 'path does not exist' message, got: %s", gitErr.Message)
		}
	})

	t.Run("path outside git repository", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "not-a-git-repo-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		_, err = EnsureGitRepo(tmpDir)
		if err == nil {
			t.Error("EnsureGitRepo should fail for path outside git repo")
		}

		gitErr, ok := err.(*GitRepoError)
		if !ok {
			t.Errorf("expected GitRepoError, got %T", err)
		}
		if gitErr.Message != "not inside a git repository" {
			t.Errorf("expected 'not inside a git repository' message, got: %s", gitErr.Message)
		}
	})
}

func TestIsGitRepo(t *testing.T) {
	t.Run("valid git repository", func(t *testing.T) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}

		if !IsGitRepo(cwd) {
			t.Error("IsGitRepo should return true for valid git repo")
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "not-a-git-repo-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		if IsGitRepo(tmpDir) {
			t.Error("IsGitRepo should return false for non-git directory")
		}
	})
}

func TestGitRepoError(t *testing.T) {
	err := &GitRepoError{
		Path:    "/some/path",
		Message: "test error message",
	}

	expected := "git repository error at /some/path: test error message"
	if err.Error() != expected {
		t.Errorf("unexpected error string: got %q, want %q", err.Error(), expected)
	}
}
