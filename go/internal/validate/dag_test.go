package validate

import (
	"testing"
)

func TestDetectCycles_NoCycle(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{"T002"}},
	}

	err := DetectCycles(tasks)
	if err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{"T003"}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{"T002"}},
	}

	err := DetectCycles(tasks)
	if err == nil {
		t.Error("expected cycle error, got nil")
		return
	}

	if len(err.Cycle) < 2 {
		t.Errorf("expected cycle with at least 2 elements, got: %v", err.Cycle)
	}
}

func TestDetectCycles_SelfCycle(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{"T001"}},
	}

	err := DetectCycles(tasks)
	if err == nil {
		t.Error("expected cycle error for self-referencing task, got nil")
		return
	}

	found := false
	for _, id := range err.Cycle {
		if id == "T001" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected T001 in cycle, got: %v", err.Cycle)
	}
}

func TestDetectCycles_DisconnectedComponents(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{}},
		"T004": {ID: "T004", DependsOn: []string{"T003"}},
	}

	err := DetectCycles(tasks)
	if err != nil {
		t.Errorf("expected no cycle with disconnected components, got: %v", err)
	}
}

func TestDetectCycles_ComplexDAG(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{"T001"}},
		"T004": {ID: "T004", DependsOn: []string{"T002", "T003"}},
		"T005": {ID: "T005", DependsOn: []string{"T004"}},
	}

	err := DetectCycles(tasks)
	if err != nil {
		t.Errorf("expected no cycle in valid DAG, got: %v", err)
	}
}

func TestCheckDependencyExistence_AllExist(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{"T001", "T002"}},
	}

	errs := CheckDependencyExistence(tasks)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestCheckDependencyExistence_MissingDeps(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001", "T099"}},
		"T003": {ID: "T003", DependsOn: []string{"T999"}},
	}

	errs := CheckDependencyExistence(tasks)
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
	}

	foundT002 := false
	foundT003 := false
	for _, err := range errs {
		if err.TaskID == "T002" {
			foundT002 = true
			if len(err.MissingDeps) != 1 || err.MissingDeps[0] != "T099" {
				t.Errorf("expected T002 to miss T099, got: %v", err.MissingDeps)
			}
		}
		if err.TaskID == "T003" {
			foundT003 = true
			if len(err.MissingDeps) != 1 || err.MissingDeps[0] != "T999" {
				t.Errorf("expected T003 to miss T999, got: %v", err.MissingDeps)
			}
		}
	}

	if !foundT002 {
		t.Error("expected error for T002")
	}
	if !foundT003 {
		t.Error("expected error for T003")
	}
}

func TestCheckDependencyExistence_EmptyTasks(t *testing.T) {
	tasks := map[string]Task{}

	errs := CheckDependencyExistence(tasks)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty task map, got: %v", errs)
	}
}

func TestValidateSteelThread_NoSteelThread(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}, SteelThread: false},
		"T002": {ID: "T002", DependsOn: []string{"T001"}, SteelThread: false},
	}

	err := ValidateSteelThread(tasks)
	if err != nil {
		t.Errorf("expected no error when no steel thread tasks, got: %v", err)
	}
}

func TestValidateSteelThread_ValidPath(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}, SteelThread: true},
		"T002": {ID: "T002", DependsOn: []string{"T001"}, SteelThread: true},
		"T003": {ID: "T003", DependsOn: []string{"T002"}, SteelThread: true},
	}

	err := ValidateSteelThread(tasks)
	if err != nil {
		t.Errorf("expected no error for valid steel thread, got: %v", err)
	}
}

func TestValidateSteelThread_DependsOnNonSteelThread(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}, SteelThread: false},
		"T002": {ID: "T002", DependsOn: []string{"T001"}, SteelThread: true},
	}

	err := ValidateSteelThread(tasks)
	if err == nil {
		t.Error("expected error when steel thread depends on non-steel-thread task")
		return
	}

	expected := "steel thread task T002 depends on non-steel-thread task T001"
	if err.Message != expected {
		t.Errorf("expected error message '%s', got: '%s'", expected, err.Message)
	}
}

func TestValidateSteelThread_DependsOnNonExistent(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{"T999"}, SteelThread: true},
	}

	err := ValidateSteelThread(tasks)
	if err == nil {
		t.Error("expected error when steel thread depends on non-existent task")
		return
	}

	expected := "steel thread task T001 depends on non-existent task T999"
	if err.Message != expected {
		t.Errorf("expected error message '%s', got: '%s'", expected, err.Message)
	}
}

func TestValidateDAG_Valid(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{"T001", "T002"}},
	}

	result := ValidateDAG(tasks)
	if !result.Valid {
		t.Errorf("expected valid DAG, got errors: %v", result.Errors)
	}
}

func TestValidateDAG_MultipleErrors(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{"T003"}},
		"T002": {ID: "T002", DependsOn: []string{"T001", "T999"}},
		"T003": {ID: "T003", DependsOn: []string{"T002"}},
	}

	result := ValidateDAG(tasks)
	if result.Valid {
		t.Error("expected invalid DAG")
	}

	if len(result.Errors) < 2 {
		t.Errorf("expected at least 2 errors (cycle + missing dep), got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestTopologicalSort_Valid(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
		"T003": {ID: "T003", DependsOn: []string{"T001"}},
		"T004": {ID: "T004", DependsOn: []string{"T002", "T003"}},
	}

	order, err := TopologicalSort(tasks)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if len(order) != 4 {
		t.Errorf("expected 4 tasks in order, got %d: %v", len(order), order)
		return
	}

	positions := make(map[string]int)
	for i, id := range order {
		positions[id] = i
	}

	if positions["T001"] > positions["T002"] {
		t.Error("T001 should come before T002")
	}
	if positions["T001"] > positions["T003"] {
		t.Error("T001 should come before T003")
	}
	if positions["T002"] > positions["T004"] {
		t.Error("T002 should come before T004")
	}
	if positions["T003"] > positions["T004"] {
		t.Error("T003 should come before T004")
	}
}

func TestTopologicalSort_WithCycle(t *testing.T) {
	tasks := map[string]Task{
		"T001": {ID: "T001", DependsOn: []string{"T002"}},
		"T002": {ID: "T002", DependsOn: []string{"T001"}},
	}

	_, err := TopologicalSort(tasks)
	if err == nil {
		t.Error("expected error for cycle")
		return
	}

	_, ok := err.(*CycleError)
	if !ok {
		t.Errorf("expected CycleError, got: %T", err)
	}
}

func TestCycleError_Error(t *testing.T) {
	err := &CycleError{Cycle: []string{"T001", "T002", "T001"}}
	expected := "dependency cycle detected: [T001 T002 T001]"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestMissingDependencyError_Error(t *testing.T) {
	err := &MissingDependencyError{TaskID: "T002", MissingDeps: []string{"T099", "T100"}}
	expected := "task T002 references non-existent dependencies: [T099 T100]"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestSteelThreadError_Error(t *testing.T) {
	err := &SteelThreadError{Message: "test message"}
	if err.Error() != "test message" {
		t.Errorf("expected 'test message', got '%s'", err.Error())
	}
}
