package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestState(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current":   "execution",
			"completed": []string{"ingestion", "planning"},
		},
		"target_dir": "/test/target",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"name":   "Test Task 1",
				"status": "pending",
				"phase":  0,
			},
			"T002": map[string]interface{}{
				"id":        "T002",
				"name":      "Test Task 2",
				"status":    "pending",
				"phase":     0,
				"depends_on": []string{"T001"},
			},
		},
		"execution": map[string]interface{}{
			"current_phase": 0,
			"active_tasks":  []string{},
		},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	return tmpDir
}

func withPlanningDir(tmpDir string, fn func()) {
	original := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = original }()
	fn()
}

func TestInitCmd(t *testing.T) {
	t.Run("initializes new decomposition", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := initCmd.RunE(initCmd, []string{targetDir})
			if err != nil {
				t.Errorf("initCmd should succeed, got: %v", err)
			}

			statePath := filepath.Join(tmpDir, "state.json")
			if _, err := os.Stat(statePath); os.IsNotExist(err) {
				t.Error("expected state.json to be created")
			}
		})
	})
}

func TestStatusCmd(t *testing.T) {
	t.Run("shows status for valid state", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := statusCmd.RunE(statusCmd, []string{})
			if err != nil {
				t.Errorf("statusCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails when state missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := statusCmd.RunE(statusCmd, []string{})
			if err == nil {
				t.Error("statusCmd should fail when state is missing")
			}
		})
	})
}

func TestTaskStartCmd(t *testing.T) {
	t.Run("starts a pending task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := taskStartCmd.RunE(taskStartCmd, []string{"T001"})
			if err != nil {
				t.Errorf("taskStartCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for nonexistent task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := taskStartCmd.RunE(taskStartCmd, []string{"TXXX"})
			if err == nil {
				t.Error("taskStartCmd should fail for nonexistent task")
			}
		})
	})
}

func TestTaskCompleteCmd(t *testing.T) {
	t.Run("completes a running task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			_ = taskStartCmd.RunE(taskStartCmd, []string{"T001"})

			err := taskCompleteCmd.RunE(taskCompleteCmd, []string{"T001"})
			if err != nil {
				t.Errorf("taskCompleteCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for non-running task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := taskCompleteCmd.RunE(taskCompleteCmd, []string{"T001"})
			if err == nil {
				t.Error("taskCompleteCmd should fail for non-running task")
			}
		})
	})
}

func TestTaskFailCmd(t *testing.T) {
	t.Run("fails a running task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			_ = taskStartCmd.RunE(taskStartCmd, []string{"T001"})

			err := taskFailCmd.RunE(taskFailCmd, []string{"T001", "test error"})
			if err != nil {
				t.Errorf("taskFailCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for non-running task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := taskFailCmd.RunE(taskFailCmd, []string{"T001", "test error"})
			if err == nil {
				t.Error("taskFailCmd should fail for non-running task")
			}
		})
	})
}

func TestTaskRetryCmd(t *testing.T) {
	t.Run("retries a failed task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			_ = taskStartCmd.RunE(taskStartCmd, []string{"T001"})
			_ = taskFailCmd.RunE(taskFailCmd, []string{"T001", "test error"})

			err := taskRetryCmd.RunE(taskRetryCmd, []string{"T001"})
			if err != nil {
				t.Errorf("taskRetryCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for non-failed task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := taskRetryCmd.RunE(taskRetryCmd, []string{"T001"})
			if err == nil {
				t.Error("taskRetryCmd should fail for non-failed task")
			}
		})
	})
}

func TestTaskSkipCmd(t *testing.T) {
	t.Run("skips a pending task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := taskSkipCmd.RunE(taskSkipCmd, []string{"T001"})
			if err != nil {
				t.Errorf("taskSkipCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for running task", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			_ = taskStartCmd.RunE(taskStartCmd, []string{"T001"})

			err := taskSkipCmd.RunE(taskSkipCmd, []string{"T001"})
			if err == nil {
				t.Error("taskSkipCmd should fail for running task")
			}
		})
	})
}

func TestReadyCmd(t *testing.T) {
	t.Run("lists ready tasks", func(t *testing.T) {
		tmpDir := setupTestState(t)

		withPlanningDir(tmpDir, func() {
			err := readyCmd.RunE(readyCmd, []string{})
			if err != nil {
				t.Errorf("readyCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails when state missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := readyCmd.RunE(readyCmd, []string{})
			if err == nil {
				t.Error("readyCmd should fail when state is missing")
			}
		})
	})

	t.Run("handles no ready tasks", func(t *testing.T) {
		tmpDir := t.TempDir()

		state := map[string]interface{}{
			"version": "2.0",
			"phase": map[string]interface{}{
				"current":   "execution",
				"completed": []string{},
			},
			"target_dir": "/test/target",
			"created_at": "2026-01-18T10:00:00Z",
			"tasks":      map[string]interface{}{},
			"execution":  map[string]interface{}{},
		}

		data, _ := json.MarshalIndent(state, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644)

		withPlanningDir(tmpDir, func() {
			err := readyCmd.RunE(readyCmd, []string{})
			if err != nil {
				t.Errorf("readyCmd should handle no ready tasks, got: %v", err)
			}
		})
	})
}

func TestStatusCmd_WithActiveTasks(t *testing.T) {
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current":   "execution",
			"completed": []string{},
		},
		"target_dir": "/test/target",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"name":   "Test Task 1",
				"status": "in_progress",
				"phase":  0,
			},
		},
		"execution": map[string]interface{}{
			"current_phase": 0,
			"active_tasks":  []string{"T001"},
		},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644)

	withPlanningDir(tmpDir, func() {
		err := statusCmd.RunE(statusCmd, []string{})
		if err != nil {
			t.Errorf("statusCmd should succeed with active tasks, got: %v", err)
		}
	})
}
