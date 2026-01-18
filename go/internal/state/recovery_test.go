package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoverStateValidState(t *testing.T) {
	tmpDir := t.TempDir()

	validState := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing", Completed: []string{"ingestion"}},
		TargetDir: "/test/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
		},
		Execution: Execution{CompletedCount: 1},
	}

	data, _ := json.MarshalIndent(validState, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if result.Recovered {
		t.Error("expected no recovery needed for valid state")
	}
	if state.Version != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", state.Version)
	}
	if state.Phase.Current != "executing" {
		t.Errorf("expected phase 'executing', got '%s'", state.Phase.Current)
	}
}

func TestRecoverStateCorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()

	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{ invalid json"), 0644); err != nil {
		t.Fatalf("failed to write corrupted state: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if !result.Recovered {
		t.Error("expected recovery for corrupted JSON")
	}
	if result.BackupPath == "" {
		t.Error("expected backup path to be set")
	}
	if state.Version != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", state.Version)
	}

	if _, err := os.Stat(result.BackupPath); os.IsNotExist(err) {
		t.Error("backup file was not created")
	}
}

func TestRecoverStatePartialData(t *testing.T) {
	tmpDir := t.TempDir()

	partial := map[string]interface{}{
		"version":    "2.0",
		"target_dir": "/my/target",
		"created_at": "2026-01-18T10:00:00Z",
		"phase": map[string]interface{}{
			"current":   "executing",
			"completed": []string{"ingestion"},
		},
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"status": "complete",
				"phase":  1,
			},
		},
	}

	data, _ := json.MarshalIndent(partial, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write partial state: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if result.Recovered {
		t.Error("expected no recovery for valid partial state")
	}
	if state.TargetDir != "/my/target" {
		t.Errorf("expected target_dir '/my/target', got '%s'", state.TargetDir)
	}
	if state.Phase.Current != "executing" {
		t.Errorf("expected phase 'executing', got '%s'", state.Phase.Current)
	}
	if len(state.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(state.Tasks))
	}
}

func TestRecoverStateInvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()

	invalid := &State{
		Version:   "1.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/test",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
	}

	data, _ := json.MarshalIndent(invalid, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write invalid state: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if !result.Recovered {
		t.Error("expected recovery for invalid version")
	}
	if state.Version != "2.0" {
		t.Errorf("expected recovered version '2.0', got '%s'", state.Version)
	}
}

func TestRecoverStateInvalidPhase(t *testing.T) {
	tmpDir := t.TempDir()

	invalid := map[string]interface{}{
		"version":    "2.0",
		"target_dir": "/test",
		"created_at": "2026-01-18T10:00:00Z",
		"phase": map[string]interface{}{
			"current": "invalid_phase",
		},
		"tasks": map[string]interface{}{},
	}

	data, _ := json.MarshalIndent(invalid, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write invalid state: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if !result.Recovered {
		t.Error("expected recovery for invalid phase")
	}
	if !isValidPhase(state.Phase.Current) {
		t.Errorf("expected valid phase after recovery, got '%s'", state.Phase.Current)
	}
}

func TestRecoverStatePreservesValidTasks(t *testing.T) {
	tmpDir := t.TempDir()

	partial := map[string]interface{}{
		"version": "1.0",
		"phase": map[string]interface{}{
			"current": "executing",
		},
		"target_dir": "/test",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":            "T001",
				"name":          "First task",
				"status":        "complete",
				"phase":         1,
				"started_at":    "2026-01-18T10:00:00Z",
				"completed_at":  "2026-01-18T10:01:00Z",
				"files_created": []string{"file1.go", "file2.go"},
			},
			"T002": map[string]interface{}{
				"id":         "T002",
				"name":       "Second task",
				"status":     "running",
				"phase":      1,
				"depends_on": []string{"T001"},
				"started_at": "2026-01-18T10:01:00Z",
			},
		},
	}

	data, _ := json.MarshalIndent(partial, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write partial state: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if !result.Recovered {
		t.Error("expected recovery")
	}
	if result.TasksRecovered != 2 {
		t.Errorf("expected 2 tasks recovered, got %d", result.TasksRecovered)
	}

	t1, exists := state.Tasks["T001"]
	if !exists {
		t.Error("T001 not found")
	} else {
		if t1.Name != "First task" {
			t.Errorf("expected name 'First task', got '%s'", t1.Name)
		}
		if t1.Status != "complete" {
			t.Errorf("expected status 'complete', got '%s'", t1.Status)
		}
		if len(t1.FilesCreated) != 2 {
			t.Errorf("expected 2 files_created, got %d", len(t1.FilesCreated))
		}
	}

	t2, exists := state.Tasks["T002"]
	if !exists {
		t.Error("T002 not found")
	} else {
		if t2.Status != "running" {
			t.Errorf("expected status 'running', got '%s'", t2.Status)
		}
		if len(t2.DependsOn) != 1 || t2.DependsOn[0] != "T001" {
			t.Errorf("expected depends_on ['T001'], got %v", t2.DependsOn)
		}
	}
}

func TestRecoverStateFromTaskFiles(t *testing.T) {
	tmpDir := t.TempDir()

	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks directory: %v", err)
	}

	task := TaskDefinition{
		ID:    "T001",
		Name:  "Task from file",
		Phase: 1,
	}
	taskData, _ := json.MarshalIndent(task, "", "  ")
	if err := os.WriteFile(filepath.Join(tasksDir, "T001.json"), taskData, 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{ invalid"), 0644); err != nil {
		t.Fatalf("failed to write corrupted state: %v", err)
	}

	state, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if !result.Recovered {
		t.Error("expected recovery")
	}

	t1, exists := state.Tasks["T001"]
	if !exists {
		t.Error("T001 not found - should be recovered from task files")
	} else {
		if t1.Name != "Task from file" {
			t.Errorf("expected name 'Task from file', got '%s'", t1.Name)
		}
	}
}

func TestRecoverStateRecalculatesExecution(t *testing.T) {
	tmpDir := t.TempDir()

	partial := map[string]interface{}{
		"version":    "1.0",
		"target_dir": "/test",
		"created_at": "2026-01-18T10:00:00Z",
		"phase": map[string]interface{}{
			"current": "executing",
		},
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{"id": "T001", "status": "complete", "phase": 1},
			"T002": map[string]interface{}{"id": "T002", "status": "skipped", "phase": 1},
			"T003": map[string]interface{}{"id": "T003", "status": "failed", "phase": 1},
			"T004": map[string]interface{}{"id": "T004", "status": "running", "phase": 1},
		},
		"execution": map[string]interface{}{
			"completed_count": 0,
			"failed_count":    0,
		},
	}

	data, _ := json.MarshalIndent(partial, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write partial state: %v", err)
	}

	state, _, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if state.Execution.CompletedCount != 2 {
		t.Errorf("expected completed_count 2, got %d", state.Execution.CompletedCount)
	}
	if state.Execution.FailedCount != 1 {
		t.Errorf("expected failed_count 1, got %d", state.Execution.FailedCount)
	}
	if len(state.Execution.ActiveTasks) != 1 || state.Execution.ActiveTasks[0] != "T004" {
		t.Errorf("expected active_tasks ['T004'], got %v", state.Execution.ActiveTasks)
	}
}

func TestRecoverStateNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := RecoverState(tmpDir)
	if err == nil {
		t.Error("expected error for missing state file")
	}
}

func TestRecoverStateAddsRecoveryEvent(t *testing.T) {
	tmpDir := t.TempDir()

	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{ invalid"), 0644); err != nil {
		t.Fatalf("failed to write corrupted state: %v", err)
	}

	state, _, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if len(state.Events) == 0 {
		t.Fatal("expected events to include recovery event")
	}

	foundRecoveryEvent := false
	for _, event := range state.Events {
		if event.Type == "state_recovered" {
			foundRecoveryEvent = true
			break
		}
	}
	if !foundRecoveryEvent {
		t.Error("expected state_recovered event")
	}
}

func TestRecoverStateDataLostTracking(t *testing.T) {
	tmpDir := t.TempDir()

	partial := map[string]interface{}{
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"status": "invalid_status",
			},
		},
	}

	data, _ := json.MarshalIndent(partial, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write partial state: %v", err)
	}

	_, result, err := RecoverState(tmpDir)
	if err != nil {
		t.Fatalf("RecoverState failed: %v", err)
	}

	if len(result.DataLost) == 0 {
		t.Error("expected data_lost to track missing fields")
	}

	hasVersionLost := false
	hasTargetDirLost := false
	for _, lost := range result.DataLost {
		if strings.Contains(lost, "version") {
			hasVersionLost = true
		}
		if strings.Contains(lost, "target_dir") {
			hasTargetDirLost = true
		}
	}
	if !hasVersionLost {
		t.Error("expected version to be tracked as lost")
	}
	if !hasTargetDirLost {
		t.Error("expected target_dir to be tracked as lost")
	}
}

func TestRecoverTaskInvalidStatus(t *testing.T) {
	data := map[string]interface{}{
		"id":     "T001",
		"status": "invalid_status",
		"phase":  1,
	}

	task, lost := recoverTask("T001", data)

	if task.Status != "pending" {
		t.Errorf("expected status to default to 'pending', got '%s'", task.Status)
	}
	if len(lost) == 0 {
		t.Error("expected data loss to be tracked for invalid status")
	}
}

func TestIsValidPhase(t *testing.T) {
	validPhases := []string{
		"ingestion", "logical", "physical", "definition",
		"sequencing", "ready", "executing", "complete",
		"spec_review", "validation",
	}

	for _, phase := range validPhases {
		if !isValidPhase(phase) {
			t.Errorf("expected '%s' to be valid phase", phase)
		}
	}

	if isValidPhase("invalid") {
		t.Error("expected 'invalid' to be invalid phase")
	}
}

func TestIsValidStatus(t *testing.T) {
	validStatuses := []string{
		"pending", "ready", "running", "complete",
		"failed", "blocked", "skipped",
	}

	for _, status := range validStatuses {
		if !isValidStatus(status) {
			t.Errorf("expected '%s' to be valid status", status)
		}
	}

	if isValidStatus("invalid") {
		t.Error("expected 'invalid' to be invalid status")
	}
}
