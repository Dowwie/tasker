package fsm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompileFromSpec(t *testing.T) {
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Test Workflow",
				Trigger:       "user action",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{
						Name:        "Step 1",
						Description: "First step",
						Behaviors:   []string{"B001"},
					},
					{
						Name:        "Step 2",
						Description: "Second step",
						Behaviors:   []string{"B002"},
					},
				},
				Postconditions: []string{"Workflow completed successfully"},
			},
		},
		Invariants: []map[string]interface{}{
			{
				"id":   "INV-001",
				"rule": "All inputs must be validated",
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromSpec(specData)

	if err != nil {
		t.Fatalf("CompileFromSpec failed: %v", err)
	}

	if machine == nil {
		t.Fatal("Expected machine to be non-nil")
	}

	if machine.ID != "M1" {
		t.Errorf("Expected machine ID 'M1', got '%s'", machine.ID)
	}

	if machine.Name != "Test Workflow" {
		t.Errorf("Expected machine name 'Test Workflow', got '%s'", machine.Name)
	}

	if machine.Level != "steel_thread" {
		t.Errorf("Expected level 'steel_thread', got '%s'", machine.Level)
	}

	if len(machine.States) < 4 {
		t.Errorf("Expected at least 4 states (initial + 2 steps + success), got %d", len(machine.States))
	}

	initialFound := false
	successFound := false
	for _, s := range machine.States {
		if s.Type == "initial" {
			initialFound = true
		}
		if s.Type == "success" {
			successFound = true
		}
	}
	if !initialFound {
		t.Error("Expected to find initial state")
	}
	if !successFound {
		t.Error("Expected to find success state")
	}

	if len(machine.Transitions) < 3 {
		t.Errorf("Expected at least 3 transitions, got %d", len(machine.Transitions))
	}
}

func TestCompileFromSpec_NoWorkflow(t *testing.T) {
	specData := &SpecData{
		Workflows:  []Workflow{},
		Invariants: nil,
	}

	compiler := NewCompiler()
	_, err := compiler.CompileFromSpec(specData)

	if err == nil {
		t.Fatal("Expected error for empty workflows")
	}
}

func TestCompileFromSpec_EmptySteps(t *testing.T) {
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:  "Empty Workflow",
				Steps: []WorkflowStep{},
			},
		},
	}

	compiler := NewCompiler()
	_, err := compiler.CompileFromSpec(specData)

	if err == nil {
		t.Fatal("Expected error for workflow with no steps")
	}
}

func TestCompileFromSpec_WithVariants(t *testing.T) {
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Variant Workflow",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Process"},
				},
				Variants: []WorkflowVariant{
					{
						Condition: "If validation fails",
						Outcome:   "Retry",
					},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromSpec(specData)

	if err != nil {
		t.Fatalf("CompileFromSpec with variants failed: %v", err)
	}

	variantTransitionFound := false
	for _, tr := range machine.Transitions {
		if tr.Trigger == "If validation fails" {
			variantTransitionFound = true
			break
		}
	}
	if !variantTransitionFound {
		t.Error("Expected variant transition not found")
	}
}

func TestCompileFromSpec_WithFailures(t *testing.T) {
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Failure Workflow",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Process"},
				},
				Failures: []WorkflowFailure{
					{
						Condition: "System error",
						Outcome:   "Error state",
					},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromSpec(specData)

	if err != nil {
		t.Fatalf("CompileFromSpec with failures failed: %v", err)
	}

	failureStateFound := false
	for _, s := range machine.States {
		if s.Type == "failure" {
			failureStateFound = true
			break
		}
	}
	if !failureStateFound {
		t.Error("Expected failure state not found")
	}

	failureTransitionFound := false
	for _, tr := range machine.Transitions {
		if tr.IsFailurePath {
			failureTransitionFound = true
			break
		}
	}
	if !failureTransitionFound {
		t.Error("Expected failure transition not found")
	}
}

func TestCompileFromCapabilityMap(t *testing.T) {
	capMap := &CapabilityMap{
		SpecChecksum: "abc123",
		Domains: []CapabilityMapDomain{
			{
				Capabilities: []CapabilityMapCapability{
					{
						Behaviors: []CapabilityMapBehavior{
							{ID: "B001", Name: "ValidateInput", Description: "Validate user input"},
							{ID: "B002", Name: "ProcessData", Description: "Process the data"},
						},
					},
				},
			},
		},
		Flows: []CapabilityMapFlow{
			{
				Name:          "Main Flow",
				IsSteelThread: true,
				Steps: []map[string]interface{}{
					{"behavior_id": "B001", "description": "Validate input"},
					{"behavior_id": "B002", "description": "Process data"},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromCapabilityMap(capMap, "")

	if err != nil {
		t.Fatalf("CompileFromCapabilityMap failed: %v", err)
	}

	if machine == nil {
		t.Fatal("Expected machine to be non-nil")
	}

	if machine.Name != "Main Flow" {
		t.Errorf("Expected machine name 'Main Flow', got '%s'", machine.Name)
	}

	behaviorFound := false
	for _, s := range machine.States {
		if len(s.Behaviors) > 0 {
			behaviorFound = true
			break
		}
	}
	if !behaviorFound {
		t.Error("Expected behaviors to be linked to states")
	}
}

func TestCompileFromCapabilityMap_NoFlows(t *testing.T) {
	capMap := &CapabilityMap{
		Flows: []CapabilityMapFlow{},
	}

	compiler := NewCompiler()
	_, err := compiler.CompileFromCapabilityMap(capMap, "")

	if err == nil {
		t.Fatal("Expected error for capability map with no flows")
	}
}

func TestCompileFromCapabilityMap_Nil(t *testing.T) {
	compiler := NewCompiler()
	_, err := compiler.CompileFromCapabilityMap(nil, "")

	if err == nil {
		t.Fatal("Expected error for nil capability map")
	}
}

func TestWriteStates(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.states.json")

	machine := &Machine{
		ID:    "M1",
		Name:  "Test Machine",
		Level: "steel_thread",
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial", SpecRef: &SpecRef{Quote: "Start"}},
			{ID: "S2", Name: "Processing", Type: "normal", SpecRef: &SpecRef{Quote: "Process"}},
			{ID: "S3", Name: "Complete", Type: "success", SpecRef: &SpecRef{Quote: "Done"}},
		},
	}

	compiler := NewCompiler()
	err := compiler.WriteStates(machine, outputPath)

	if err != nil {
		t.Fatalf("WriteStates failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var statesFile StatesFile
	if err := json.Unmarshal(data, &statesFile); err != nil {
		t.Fatalf("Failed to parse states file: %v", err)
	}

	if statesFile.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", statesFile.Version)
	}

	if statesFile.MachineID != "M1" {
		t.Errorf("Expected machine_id 'M1', got '%s'", statesFile.MachineID)
	}

	if statesFile.InitialState != "S1" {
		t.Errorf("Expected initial_state 'S1', got '%s'", statesFile.InitialState)
	}

	if len(statesFile.TerminalStates) != 1 || statesFile.TerminalStates[0] != "S3" {
		t.Errorf("Expected terminal_states ['S3'], got %v", statesFile.TerminalStates)
	}

	if len(statesFile.States) != 3 {
		t.Errorf("Expected 3 states, got %d", len(statesFile.States))
	}
}

func TestWriteStates_NilMachine(t *testing.T) {
	compiler := NewCompiler()
	err := compiler.WriteStates(nil, "/tmp/test.json")

	if err == nil {
		t.Fatal("Expected error for nil machine")
	}
}

func TestWriteStates_FailureState(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.states.json")

	machine := &Machine{
		ID:    "M1",
		Name:  "Test Machine",
		Level: "steel_thread",
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial", SpecRef: &SpecRef{Quote: "Start"}},
			{ID: "S2", Name: "Success", Type: "success", SpecRef: &SpecRef{Quote: "Done"}},
			{ID: "S3", Name: "Error", Type: "failure", SpecRef: &SpecRef{Quote: "Error"}},
		},
	}

	compiler := NewCompiler()
	err := compiler.WriteStates(machine, outputPath)

	if err != nil {
		t.Fatalf("WriteStates failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var statesFile StatesFile
	if err := json.Unmarshal(data, &statesFile); err != nil {
		t.Fatalf("Failed to parse states file: %v", err)
	}

	if len(statesFile.TerminalStates) != 2 {
		t.Errorf("Expected 2 terminal states (success + failure), got %d", len(statesFile.TerminalStates))
	}
}

func TestWriteTransitions(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.transitions.json")

	machine := &Machine{
		ID:    "M1",
		Name:  "Test Machine",
		Level: "steel_thread",
		Transitions: []Transition{
			{
				ID:        "TR1",
				FromState: "S1",
				ToState:   "S2",
				Trigger:   "start",
				SpecRef:   &SpecRef{Quote: "Begin process"},
			},
			{
				ID:        "TR2",
				FromState: "S2",
				ToState:   "S3",
				Trigger:   "complete",
				Guards: []Guard{
					{Condition: "valid input", InvariantID: "INV-001"},
				},
				SpecRef: &SpecRef{Quote: "Finish process"},
			},
		},
	}

	compiler := NewCompiler()
	err := compiler.WriteTransitions(machine, outputPath)

	if err != nil {
		t.Fatalf("WriteTransitions failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var transitionsFile TransitionsFile
	if err := json.Unmarshal(data, &transitionsFile); err != nil {
		t.Fatalf("Failed to parse transitions file: %v", err)
	}

	if transitionsFile.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", transitionsFile.Version)
	}

	if transitionsFile.MachineID != "M1" {
		t.Errorf("Expected machine_id 'M1', got '%s'", transitionsFile.MachineID)
	}

	if len(transitionsFile.Transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(transitionsFile.Transitions))
	}

	if transitionsFile.GuardsIndex["INV-001"] == nil || len(transitionsFile.GuardsIndex["INV-001"]) != 1 {
		t.Errorf("Expected guards_index to contain INV-001 -> [TR2]")
	}
}

func TestWriteTransitions_NilMachine(t *testing.T) {
	compiler := NewCompiler()
	err := compiler.WriteTransitions(nil, "/tmp/test.json")

	if err == nil {
		t.Fatal("Expected error for nil machine")
	}
}

func TestWriteTransitions_FailurePath(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test.transitions.json")

	machine := &Machine{
		ID:    "M1",
		Name:  "Test Machine",
		Level: "steel_thread",
		Transitions: []Transition{
			{
				ID:            "TR1",
				FromState:     "S1",
				ToState:       "S3",
				Trigger:       "error",
				IsFailurePath: true,
				SpecRef:       &SpecRef{Quote: "Error handler"},
			},
		},
	}

	compiler := NewCompiler()
	err := compiler.WriteTransitions(machine, outputPath)

	if err != nil {
		t.Fatalf("WriteTransitions failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var transitionsFile TransitionsFile
	if err := json.Unmarshal(data, &transitionsFile); err != nil {
		t.Fatalf("Failed to parse transitions file: %v", err)
	}

	if !transitionsFile.Transitions[0].IsFailurePath {
		t.Error("Expected is_failure_path to be true")
	}
}

func TestExport(t *testing.T) {
	tempDir := t.TempDir()

	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Test Workflow",
				Trigger:       "start",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Step 1", Description: "First step"},
				},
				Postconditions: []string{"Done"},
			},
		},
		Invariants: []map[string]interface{}{
			{"id": "INV-001", "rule": "Test rule"},
		},
	}

	compiler := NewCompiler()
	_, err := compiler.CompileFromSpec(specData)
	if err != nil {
		t.Fatalf("CompileFromSpec failed: %v", err)
	}

	index, err := compiler.Export(tempDir, "test-spec", "abc123")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if index.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", index.Version)
	}

	if index.SpecSlug != "test-spec" {
		t.Errorf("Expected spec_slug 'test-spec', got '%s'", index.SpecSlug)
	}

	if index.SpecChecksum != "abc123" {
		t.Errorf("Expected spec_checksum 'abc123', got '%s'", index.SpecChecksum)
	}

	if index.PrimaryMachine != "M1" {
		t.Errorf("Expected primary_machine 'M1', got '%s'", index.PrimaryMachine)
	}

	if len(index.Machines) != 1 {
		t.Errorf("Expected 1 machine, got %d", len(index.Machines))
	}

	indexPath := filepath.Join(tempDir, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Expected index.json to exist")
	}

	statesPath := filepath.Join(tempDir, "steel-thread.states.json")
	if _, err := os.Stat(statesPath); os.IsNotExist(err) {
		t.Error("Expected steel-thread.states.json to exist")
	}

	transitionsPath := filepath.Join(tempDir, "steel-thread.transitions.json")
	if _, err := os.Stat(transitionsPath); os.IsNotExist(err) {
		t.Error("Expected steel-thread.transitions.json to exist")
	}
}

func TestComputeChecksum(t *testing.T) {
	checksum1 := ComputeChecksum("test content")
	checksum2 := ComputeChecksum("test content")
	checksum3 := ComputeChecksum("different content")

	if checksum1 != checksum2 {
		t.Error("Same content should produce same checksum")
	}

	if checksum1 == checksum3 {
		t.Error("Different content should produce different checksum")
	}

	if len(checksum1) != 16 {
		t.Errorf("Expected checksum length 16, got %d", len(checksum1))
	}
}

func TestExtractGuards_WithInvariant(t *testing.T) {
	compiler := NewCompiler()
	compiler.invariants = []map[string]interface{}{
		{
			"id":   "INV-001",
			"rule": "Input must be validated before processing",
		},
	}

	guards := compiler.extractGuards("Input validation failed")

	if len(guards) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(guards))
	}

	if guards[0].InvariantID != "INV-001" {
		t.Errorf("Expected invariant_id 'INV-001', got '%s'", guards[0].InvariantID)
	}
}

func TestExtractGuards_NoMatch(t *testing.T) {
	compiler := NewCompiler()
	compiler.invariants = []map[string]interface{}{
		{
			"id":   "INV-001",
			"rule": "Completely unrelated rule about something else",
		},
	}

	guards := compiler.extractGuards("Network timeout occurred")

	if len(guards) != 1 {
		t.Fatalf("Expected 1 guard (fallback), got %d", len(guards))
	}

	if guards[0].InvariantID != "" {
		t.Error("Expected empty invariant_id for non-matching guard")
	}
}

func TestExtractGuards_MissingRuleText(t *testing.T) {
	compiler := NewCompiler()
	compiler.invariants = []map[string]interface{}{
		{
			"id": "INV-001",
		},
		{
			"id":   "INV-002",
			"rule": "",
		},
	}

	guards := compiler.extractGuards("Some condition")

	if len(guards) != 1 {
		t.Fatalf("Expected 1 guard (fallback), got %d", len(guards))
	}
	if guards[0].InvariantID != "" {
		t.Error("Expected empty invariant_id when invariants have no rule text")
	}
}

func TestExtractGuards_WithMustKeyword(t *testing.T) {
	compiler := NewCompiler()
	compiler.invariants = []map[string]interface{}{
		{
			"id":   "INV-001",
			"rule": "Data must be formatted correctly",
		},
	}

	guards := compiler.extractGuards("Input must be validated")

	if len(guards) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(guards))
	}
	if guards[0].InvariantID != "INV-001" {
		t.Errorf("Expected invariant_id 'INV-001', got '%s'", guards[0].InvariantID)
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{"first non-empty", []string{"", "second", "third"}, "second"},
		{"all empty", []string{"", "", ""}, ""},
		{"first value", []string{"first", "second"}, "first"},
		{"single empty", []string{""}, ""},
		{"single value", []string{"only"}, "only"},
		{"no values", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesce(tt.values...)
			if result != tt.expected {
				t.Errorf("coalesce(%v) = %q, want %q", tt.values, result, tt.expected)
			}
		})
	}
}

func TestCompileFromSpec_WithFailuresReusesExistingState(t *testing.T) {
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Failure Reuse Workflow",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Process"},
				},
				Failures: []WorkflowFailure{
					{
						Condition: "Error occurred",
						Outcome:   "Error",
					},
					{
						Condition: "Another error",
						Outcome:   "Error",
					},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromSpec(specData)

	if err != nil {
		t.Fatalf("CompileFromSpec failed: %v", err)
	}

	failureStates := 0
	for _, s := range machine.States {
		if s.Type == "failure" {
			failureStates++
		}
	}
	if failureStates != 1 {
		t.Errorf("Expected 1 failure state (reused), got %d", failureStates)
	}
}

func TestCompileFromSpec_WithFailureFromStep(t *testing.T) {
	fromStep := 1
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "FromStep Workflow",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Step One"},
					{Name: "Step Two"},
					{Name: "Step Three"},
				},
				Failures: []WorkflowFailure{
					{
						Condition: "Step specific error",
						Outcome:   "Step Error",
						FromStep:  &fromStep,
					},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromSpec(specData)

	if err != nil {
		t.Fatalf("CompileFromSpec failed: %v", err)
	}

	failureTransitions := 0
	for _, tr := range machine.Transitions {
		if tr.IsFailurePath {
			failureTransitions++
		}
	}
	if failureTransitions != 1 {
		t.Errorf("Expected 1 failure transition (from specific step), got %d", failureTransitions)
	}
}

func TestCompileFromSpec_WithFailureEmptyOutcome(t *testing.T) {
	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Default Outcome Workflow",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Process"},
				},
				Failures: []WorkflowFailure{
					{
						Condition: "Something went wrong",
						Outcome:   "",
					},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromSpec(specData)

	if err != nil {
		t.Fatalf("CompileFromSpec failed: %v", err)
	}

	foundErrorState := false
	for _, s := range machine.States {
		if s.Type == "failure" && s.Name == "Error" {
			foundErrorState = true
			break
		}
	}
	if !foundErrorState {
		t.Error("Expected default 'Error' failure state")
	}
}

func TestExport_NoMachinesCompiled(t *testing.T) {
	tempDir := t.TempDir()
	compiler := NewCompiler()

	index, err := compiler.Export(tempDir, "test-spec", "abc123")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if index.PrimaryMachine != "" {
		t.Errorf("Expected empty primary_machine when no machines compiled, got '%s'", index.PrimaryMachine)
	}
	if len(index.Machines) != 0 {
		t.Errorf("Expected 0 machines, got %d", len(index.Machines))
	}
}

func TestExport_SteelThreadHasPriority(t *testing.T) {
	tempDir := t.TempDir()

	specData := &SpecData{
		Workflows: []Workflow{
			{
				Name:          "Steel Thread",
				Trigger:       "start",
				IsSteelThread: true,
				Steps: []WorkflowStep{
					{Name: "Step 1", Description: "First step"},
				},
				Postconditions: []string{"Done"},
			},
		},
	}

	compiler := NewCompiler()
	_, err := compiler.CompileFromSpec(specData)
	if err != nil {
		t.Fatalf("CompileFromSpec failed: %v", err)
	}

	index, err := compiler.Export(tempDir, "test-spec", "abc123")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(index.Machines) != 1 {
		t.Errorf("Expected 1 machine, got %d", len(index.Machines))
	}

	if index.PrimaryMachine != "M1" {
		t.Errorf("Expected primary_machine 'M1', got '%s'", index.PrimaryMachine)
	}

	if index.Machines[0].Level != "steel_thread" {
		t.Errorf("Expected level 'steel_thread', got '%s'", index.Machines[0].Level)
	}
}

func TestWriteStates_InvalidPath(t *testing.T) {
	machine := &Machine{
		ID:    "M1",
		Name:  "Test",
		Level: "steel_thread",
		States: []State{
			{ID: "S1", Name: "Initial", Type: "initial"},
		},
	}

	compiler := NewCompiler()
	err := compiler.WriteStates(machine, "/nonexistent/path/states.json")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestWriteTransitions_InvalidPath(t *testing.T) {
	machine := &Machine{
		ID:    "M1",
		Name:  "Test",
		Level: "steel_thread",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2"},
		},
	}

	compiler := NewCompiler()
	err := compiler.WriteTransitions(machine, "/nonexistent/path/transitions.json")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestCompileFromCapabilityMap_NoSteelThreadFlow(t *testing.T) {
	capMap := &CapabilityMap{
		SpecChecksum: "abc123",
		Flows: []CapabilityMapFlow{
			{
				Name:          "Non-Steel Flow",
				IsSteelThread: false,
				Steps: []map[string]interface{}{
					{"behavior_id": "B001"},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromCapabilityMap(capMap, "")

	if err != nil {
		t.Fatalf("CompileFromCapabilityMap failed: %v", err)
	}

	if machine.Name != "Non-Steel Flow" {
		t.Errorf("Expected name 'Non-Steel Flow', got '%s'", machine.Name)
	}
}

func TestCompileFromCapabilityMap_WithSpecPath(t *testing.T) {
	capMap := &CapabilityMap{
		SpecChecksum: "abc123",
		Flows: []CapabilityMapFlow{
			{
				Name:          "Flow",
				IsSteelThread: true,
				Steps: []map[string]interface{}{
					{"behavior_id": "B001"},
				},
			},
		},
	}

	compiler := NewCompiler()
	machine, err := compiler.CompileFromCapabilityMap(capMap, "/path/to/spec.md")

	if err != nil {
		t.Fatalf("CompileFromCapabilityMap failed: %v", err)
	}

	if machine == nil {
		t.Fatal("Expected machine to be non-nil")
	}
}
