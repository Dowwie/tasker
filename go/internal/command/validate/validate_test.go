package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgordon/tasker/internal/validate"
)

func TestLoadTasksForValidation(t *testing.T) {
	t.Run("loads tasks from directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		if err := os.MkdirAll(tasksDir, 0755); err != nil {
			t.Fatalf("failed to create tasks dir: %v", err)
		}

		task1 := taskDefinition{
			ID:          "T001",
			DependsOn:   []string{},
			SteelThread: true,
		}
		task2 := taskDefinition{
			ID:          "T002",
			DependsOn:   []string{"T001"},
			SteelThread: false,
		}

		writeTaskFile(t, tasksDir, "T001.json", task1)
		writeTaskFile(t, tasksDir, "T002.json", task2)

		tasks, err := loadTasksForValidation(tmpDir)
		if err != nil {
			t.Fatalf("loadTasksForValidation failed: %v", err)
		}

		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}

		if _, exists := tasks["T001"]; !exists {
			t.Error("expected T001 to exist")
		}
		if _, exists := tasks["T002"]; !exists {
			t.Error("expected T002 to exist")
		}

		if !tasks["T001"].SteelThread {
			t.Error("expected T001 to be steel thread")
		}
		if tasks["T002"].SteelThread {
			t.Error("expected T002 to not be steel thread")
		}
	})

	t.Run("returns error for missing tasks directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := loadTasksForValidation(tmpDir)
		if err == nil {
			t.Error("expected error for missing tasks directory")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		if err := os.MkdirAll(tasksDir, 0755); err != nil {
			t.Fatalf("failed to create tasks dir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(tasksDir, "bad.json"), []byte("not json"), 0644); err != nil {
			t.Fatalf("failed to write bad file: %v", err)
		}

		_, err := loadTasksForValidation(tmpDir)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("returns error for task missing ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		if err := os.MkdirAll(tasksDir, 0755); err != nil {
			t.Fatalf("failed to create tasks dir: %v", err)
		}

		task := taskDefinition{ID: ""}
		writeTaskFile(t, tasksDir, "noid.json", task)

		_, err := loadTasksForValidation(tmpDir)
		if err == nil {
			t.Error("expected error for task missing ID")
		}
	})

	t.Run("skips directories and non-json files", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		if err := os.MkdirAll(tasksDir, 0755); err != nil {
			t.Fatalf("failed to create tasks dir: %v", err)
		}

		task1 := taskDefinition{ID: "T001"}
		writeTaskFile(t, tasksDir, "T001.json", task1)

		if err := os.MkdirAll(filepath.Join(tasksDir, "subdir"), 0755); err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tasksDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
			t.Fatalf("failed to write txt file: %v", err)
		}

		tasks, err := loadTasksForValidation(tmpDir)
		if err != nil {
			t.Fatalf("loadTasksForValidation failed: %v", err)
		}

		if len(tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(tasks))
		}
	})
}

func TestFilterSteelThread(t *testing.T) {
	tasks := map[string]validate.Task{
		"T001": {ID: "T001", SteelThread: true},
		"T002": {ID: "T002", SteelThread: false},
		"T003": {ID: "T003", SteelThread: true},
		"T004": {ID: "T004", SteelThread: false},
	}

	result := filterSteelThread(tasks)

	if len(result) != 2 {
		t.Errorf("expected 2 steel thread tasks, got %d", len(result))
	}

	if _, exists := result["T001"]; !exists {
		t.Error("expected T001 in result")
	}
	if _, exists := result["T003"]; !exists {
		t.Error("expected T003 in result")
	}
	if _, exists := result["T002"]; exists {
		t.Error("T002 should not be in result")
	}
}

func TestFilterSteelThread_Empty(t *testing.T) {
	tasks := map[string]validate.Task{
		"T001": {ID: "T001", SteelThread: false},
		"T002": {ID: "T002", SteelThread: false},
	}

	result := filterSteelThread(tasks)

	if len(result) != 0 {
		t.Errorf("expected 0 steel thread tasks, got %d", len(result))
	}
}

func writeTaskFile(t *testing.T, dir, filename string, task taskDefinition) {
	t.Helper()
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}
}

func TestDagCmd_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	task1 := taskDefinition{ID: "T001", DependsOn: []string{}}
	task2 := taskDefinition{ID: "T002", DependsOn: []string{"T001"}}
	writeTaskFile(t, tasksDir, "T001.json", task1)
	writeTaskFile(t, tasksDir, "T002.json", task2)

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := dagCmd.RunE(dagCmd, []string{})
	if err != nil {
		t.Errorf("dagCmd should pass for valid DAG, got: %v", err)
	}
}

func TestDagCmd_Cycle(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	task1 := taskDefinition{ID: "T001", DependsOn: []string{"T002"}}
	task2 := taskDefinition{ID: "T002", DependsOn: []string{"T001"}}
	writeTaskFile(t, tasksDir, "T001.json", task1)
	writeTaskFile(t, tasksDir, "T002.json", task2)

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := dagCmd.RunE(dagCmd, []string{})
	if err == nil {
		t.Error("dagCmd should fail for cyclic DAG")
	}
}

func TestDagCmd_MissingTasksDir(t *testing.T) {
	tmpDir := t.TempDir()

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := dagCmd.RunE(dagCmd, []string{})
	if err == nil {
		t.Error("dagCmd should fail when tasks dir is missing")
	}
}

func TestSteelThreadCmd_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	task1 := taskDefinition{ID: "T001", DependsOn: []string{}, SteelThread: true}
	task2 := taskDefinition{ID: "T002", DependsOn: []string{"T001"}, SteelThread: true}
	writeTaskFile(t, tasksDir, "T001.json", task1)
	writeTaskFile(t, tasksDir, "T002.json", task2)

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := steelThreadCmd.RunE(steelThreadCmd, []string{})
	if err != nil {
		t.Errorf("steelThreadCmd should pass for valid steel thread, got: %v", err)
	}
}

func TestSteelThreadCmd_NoSteelThread(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	task1 := taskDefinition{ID: "T001", DependsOn: []string{}, SteelThread: false}
	writeTaskFile(t, tasksDir, "T001.json", task1)

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := steelThreadCmd.RunE(steelThreadCmd, []string{})
	if err != nil {
		t.Errorf("steelThreadCmd should return nil when no steel thread tasks, got: %v", err)
	}
}

func TestSteelThreadCmd_MissingTasksDir(t *testing.T) {
	tmpDir := t.TempDir()

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := steelThreadCmd.RunE(steelThreadCmd, []string{})
	if err == nil {
		t.Error("steelThreadCmd should fail when tasks dir is missing")
	}
}

func TestGatesCmd_WithState(t *testing.T) {
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current":   "ready",
			"completed": []string{"ingestion", "planning"},
		},
		"target_dir": "/some/path",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks":      map[string]interface{}{},
		"artifacts": map[string]interface{}{
			"validation_results": map[string]interface{}{
				"spec_coverage": map[string]interface{}{
					"passed":    true,
					"ratio":     0.95,
					"threshold": 0.9,
				},
				"phase_leakage": map[string]interface{}{
					"passed": true,
				},
				"dependency_existence": map[string]interface{}{
					"passed": true,
				},
			},
		},
		"execution": map[string]interface{}{},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := gatesCmd.RunE(gatesCmd, []string{})
	if err != nil {
		t.Errorf("gatesCmd should pass with valid state, got: %v", err)
	}
}

func TestGatesCmd_NoState(t *testing.T) {
	tmpDir := t.TempDir()

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := gatesCmd.RunE(gatesCmd, []string{})
	if err == nil {
		t.Error("gatesCmd should fail when state file is missing")
	}
}

func TestGatesCmd_WithFailedValidation(t *testing.T) {
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current":   "ready",
			"completed": []string{},
		},
		"target_dir": "/some/path",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks":      map[string]interface{}{},
		"artifacts": map[string]interface{}{
			"validation_results": map[string]interface{}{
				"spec_coverage": map[string]interface{}{
					"passed":    false,
					"ratio":     0.5,
					"threshold": 0.9,
				},
				"phase_leakage": map[string]interface{}{
					"passed": false,
				},
				"dependency_existence": map[string]interface{}{
					"passed": false,
				},
			},
		},
		"execution": map[string]interface{}{},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	originalGetPlanningDir := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = originalGetPlanningDir }()

	err := gatesCmd.RunE(gatesCmd, []string{})
	if err != nil {
		t.Errorf("gatesCmd should not error even with failed validations, got: %v", err)
	}
}
