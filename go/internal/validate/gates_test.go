package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSpecCoverage_FullCoverage(t *testing.T) {
	tasks := []TaskDefinition{
		{ID: "T001", Behaviors: []string{"B1", "B2"}},
		{ID: "T002", Behaviors: []string{"B3"}},
	}

	capMap := &CapabilityMap{
		Domains: []Domain{
			{
				ID:   "D1",
				Name: "Test Domain",
				Capabilities: []Capability{
					{
						ID:   "C1",
						Name: "Test Capability",
						Behaviors: []Behavior{
							{ID: "B1", Name: "Behavior1"},
							{ID: "B2", Name: "Behavior2"},
							{ID: "B3", Name: "Behavior3"},
						},
					},
				},
			},
		},
	}

	result := CheckSpecCoverage(tasks, capMap, 1.0)

	if !result.Passed {
		t.Error("expected full coverage to pass")
	}
	if result.Ratio != 1.0 {
		t.Errorf("expected ratio 1.0, got %f", result.Ratio)
	}
	if result.TotalBehaviors != 3 {
		t.Errorf("expected 3 total behaviors, got %d", result.TotalBehaviors)
	}
	if result.CoveredBehaviors != 3 {
		t.Errorf("expected 3 covered behaviors, got %d", result.CoveredBehaviors)
	}
	if len(result.UncoveredBehaviors) != 0 {
		t.Errorf("expected no uncovered behaviors, got %v", result.UncoveredBehaviors)
	}
}

func TestCheckSpecCoverage_PartialCoverage(t *testing.T) {
	tasks := []TaskDefinition{
		{ID: "T001", Behaviors: []string{"B1"}},
	}

	capMap := &CapabilityMap{
		Domains: []Domain{
			{
				ID: "D1",
				Capabilities: []Capability{
					{
						ID: "C1",
						Behaviors: []Behavior{
							{ID: "B1", Name: "Behavior1"},
							{ID: "B2", Name: "Behavior2"},
						},
					},
				},
			},
		},
	}

	result := CheckSpecCoverage(tasks, capMap, 1.0)

	if result.Passed {
		t.Error("expected partial coverage to fail with 100% threshold")
	}
	if result.Ratio != 0.5 {
		t.Errorf("expected ratio 0.5, got %f", result.Ratio)
	}
	if len(result.UncoveredBehaviors) != 1 {
		t.Errorf("expected 1 uncovered behavior, got %d", len(result.UncoveredBehaviors))
	}
}

func TestCheckSpecCoverage_ThresholdMet(t *testing.T) {
	tasks := []TaskDefinition{
		{ID: "T001", Behaviors: []string{"B1", "B2"}},
	}

	capMap := &CapabilityMap{
		Domains: []Domain{
			{
				ID: "D1",
				Capabilities: []Capability{
					{
						ID: "C1",
						Behaviors: []Behavior{
							{ID: "B1", Name: "Behavior1"},
							{ID: "B2", Name: "Behavior2"},
							{ID: "B3", Name: "Behavior3"},
						},
					},
				},
			},
		},
	}

	result := CheckSpecCoverage(tasks, capMap, 0.6)

	if !result.Passed {
		t.Error("expected 66% coverage to pass with 60% threshold")
	}
}

func TestCheckSpecCoverage_NilCapabilityMap(t *testing.T) {
	tasks := []TaskDefinition{
		{ID: "T001", Behaviors: []string{"B1"}},
	}

	result := CheckSpecCoverage(tasks, nil, 1.0)

	if result.Passed {
		t.Error("expected nil capability map to fail")
	}
	if result.Ratio != 0 {
		t.Errorf("expected ratio 0, got %f", result.Ratio)
	}
}

func TestCheckSpecCoverage_MultipleDomains(t *testing.T) {
	tasks := []TaskDefinition{
		{ID: "T001", Behaviors: []string{"B1"}},
		{ID: "T002", Behaviors: []string{"B3"}},
	}

	capMap := &CapabilityMap{
		Domains: []Domain{
			{
				ID: "D1",
				Capabilities: []Capability{
					{ID: "C1", Behaviors: []Behavior{{ID: "B1", Name: "B1"}, {ID: "B2", Name: "B2"}}},
				},
			},
			{
				ID: "D2",
				Capabilities: []Capability{
					{ID: "C2", Behaviors: []Behavior{{ID: "B3", Name: "B3"}, {ID: "B4", Name: "B4"}}},
				},
			},
		},
	}

	result := CheckSpecCoverage(tasks, capMap, 0.5)

	if !result.Passed {
		t.Error("expected 50% coverage to pass with 50% threshold")
	}

	if len(result.CoverageByDomain) != 2 {
		t.Errorf("expected 2 domains in coverage, got %d", len(result.CoverageByDomain))
	}
	if result.CoverageByDomain["D1"] != 0.5 {
		t.Errorf("expected D1 coverage 0.5, got %f", result.CoverageByDomain["D1"])
	}
	if result.CoverageByDomain["D2"] != 0.5 {
		t.Errorf("expected D2 coverage 0.5, got %f", result.CoverageByDomain["D2"])
	}
}

func TestDetectPhaseLeakage_NoLeakage(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID:    "T001",
			Phase: 1,
			Name:  "Implement core functionality",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "Unit tests pass", Verification: "go test ./..."},
			},
		},
	}

	result := DetectPhaseLeakage(tasks, 1, nil)

	if !result.Passed {
		t.Errorf("expected no leakage, got violations: %v", result.Violations)
	}
}

func TestDetectPhaseLeakage_DetectsLeakage(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID:    "T001",
			Phase: 1,
			Name:  "Implement deployment automation",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "Deploy to production", Verification: "deploy.sh"},
			},
		},
	}

	result := DetectPhaseLeakage(tasks, 1, nil)

	if result.Passed {
		t.Error("expected leakage detection to fail")
	}
	if len(result.Violations) == 0 {
		t.Error("expected violations to be reported")
	}
}

func TestDetectPhaseLeakage_CustomKeywords(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID:    "T001",
			Phase: 1,
			Name:  "Setup kubernetes cluster",
		},
	}

	customKeywords := map[int][]string{
		2: {"kubernetes", "docker"},
	}

	result := DetectPhaseLeakage(tasks, 1, customKeywords)

	if result.Passed {
		t.Error("expected custom keyword detection to find leakage")
	}
	foundKubernetes := false
	for _, v := range result.Violations {
		if v.TaskID == "T001" {
			foundKubernetes = true
		}
	}
	if !foundKubernetes {
		t.Error("expected to find kubernetes keyword violation")
	}
}

func TestDetectPhaseLeakage_SkipsOtherPhases(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID:    "T001",
			Phase: 2,
			Name:  "Production deployment",
		},
	}

	result := DetectPhaseLeakage(tasks, 1, nil)

	if !result.Passed {
		t.Error("expected phase 2 tasks to be skipped when checking phase 1")
	}
}

func TestDetectPhaseLeakage_MultipleViolations(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID:    "T001",
			Phase: 1,
			Name:  "Production deployment with scale",
		},
		{
			ID:    "T002",
			Phase: 1,
			Name:  "Performance optimization",
		},
	}

	result := DetectPhaseLeakage(tasks, 1, nil)

	if result.Passed {
		t.Error("expected multiple violations")
	}
	if len(result.Violations) < 2 {
		t.Errorf("expected at least 2 violations, got %d", len(result.Violations))
	}
}

func TestValidateAcceptanceCriteria_Valid(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID: "T001",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "Unit tests pass for all modules", Verification: "go test ./..."},
				{Criterion: "Integration tests succeed", Verification: "pytest tests/"},
			},
		},
	}

	result := ValidateAcceptanceCriteria(tasks)

	if !result.Passed {
		t.Errorf("expected valid AC to pass, got issues: %v", result.Issues)
	}
}

func TestValidateAcceptanceCriteria_MissingAC(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID:                 "T001",
			AcceptanceCriteria: []AcceptanceCriterion{},
		},
	}

	result := ValidateAcceptanceCriteria(tasks)

	if result.Passed {
		t.Error("expected missing AC to fail")
	}
	found := false
	for _, issue := range result.Issues {
		if issue.TaskID == "T001" && issue.Issue == "Task has no acceptance criteria" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'no acceptance criteria' issue")
	}
}

func TestValidateAcceptanceCriteria_EmptyCriterion(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID: "T001",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "", Verification: "go test ./..."},
			},
		},
	}

	result := ValidateAcceptanceCriteria(tasks)

	if result.Passed {
		t.Error("expected empty criterion to fail")
	}
}

func TestValidateAcceptanceCriteria_MissingVerification(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID: "T001",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "Feature works correctly", Verification: ""},
			},
		},
	}

	result := ValidateAcceptanceCriteria(tasks)

	if result.Passed {
		t.Error("expected missing verification to fail")
	}
}

func TestValidateAcceptanceCriteria_ShortCriterion(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID: "T001",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "Works", Verification: "go test ./..."},
			},
		},
	}

	result := ValidateAcceptanceCriteria(tasks)

	if result.Passed {
		t.Error("expected short criterion to fail")
	}
}

func TestValidateAcceptanceCriteria_VariousVerificationCommands(t *testing.T) {
	tasks := []TaskDefinition{
		{
			ID: "T001",
			AcceptanceCriteria: []AcceptanceCriterion{
				{Criterion: "Go tests pass successfully", Verification: "go test ./..."},
				{Criterion: "Python tests pass successfully", Verification: "pytest tests/"},
				{Criterion: "NPM tests pass successfully", Verification: "npm test"},
				{Criterion: "Make tests pass successfully", Verification: "make test"},
				{Criterion: "Cargo tests pass successfully", Verification: "cargo test"},
				{Criterion: "Script runs successfully", Verification: "./run-tests.sh"},
				{Criterion: "Bash script runs successfully", Verification: "bash test.sh"},
			},
		},
	}

	result := ValidateAcceptanceCriteria(tasks)

	if !result.Passed {
		t.Errorf("expected various verification commands to pass, got issues: %v", result.Issues)
	}
}

func TestRunAllGates_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatal(err)
	}

	task := TaskDefinition{
		ID:        "T001",
		Phase:     1,
		Behaviors: []string{"B1"},
		AcceptanceCriteria: []AcceptanceCriterion{
			{Criterion: "Feature works correctly", Verification: "go test ./..."},
		},
	}
	taskData, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(tasksDir, "T001.json"), taskData, 0644); err != nil {
		t.Fatal(err)
	}

	capMap := CapabilityMap{
		Domains: []Domain{
			{
				ID: "D1",
				Capabilities: []Capability{
					{ID: "C1", Behaviors: []Behavior{{ID: "B1", Name: "B1"}}},
				},
			},
		},
	}
	capMapData, _ := json.Marshal(capMap)
	if err := os.WriteFile(filepath.Join(artifactsDir, "capability-map.json"), capMapData, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := RunAllGates(tmpDir, 1, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected all gates to pass, got: %+v", result)
	}
	if len(result.Gates) != 3 {
		t.Errorf("expected 3 gates, got %d", len(result.Gates))
	}
}

func TestRunAllGates_TasksNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := RunAllGates(tmpDir, 1, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected to fail when tasks not found")
	}
	if len(result.Gates) == 0 {
		t.Error("expected at least one gate result")
	}
	if result.Gates[0].Name != "load_tasks" || result.Gates[0].Passed {
		t.Error("expected load_tasks gate to fail")
	}
}

func TestRunAllGates_CapabilityMapNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	task := TaskDefinition{
		ID:        "T001",
		Phase:     1,
		Behaviors: []string{"B1"},
		AcceptanceCriteria: []AcceptanceCriterion{
			{Criterion: "Feature works correctly", Verification: "go test ./..."},
		},
	}
	taskData, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(tasksDir, "T001.json"), taskData, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := RunAllGates(tmpDir, 1, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundCapMapError := false
	for _, gate := range result.Gates {
		if gate.Name == "load_capability_map" && !gate.Passed {
			foundCapMapError = true
		}
	}
	if !foundCapMapError {
		t.Error("expected load_capability_map gate to fail")
	}
}

func TestRunAllGates_FailingGates(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatal(err)
	}

	task := TaskDefinition{
		ID:                 "T001",
		Phase:              1,
		Behaviors:          []string{"B1"},
		Name:               "Deploy to production",
		AcceptanceCriteria: []AcceptanceCriterion{},
	}
	taskData, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(tasksDir, "T001.json"), taskData, 0644); err != nil {
		t.Fatal(err)
	}

	capMap := CapabilityMap{
		Domains: []Domain{
			{
				ID: "D1",
				Capabilities: []Capability{
					{ID: "C1", Behaviors: []Behavior{{ID: "B1", Name: "B1"}, {ID: "B2", Name: "B2"}}},
				},
			},
		},
	}
	capMapData, _ := json.Marshal(capMap)
	if err := os.WriteFile(filepath.Join(artifactsDir, "capability-map.json"), capMapData, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := RunAllGates(tmpDir, 1, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected gates to fail")
	}

	if result.SpecCoverage == nil || result.SpecCoverage.Passed {
		t.Error("expected spec coverage gate to fail")
	}
	if result.PhaseLeakage == nil || result.PhaseLeakage.Passed {
		t.Error("expected phase leakage gate to fail")
	}
	if result.ACValidation == nil || result.ACValidation.Passed {
		t.Error("expected AC validation gate to fail")
	}
}

func TestLoadTaskDefinitions_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	task1 := TaskDefinition{ID: "T001", Name: "Task 1"}
	task2 := TaskDefinition{ID: "T002", Name: "Task 2"}

	data1, _ := json.Marshal(task1)
	data2, _ := json.Marshal(task2)

	if err := os.WriteFile(filepath.Join(tasksDir, "T001.json"), data1, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "T002.json"), data2, 0644); err != nil {
		t.Fatal(err)
	}

	tasks, err := LoadTaskDefinitions(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "T001" || tasks[1].ID != "T002" {
		t.Error("tasks not sorted correctly")
	}
}

func TestLoadCapabilityMap_Success(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatal(err)
	}

	capMap := CapabilityMap{
		Version: "1.0",
		Domains: []Domain{{ID: "D1", Name: "Domain 1"}},
	}
	data, _ := json.Marshal(capMap)

	if err := os.WriteFile(filepath.Join(artifactsDir, "capability-map.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := LoadCapabilityMap(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", result.Version)
	}
	if len(result.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(result.Domains))
	}
}
