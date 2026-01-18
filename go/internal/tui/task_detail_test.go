package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgordon/tasker/internal/state"
)

func TestRenderTaskDetail(t *testing.T) {
	task := state.Task{
		ID:        "T001",
		Name:      "Test task",
		Status:    "running",
		Phase:     1,
		DependsOn: []string{"T000"},
		Blocks:    []string{"T002", "T003"},
	}

	detail := TaskDetail{
		Task:               task,
		AcceptanceCriteria: nil,
		Files:              nil,
	}

	result := RenderTaskDetail(detail, 80)

	expectedContent := []string{
		"T001",
		"Test task",
		"running",
		"T000",
		"Dependencies",
		"Blocks",
		"T002",
		"T003",
	}

	for _, expected := range expectedContent {
		if !containsString(result, expected) {
			t.Errorf("RenderTaskDetail should contain '%s', got:\n%s", expected, result)
		}
	}
}

func TestRenderTaskDetailWithPhase(t *testing.T) {
	task := state.Task{
		ID:     "T005",
		Name:   "Phase test",
		Status: "pending",
		Phase:  2,
	}

	detail := TaskDetail{Task: task}
	result := RenderTaskDetail(detail, 80)

	if !containsString(result, "Phase:") {
		t.Error("RenderTaskDetail should show Phase label")
	}
}

func TestTaskDetailCriteria(t *testing.T) {
	task := state.Task{
		ID:     "T001",
		Name:   "Task with criteria",
		Status: "complete",
		Phase:  1,
	}

	criteria := []AcceptanceCriterion{
		{
			Criterion:    "First criterion must pass",
			Verification: "go test ./... -run TestFirst -v",
		},
		{
			Criterion:    "Second criterion validated",
			Verification: "go test ./... -run TestSecond -v",
		},
	}

	detail := TaskDetail{
		Task:               task,
		AcceptanceCriteria: criteria,
	}

	result := RenderTaskDetail(detail, 80)

	expectedContent := []string{
		"Acceptance Criteria",
		"First criterion must pass",
		"go test ./... -run TestFirst -v",
		"Second criterion validated",
		"go test ./... -run TestSecond -v",
	}

	for _, expected := range expectedContent {
		if !containsString(result, expected) {
			t.Errorf("Task detail should contain '%s', got:\n%s", expected, result)
		}
	}
}

func TestTaskDetailCriteriaWithVerificationStatus(t *testing.T) {
	task := state.Task{
		ID:     "T001",
		Name:   "Task with verification",
		Status: "complete",
		Phase:  1,
		Verification: &state.TaskVerification{
			Verdict: "PASS",
			Criteria: []state.VerificationCriterion{
				{Name: "First", Score: "PASS", Evidence: "test passed"},
				{Name: "Second", Score: "FAIL", Evidence: "assertion failed"},
			},
		},
	}

	criteria := []AcceptanceCriterion{
		{Criterion: "First criterion", Verification: "go test -run First"},
		{Criterion: "Second criterion", Verification: "go test -run Second"},
	}

	detail := TaskDetail{
		Task:               task,
		AcceptanceCriteria: criteria,
	}

	result := RenderTaskDetail(detail, 80)

	if !containsString(result, "PASS") {
		t.Error("Should show PASS status for first criterion")
	}

	if !containsString(result, "FAIL") {
		t.Error("Should show FAIL status for second criterion")
	}
}

func TestTaskDetailFiles(t *testing.T) {
	task := state.Task{
		ID:     "T001",
		Name:   "Task with files",
		Status: "pending",
		Phase:  1,
	}

	files := []TaskFile{
		{Path: "internal/pkg/main.go", Action: "create", Purpose: "Main package"},
		{Path: "internal/pkg/util.go", Action: "modify", Purpose: "Utility functions"},
		{Path: "internal/pkg/old.go", Action: "delete", Purpose: "Old implementation"},
	}

	detail := TaskDetail{
		Task:  task,
		Files: files,
	}

	result := RenderTaskDetail(detail, 80)

	expectedContent := []string{
		"Files",
		"internal/pkg/main.go",
		"create",
		"internal/pkg/util.go",
		"modify",
		"internal/pkg/old.go",
		"delete",
	}

	for _, expected := range expectedContent {
		if !containsString(result, expected) {
			t.Errorf("Task detail should contain '%s', got:\n%s", expected, result)
		}
	}
}

func TestTaskDetailFilesWithActionTypes(t *testing.T) {
	testCases := []struct {
		action   string
		expected string
	}{
		{action: "create", expected: "create"},
		{action: "modify", expected: "modify"},
		{action: "delete", expected: "delete"},
	}

	for _, tc := range testCases {
		t.Run(tc.action, func(t *testing.T) {
			files := []TaskFile{
				{Path: "test.go", Action: tc.action, Purpose: "test"},
			}

			detail := TaskDetail{
				Task:  state.Task{ID: "T001", Name: "Test", Status: "pending"},
				Files: files,
			}

			result := RenderTaskDetail(detail, 80)

			if !containsString(result, tc.expected) {
				t.Errorf("Expected action '%s' in output, got:\n%s", tc.expected, result)
			}
		})
	}
}

func TestBuildTaskDetail(t *testing.T) {
	task := state.Task{
		ID:     "T001",
		Name:   "Test task",
		Status: "pending",
		Phase:  1,
		File:   "",
	}

	detail := BuildTaskDetail(task)

	if detail.Task.ID != "T001" {
		t.Errorf("Expected task ID T001, got %s", detail.Task.ID)
	}

	if detail.AcceptanceCriteria != nil {
		t.Error("Expected nil AcceptanceCriteria when no file path")
	}

	if detail.Files != nil {
		t.Error("Expected nil Files when no file path")
	}
}

func TestLoadTaskDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "T001.json")

	taskJSON := `{
		"id": "T001",
		"name": "Test task",
		"acceptance_criteria": [
			{
				"criterion": "Feature works correctly",
				"verification": "go test ./..."
			}
		],
		"files": [
			{
				"path": "internal/pkg/main.go",
				"action": "create",
				"purpose": "Main package"
			}
		]
	}`

	if err := os.WriteFile(taskFile, []byte(taskJSON), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	def, err := LoadTaskDefinition(taskFile)
	if err != nil {
		t.Fatalf("LoadTaskDefinition failed: %v", err)
	}

	if def.ID != "T001" {
		t.Errorf("Expected ID T001, got %s", def.ID)
	}

	if len(def.AcceptanceCriteria) != 1 {
		t.Errorf("Expected 1 criterion, got %d", len(def.AcceptanceCriteria))
	}

	if def.AcceptanceCriteria[0].Criterion != "Feature works correctly" {
		t.Errorf("Unexpected criterion: %s", def.AcceptanceCriteria[0].Criterion)
	}

	if len(def.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(def.Files))
	}

	if def.Files[0].Action != "create" {
		t.Errorf("Expected action 'create', got '%s'", def.Files[0].Action)
	}
}

func TestLoadTaskDefinitionNotFound(t *testing.T) {
	_, err := LoadTaskDefinition("/nonexistent/path/T001.json")

	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadTaskDefinitionInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(taskFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadTaskDefinition(taskFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestRenderActionType(t *testing.T) {
	testCases := []struct {
		action   string
		expected string
	}{
		{"create", "create"},
		{"modify", "modify"},
		{"delete", "delete"},
		{"unknown", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.action, func(t *testing.T) {
			result := renderActionType(tc.action)
			if !containsString(result, tc.expected) {
				t.Errorf("renderActionType(%s) should contain '%s', got '%s'",
					tc.action, tc.expected, result)
			}
		})
	}
}

func TestRenderCriterionStatus(t *testing.T) {
	testCases := []struct {
		name     string
		task     state.Task
		index    int
		expected string
	}{
		{
			name:     "complete task shows PASS",
			task:     state.Task{Status: "complete"},
			index:    0,
			expected: "PASS",
		},
		{
			name:     "failed task shows FAIL",
			task:     state.Task{Status: "failed"},
			index:    0,
			expected: "FAIL",
		},
		{
			name:     "pending task shows placeholder",
			task:     state.Task{Status: "pending"},
			index:    0,
			expected: "----",
		},
		{
			name: "verified criterion with PASS",
			task: state.Task{
				Status: "complete",
				Verification: &state.TaskVerification{
					Criteria: []state.VerificationCriterion{
						{Score: "PASS"},
					},
				},
			},
			index:    0,
			expected: "PASS",
		},
		{
			name: "verified criterion with FAIL",
			task: state.Task{
				Status: "complete",
				Verification: &state.TaskVerification{
					Criteria: []state.VerificationCriterion{
						{Score: "FAIL"},
					},
				},
			},
			index:    0,
			expected: "FAIL",
		},
		{
			name: "verified criterion with PARTIAL",
			task: state.Task{
				Status: "complete",
				Verification: &state.TaskVerification{
					Criteria: []state.VerificationCriterion{
						{Score: "PARTIAL"},
					},
				},
			},
			index:    0,
			expected: "PART",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := renderCriterionStatus(tc.task, tc.index)
			if !containsString(result, tc.expected) {
				t.Errorf("Expected status to contain '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestRenderTaskDetailWithError(t *testing.T) {
	task := state.Task{
		ID:     "T001",
		Name:   "Failed task",
		Status: "failed",
		Phase:  1,
		Error:  "Build failed: compilation error",
	}

	detail := TaskDetail{Task: task}
	result := RenderTaskDetail(detail, 80)

	expectedContent := []string{
		"Error",
		"Build failed: compilation error",
	}

	for _, expected := range expectedContent {
		if !containsString(result, expected) {
			t.Errorf("Task detail should contain '%s', got:\n%s", expected, result)
		}
	}
}

func TestBuildTaskDetailWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "T001.json")

	taskJSON := `{
		"id": "T001",
		"name": "Test task",
		"acceptance_criteria": [
			{"criterion": "Test works", "verification": "go test"}
		],
		"files": [
			{"path": "main.go", "action": "create", "purpose": "Main"}
		]
	}`

	if err := os.WriteFile(taskFile, []byte(taskJSON), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	task := state.Task{
		ID:     "T001",
		Name:   "Test task",
		Status: "pending",
		Phase:  1,
		File:   taskFile,
	}

	detail := BuildTaskDetail(task)

	if len(detail.AcceptanceCriteria) != 1 {
		t.Errorf("Expected 1 acceptance criterion, got %d", len(detail.AcceptanceCriteria))
	}

	if len(detail.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(detail.Files))
	}
}
