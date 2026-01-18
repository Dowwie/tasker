package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createTestState(t *testing.T, tmpDir string) string {
	t.Helper()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing", Completed: []string{"ingestion", "logical"}},
		TargetDir: "/test/project",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "running", Phase: 1},
		},
		Execution: Execution{CurrentPhase: 1, ActiveTasks: []string{"T002"}},
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test state: %v", err)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("failed to write test state: %v", err)
	}

	return statePath
}

func TestRequestHalt(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	err := RequestHalt(statePath, "User requested pause for review", "orchestrator")
	if err != nil {
		t.Fatalf("RequestHalt failed: %v", err)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state after halt: %v", err)
	}

	if state.Halt == nil {
		t.Fatal("expected Halt to be set")
	}
	if !state.Halt.Requested {
		t.Error("expected Halt.Requested to be true")
	}
	if state.Halt.Reason != "User requested pause for review" {
		t.Errorf("expected reason 'User requested pause for review', got '%s'", state.Halt.Reason)
	}
	if state.Halt.RequestedBy != "orchestrator" {
		t.Errorf("expected requested_by 'orchestrator', got '%s'", state.Halt.RequestedBy)
	}
	if state.Halt.RequestedAt == "" {
		t.Error("expected RequestedAt to be set")
	}

	foundEvent := false
	for _, event := range state.Events {
		if event.Type == "halt_requested" {
			foundEvent = true
			if event.Details["reason"] != "User requested pause for review" {
				t.Errorf("event reason mismatch")
			}
			break
		}
	}
	if !foundEvent {
		t.Error("expected halt_requested event to be recorded")
	}
}

func TestRequestHaltOverwritesPrevious(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	err := RequestHalt(statePath, "First reason", "user1")
	if err != nil {
		t.Fatalf("first RequestHalt failed: %v", err)
	}

	err = RequestHalt(statePath, "Second reason", "user2")
	if err != nil {
		t.Fatalf("second RequestHalt failed: %v", err)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if state.Halt.Reason != "Second reason" {
		t.Errorf("expected reason 'Second reason', got '%s'", state.Halt.Reason)
	}
	if state.Halt.RequestedBy != "user2" {
		t.Errorf("expected requested_by 'user2', got '%s'", state.Halt.RequestedBy)
	}
}

func TestCheckHalt(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	halted, err := CheckHalt(statePath)
	if err != nil {
		t.Fatalf("CheckHalt failed: %v", err)
	}
	if halted {
		t.Error("expected halted to be false initially")
	}

	err = RequestHalt(statePath, "Test halt", "test")
	if err != nil {
		t.Fatalf("RequestHalt failed: %v", err)
	}

	halted, err = CheckHalt(statePath)
	if err != nil {
		t.Fatalf("CheckHalt after halt failed: %v", err)
	}
	if !halted {
		t.Error("expected halted to be true after RequestHalt")
	}
}

func TestCheckHaltWithNoHaltField(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/test",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(statePath, data, 0644)

	halted, err := CheckHalt(statePath)
	if err != nil {
		t.Fatalf("CheckHalt failed: %v", err)
	}
	if halted {
		t.Error("expected halted to be false when Halt field is nil")
	}
}

func TestResumeExecution(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	err := RequestHalt(statePath, "Pausing for maintenance", "admin")
	if err != nil {
		t.Fatalf("RequestHalt failed: %v", err)
	}

	halted, _ := CheckHalt(statePath)
	if !halted {
		t.Fatal("expected halted before resume")
	}

	err = ResumeExecution(statePath)
	if err != nil {
		t.Fatalf("ResumeExecution failed: %v", err)
	}

	halted, err = CheckHalt(statePath)
	if err != nil {
		t.Fatalf("CheckHalt after resume failed: %v", err)
	}
	if halted {
		t.Error("expected halted to be false after ResumeExecution")
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	foundEvent := false
	for _, event := range state.Events {
		if event.Type == "execution_resumed" {
			foundEvent = true
			if event.Details["previous_reason"] != "Pausing for maintenance" {
				t.Errorf("expected previous_reason in event details")
			}
			break
		}
	}
	if !foundEvent {
		t.Error("expected execution_resumed event to be recorded")
	}
}

func TestResumeExecutionWhenNotHalted(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	err := ResumeExecution(statePath)
	if err != nil {
		t.Fatalf("ResumeExecution when not halted should not fail: %v", err)
	}

	halted, _ := CheckHalt(statePath)
	if halted {
		t.Error("expected halted to remain false")
	}
}

func TestGetHaltStatus(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	status, err := GetHaltStatus(statePath)
	if err != nil {
		t.Fatalf("GetHaltStatus failed: %v", err)
	}
	if status.Halted {
		t.Error("expected Halted to be false initially")
	}
	if status.Reason != "" {
		t.Error("expected Reason to be empty initially")
	}

	err = RequestHalt(statePath, "Quota exceeded", "monitor")
	if err != nil {
		t.Fatalf("RequestHalt failed: %v", err)
	}

	status, err = GetHaltStatus(statePath)
	if err != nil {
		t.Fatalf("GetHaltStatus after halt failed: %v", err)
	}
	if !status.Halted {
		t.Error("expected Halted to be true")
	}
	if status.Reason != "Quota exceeded" {
		t.Errorf("expected reason 'Quota exceeded', got '%s'", status.Reason)
	}
	if status.RequestedBy != "monitor" {
		t.Errorf("expected requested_by 'monitor', got '%s'", status.RequestedBy)
	}
	if status.RequestedAt == "" {
		t.Error("expected RequestedAt to be set")
	}
}

func TestGetHaltStatusAfterResume(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := createTestState(t, tmpDir)

	RequestHalt(statePath, "Testing", "test")
	ResumeExecution(statePath)

	status, err := GetHaltStatus(statePath)
	if err != nil {
		t.Fatalf("GetHaltStatus failed: %v", err)
	}
	if status.Halted {
		t.Error("expected Halted to be false after resume")
	}
}

func TestStateManagerHaltMethods(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/test",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(statePath, data, 0644)

	sm := NewStateManager(tmpDir)

	halted, err := sm.CheckHalt()
	if err != nil {
		t.Fatalf("sm.CheckHalt failed: %v", err)
	}
	if halted {
		t.Error("expected not halted initially")
	}

	err = sm.RequestHalt("Testing via StateManager", "test")
	if err != nil {
		t.Fatalf("sm.RequestHalt failed: %v", err)
	}

	halted, _ = sm.CheckHalt()
	if !halted {
		t.Error("expected halted after RequestHalt")
	}

	status, err := sm.GetHaltStatus()
	if err != nil {
		t.Fatalf("sm.GetHaltStatus failed: %v", err)
	}
	if status.Reason != "Testing via StateManager" {
		t.Errorf("unexpected reason: %s", status.Reason)
	}

	err = sm.ResumeExecution()
	if err != nil {
		t.Fatalf("sm.ResumeExecution failed: %v", err)
	}

	halted, _ = sm.CheckHalt()
	if halted {
		t.Error("expected not halted after resume")
	}
}

func TestHaltWithInvalidStatePath(t *testing.T) {
	_, err := CheckHalt("/nonexistent/path/state.json")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}

	err = RequestHalt("/nonexistent/path/state.json", "test", "test")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}

	err = ResumeExecution("/nonexistent/path/state.json")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}

	_, err = GetHaltStatus("/nonexistent/path/state.json")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}
