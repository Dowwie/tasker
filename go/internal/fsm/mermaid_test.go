package fsm

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S3", "S4"},
		States: []State{
			{ID: "S1", Name: "Awaiting start", Type: "initial", SpecRef: &SpecRef{Quote: "Start"}},
			{ID: "S2", Name: "Processing", Type: "normal", SpecRef: &SpecRef{Quote: "Process"}},
			{ID: "S3", Name: "Complete", Type: "success", SpecRef: &SpecRef{Quote: "Done"}},
			{ID: "S4", Name: "Error", Type: "failure", SpecRef: &SpecRef{Quote: "Failed"}},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "start", SpecRef: &SpecRef{Quote: "Begin"}},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "complete", SpecRef: &SpecRef{Quote: "Finish"}},
			{ID: "TR3", FromState: "S2", ToState: "S4", Trigger: "error", IsFailurePath: true, SpecRef: &SpecRef{Quote: "Fail"}},
		},
		GuardsIndex: map[string][]string{},
	}

	mermaid := GenerateMermaid("Test Machine", states, transitions)

	if !strings.Contains(mermaid, "stateDiagram-v2") {
		t.Error("Expected mermaid output to contain 'stateDiagram-v2'")
	}

	if !strings.Contains(mermaid, "title Test Machine") {
		t.Error("Expected mermaid output to contain 'title Test Machine'")
	}

	if !strings.Contains(mermaid, "[*] --> S1") {
		t.Error("Expected initial state transition from [*]")
	}

	if !strings.Contains(mermaid, "S3 --> [*]") {
		t.Error("Expected success state transition to [*]")
	}

	if !strings.Contains(mermaid, "[FAILURE]") {
		t.Error("Expected failure state to have [FAILURE] marker")
	}

	if !strings.Contains(mermaid, "S1 --> S2: start") {
		t.Error("Expected transition S1 --> S2: start")
	}

	if !strings.Contains(mermaid, "S2 --> S3: complete") {
		t.Error("Expected transition S2 --> S3: complete")
	}
}

func TestGenerateMermaid_NilInputs(t *testing.T) {
	result := GenerateMermaid("Test", nil, nil)
	if result != "" {
		t.Error("Expected empty string for nil inputs")
	}

	result = GenerateMermaid("Test", &StatesFile{}, nil)
	if result != "" {
		t.Error("Expected empty string for nil transitions")
	}

	result = GenerateMermaid("Test", nil, &TransitionsFile{})
	if result != "" {
		t.Error("Expected empty string for nil states")
	}
}

func TestGenerateMermaid_LongTrigger(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "normal"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "This is a very long trigger name that should be truncated"},
		},
	}

	mermaid := GenerateMermaid("Test", states, transitions)

	if strings.Contains(mermaid, "truncated") {
		t.Error("Expected long trigger to be truncated")
	}

	if !strings.Contains(mermaid, "...") {
		t.Error("Expected truncated trigger to end with '...'")
	}
}

func TestGenerateMermaid_QuotesInNames(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: `State with "quotes"`, Type: "initial"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S1", Trigger: `Trigger with "quotes"`},
		},
	}

	mermaid := GenerateMermaid("Test", states, transitions)

	if strings.Contains(mermaid, `"quotes"`) {
		t.Error("Expected double quotes to be replaced with single quotes")
	}

	if !strings.Contains(mermaid, `'quotes'`) {
		t.Error("Expected quotes to be replaced with single quotes")
	}
}

func TestGenerateNotes(t *testing.T) {
	states := &StatesFile{
		Version:        "1.0",
		MachineID:      "M1",
		InitialState:   "S1",
		TerminalStates: []string{"S3"},
		States: []State{
			{ID: "S1", Name: "Awaiting start", Type: "initial", Description: "Initial state", SpecRef: &SpecRef{Quote: "Start"}},
			{ID: "S2", Name: "Processing", Type: "normal", Description: "Processing data", SpecRef: &SpecRef{Quote: "Process"}},
			{ID: "S3", Name: "Complete", Type: "success", Description: "Done", SpecRef: &SpecRef{Quote: "Done"}},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{
				ID:        "TR1",
				FromState: "S1",
				ToState:   "S2",
				Trigger:   "start",
				Guards:    []Guard{{Condition: "input valid", InvariantID: "INV-001"}},
				SpecRef:   &SpecRef{Quote: "Begin"},
			},
			{
				ID:        "TR2",
				FromState: "S2",
				ToState:   "S3",
				Trigger:   "complete",
				SpecRef:   &SpecRef{Quote: "Finish"},
			},
		},
		GuardsIndex: map[string][]string{
			"INV-001": {"TR1"},
		},
	}

	notes := GenerateNotes("Test Machine", states, transitions)

	if !strings.Contains(notes, "# Test Machine") {
		t.Error("Expected markdown header with machine name")
	}

	if !strings.Contains(notes, "## States") {
		t.Error("Expected States section")
	}

	if !strings.Contains(notes, "**Initial State:**") {
		t.Error("Expected initial state callout")
	}

	if !strings.Contains(notes, "**Terminal States:**") {
		t.Error("Expected terminal states callout")
	}

	if !strings.Contains(notes, "### All States") {
		t.Error("Expected All States subsection")
	}

	if !strings.Contains(notes, "| ID | Name | Type | Description |") {
		t.Error("Expected states table header")
	}

	if !strings.Contains(notes, "| S1 |") {
		t.Error("Expected state S1 in table")
	}

	if !strings.Contains(notes, "## Transitions") {
		t.Error("Expected Transitions section")
	}

	if !strings.Contains(notes, "| ID | From | To | Trigger | Guards |") {
		t.Error("Expected transitions table header")
	}

	if !strings.Contains(notes, "INV-001") {
		t.Error("Expected guard with invariant ID in transitions")
	}

	if !strings.Contains(notes, "## Guards Index") {
		t.Error("Expected Guards Index section")
	}

	if !strings.Contains(notes, "**INV-001**") {
		t.Error("Expected invariant in guards index")
	}

	if !strings.Contains(notes, "## Diagram") {
		t.Error("Expected Diagram section")
	}

	if !strings.Contains(notes, "```mermaid") {
		t.Error("Expected mermaid code block")
	}

	if !strings.Contains(notes, "stateDiagram-v2") {
		t.Error("Expected embedded mermaid diagram")
	}
}

func TestGenerateNotes_NilInputs(t *testing.T) {
	result := GenerateNotes("Test", nil, nil)
	if result != "" {
		t.Error("Expected empty string for nil inputs")
	}

	result = GenerateNotes("Test", &StatesFile{}, nil)
	if result != "" {
		t.Error("Expected empty string for nil transitions")
	}

	result = GenerateNotes("Test", nil, &TransitionsFile{})
	if result != "" {
		t.Error("Expected empty string for nil states")
	}
}

func TestGenerateNotes_NoGuardsIndex(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
		},
	}

	transitions := &TransitionsFile{
		Version:     "1.0",
		MachineID:   "M1",
		Transitions: []Transition{},
		GuardsIndex: map[string][]string{},
	}

	notes := GenerateNotes("Test", states, transitions)

	if strings.Contains(notes, "## Guards Index") {
		t.Error("Expected no Guards Index section when empty")
	}
}

func TestGenerateNotes_FailurePath(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "Error", Type: "failure"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "fail", IsFailurePath: true},
		},
		GuardsIndex: map[string][]string{},
	}

	notes := GenerateNotes("Test", states, transitions)

	if !strings.Contains(notes, "[FAILURE]") {
		t.Error("Expected [FAILURE] marker for failure path transition")
	}
}

func TestGenerateNotes_LongDescription(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial", Description: "This is a very long description that should definitely be truncated because it exceeds the fifty character limit"},
		},
	}

	transitions := &TransitionsFile{
		Version:     "1.0",
		MachineID:   "M1",
		Transitions: []Transition{},
		GuardsIndex: map[string][]string{},
	}

	notes := GenerateNotes("Test", states, transitions)

	if strings.Contains(notes, "fifty character limit") {
		t.Error("Expected long description to be truncated")
	}

	if !strings.Contains(notes, "...") {
		t.Error("Expected truncated description to end with '...'")
	}
}

func TestGenerateNotes_FallbackToSpecRef(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial", SpecRef: &SpecRef{Quote: "Spec reference text"}},
		},
	}

	transitions := &TransitionsFile{
		Version:     "1.0",
		MachineID:   "M1",
		Transitions: []Transition{},
		GuardsIndex: map[string][]string{},
	}

	notes := GenerateNotes("Test", states, transitions)

	if !strings.Contains(notes, "Spec reference text") {
		t.Error("Expected spec_ref quote to be used when description is empty")
	}
}

func TestGenerateNotes_GuardWithConditionOnly(t *testing.T) {
	states := &StatesFile{
		Version:   "1.0",
		MachineID: "M1",
		States: []State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "success"},
		},
	}

	transitions := &TransitionsFile{
		Version:   "1.0",
		MachineID: "M1",
		Transitions: []Transition{
			{
				ID:        "TR1",
				FromState: "S1",
				ToState:   "S2",
				Trigger:   "go",
				Guards:    []Guard{{Condition: "ready"}},
			},
		},
		GuardsIndex: map[string][]string{},
	}

	notes := GenerateNotes("Test", states, transitions)

	if !strings.Contains(notes, "ready") {
		t.Error("Expected guard condition to appear in notes")
	}
}
