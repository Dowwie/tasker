package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDirectories(t *testing.T) {
	t.Run("creates planning directory with subdirs", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "init-dirs-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		planningPath, err := InitDirectories(tmpDir, "")
		if err != nil {
			t.Fatalf("InitDirectories failed: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, PlanningDirName)
		if planningPath != expectedPath {
			t.Errorf("unexpected planning path: got %s, want %s", planningPath, expectedPath)
		}

		info, err := os.Stat(planningPath)
		if err != nil {
			t.Fatalf("failed to stat planning dir: %v", err)
		}
		if !info.IsDir() {
			t.Error("planning path is not a directory")
		}

		for _, subdir := range PlanningSubdirs {
			subdirPath := filepath.Join(planningPath, subdir)
			info, err := os.Stat(subdirPath)
			if err != nil {
				t.Errorf("subdirectory %s does not exist: %v", subdir, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("subdirectory %s is not a directory", subdir)
			}
		}
	})

	t.Run("creates custom named planning directory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "init-dirs-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		customName := "custom-planning"
		planningPath, err := InitDirectories(tmpDir, customName)
		if err != nil {
			t.Fatalf("InitDirectories failed: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, customName)
		if planningPath != expectedPath {
			t.Errorf("unexpected planning path: got %s, want %s", planningPath, expectedPath)
		}

		if _, err := os.Stat(planningPath); os.IsNotExist(err) {
			t.Error("custom planning directory was not created")
		}
	})

	t.Run("fails for non-existent root", func(t *testing.T) {
		_, err := InitDirectories("/nonexistent/root/path", "")
		if err == nil {
			t.Error("InitDirectories should fail for non-existent root")
		}

		dirErr, ok := err.(*DirectoryError)
		if !ok {
			t.Errorf("expected DirectoryError, got %T", err)
		}
		if dirErr.Op != "stat" {
			t.Errorf("expected op 'stat', got: %s", dirErr.Op)
		}
	})

	t.Run("fails when root is a file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "init-dirs-test-file-*")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		_, err = InitDirectories(tmpFile.Name(), "")
		if err == nil {
			t.Error("InitDirectories should fail when root is a file")
		}

		dirErr, ok := err.(*DirectoryError)
		if !ok {
			t.Errorf("expected DirectoryError, got %T", err)
		}
		if dirErr.Op != "validate" {
			t.Errorf("expected op 'validate', got: %s", dirErr.Op)
		}
	})

	t.Run("idempotent - succeeds if already exists", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "init-dirs-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		path1, err := InitDirectories(tmpDir, "")
		if err != nil {
			t.Fatalf("first InitDirectories failed: %v", err)
		}

		path2, err := InitDirectories(tmpDir, "")
		if err != nil {
			t.Fatalf("second InitDirectories failed: %v", err)
		}

		if path1 != path2 {
			t.Errorf("paths differ between calls: %s vs %s", path1, path2)
		}
	})
}

func TestEnsurePlanningDir(t *testing.T) {
	t.Run("valid planning directory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ensure-planning-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		planningPath, err := InitDirectories(tmpDir, "")
		if err != nil {
			t.Fatalf("InitDirectories failed: %v", err)
		}

		err = EnsurePlanningDir(planningPath)
		if err != nil {
			t.Errorf("EnsurePlanningDir failed for valid dir: %v", err)
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		err := EnsurePlanningDir("/nonexistent/planning/dir")
		if err == nil {
			t.Error("EnsurePlanningDir should fail for non-existent dir")
		}

		dirErr, ok := err.(*DirectoryError)
		if !ok {
			t.Errorf("expected DirectoryError, got %T", err)
		}
		if dirErr.Op != "stat" {
			t.Errorf("expected op 'stat', got: %s", dirErr.Op)
		}
	})

	t.Run("missing required subdirectory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "ensure-planning-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		planningDir := filepath.Join(tmpDir, "planning")
		if err := os.MkdirAll(planningDir, 0755); err != nil {
			t.Fatalf("failed to create planning dir: %v", err)
		}

		err = EnsurePlanningDir(planningDir)
		if err == nil {
			t.Error("EnsurePlanningDir should fail when subdirs are missing")
		}

		dirErr, ok := err.(*DirectoryError)
		if !ok {
			t.Errorf("expected DirectoryError, got %T", err)
		}
		if dirErr.Op != "stat" {
			t.Errorf("expected op 'stat', got: %s", dirErr.Op)
		}
	})

	t.Run("path is a file not directory", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "ensure-planning-test-file-*")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		err = EnsurePlanningDir(tmpFile.Name())
		if err == nil {
			t.Error("EnsurePlanningDir should fail when path is a file")
		}

		dirErr, ok := err.(*DirectoryError)
		if !ok {
			t.Errorf("expected DirectoryError, got %T", err)
		}
		if dirErr.Op != "validate" {
			t.Errorf("expected op 'validate', got: %s", dirErr.Op)
		}
	})
}

func TestDirectoryError(t *testing.T) {
	err := &DirectoryError{
		Path:    "/some/path",
		Op:      "create",
		Message: "permission denied",
	}

	expected := "directory create failed for /some/path: permission denied"
	if err.Error() != expected {
		t.Errorf("unexpected error string: got %q, want %q", err.Error(), expected)
	}
}
