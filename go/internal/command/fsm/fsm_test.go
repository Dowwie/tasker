package fsm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsmlib "github.com/dgordon/tasker/internal/fsm"
)

func withPlanningDir(tmpDir string, fn func()) {
	original := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = original }()
	fn()
}

func TestCompileCmd(t *testing.T) {
	t.Run("compiles valid spec", func(t *testing.T) {
		tmpDir := t.TempDir()

		spec := fsmlib.SpecData{
			Workflows: []fsmlib.Workflow{
				{
					Name:          "Test Workflow",
					IsSteelThread: true,
					Steps: []fsmlib.WorkflowStep{
						{Name: "Step 1", Description: "First step"},
						{Name: "Step 2", Description: "Second step"},
					},
				},
			},
		}

		specPath := filepath.Join(tmpDir, "spec.json")
		data, _ := json.MarshalIndent(spec, "", "  ")
		os.WriteFile(specPath, data, 0644)

		outputDir = filepath.Join(tmpDir, "fsm")

		withPlanningDir(tmpDir, func() {
			err := compileCmd.RunE(compileCmd, []string{specPath})
			if err != nil {
				t.Errorf("compileCmd should succeed, got: %v", err)
			}
		})

		outputDir = ""
	})

	t.Run("fails for nonexistent file", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := compileCmd.RunE(compileCmd, []string{"/nonexistent/spec.json"})
			if err == nil {
				t.Error("compileCmd should fail for nonexistent file")
			}
		})
	})

	t.Run("fails for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "spec.json")
		os.WriteFile(specPath, []byte("not json"), 0644)

		withPlanningDir(tmpDir, func() {
			err := compileCmd.RunE(compileCmd, []string{specPath})
			if err == nil {
				t.Error("compileCmd should fail for invalid JSON")
			}
		})
	})

	t.Run("uses default output dir", func(t *testing.T) {
		tmpDir := t.TempDir()

		spec := fsmlib.SpecData{
			Workflows: []fsmlib.Workflow{
				{
					Name:          "Test",
					IsSteelThread: true,
					Steps: []fsmlib.WorkflowStep{
						{Name: "Step"},
					},
				},
			},
		}

		specPath := filepath.Join(tmpDir, "spec.json")
		data, _ := json.MarshalIndent(spec, "", "  ")
		os.WriteFile(specPath, data, 0644)

		outputDir = ""

		withPlanningDir(tmpDir, func() {
			err := compileCmd.RunE(compileCmd, []string{specPath})
			if err != nil {
				t.Errorf("compileCmd should succeed, got: %v", err)
			}

			fsmPath := filepath.Join(tmpDir, "fsm", "index.json")
			if _, err := os.Stat(fsmPath); os.IsNotExist(err) {
				t.Error("expected FSM to be created in default location")
			}
		})
	})
}

func TestFromCapMapCmd(t *testing.T) {
	t.Run("compiles from capability map", func(t *testing.T) {
		tmpDir := t.TempDir()

		capMap := fsmlib.CapabilityMap{
			SpecChecksum: "abc123",
			Flows: []fsmlib.CapabilityMapFlow{
				{
					Name:          "Main Flow",
					IsSteelThread: true,
					Steps: []map[string]interface{}{
						{"behavior_id": "B001", "description": "Step 1"},
					},
				},
			},
		}

		capMapPath := filepath.Join(tmpDir, "capabilities.json")
		data, _ := json.MarshalIndent(capMap, "", "  ")
		os.WriteFile(capMapPath, data, 0644)

		outputDir = filepath.Join(tmpDir, "fsm")

		withPlanningDir(tmpDir, func() {
			err := fromCapMapCmd.RunE(fromCapMapCmd, []string{capMapPath})
			if err != nil {
				t.Errorf("fromCapMapCmd should succeed, got: %v", err)
			}
		})

		outputDir = ""
	})

	t.Run("accepts optional spec file", func(t *testing.T) {
		tmpDir := t.TempDir()

		capMap := fsmlib.CapabilityMap{
			Flows: []fsmlib.CapabilityMapFlow{
				{
					Name:          "Flow",
					IsSteelThread: true,
					Steps: []map[string]interface{}{
						{"behavior_id": "B001"},
					},
				},
			},
		}

		capMapPath := filepath.Join(tmpDir, "capabilities.json")
		data, _ := json.MarshalIndent(capMap, "", "  ")
		os.WriteFile(capMapPath, data, 0644)

		specPath := filepath.Join(tmpDir, "spec.md")
		os.WriteFile(specPath, []byte("# Spec\n"), 0644)

		outputDir = filepath.Join(tmpDir, "fsm")

		withPlanningDir(tmpDir, func() {
			err := fromCapMapCmd.RunE(fromCapMapCmd, []string{capMapPath, specPath})
			if err != nil {
				t.Errorf("fromCapMapCmd with spec should succeed, got: %v", err)
			}
		})

		outputDir = ""
	})

	t.Run("fails for nonexistent file", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := fromCapMapCmd.RunE(fromCapMapCmd, []string{"/nonexistent/cap.json"})
			if err == nil {
				t.Error("fromCapMapCmd should fail for nonexistent file")
			}
		})
	})
}

func TestMermaidCmd(t *testing.T) {
	t.Run("generates mermaid diagram", func(t *testing.T) {
		tmpDir := t.TempDir()
		fsmDir := filepath.Join(tmpDir, "fsm")
		os.MkdirAll(fsmDir, 0755)

		index := fsmlib.IndexFile{
			Version:        "1.0",
			SpecSlug:       "test",
			PrimaryMachine: "M1",
			Machines: []fsmlib.MachineEntry{
				{
					ID:    "M1",
					Name:  "Test Machine",
					Level: "steel_thread",
					Files: map[string]string{
						"states":      "states.json",
						"transitions": "transitions.json",
					},
				},
			},
		}

		states := fsmlib.StatesFile{
			Version:        "1.0",
			MachineID:      "M1",
			InitialState:   "S1",
			TerminalStates: []string{"S2"},
			States: []fsmlib.State{
				{ID: "S1", Name: "Start", Type: "initial"},
				{ID: "S2", Name: "Done", Type: "success"},
			},
		}

		transitions := fsmlib.TransitionsFile{
			Version:   "1.0",
			MachineID: "M1",
			Transitions: []fsmlib.Transition{
				{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "go"},
			},
		}

		writeJSON(t, fsmDir, "index.json", index)
		writeJSON(t, fsmDir, "states.json", states)
		writeJSON(t, fsmDir, "transitions.json", transitions)

		err := mermaidCmd.RunE(mermaidCmd, []string{fsmDir})
		if err != nil {
			t.Errorf("mermaidCmd should succeed, got: %v", err)
		}
	})

	t.Run("fails for nonexistent dir", func(t *testing.T) {
		err := mermaidCmd.RunE(mermaidCmd, []string{"/nonexistent/fsm"})
		if err == nil {
			t.Error("mermaidCmd should fail for nonexistent directory")
		}
	})
}

func TestGenerateMermaid(t *testing.T) {
	states := &fsmlib.StatesFile{
		States: []fsmlib.State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "Processing", Type: "normal"},
			{ID: "S3", Name: "Success", Type: "success"},
			{ID: "S4", Name: "Failed", Type: "failure"},
		},
	}

	transitions := &fsmlib.TransitionsFile{
		Transitions: []fsmlib.Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "begin"},
			{ID: "TR2", FromState: "S2", ToState: "S3", Trigger: "complete"},
			{ID: "TR3", FromState: "S2", ToState: "S4", Trigger: "error"},
		},
	}

	result := generateMermaid("Test Machine", states, transitions)

	if !strings.Contains(result, "stateDiagram-v2") {
		t.Error("expected mermaid state diagram header")
	}
	if !strings.Contains(result, "[*] --> S1") {
		t.Error("expected initial state transition")
	}
	if !strings.Contains(result, "S3 --> [*]") {
		t.Error("expected success terminal transition")
	}
	if !strings.Contains(result, "[FAILURE]") {
		t.Error("expected failure state marker")
	}
	if !strings.Contains(result, "S1 --> S2: begin") {
		t.Error("expected transition with trigger")
	}
}

func TestGenerateMermaid_LongTrigger(t *testing.T) {
	states := &fsmlib.StatesFile{
		States: []fsmlib.State{
			{ID: "S1", Name: "Start", Type: "initial"},
			{ID: "S2", Name: "End", Type: "success"},
		},
	}

	transitions := &fsmlib.TransitionsFile{
		Transitions: []fsmlib.Transition{
			{ID: "TR1", FromState: "S1", ToState: "S2", Trigger: "this is a very long trigger that should be truncated"},
		},
	}

	result := generateMermaid("Test", states, transitions)

	if strings.Contains(result, "truncated") {
		t.Error("expected long trigger to be truncated")
	}
	if !strings.Contains(result, "...") {
		t.Error("expected ellipsis for truncated trigger")
	}
}

func writeJSON(t *testing.T, dir, filename string, data interface{}) {
	t.Helper()
	content, _ := json.MarshalIndent(data, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, filename), content, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", filename, err)
	}
}
