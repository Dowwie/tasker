package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetStatus(t *testing.T) {
	state := &State{
		Phase: PhaseState{Current: "executing", Completed: []string{"ingestion", "ready"}},
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "running", Phase: 1},
			"T003": {ID: "T003", Status: "pending", Phase: 2, DependsOn: []string{"T001"}},
			"T004": {ID: "T004", Status: "failed", Phase: 1},
			"T005": {ID: "T005", Status: "skipped", Phase: 1},
		},
		Execution: Execution{CurrentPhase: 1},
	}

	summary := GetStatus(state)

	if summary.Phase != "executing" {
		t.Errorf("expected phase 'executing', got '%s'", summary.Phase)
	}
	if summary.TotalTasks != 5 {
		t.Errorf("expected 5 total tasks, got %d", summary.TotalTasks)
	}
	if summary.ByStatus["complete"] != 1 {
		t.Errorf("expected 1 complete, got %d", summary.ByStatus["complete"])
	}
	if summary.ByStatus["running"] != 1 {
		t.Errorf("expected 1 running, got %d", summary.ByStatus["running"])
	}
	if summary.ByStatus["pending"] != 1 {
		t.Errorf("expected 1 pending, got %d", summary.ByStatus["pending"])
	}
	if summary.ByStatus["failed"] != 1 {
		t.Errorf("expected 1 failed, got %d", summary.ByStatus["failed"])
	}
	if summary.ByStatus["skipped"] != 1 {
		t.Errorf("expected 1 skipped, got %d", summary.ByStatus["skipped"])
	}
	if len(summary.ActiveTasks) != 1 || summary.ActiveTasks[0] != "T002" {
		t.Errorf("expected active tasks ['T002'], got %v", summary.ActiveTasks)
	}
	if len(summary.FailedTasks) != 1 || summary.FailedTasks[0] != "T004" {
		t.Errorf("expected failed tasks ['T004'], got %v", summary.FailedTasks)
	}
	if summary.CurrentPhase != 1 {
		t.Errorf("expected current phase 1, got %d", summary.CurrentPhase)
	}
	if summary.PhaseProgress != "2/5" {
		t.Errorf("expected phase progress '2/5', got '%s'", summary.PhaseProgress)
	}
}

func TestGetStatusEmptyTasks(t *testing.T) {
	state := &State{
		Phase: PhaseState{Current: "ingestion"},
		Tasks: map[string]Task{},
	}

	summary := GetStatus(state)

	if summary.TotalTasks != 0 {
		t.Errorf("expected 0 total tasks, got %d", summary.TotalTasks)
	}
	if summary.PhaseProgress != "0/0" {
		t.Errorf("expected phase progress '0/0', got '%s'", summary.PhaseProgress)
	}
}

func TestGetStatusReadyTasks(t *testing.T) {
	state := &State{
		Phase: PhaseState{Current: "executing"},
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1, DependsOn: []string{"T001"}},
			"T003": {ID: "T003", Status: "pending", Phase: 1, DependsOn: []string{"T002"}},
		},
	}

	summary := GetStatus(state)

	if len(summary.ReadyTasks) != 1 || summary.ReadyTasks[0] != "T002" {
		t.Errorf("expected ready tasks ['T002'], got %v", summary.ReadyTasks)
	}
}

func TestAdvancePhase(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion", Completed: []string{}},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	newPhase, err := AdvancePhase(sm)
	if err != nil {
		t.Fatalf("AdvancePhase failed: %v", err)
	}

	if newPhase != "spec_review" {
		t.Errorf("expected phase 'spec_review', got '%s'", newPhase)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Phase.Current != "spec_review" {
		t.Errorf("expected current phase 'spec_review', got '%s'", loaded.Phase.Current)
	}
	if len(loaded.Phase.Completed) != 1 || loaded.Phase.Completed[0] != "ingestion" {
		t.Errorf("expected completed phases ['ingestion'], got %v", loaded.Phase.Completed)
	}
}

func TestAdvancePhaseFromReady(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ready", Completed: []string{"ingestion", "logical"}},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	newPhase, err := AdvancePhase(sm)
	if err != nil {
		t.Fatalf("AdvancePhase failed: %v", err)
	}

	if newPhase != "executing" {
		t.Errorf("expected phase 'executing', got '%s'", newPhase)
	}
}

func TestAdvancePhaseFromExecutingAllComplete(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing", Completed: []string{"ready"}},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "skipped", Phase: 1},
		},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	newPhase, err := AdvancePhase(sm)
	if err != nil {
		t.Fatalf("AdvancePhase failed: %v", err)
	}

	if newPhase != "complete" {
		t.Errorf("expected phase 'complete', got '%s'", newPhase)
	}
}

func TestAdvancePhaseFromExecutingNotComplete(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing", Completed: []string{"ready"}},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "running", Phase: 1},
		},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	_, err := AdvancePhase(sm)
	if err == nil {
		t.Error("expected error when advancing from executing with incomplete tasks")
	}
}

func TestAdvancePhaseFromComplete(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "complete", Completed: []string{"executing"}},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	_, err := AdvancePhase(sm)
	if err == nil {
		t.Error("expected error when already at final phase")
	}
}

func TestAdvancePhaseEvent(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	_, err := AdvancePhase(sm)
	if err != nil {
		t.Fatalf("AdvancePhase failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Type != "phase_advanced" {
		t.Errorf("expected event type 'phase_advanced', got '%s'", loaded.Events[0].Type)
	}
	if loaded.Events[0].Details["from"] != "ingestion" {
		t.Errorf("expected from 'ingestion', got '%v'", loaded.Events[0].Details["from"])
	}
	if loaded.Events[0].Details["to"] != "spec_review" {
		t.Errorf("expected to 'spec_review', got '%v'", loaded.Events[0].Details["to"])
	}
}

func TestValidateArtifact(t *testing.T) {
	tmpDir := t.TempDir()

	artifact := map[string]interface{}{
		"id":      "CAP001",
		"name":    "Test Capability",
		"version": "1.0",
	}
	artifactData, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "artifact.json"), artifactData, 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	result, err := ValidateArtifact(tmpDir, "artifact.json", "")
	if err != nil {
		t.Fatalf("ValidateArtifact failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid artifact, got errors: %v", result.Errors)
	}
	if result.Path != "artifact.json" {
		t.Errorf("expected path 'artifact.json', got '%s'", result.Path)
	}
	if result.ValidatedAt == "" {
		t.Error("expected validated_at to be set")
	}
}

func TestValidateArtifactWithSchema(t *testing.T) {
	tmpDir := t.TempDir()

	artifact := map[string]interface{}{
		"id":   "CAP001",
		"name": "Test Capability",
	}
	artifactData, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "artifact.json"), artifactData, 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	schema := JSONSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"id":   {Type: "string"},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}
	schemaData, _ := json.MarshalIndent(schema, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "schema.json"), schemaData, 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	result, err := ValidateArtifact(tmpDir, "artifact.json", "schema.json")
	if err != nil {
		t.Fatalf("ValidateArtifact failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid artifact, got errors: %v", result.Errors)
	}
}

func TestValidateArtifactMissingRequired(t *testing.T) {
	tmpDir := t.TempDir()

	artifact := map[string]interface{}{
		"id": "CAP001",
	}
	artifactData, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "artifact.json"), artifactData, 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	schema := JSONSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"id":   {Type: "string"},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}
	schemaData, _ := json.MarshalIndent(schema, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "schema.json"), schemaData, 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	result, err := ValidateArtifact(tmpDir, "artifact.json", "schema.json")
	if err != nil {
		t.Fatalf("ValidateArtifact failed: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid artifact for missing required field")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors for missing required field")
	}
}

func TestValidateArtifactWrongType(t *testing.T) {
	tmpDir := t.TempDir()

	artifact := map[string]interface{}{
		"id":   123,
		"name": "Test",
	}
	artifactData, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "artifact.json"), artifactData, 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	schema := JSONSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"id":   {Type: "string"},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}
	schemaData, _ := json.MarshalIndent(schema, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "schema.json"), schemaData, 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	result, err := ValidateArtifact(tmpDir, "artifact.json", "schema.json")
	if err != nil {
		t.Fatalf("ValidateArtifact failed: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid artifact for wrong type")
	}
}

func TestValidateArtifactNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ValidateArtifact(tmpDir, "nonexistent.json", "")
	if err != nil {
		t.Fatalf("ValidateArtifact should return result, not error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for missing file")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors for missing file")
	}
}

func TestValidateArtifactInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "artifact.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	result, err := ValidateArtifact(tmpDir, "artifact.json", "")
	if err != nil {
		t.Fatalf("ValidateArtifact should return result, not error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for invalid JSON")
	}
}

func TestValidateArtifactSchemaNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	artifact := map[string]interface{}{"id": "test"}
	artifactData, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "artifact.json"), artifactData, 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	result, err := ValidateArtifact(tmpDir, "artifact.json", "nonexistent-schema.json")
	if err != nil {
		t.Fatalf("ValidateArtifact should return result, not error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for missing schema")
	}
}

func TestCheckType(t *testing.T) {
	tests := []struct {
		val      interface{}
		expected string
		valid    bool
	}{
		{"hello", "string", true},
		{123.0, "number", true},
		{123.0, "integer", true},
		{123.5, "integer", false},
		{true, "boolean", true},
		{[]interface{}{1, 2}, "array", true},
		{map[string]interface{}{"a": 1}, "object", true},
		{nil, "null", true},
		{"hello", "number", false},
		{123.0, "string", false},
	}

	for _, tc := range tests {
		result := checkType(tc.val, tc.expected)
		if result != tc.valid {
			t.Errorf("checkType(%v, %s) = %v, expected %v", tc.val, tc.expected, result, tc.valid)
		}
	}
}
