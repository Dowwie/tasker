package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitDecomposition(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := "/test/target"

	state, err := InitDecomposition(tmpDir, targetDir)
	if err != nil {
		t.Fatalf("InitDecomposition failed: %v", err)
	}

	if state.Version != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", state.Version)
	}
	if state.Phase.Current != "ingestion" {
		t.Errorf("expected phase 'ingestion', got '%s'", state.Phase.Current)
	}
	if state.TargetDir != targetDir {
		t.Errorf("expected target_dir '%s', got '%s'", targetDir, state.TargetDir)
	}
	if state.CreatedAt == "" {
		t.Error("expected created_at to be set")
	}
	if len(state.Tasks) != 0 {
		t.Errorf("expected empty tasks map, got %d tasks", len(state.Tasks))
	}

	statePath := filepath.Join(tmpDir, "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("state.json was not created")
	}
}

func TestInitDecompositionAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	if err := os.WriteFile(statePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create existing state file: %v", err)
	}

	_, err := InitDecomposition(tmpDir, "/target")
	if err == nil {
		t.Error("expected error when state.json already exists")
	}
}

func TestInitDecompositionEvents(t *testing.T) {
	tmpDir := t.TempDir()

	state, err := InitDecomposition(tmpDir, "/target")
	if err != nil {
		t.Fatalf("InitDecomposition failed: %v", err)
	}

	if len(state.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(state.Events))
	}

	event := state.Events[0]
	if event.Type != "decomposition_initialized" {
		t.Errorf("expected event type 'decomposition_initialized', got '%s'", event.Type)
	}
}

func TestLoadTasks(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks directory: %v", err)
	}

	task1 := TaskDefinition{
		ID:        "T001",
		Name:      "First task",
		Phase:     1,
		DependsOn: []string{},
	}
	task2 := TaskDefinition{
		ID:        "T002",
		Name:      "Second task",
		Phase:     1,
		DependsOn: []string{"T001"},
	}

	writeTaskFile(t, tasksDir, "T001.json", task1)
	writeTaskFile(t, tasksDir, "T002.json", task2)

	tasks, err := LoadTasks(tmpDir)
	if err != nil {
		t.Fatalf("LoadTasks failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}

	t1, exists := tasks["T001"]
	if !exists {
		t.Error("task T001 not found")
	} else {
		if t1.Name != "First task" {
			t.Errorf("expected name 'First task', got '%s'", t1.Name)
		}
		if t1.Status != "pending" {
			t.Errorf("expected status 'pending', got '%s'", t1.Status)
		}
		if t1.Phase != 1 {
			t.Errorf("expected phase 1, got %d", t1.Phase)
		}
	}

	t2, exists := tasks["T002"]
	if !exists {
		t.Error("task T002 not found")
	} else {
		if len(t2.DependsOn) != 1 || t2.DependsOn[0] != "T001" {
			t.Errorf("expected depends_on ['T001'], got %v", t2.DependsOn)
		}
	}
}

func TestLoadTasksMissingDirectory(t *testing.T) {
	_, err := LoadTasks("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing tasks directory")
	}
}

func TestLoadTasksMissingID(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks directory: %v", err)
	}

	task := TaskDefinition{
		Name:  "No ID task",
		Phase: 1,
	}
	writeTaskFile(t, tasksDir, "bad.json", task)

	_, err := LoadTasks(tmpDir)
	if err == nil {
		t.Error("expected error for task missing ID")
	}
}

func TestLoadTasksIgnoresNonJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks directory: %v", err)
	}

	task := TaskDefinition{ID: "T001", Name: "Valid", Phase: 1}
	writeTaskFile(t, tasksDir, "T001.json", task)

	if err := os.WriteFile(filepath.Join(tasksDir, "README.md"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("failed to write non-JSON file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	tasks, err := LoadTasks(tmpDir)
	if err != nil {
		t.Fatalf("LoadTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestGetReadyTasks(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 1, DependsOn: []string{}},
			"T002": {ID: "T002", Status: "pending", Phase: 1, DependsOn: []string{"T001"}},
			"T003": {ID: "T003", Status: "pending", Phase: 2, DependsOn: []string{}},
		},
	}

	ready := GetReadyTasks(state)

	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks, got %d", len(ready))
	}

	if ready[0].ID != "T001" {
		t.Errorf("expected first ready task 'T001', got '%s'", ready[0].ID)
	}
	if ready[1].ID != "T003" {
		t.Errorf("expected second ready task 'T003', got '%s'", ready[1].ID)
	}
}

func TestGetReadyTasksWithCompletedDeps(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1, DependsOn: []string{"T001"}},
		},
	}

	ready := GetReadyTasks(state)

	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != "T002" {
		t.Errorf("expected ready task 'T002', got '%s'", ready[0].ID)
	}
}

func TestGetReadyTasksWithSkippedDeps(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "skipped", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1, DependsOn: []string{"T001"}},
		},
	}

	ready := GetReadyTasks(state)

	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != "T002" {
		t.Errorf("expected ready task 'T002', got '%s'", ready[0].ID)
	}
}

func TestGetReadyTasksSortedByPhase(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T003": {ID: "T003", Status: "pending", Phase: 2},
			"T001": {ID: "T001", Status: "pending", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1},
		},
	}

	ready := GetReadyTasks(state)

	if len(ready) != 3 {
		t.Fatalf("expected 3 ready tasks, got %d", len(ready))
	}

	if ready[0].Phase != 1 || ready[1].Phase != 1 || ready[2].Phase != 2 {
		t.Error("tasks not sorted by phase")
	}
}

func TestGetReadyTasksExcludesRunning(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "running", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1},
		},
	}

	ready := GetReadyTasks(state)

	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != "T002" {
		t.Errorf("expected ready task 'T002', got '%s'", ready[0].ID)
	}
}

func TestStartTask(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 1},
		},
		Execution: Execution{ActiveTasks: []string{}},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := StartTask(sm, "T001")
	if err != nil {
		t.Fatalf("StartTask failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task := loaded.Tasks["T001"]
	if task.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", task.Status)
	}
	if task.StartedAt == "" {
		t.Error("expected started_at to be set")
	}
	if task.Attempts != 1 {
		t.Errorf("expected attempts 1, got %d", task.Attempts)
	}

	if len(loaded.Execution.ActiveTasks) != 1 || loaded.Execution.ActiveTasks[0] != "T001" {
		t.Errorf("expected active_tasks ['T001'], got %v", loaded.Execution.ActiveTasks)
	}

	if len(loaded.Events) != 1 || loaded.Events[0].Type != "task_started" {
		t.Error("expected task_started event")
	}
}

func TestStartTaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := StartTask(sm, "T999")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestStartTaskWrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := StartTask(sm, "T001")
	if err == nil {
		t.Error("expected error for completed task")
	}
}

func TestCompleteTask(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "running", Phase: 1, StartedAt: "2026-01-18T10:00:00Z"},
		},
		Execution: Execution{ActiveTasks: []string{"T001"}, CompletedCount: 0},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	filesCreated := []string{"src/file1.go", "src/file2.go"}
	filesModified := []string{"go.mod"}

	err := CompleteTask(sm, "T001", filesCreated, filesModified)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task := loaded.Tasks["T001"]
	if task.Status != "complete" {
		t.Errorf("expected status 'complete', got '%s'", task.Status)
	}
	if task.CompletedAt == "" {
		t.Error("expected completed_at to be set")
	}
	if len(task.FilesCreated) != 2 {
		t.Errorf("expected 2 files_created, got %d", len(task.FilesCreated))
	}
	if len(task.FilesModified) != 1 {
		t.Errorf("expected 1 files_modified, got %d", len(task.FilesModified))
	}

	if len(loaded.Execution.ActiveTasks) != 0 {
		t.Errorf("expected empty active_tasks, got %v", loaded.Execution.ActiveTasks)
	}
	if loaded.Execution.CompletedCount != 1 {
		t.Errorf("expected completed_count 1, got %d", loaded.Execution.CompletedCount)
	}

	if len(loaded.Events) != 1 || loaded.Events[0].Type != "task_completed" {
		t.Error("expected task_completed event")
	}
}

func TestCompleteTaskNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := CompleteTask(sm, "T001", nil, nil)
	if err == nil {
		t.Error("expected error for non-running task")
	}
}

func TestFailTask(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "running", Phase: 1, StartedAt: "2026-01-18T10:00:00Z"},
		},
		Execution: Execution{ActiveTasks: []string{"T001"}, FailedCount: 0},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := FailTask(sm, "T001", "test failed", "test", true)
	if err != nil {
		t.Fatalf("FailTask failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task := loaded.Tasks["T001"]
	if task.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", task.Status)
	}
	if task.Error != "test failed" {
		t.Errorf("expected error 'test failed', got '%s'", task.Error)
	}
	if task.Failure == nil {
		t.Error("expected failure to be set")
	} else {
		if task.Failure.Category != "test" {
			t.Errorf("expected category 'test', got '%s'", task.Failure.Category)
		}
		if !task.Failure.Retryable {
			t.Error("expected retryable to be true")
		}
	}

	if len(loaded.Execution.ActiveTasks) != 0 {
		t.Errorf("expected empty active_tasks, got %v", loaded.Execution.ActiveTasks)
	}
	if loaded.Execution.FailedCount != 1 {
		t.Errorf("expected failed_count 1, got %d", loaded.Execution.FailedCount)
	}

	if len(loaded.Events) != 1 || loaded.Events[0].Type != "task_failed" {
		t.Error("expected task_failed event")
	}
}

func TestFailTaskNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := FailTask(sm, "T001", "error", "test", true)
	if err == nil {
		t.Error("expected error for non-running task")
	}
}

func TestRetryTask(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {
				ID:          "T001",
				Status:      "failed",
				Phase:       1,
				Attempts:    1,
				StartedAt:   "2026-01-18T10:00:00Z",
				CompletedAt: "2026-01-18T10:01:00Z",
				Error:       "previous error",
				Failure:     &TaskFailure{Category: "test", Retryable: true},
			},
		},
		Execution: Execution{FailedCount: 1},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := RetryTask(sm, "T001")
	if err != nil {
		t.Fatalf("RetryTask failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task := loaded.Tasks["T001"]
	if task.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", task.Status)
	}
	if task.StartedAt != "" {
		t.Error("expected started_at to be cleared")
	}
	if task.CompletedAt != "" {
		t.Error("expected completed_at to be cleared")
	}
	if task.Error != "" {
		t.Error("expected error to be cleared")
	}
	if task.Failure != nil {
		t.Error("expected failure to be cleared")
	}
	if task.Attempts != 1 {
		t.Errorf("expected attempts to remain 1, got %d", task.Attempts)
	}

	if loaded.Execution.FailedCount != 0 {
		t.Errorf("expected failed_count 0, got %d", loaded.Execution.FailedCount)
	}

	if len(loaded.Events) != 1 || loaded.Events[0].Type != "task_retried" {
		t.Error("expected task_retried event")
	}
}

func TestRetryTaskNotFailed(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := RetryTask(sm, "T001")
	if err == nil {
		t.Error("expected error for non-failed task")
	}
}

func TestRetryTaskNotRetryable(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {
				ID:      "T001",
				Status:  "failed",
				Phase:   1,
				Failure: &TaskFailure{Category: "spec", Retryable: false},
			},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := RetryTask(sm, "T001")
	if err == nil {
		t.Error("expected error for non-retryable task")
	}
}

func TestSkipTask(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "pending", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1, DependsOn: []string{"T001"}},
		},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := SkipTask(sm, "T001", "not needed for MVP")
	if err != nil {
		t.Fatalf("SkipTask failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task := loaded.Tasks["T001"]
	if task.Status != "skipped" {
		t.Errorf("expected status 'skipped', got '%s'", task.Status)
	}
	if task.CompletedAt == "" {
		t.Error("expected completed_at to be set")
	}
	if task.Error != "not needed for MVP" {
		t.Errorf("expected error reason, got '%s'", task.Error)
	}

	if len(loaded.Events) != 1 || loaded.Events[0].Type != "task_skipped" {
		t.Error("expected task_skipped event")
	}

	ready := GetReadyTasks(loaded)
	if len(ready) != 1 || ready[0].ID != "T002" {
		t.Error("T002 should be ready after T001 skipped")
	}
}

func TestSkipTaskWrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "running", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := SkipTask(sm, "T001", "reason")
	if err == nil {
		t.Error("expected error for running task")
	}
}

func TestSkipTaskBlockedStatus(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "blocked", Phase: 1},
		},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := SkipTask(sm, "T001", "unblocking")
	if err != nil {
		t.Fatalf("SkipTask should work for blocked tasks: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Tasks["T001"].Status != "skipped" {
		t.Error("blocked task should be skippable")
	}
}

func TestCommitTaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: tmpDir,
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	_, err := CommitTask(sm, "T001")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestCommitTaskNotComplete(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: tmpDir,
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "running", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	_, err := CommitTask(sm, "T001")
	if err == nil {
		t.Error("expected error for non-complete task")
	}
	if !strings.Contains(err.Error(), "is running") {
		t.Errorf("expected status error, got: %v", err)
	}
}

func TestCommitTaskNoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: tmpDir,
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	_, err := CommitTask(sm, "T001")
	if err == nil {
		t.Error("expected error for task with no files")
	}
	if !strings.Contains(err.Error(), "no files to commit") {
		t.Errorf("expected 'no files to commit' error, got: %v", err)
	}
}

func writeTaskFile(t *testing.T, dir, name string, task TaskDefinition) {
	t.Helper()
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}
}
