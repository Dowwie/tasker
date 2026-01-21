package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchivePlanning_CreatesTimestampedArchive(t *testing.T) {
	tmpDir := t.TempDir()

	planningDir := filepath.Join(tmpDir, ".tasker")
	if err := os.MkdirAll(planningDir, 0755); err != nil {
		t.Fatalf("failed to create planning dir: %v", err)
	}

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current":   "executing",
			"completed": []string{"ingestion", "logical"},
		},
		"target_dir": "/some/target",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{"id": "T001", "status": "complete"},
			"T002": map[string]interface{}{"id": "T002", "status": "running"},
			"T003": map[string]interface{}{"id": "T003", "status": "pending"},
		},
	}
	stateData, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(planningDir, "state.json"), stateData, 0644); err != nil {
		t.Fatalf("failed to write state.json: %v", err)
	}

	inputsDir := filepath.Join(planningDir, "inputs")
	if err := os.MkdirAll(inputsDir, 0755); err != nil {
		t.Fatalf("failed to create inputs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputsDir, "spec.md"), []byte("# Spec"), 0644); err != nil {
		t.Fatalf("failed to write spec.md: %v", err)
	}

	archiveRoot := filepath.Join(tmpDir, "archive")
	projectName := "test-project"

	result, err := ArchivePlanning(planningDir, archiveRoot, projectName)
	if err != nil {
		t.Fatalf("ArchivePlanning failed: %v", err)
	}

	if result.ArchiveID == "" {
		t.Error("ArchiveID should not be empty")
	}

	if !strings.Contains(result.ArchiveID, "_") {
		t.Errorf("ArchiveID should be timestamp format YYYYMMDD_HHMMSS, got %s", result.ArchiveID)
	}

	if result.ArchivedAt == "" {
		t.Error("ArchivedAt should not be empty")
	}

	expectedPath := filepath.Join(archiveRoot, projectName, "planning", result.ArchiveID)
	if result.ArchivePath != expectedPath {
		t.Errorf("ArchivePath = %s, want %s", result.ArchivePath, expectedPath)
	}

	if _, err := os.Stat(result.ArchivePath); os.IsNotExist(err) {
		t.Error("Archive directory was not created")
	}

	archivedState := filepath.Join(result.ArchivePath, "state.json")
	if _, err := os.Stat(archivedState); os.IsNotExist(err) {
		t.Error("state.json was not archived")
	}

	archivedInputs := filepath.Join(result.ArchivePath, "inputs", "spec.md")
	if _, err := os.Stat(archivedInputs); os.IsNotExist(err) {
		t.Error("inputs/spec.md was not archived")
	}

	manifestPath := filepath.Join(result.ArchivePath, "archive-manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var manifest ArchiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest.Version != ArchiveVersion {
		t.Errorf("manifest version = %s, want %s", manifest.Version, ArchiveVersion)
	}
	if manifest.ArchiveType != ArchiveTypePlanning {
		t.Errorf("manifest archive_type = %s, want %s", manifest.ArchiveType, ArchiveTypePlanning)
	}
	if manifest.ProjectName != projectName {
		t.Errorf("manifest project_name = %s, want %s", manifest.ProjectName, projectName)
	}
	if manifest.PhaseAtArchive != "executing" {
		t.Errorf("manifest phase_at_archive = %s, want executing", manifest.PhaseAtArchive)
	}
	if manifest.TaskSummary == nil {
		t.Fatal("manifest task_summary should not be nil")
	}
	if manifest.TaskSummary.Total != 3 {
		t.Errorf("manifest task_summary.total = %d, want 3", manifest.TaskSummary.Total)
	}
	if manifest.TaskSummary.ByStatus["complete"] != 1 {
		t.Errorf("manifest task_summary.by_status[complete] = %d, want 1", manifest.TaskSummary.ByStatus["complete"])
	}
}

func TestArchivePlanning_PlanningDirNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "nonexistent")
	archiveRoot := filepath.Join(tmpDir, "archive")

	_, err := ArchivePlanning(nonExistentDir, archiveRoot, "test")
	if err == nil {
		t.Fatal("expected error for non-existent planning dir")
	}

	archiveErr, ok := err.(*ArchiveError)
	if !ok {
		t.Fatalf("expected ArchiveError, got %T", err)
	}
	if archiveErr.Op != "stat" {
		t.Errorf("error op = %s, want stat", archiveErr.Op)
	}
}

func TestArchivePlanning_NoStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	planningDir := filepath.Join(tmpDir, ".tasker")
	if err := os.MkdirAll(planningDir, 0755); err != nil {
		t.Fatalf("failed to create planning dir: %v", err)
	}

	archiveRoot := filepath.Join(tmpDir, "archive")

	_, err := ArchivePlanning(planningDir, archiveRoot, "test")
	if err == nil {
		t.Fatal("expected error for missing state.json")
	}

	archiveErr, ok := err.(*ArchiveError)
	if !ok {
		t.Fatalf("expected ArchiveError, got %T", err)
	}
	if archiveErr.Op != "read" {
		t.Errorf("error op = %s, want read", archiveErr.Op)
	}
}

func TestArchivePlanning_EmptyDirectoriesSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	planningDir := filepath.Join(tmpDir, ".tasker")
	if err := os.MkdirAll(planningDir, 0755); err != nil {
		t.Fatalf("failed to create planning dir: %v", err)
	}

	state := map[string]interface{}{
		"version": "2.0",
		"phase":   map[string]interface{}{"current": "ready"},
		"tasks":   map[string]interface{}{},
	}
	stateData, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(planningDir, "state.json"), stateData, 0644); err != nil {
		t.Fatalf("failed to write state.json: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(planningDir, "inputs"), 0755); err != nil {
		t.Fatalf("failed to create empty inputs dir: %v", err)
	}

	archiveRoot := filepath.Join(tmpDir, "archive")
	result, err := ArchivePlanning(planningDir, archiveRoot, "test")
	if err != nil {
		t.Fatalf("ArchivePlanning failed: %v", err)
	}

	archivedInputs := filepath.Join(result.ArchivePath, "inputs")
	if _, err := os.Stat(archivedInputs); !os.IsNotExist(err) {
		t.Error("empty inputs directory should not be archived")
	}
}

func TestArchivePlanning_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	planningDir := filepath.Join(tmpDir, ".tasker")
	if err := os.MkdirAll(planningDir, 0755); err != nil {
		t.Fatalf("failed to create planning dir: %v", err)
	}

	state := map[string]interface{}{
		"version": "2.0",
		"phase":   map[string]interface{}{"current": "ready"},
		"tasks":   map[string]interface{}{},
	}
	stateData, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(planningDir, "state.json"), stateData, 0644); err != nil {
		t.Fatalf("failed to write state.json: %v", err)
	}

	nestedDir := filepath.Join(planningDir, "artifacts", "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "nested.txt"), []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}

	archiveRoot := filepath.Join(tmpDir, "archive")
	result, err := ArchivePlanning(planningDir, archiveRoot, "test")
	if err != nil {
		t.Fatalf("ArchivePlanning failed: %v", err)
	}

	nestedArchived := filepath.Join(result.ArchivePath, "artifacts", "subdir", "nested.txt")
	if _, err := os.Stat(nestedArchived); os.IsNotExist(err) {
		t.Error("nested file should be archived")
	}

	content, err := os.ReadFile(nestedArchived)
	if err != nil {
		t.Fatalf("failed to read archived nested file: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("archived content = %s, want 'nested content'", string(content))
	}
}

func TestArchiveError_Format(t *testing.T) {
	err := &ArchiveError{
		Path:    "/some/path",
		Op:      "read",
		Message: "file not found",
	}

	expected := "archive read failed for /some/path: file not found"
	if err.Error() != expected {
		t.Errorf("error string = %s, want %s", err.Error(), expected)
	}
}
