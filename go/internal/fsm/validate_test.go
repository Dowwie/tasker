package fsm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateInvariants_ValidFSM(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "machine.states.json",
					"transitions": "machine.transitions.json",
				},
			},
		},
	}

	states := StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S3"},
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial"},
			{ID: "S2", Name: "Processing", Type: "normal"},
			{ID: "S3", Name: "Complete", Type: "success"},
		},
	}

	transitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "start"},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "complete"},
		},
	}

	writeJSON(t, tempDir, "index.json", index)
	writeJSON(t, tempDir, "machine.states.json", states)
	writeJSON(t, tempDir, "machine.transitions.json", transitions)

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected validation to pass, got issues: %+v", result.Issues)
	}
}

func TestValidateInvariants_MissingPrimaryMachine(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "",
		Machines:       []MachineEntry{},
	}

	writeJSON(t, tempDir, "index.json", index)

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for missing primary_machine")
	}

	foundI1Issue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I1" {
			foundI1Issue = true
			break
		}
	}
	if !foundI1Issue {
		t.Error("Expected I1 invariant issue for missing primary_machine")
	}
}

func TestValidateInvariants_WrongLevel(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "domain",
				Files: map[string]string{
					"states":      "machine.states.json",
					"transitions": "machine.transitions.json",
				},
			},
		},
	}

	states := StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S2"},
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial"},
			{ID: "S2", Name: "Done", Type: "success"},
		},
	}

	transitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	writeJSON(t, tempDir, "index.json", index)
	writeJSON(t, tempDir, "machine.states.json", states)
	writeJSON(t, tempDir, "machine.transitions.json", transitions)

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for wrong level on primary_machine")
	}

	foundI1Issue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I1" && issue.Context != nil && issue.Context["level"] == "domain" {
			foundI1Issue = true
			break
		}
	}
	if !foundI1Issue {
		t.Error("Expected I1 invariant issue for wrong level")
	}
}

func TestCheckCompleteness_ValidMachine(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S3"},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "Middle", Type: "normal"},
			{ID: "S3", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "begin"},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "finish"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if !result.Passed {
		t.Errorf("Expected completeness check to pass, got issues: %+v", result.Issues)
	}
}

func TestCheckCompleteness_NoInitialState(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "",
		TerminalStates: []string{"S2"},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if result.Passed {
		t.Error("Expected completeness check to fail for missing initial_state")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" && issue.Message == "Machine M1: No initial_state defined" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected I3 issue for missing initial_state")
	}
}

func TestCheckCompleteness_DeadEndState(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S3"},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "DeadEnd", Type: "normal"},
			{ID: "S3", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if result.Passed {
		t.Error("Expected completeness check to fail for dead-end state")
	}

	foundDeadEnd := false
	foundUnreachable := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" && issue.Message == "Machine M1: Non-terminal state 'S2' (DeadEnd) has no outgoing transitions" {
			foundDeadEnd = true
		}
		if issue.Invariant == "I3" && issue.Message == "Machine M1: State 'S3' unreachable from initial" {
			foundUnreachable = true
		}
	}
	if !foundDeadEnd {
		t.Error("Expected I3 issue for dead-end state without outgoing transitions")
	}
	if !foundUnreachable {
		t.Error("Expected I3 issue for unreachable state S3")
	}
}

func TestCheckCompleteness_InvalidTransitionReferences(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S2"},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
			{ID: "TR2", FromState: "S99", ToState: "S100", Trigger: "invalid"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if result.Passed {
		t.Error("Expected completeness check to fail for invalid transition references")
	}

	foundFromIssue := false
	foundToIssue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" && issue.Message == "Machine M1: Transition TR2 references unknown from_state 'S99'" {
			foundFromIssue = true
		}
		if issue.Invariant == "I3" && issue.Message == "Machine M1: Transition TR2 references unknown to_state 'S100'" {
			foundToIssue = true
		}
	}
	if !foundFromIssue {
		t.Error("Expected I3 issue for unknown from_state")
	}
	if !foundToIssue {
		t.Error("Expected I3 issue for unknown to_state")
	}
}

func TestComputeCoverage_FullCoverage(t *testing.T) {
	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go", Behaviors: []string{"B001"}},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "finish", Behaviors: []string{"B002"}},
		},
	}

	result := ComputeCoverage(transitions)

	if result.TotalTransitions != 2 {
		t.Errorf("Expected 2 total transitions, got %d", result.TotalTransitions)
	}
	if result.CoveredTransitions != 2 {
		t.Errorf("Expected 2 covered transitions, got %d", result.CoveredTransitions)
	}
	if result.CoveragePercent != 100 {
		t.Errorf("Expected 100%% coverage, got %.1f%%", result.CoveragePercent)
	}
	if len(result.UncoveredIDs) != 0 {
		t.Errorf("Expected no uncovered transitions, got %v", result.UncoveredIDs)
	}
}

func TestComputeCoverage_PartialCoverage(t *testing.T) {
	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go", Behaviors: []string{"B001"}},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "finish", Behaviors: []string{}},
			{ID: "TR3", FromState: "S1", ToState: "S3", Trigger: "skip"},
		},
	}

	result := ComputeCoverage(transitions)

	if result.TotalTransitions != 3 {
		t.Errorf("Expected 3 total transitions, got %d", result.TotalTransitions)
	}
	if result.CoveredTransitions != 1 {
		t.Errorf("Expected 1 covered transition, got %d", result.CoveredTransitions)
	}
	if result.CoveragePercent < 33 || result.CoveragePercent > 34 {
		t.Errorf("Expected ~33%% coverage, got %.1f%%", result.CoveragePercent)
	}
	if len(result.UncoveredIDs) != 2 {
		t.Errorf("Expected 2 uncovered transitions, got %v", result.UncoveredIDs)
	}
}

func TestComputeCoverage_ZeroCoverage(t *testing.T) {
	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "finish"},
		},
	}

	result := ComputeCoverage(transitions)

	if result.CoveredTransitions != 0 {
		t.Errorf("Expected 0 covered transitions, got %d", result.CoveredTransitions)
	}
	if result.CoveragePercent != 0 {
		t.Errorf("Expected 0%% coverage, got %.1f%%", result.CoveragePercent)
	}
}

func TestComputeCoverage_EmptyTransitions(t *testing.T) {
	transitions := &TransitionsFile{
		Version:     "1.0",
		MachineID:   "M1",
		Transitions: []Transition{},
	}

	result := ComputeCoverage(transitions)

	if result.TotalTransitions != 0 {
		t.Errorf("Expected 0 total transitions, got %d", result.TotalTransitions)
	}
	if result.CoveragePercent != 100 {
		t.Errorf("Expected 100%% coverage for empty transitions, got %.1f%%", result.CoveragePercent)
	}
}

func TestCheckTaskCoverage_FullCoverage(t *testing.T) {
	tempDir := t.TempDir()

	index := &IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Steel Thread",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "m1.states.json",
					"transitions": "m1.transitions.json",
				},
			},
		},
	}

	transitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "finish"},
		},
	}

	writeJSON(t, tempDir, "m1.transitions.json", transitions)

	tasksCoverage := []TaskTransitionCoverage{
		{TaskID: "T001", TransitionsCovered: []string{"TR1"}},
		{TaskID: "T002", TransitionsCovered: []string{"TR2"}},
	}

	result, err := CheckTaskCoverage(index, tempDir, tasksCoverage, 1.0, 0.9)
	if err != nil {
		t.Fatalf("CheckTaskCoverage failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected task coverage check to pass, got issues: %+v", result.Issues)
	}
	if result.SteelThreadCoverage.CoveragePercent != 100 {
		t.Errorf("Expected 100%% steel thread coverage, got %.1f%%", result.SteelThreadCoverage.CoveragePercent)
	}
}

func TestCheckTaskCoverage_BelowThreshold(t *testing.T) {
	tempDir := t.TempDir()

	index := &IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Steel Thread",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "m1.states.json",
					"transitions": "m1.transitions.json",
				},
			},
		},
	}

	transitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "finish"},
			{ID: "TR3", FromState: "S3", ToState: "S4", Trigger: "more"},
			{ID: "TR4", FromState: "S4", ToState: "S5", Trigger: "done"},
		},
	}

	writeJSON(t, tempDir, "m1.transitions.json", transitions)

	tasksCoverage := []TaskTransitionCoverage{
		{TaskID: "T001", TransitionsCovered: []string{"TR1"}},
	}

	result, err := CheckTaskCoverage(index, tempDir, tasksCoverage, 1.0, 0.9)
	if err != nil {
		t.Fatalf("CheckTaskCoverage failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected task coverage check to fail due to low steel thread coverage")
	}

	foundIssue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "TASK_COVERAGE" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Error("Expected TASK_COVERAGE issue")
	}
}

func TestCheckTaskCoverage_MultipleMachines(t *testing.T) {
	tempDir := t.TempDir()

	index := &IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Steel Thread",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "m1.states.json",
					"transitions": "m1.transitions.json",
				},
			},
			{
				ID:    "M2",
				Name:  "Domain Machine",
				Level: "domain",
				Files: map[string]string{
					"states":      "m2.states.json",
					"transitions": "m2.transitions.json",
				},
			},
		},
	}

	steelTransitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	domainTransitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M2",
		Transitions: []Transition{
			{ID: "TR10", FromState: "D1", ToState: "D2", Trigger: "domain_go"},
			{ID: "TR11", FromState: "D2", ToState: "D3", Trigger: "domain_done"},
		},
	}

	writeJSON(t, tempDir, "m1.transitions.json", steelTransitions)
	writeJSON(t, tempDir, "m2.transitions.json", domainTransitions)

	tasksCoverage := []TaskTransitionCoverage{
		{TaskID: "T001", TransitionsCovered: []string{"TR1"}},
		{TaskID: "T002", TransitionsCovered: []string{"TR10", "TR11"}},
	}

	result, err := CheckTaskCoverage(index, tempDir, tasksCoverage, 1.0, 1.0)
	if err != nil {
		t.Fatalf("CheckTaskCoverage failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected task coverage check to pass, got issues: %+v", result.Issues)
	}

	if result.SteelThreadCoverage.TotalTransitions != 1 {
		t.Errorf("Expected 1 steel thread transition, got %d", result.SteelThreadCoverage.TotalTransitions)
	}
	if result.NonSteelCoverage.TotalTransitions != 2 {
		t.Errorf("Expected 2 non-steel thread transitions, got %d", result.NonSteelCoverage.TotalTransitions)
	}
}

func TestValidateI4GuardLinkage_WithLinkedGuards(t *testing.T) {
	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{
				ID:        "TR1",
				FromState: "S1",
				ToState:   "S2",
				Trigger:   "validate",
				Guards: []Guard{
					{Condition: "input is valid", InvariantID: "INV-001"},
				},
			},
		},
	}

	result := NewValidationResult()
	validateI4GuardLinkage(transitions, result)

	if len(result.Warnings) > 0 {
		t.Errorf("Expected no warnings for linked guards, got: %+v", result.Warnings)
	}
}

func TestValidateI4GuardLinkage_WithUnlinkedGuards(t *testing.T) {
	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{
				ID:        "TR1",
				FromState: "S1",
				ToState:   "S2",
				Trigger:   "validate",
				Guards: []Guard{
					{Condition: "some condition", InvariantID: ""},
				},
			},
		},
	}

	result := NewValidationResult()
	validateI4GuardLinkage(transitions, result)

	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning for unlinked guard, got %d", len(result.Warnings))
	}
}

func TestValidateInvariants_IndexNotFound(t *testing.T) {
	_, err := ValidateInvariants("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestValidateInvariants_InvalidIndexJSON(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "index.json"), []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid index.json: %v", err)
	}

	_, err := ValidateInvariants(tempDir)
	if err == nil {
		t.Error("Expected error for invalid JSON in index.json")
	}
}

func TestValidateInvariants_PrimaryMachineNotFound(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M999",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "machine.states.json",
					"transitions": "machine.transitions.json",
				},
			},
		},
	}

	writeJSON(t, tempDir, "index.json", index)

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for primary_machine not found")
	}

	foundI1Issue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I1" && issue.Message == "primary_machine 'M999' not found in machines list" {
			foundI1Issue = true
			break
		}
	}
	if !foundI1Issue {
		t.Error("Expected I1 issue for missing primary_machine in machines list")
	}
}

func TestValidateInvariants_StatesFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "missing.states.json",
					"transitions": "machine.transitions.json",
				},
			},
		},
	}

	writeJSON(t, tempDir, "index.json", index)

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for missing states file")
	}

	foundI3Issue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" {
			foundI3Issue = true
			break
		}
	}
	if !foundI3Issue {
		t.Error("Expected I3 issue for missing states file")
	}
}

func TestValidateInvariants_TransitionsFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "machine.states.json",
					"transitions": "missing.transitions.json",
				},
			},
		},
	}

	states := StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S2"},
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial"},
			{ID: "S2", Name: "Done", Type: "success"},
		},
	}

	writeJSON(t, tempDir, "index.json", index)
	writeJSON(t, tempDir, "machine.states.json", states)

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for missing transitions file")
	}
}

func TestValidateInvariants_InvalidStatesJSON(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "machine.states.json",
					"transitions": "machine.transitions.json",
				},
			},
		},
	}

	writeJSON(t, tempDir, "index.json", index)
	if err := os.WriteFile(filepath.Join(tempDir, "machine.states.json"), []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid states file: %v", err)
	}

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for invalid states JSON")
	}
}

func TestValidateInvariants_InvalidTransitionsJSON(t *testing.T) {
	tempDir := t.TempDir()

	index := IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Test Machine",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "machine.states.json",
					"transitions": "machine.transitions.json",
				},
			},
		},
	}

	states := StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S2"},
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial"},
			{ID: "S2", Name: "Done", Type: "success"},
		},
	}

	writeJSON(t, tempDir, "index.json", index)
	writeJSON(t, tempDir, "machine.states.json", states)
	if err := os.WriteFile(filepath.Join(tempDir, "machine.transitions.json"), []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid transitions file: %v", err)
	}

	result, err := ValidateInvariants(tempDir)
	if err != nil {
		t.Fatalf("ValidateInvariants failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected validation to fail for invalid transitions JSON")
	}
}

func TestCheckCompleteness_NoTerminalStates(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S1", Trigger: "loop"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if result.Passed {
		t.Error("Expected completeness check to fail for no terminal states")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" && issue.Message == "Machine M1: No terminal_states defined" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected I3 issue for no terminal states")
	}
}

func TestCheckCompleteness_InvalidInitialStateReference(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S99",
		TerminalStates: []string{"S2"},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if result.Passed {
		t.Error("Expected completeness check to fail for invalid initial_state reference")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" && issue.Message == "Machine M1: initial_state 'S99' not in states list" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected I3 issue for invalid initial_state, got: %v", result.Issues)
	}
}

func TestCheckCompleteness_InvalidTerminalStateReference(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S99"},
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	result := CheckCompleteness(states, transitions)

	if result.Passed {
		t.Error("Expected completeness check to fail for invalid terminal_state reference")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Invariant == "I3" && issue.Message == "Machine M1: terminal_state 'S99' not in states list" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected I3 issue for invalid terminal_state, got: %v", result.Issues)
	}
}

func TestCheckTaskCoverage_NonSteelBelowThreshold(t *testing.T) {
	tempDir := t.TempDir()

	index := &IndexFile{
		Version:        "1.0",
		SpecSlug:       "test-spec",
		SpecChecksum:   "abc1234567890123",
		PrimaryMachine: "M1",
		Machines: []MachineEntry{
			{
				ID:    "M1",
				Name:  "Steel Thread",
				Level: "steel_thread",
				Files: map[string]string{
					"states":      "m1.states.json",
					"transitions": "m1.transitions.json",
				},
			},
			{
				ID:    "M2",
				Name:  "Domain",
				Level: "domain",
				Files: map[string]string{
					"states":      "m2.states.json",
					"transitions": "m2.transitions.json",
				},
			},
		},
	}

	steelTransitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
		},
	}

	domainTransitions := TransitionsFile{
		Version:   "1.0",
		MachineID: "M2",
		Transitions: []Transition{
			{ID: "TR10", FromState: "D1", ToState: "D2", Trigger: "a"},
			{ID: "TR11", FromState: "D2", ToState: "D3", Trigger: "b"},
			{ID: "TR12", FromState: "D3", ToState: "D4", Trigger: "c"},
			{ID: "TR13", FromState: "D4", ToState: "D5", Trigger: "d"},
		},
	}

	writeJSON(t, tempDir, "m1.transitions.json", steelTransitions)
	writeJSON(t, tempDir, "m2.transitions.json", domainTransitions)

	tasksCoverage := []TaskTransitionCoverage{
		{TaskID: "T001", TransitionsCovered: []string{"TR1"}},
		{TaskID: "T002", TransitionsCovered: []string{"TR10"}},
	}

	result, err := CheckTaskCoverage(index, tempDir, tasksCoverage, 1.0, 0.9)
	if err != nil {
		t.Fatalf("CheckTaskCoverage failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected task coverage check to fail due to low non-steel thread coverage")
	}

	foundIssue := false
	for _, issue := range result.Issues {
		if issue.Invariant == "TASK_COVERAGE" && issue.Context["required"] == "90%" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Errorf("Expected TASK_COVERAGE issue for non-steel, got: %v", result.Issues)
	}
}

func writeJSON(t *testing.T, dir, filename string, data interface{}) {
	t.Helper()
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}
