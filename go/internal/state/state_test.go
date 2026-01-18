package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	validState := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready", Completed: []string{"ingestion"}},
		TargetDir: "/some/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 0},
		},
		Execution: Execution{CurrentPhase: 0},
	}

	data, err := json.MarshalIndent(validState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test state: %v", err)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.Version != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", loaded.Version)
	}
	if loaded.Phase.Current != "ready" {
		t.Errorf("expected phase 'ready', got '%s'", loaded.Phase.Current)
	}
	if loaded.TargetDir != "/some/path" {
		t.Errorf("expected target_dir '/some/path', got '%s'", loaded.TargetDir)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
	task, exists := loaded.Tasks["T001"]
	if !exists {
		t.Error("task T001 not found")
	} else if task.Status != "pending" {
		t.Errorf("expected task status 'pending', got '%s'", task.Status)
	}
}

func TestLoadStateNotFound(t *testing.T) {
	_, err := LoadState("/nonexistent/path/state.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadStateInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	if err := os.WriteFile(statePath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	_, err := LoadState(statePath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSaveState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion", Completed: []string{}},
		TargetDir: "/test/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{CurrentPhase: 0},
	}

	if err := SaveState(statePath, state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("state file was not created")
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load saved state: %v", err)
	}

	if loaded.Version != state.Version {
		t.Errorf("version mismatch: expected '%s', got '%s'", state.Version, loaded.Version)
	}
	if loaded.TargetDir != state.TargetDir {
		t.Errorf("target_dir mismatch: expected '%s', got '%s'", state.TargetDir, loaded.TargetDir)
	}
	if loaded.UpdatedAt == "" {
		t.Error("updated_at should be set after save")
	}
}

func TestSaveStateCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "state.json")

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion"},
		TargetDir: "/test",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}

	if err := SaveState(nestedPath, state); err != nil {
		t.Fatalf("SaveState failed for nested path: %v", err)
	}

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("state file was not created in nested directory")
	}
}

func TestSaveStateAtomicity(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	initialState := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion"},
		TargetDir: "/initial",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}
	if err := SaveState(statePath, initialState); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	tmpFile := statePath + ".tmp"
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}
}

func TestValidateState(t *testing.T) {
	validState := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready", Completed: []string{"ingestion"}},
		TargetDir: "/some/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 0},
		},
		Execution: Execution{CurrentPhase: 0},
	}

	errs := ValidateState(validState)
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateStateInvalidVersion(t *testing.T) {
	state := &State{
		Version:   "1.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
	}

	errs := ValidateState(state)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid version")
	}

	found := false
	for _, err := range errs {
		if err.Error() == "invalid version: expected '2.0', got '1.0'" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected specific version error message")
	}
}

func TestValidateStateInvalidPhase(t *testing.T) {
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "invalid_phase"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
	}

	errs := ValidateState(state)
	found := false
	for _, err := range errs {
		if err.Error() == "invalid phase: 'invalid_phase'" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected phase validation error, got: %v", errs)
	}
}

func TestValidateStateMissingTargetDir(t *testing.T) {
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
	}

	errs := ValidateState(state)
	found := false
	for _, err := range errs {
		if err.Error() == "target_dir is required" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected target_dir validation error, got: %v", errs)
	}
}

func TestValidateStateMissingCreatedAt(t *testing.T) {
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "/path",
		CreatedAt: "",
		Tasks:     map[string]Task{},
	}

	errs := ValidateState(state)
	found := false
	for _, err := range errs {
		if err.Error() == "created_at is required" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected created_at validation error, got: %v", errs)
	}
}

func TestValidateStateInvalidTaskStatus(t *testing.T) {
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "invalid_status", Phase: 0},
		},
	}

	errs := ValidateState(state)
	found := false
	for _, err := range errs {
		if err.Error() == "task T001: invalid status 'invalid_status'" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected task status validation error, got: %v", errs)
	}
}

func TestValidateStateTaskMissingID(t *testing.T) {
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "", Status: "pending", Phase: 0},
		},
	}

	errs := ValidateState(state)
	found := false
	for _, err := range errs {
		if err.Error() == "task T001: id is required" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected task id validation error, got: %v", errs)
	}
}

func TestStateManagerLoad(t *testing.T) {
	tmpDir := t.TempDir()

	validState := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}

	data, _ := json.MarshalIndent(validState, "", "  ")
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	sm := NewStateManager(tmpDir)
	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("StateManager.Load failed: %v", err)
	}

	if loaded.Version != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", loaded.Version)
	}
}

func TestStateManagerSave(t *testing.T) {
	tmpDir := t.TempDir()

	sm := NewStateManager(tmpDir)
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion"},
		TargetDir: "/test",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}

	if err := sm.Save(state); err != nil {
		t.Fatalf("StateManager.Save failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load after save: %v", err)
	}

	if loaded.TargetDir != "/test" {
		t.Errorf("expected target_dir '/test', got '%s'", loaded.TargetDir)
	}
}

func TestStateManagerPath(t *testing.T) {
	sm := NewStateManager("/some/planning/dir")
	expected := "/some/planning/dir/state.json"
	if sm.Path() != expected {
		t.Errorf("expected path '%s', got '%s'", expected, sm.Path())
	}
}

func TestStateManagerValidate(t *testing.T) {
	sm := NewStateManager("/some/dir")

	validState := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
	}

	errs := sm.Validate(validState)
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestFileLock(t *testing.T) {
	t.Run("acquire and release read lock", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockPath := filepath.Join(tmpDir, "test.json")

		lock, err := AcquireReadLock(lockPath)
		if err != nil {
			t.Fatalf("AcquireReadLock failed: %v", err)
		}

		if lock == nil {
			t.Fatal("expected non-nil lock")
		}

		if err := lock.Release(); err != nil {
			t.Errorf("Release failed: %v", err)
		}
	})

	t.Run("acquire and release write lock", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockPath := filepath.Join(tmpDir, "test.json")

		lock, err := AcquireWriteLock(lockPath)
		if err != nil {
			t.Fatalf("AcquireWriteLock failed: %v", err)
		}

		if lock == nil {
			t.Fatal("expected non-nil lock")
		}

		if err := lock.Release(); err != nil {
			t.Errorf("Release failed: %v", err)
		}
	})

	t.Run("release nil lock", func(t *testing.T) {
		lock := &FileLock{}
		if err := lock.Release(); err != nil {
			t.Errorf("Release on nil file should succeed, got: %v", err)
		}
	})

	t.Run("double release returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockPath := filepath.Join(tmpDir, "test.json")

		lock, err := AcquireWriteLock(lockPath)
		if err != nil {
			t.Fatalf("AcquireWriteLock failed: %v", err)
		}

		if err := lock.Release(); err != nil {
			t.Errorf("First release failed: %v", err)
		}

		err = lock.Release()
		if err == nil {
			t.Error("Second release should return error for closed file")
		}
	})
}

func TestStateManagerGetMetrics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		state := &State{
			Version:   "2.0",
			Phase:     PhaseState{Current: "ready"},
			TargetDir: "/path",
			CreatedAt: "2026-01-18T10:00:00Z",
			Tasks: map[string]Task{
				"T001": {ID: "T001", Status: "completed", Phase: 0},
			},
			Execution: Execution{},
		}

		data, _ := json.MarshalIndent(state, "", "  ")
		if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
			t.Fatalf("failed to write state: %v", err)
		}

		sm := NewStateManager(tmpDir)
		metrics, err := sm.GetMetrics()
		if err != nil {
			t.Fatalf("GetMetrics failed: %v", err)
		}
		if metrics == nil {
			t.Error("expected non-nil metrics")
		}
	})

	t.Run("load fails", func(t *testing.T) {
		sm := NewStateManager("/nonexistent/path")
		_, err := sm.GetMetrics()
		if err == nil {
			t.Error("expected error when Load fails")
		}
	})
}

func TestStateManagerGetPlanningMetrics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		state := &State{
			Version:   "2.0",
			Phase:     PhaseState{Current: "ready"},
			TargetDir: "/path",
			CreatedAt: "2026-01-18T10:00:00Z",
			Tasks:     map[string]Task{},
			Execution: Execution{},
		}

		data, _ := json.MarshalIndent(state, "", "  ")
		if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
			t.Fatalf("failed to write state: %v", err)
		}

		sm := NewStateManager(tmpDir)
		metrics, err := sm.GetPlanningMetrics()
		if err != nil {
			t.Fatalf("GetPlanningMetrics failed: %v", err)
		}
		if metrics == nil {
			t.Error("expected non-nil metrics")
		}
	})

	t.Run("load fails", func(t *testing.T) {
		sm := NewStateManager("/nonexistent/path")
		_, err := sm.GetPlanningMetrics()
		if err == nil {
			t.Error("expected error when Load fails")
		}
	})
}

func TestStateManagerGetFailureMetrics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		state := &State{
			Version:   "2.0",
			Phase:     PhaseState{Current: "ready"},
			TargetDir: "/path",
			CreatedAt: "2026-01-18T10:00:00Z",
			Tasks:     map[string]Task{},
			Execution: Execution{},
		}

		data, _ := json.MarshalIndent(state, "", "  ")
		if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
			t.Fatalf("failed to write state: %v", err)
		}

		sm := NewStateManager(tmpDir)
		metrics, err := sm.GetFailureMetrics()
		if err != nil {
			t.Fatalf("GetFailureMetrics failed: %v", err)
		}
		if metrics == nil {
			t.Error("expected non-nil metrics")
		}
	})

	t.Run("load fails", func(t *testing.T) {
		sm := NewStateManager("/nonexistent/path")
		_, err := sm.GetFailureMetrics()
		if err == nil {
			t.Error("expected error when Load fails")
		}
	})
}

func TestStateManagerLogTokens(t *testing.T) {
	tmpDir := t.TempDir()
	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "execution"},
		TargetDir: "/path",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "in_progress", Phase: 0},
		},
		Execution: Execution{ActiveTasks: []string{"T001"}},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	sm := NewStateManager(tmpDir)
	err := sm.LogTokens("T001", 100, 50, 0.01, "claude-3")
	if err != nil {
		t.Fatalf("LogTokens failed: %v", err)
	}

	loaded, _ := sm.Load()
	if loaded.Execution.TotalTokens != 150 {
		t.Errorf("expected total_tokens 150, got %d", loaded.Execution.TotalTokens)
	}
	if loaded.Execution.TotalCostUSD != 0.01 {
		t.Errorf("expected total_cost_usd 0.01, got %f", loaded.Execution.TotalCostUSD)
	}
}

