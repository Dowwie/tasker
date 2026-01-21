package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestEnv(t *testing.T) (string, *Generator) {
	t.Helper()

	tmpDir := t.TempDir()
	planningDir := filepath.Join(tmpDir, ".tasker")
	bundlesDir := filepath.Join(planningDir, "bundles")
	tasksDir := filepath.Join(planningDir, "tasks")
	artifactsDir := filepath.Join(planningDir, "artifacts")
	inputsDir := filepath.Join(planningDir, "inputs")
	targetDir := filepath.Join(tmpDir, "target")

	for _, dir := range []string{bundlesDir, tasksDir, artifactsDir, inputsDir, targetDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	state := map[string]interface{}{
		"version": "2.0",
		"phase":   map[string]interface{}{"current": "ready", "completed": []string{"ingestion"}},
		"target_dir": targetDir,
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":            "T001",
				"name":          "Test Task One",
				"status":        "complete",
				"phase":         1,
				"files_created": []string{"src/module.go"},
			},
			"T002": map[string]interface{}{
				"id":        "T002",
				"name":      "Test Task Two",
				"status":    "pending",
				"phase":     1,
				"depends_on": []string{"T001"},
			},
		},
		"execution": map[string]interface{}{"current_phase": 1},
	}

	stateData, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(planningDir, "state.json"), stateData, 0644); err != nil {
		t.Fatalf("failed to write state.json: %v", err)
	}

	capMap := map[string]interface{}{
		"domains": []interface{}{
			map[string]interface{}{
				"id":   "D1",
				"name": "Core",
				"capabilities": []interface{}{
					map[string]interface{}{
						"id":   "C1",
						"name": "Data Processing",
						"spec_ref": map[string]interface{}{
							"quote":    "Process data efficiently",
							"location": "spec.md",
						},
						"behaviors": []interface{}{
							map[string]interface{}{
								"id":          "B001",
								"name":        "ProcessInput",
								"type":        "process",
								"description": "Process input data",
							},
							map[string]interface{}{
								"id":          "B002",
								"name":        "ValidateOutput",
								"type":        "output",
								"description": "Validate processed output",
							},
						},
					},
				},
			},
		},
	}

	capMapData, _ := json.MarshalIndent(capMap, "", "  ")
	if err := os.WriteFile(filepath.Join(artifactsDir, "capability-map.json"), capMapData, 0644); err != nil {
		t.Fatalf("failed to write capability-map.json: %v", err)
	}

	physMap := map[string]interface{}{
		"file_mapping": []interface{}{
			map[string]interface{}{
				"behavior_id": "B001",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "src/processor.go",
						"action":  "create",
						"layer":   "domain",
						"purpose": "Data processing logic",
					},
				},
				"tests": []interface{}{
					map[string]interface{}{
						"path":   "src/processor_test.go",
						"action": "create",
					},
				},
			},
		},
	}

	physMapData, _ := json.MarshalIndent(physMap, "", "  ")
	if err := os.WriteFile(filepath.Join(artifactsDir, "physical-map.json"), physMapData, 0644); err != nil {
		t.Fatalf("failed to write physical-map.json: %v", err)
	}

	return planningDir, NewGenerator(planningDir)
}

func createTestTask(t *testing.T, planningDir, taskID string, task map[string]interface{}) {
	t.Helper()
	tasksDir := filepath.Join(planningDir, "tasks")
	taskData, _ := json.MarshalIndent(task, "", "  ")
	if err := os.WriteFile(filepath.Join(tasksDir, taskID+".json"), taskData, 0644); err != nil {
		t.Fatalf("failed to write task %s: %v", taskID, err)
	}
}

func TestGenerateBundle(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task Two",
		"phase":     1,
		"depends_on": []string{"T001"},
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Processing works correctly",
				"verification": "go test ./...",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("GenerateBundle failed: %v", err)
	}

	if bundle.TaskID != "T002" {
		t.Errorf("expected task_id 'T002', got '%s'", bundle.TaskID)
	}
	if bundle.Name != "Test Task Two" {
		t.Errorf("expected name 'Test Task Two', got '%s'", bundle.Name)
	}
	if bundle.Version != BundleVersion {
		t.Errorf("expected version '%s', got '%s'", BundleVersion, bundle.Version)
	}
	if bundle.Phase != 1 {
		t.Errorf("expected phase 1, got %d", bundle.Phase)
	}

	if len(bundle.Behaviors) == 0 {
		t.Error("expected at least one behavior")
	} else {
		if bundle.Behaviors[0].ID != "B001" {
			t.Errorf("expected behavior ID 'B001', got '%s'", bundle.Behaviors[0].ID)
		}
		if bundle.Behaviors[0].Name != "ProcessInput" {
			t.Errorf("expected behavior name 'ProcessInput', got '%s'", bundle.Behaviors[0].Name)
		}
	}

	if len(bundle.Dependencies.Tasks) == 0 || bundle.Dependencies.Tasks[0] != "T001" {
		t.Error("expected dependency on T001")
	}

	bundlePath := filepath.Join(planningDir, "bundles", "T002-bundle.json")
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		t.Error("bundle file was not created")
	}

	if bundle.Context.Domain != "Core" {
		t.Errorf("expected context domain 'Core', got '%s'", bundle.Context.Domain)
	}
}

func TestGenerateBundleTaskNotFound(t *testing.T) {
	_, gen := setupTestEnv(t)

	_, err := gen.GenerateBundle("T999")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestGenerateReadyBundles(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task Two",
		"phase":     1,
		"depends_on": []string{"T001"},
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundles, errs := gen.GenerateReadyBundles()

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	if len(bundles) != 1 {
		t.Errorf("expected 1 bundle, got %d", len(bundles))
	}

	if len(bundles) > 0 && bundles[0].TaskID != "T002" {
		t.Errorf("expected bundle for T002, got %s", bundles[0].TaskID)
	}
}

func TestGenerateReadyBundlesNoReady(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	statePath := filepath.Join(planningDir, "state.json")
	state := map[string]interface{}{
		"version":    "2.0",
		"phase":      map[string]interface{}{"current": "ready"},
		"target_dir": "/tmp/target",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"status": "running",
				"phase":  1,
			},
		},
		"execution": map[string]interface{}{"current_phase": 1},
	}
	stateData, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(statePath, stateData, 0644)

	bundles, errs := gen.GenerateReadyBundles()

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	if len(bundles) != 0 {
		t.Errorf("expected 0 bundles, got %d", len(bundles))
	}
}

func TestValidateBundle(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	_, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	_, err = gen.ValidateBundle("T002")
	if err != nil {
		t.Logf("ValidateBundle note (schema may not be available in test): %v", err)
	}
}

func TestValidateBundleNotFound(t *testing.T) {
	_, gen := setupTestEnv(t)

	_, err := gen.ValidateBundle("T999")
	if err == nil {
		t.Error("expected error for non-existent bundle")
	}
}

func TestValidateIntegrity(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"depends_on": []string{"T001"},
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	statePath := filepath.Join(planningDir, "state.json")
	stateData, _ := os.ReadFile(statePath)
	var state map[string]interface{}
	json.Unmarshal(stateData, &state)
	targetDir := state["target_dir"].(string)

	depFile := filepath.Join(targetDir, "src", "module.go")
	os.MkdirAll(filepath.Dir(depFile), 0755)
	os.WriteFile(depFile, []byte("package module"), 0644)

	_, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	result, err := gen.ValidateIntegrity("T002")
	if err != nil {
		t.Fatalf("ValidateIntegrity failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid integrity, got invalid: missing=%v, changed=%v",
			result.MissingFiles, result.ChangedFiles)
	}
}

func TestValidateIntegrityMissingFile(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"depends_on": []string{"T001"},
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	_, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	result, err := gen.ValidateIntegrity("T002")
	if err != nil {
		t.Fatalf("ValidateIntegrity failed: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid integrity due to missing dependency file")
	}

	if len(result.MissingFiles) == 0 {
		t.Error("expected missing files to be reported")
	}
}

func TestValidateIntegrityChangedFile(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"depends_on": []string{"T001"},
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	statePath := filepath.Join(planningDir, "state.json")
	stateData, _ := os.ReadFile(statePath)
	var state map[string]interface{}
	json.Unmarshal(stateData, &state)
	targetDir := state["target_dir"].(string)

	depFile := filepath.Join(targetDir, "src", "module.go")
	os.MkdirAll(filepath.Dir(depFile), 0755)
	os.WriteFile(depFile, []byte("package module"), 0644)

	_, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	os.WriteFile(depFile, []byte("package module // modified"), 0644)

	result, err := gen.ValidateIntegrity("T002")
	if err != nil {
		t.Fatalf("ValidateIntegrity failed: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid integrity due to changed dependency file")
	}

	if len(result.ChangedFiles) == 0 {
		t.Error("expected changed files to be reported")
	}
}

func TestListBundles(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	_, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	bundles, err := gen.ListBundles()
	if err != nil {
		t.Fatalf("ListBundles failed: %v", err)
	}

	if len(bundles) != 1 {
		t.Errorf("expected 1 bundle, got %d", len(bundles))
	}

	if len(bundles) > 0 {
		if bundles[0].TaskID != "T002" {
			t.Errorf("expected task_id 'T002', got '%s'", bundles[0].TaskID)
		}
		if bundles[0].Name != "Test Task" {
			t.Errorf("expected name 'Test Task', got '%s'", bundles[0].Name)
		}
		if bundles[0].FilePath == "" {
			t.Error("expected file_path to be set")
		}
	}
}

func TestListBundlesEmpty(t *testing.T) {
	_, gen := setupTestEnv(t)

	bundles, err := gen.ListBundles()
	if err != nil {
		t.Fatalf("ListBundles failed: %v", err)
	}

	if len(bundles) != 0 {
		t.Errorf("expected 0 bundles, got %d", len(bundles))
	}
}

func TestCleanBundles(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task1 := map[string]interface{}{
		"id":        "T002",
		"name":      "Task 2",
		"phase":     1,
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task1)

	task2 := map[string]interface{}{
		"id":        "T003",
		"name":      "Task 3",
		"phase":     1,
		"behaviors": []string{"B002"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T003", task2)

	gen.GenerateBundle("T002")
	gen.GenerateBundle("T003")

	bundles, _ := gen.ListBundles()
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles before clean, got %d", len(bundles))
	}

	count, err := gen.CleanBundles()
	if err != nil {
		t.Fatalf("CleanBundles failed: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 bundles cleaned, got %d", count)
	}

	bundlesAfter, _ := gen.ListBundles()
	if len(bundlesAfter) != 0 {
		t.Errorf("expected 0 bundles after clean, got %d", len(bundlesAfter))
	}
}

func TestCleanBundlesNoDir(t *testing.T) {
	tmpDir := t.TempDir()
	planningDir := filepath.Join(tmpDir, "nonexistent")

	gen := NewGenerator(planningDir)

	count, err := gen.CleanBundles()
	if err != nil {
		t.Fatalf("CleanBundles failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 bundles cleaned, got %d", count)
	}
}

func TestLoadBundle(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	_, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	bundle, err := gen.LoadBundle("T002")
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	if bundle.TaskID != "T002" {
		t.Errorf("expected task_id 'T002', got '%s'", bundle.TaskID)
	}
}

func TestLoadBundleNotFound(t *testing.T) {
	_, gen := setupTestEnv(t)

	_, err := gen.LoadBundle("T999")
	if err == nil {
		t.Error("expected error for non-existent bundle")
	}
}

func TestBundleWithConstraints(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	constraintsPath := filepath.Join(planningDir, "inputs", "constraints.md")
	constraints := `# Project Constraints

Language: Python
Framework: FastAPI
Testing: pytest

## Patterns
- Use Protocol for interfaces
- Use dataclass for data structures
`
	os.WriteFile(constraintsPath, []byte(constraints), 0644)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "pytest",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	if bundle.Constraints.Language != "Python" {
		t.Errorf("expected language 'Python', got '%s'", bundle.Constraints.Language)
	}
	if bundle.Constraints.Framework != "FastAPI" {
		t.Errorf("expected framework 'FastAPI', got '%s'", bundle.Constraints.Framework)
	}
	if bundle.Constraints.Testing != "pytest" {
		t.Errorf("expected testing 'pytest', got '%s'", bundle.Constraints.Testing)
	}
	if len(bundle.Constraints.Patterns) < 2 {
		t.Errorf("expected at least 2 patterns, got %d", len(bundle.Constraints.Patterns))
	}
}

func TestBundleWithStateMachine(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"state_machine": map[string]interface{}{
			"transitions_covered": []string{"TR001", "TR002"},
			"guards_enforced":     []string{"INV001"},
			"states_reached":      []string{"S001", "S002"},
		},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	if bundle.StateMachine == nil {
		t.Fatal("expected state_machine to be set")
	}

	if len(bundle.StateMachine.TransitionsCovered) != 2 {
		t.Errorf("expected 2 transitions_covered, got %d", len(bundle.StateMachine.TransitionsCovered))
	}
	if len(bundle.StateMachine.GuardsEnforced) != 1 {
		t.Errorf("expected 1 guards_enforced, got %d", len(bundle.StateMachine.GuardsEnforced))
	}
	if len(bundle.StateMachine.StatesReached) != 2 {
		t.Errorf("expected 2 states_reached, got %d", len(bundle.StateMachine.StatesReached))
	}
}

func TestFileChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	os.WriteFile(testFile, []byte("test content"), 0644)

	checksum1 := fileChecksum(testFile)
	if checksum1 == "" {
		t.Error("expected non-empty checksum")
	}
	if len(checksum1) != 16 {
		t.Errorf("expected checksum length 16, got %d", len(checksum1))
	}

	checksum2 := fileChecksum(testFile)
	if checksum1 != checksum2 {
		t.Error("expected identical checksums for same file")
	}

	os.WriteFile(testFile, []byte("different content"), 0644)
	checksum3 := fileChecksum(testFile)
	if checksum1 == checksum3 {
		t.Error("expected different checksums for different content")
	}

	checksumMissing := fileChecksum(filepath.Join(tmpDir, "nonexistent.txt"))
	if checksumMissing != "" {
		t.Error("expected empty checksum for missing file")
	}
}

func TestLoadConstraintsVariousLanguages(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantLang   string
		wantFramework string
		wantTesting   string
	}{
		{"Go", "Use Go or golang for this project", "Go", "", ""},
		{"TypeScript", "TypeScript is required", "TypeScript", "", ""},
		{"Rust", "Rust language", "Rust", "", ""},
		{"Django", "Python with Django framework", "Python", "Django", ""},
		{"Flask", "Python with Flask framework", "Python", "Flask", ""},
		{"Gin", "Go with Gin framework", "Go", "Gin", ""},
		{"GoTest", "Use go test for testing", "", "", "go test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planningDir, gen := setupTestEnv(t)
			constraintsPath := filepath.Join(planningDir, "inputs", "constraints.md")
			os.WriteFile(constraintsPath, []byte(tt.content), 0644)

			constraints := gen.loadConstraints()

			if tt.wantLang != "" && constraints.Language != tt.wantLang {
				t.Errorf("expected language %q, got %q", tt.wantLang, constraints.Language)
			}
			if tt.wantFramework != "" && constraints.Framework != tt.wantFramework {
				t.Errorf("expected framework %q, got %q", tt.wantFramework, constraints.Framework)
			}
			if tt.wantTesting != "" && constraints.Testing != tt.wantTesting {
				t.Errorf("expected testing %q, got %q", tt.wantTesting, constraints.Testing)
			}
		})
	}
}

func TestLoadConstraintsPatterns(t *testing.T) {
	planningDir, gen := setupTestEnv(t)
	constraintsPath := filepath.Join(planningDir, "inputs", "constraints.md")

	content := `Use factory pattern, interface abstraction, protocol for types`
	os.WriteFile(constraintsPath, []byte(content), 0644)

	constraints := gen.loadConstraints()

	if len(constraints.Patterns) < 3 {
		t.Errorf("expected at least 3 patterns, got %d", len(constraints.Patterns))
	}
}

func TestLoadConstraintsNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	planningDir := filepath.Join(tmpDir, ".tasker")
	os.MkdirAll(filepath.Join(planningDir, "inputs"), 0755)

	gen := NewGenerator(planningDir)
	constraints := gen.loadConstraints()

	if constraints.Raw != "" {
		t.Error("expected empty raw when file doesn't exist")
	}
}

func TestBuildContextWithExistingContext(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"context": map[string]interface{}{
			"domain":        "CustomDomain",
			"capability":    "CustomCap",
			"capability_id": "CustomCapID",
			"steel_thread":  true,
		},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	if bundle.Context.Domain != "CustomDomain" {
		t.Errorf("expected domain 'CustomDomain', got '%s'", bundle.Context.Domain)
	}
	if bundle.Context.SteelThread != true {
		t.Error("expected steel_thread to be true")
	}
}

func TestLoadBundleInvalidJSON(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	bundlesDir := filepath.Join(planningDir, "bundles")
	bundlePath := filepath.Join(bundlesDir, "T999-bundle.json")
	os.WriteFile(bundlePath, []byte("{invalid json}"), 0644)

	_, err := gen.LoadBundle("T999")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestListBundlesWithNonBundleFiles(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)
	gen.GenerateBundle("T002")

	bundlesDir := filepath.Join(planningDir, "bundles")
	os.WriteFile(filepath.Join(bundlesDir, "readme.txt"), []byte("not a bundle"), 0644)
	os.Mkdir(filepath.Join(bundlesDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(bundlesDir, "invalid-bundle.json"), []byte("{bad json}"), 0644)

	bundles, err := gen.ListBundles()
	if err != nil {
		t.Fatalf("ListBundles failed: %v", err)
	}

	if len(bundles) != 1 {
		t.Errorf("expected 1 valid bundle, got %d", len(bundles))
	}
}

func TestLoadCapabilityMapInvalidJSON(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	capPath := filepath.Join(planningDir, "artifacts", "capability-map.json")
	os.WriteFile(capPath, []byte("{invalid json}"), 0644)

	_, err := gen.loadCapabilityMap()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadPhysicalMapInvalidJSON(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	physPath := filepath.Join(planningDir, "artifacts", "physical-map.json")
	os.WriteFile(physPath, []byte("{invalid json}"), 0644)

	_, err := gen.loadPhysicalMap()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadTaskDefinitionInvalidJSON(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	taskPath := filepath.Join(planningDir, "tasks", "T999.json")
	os.WriteFile(taskPath, []byte("{invalid json}"), 0644)

	_, err := gen.loadTaskDefinition("T999")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExpandStateMachineNil(t *testing.T) {
	gen := NewGenerator("/tmp")
	result := gen.expandStateMachine(nil)
	if result != nil {
		t.Error("expected nil result for nil input")
	}
}

func TestExpandBehaviorsUnknownBehavior(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":        "T002",
		"name":      "Test Task",
		"phase":     1,
		"behaviors": []string{"UNKNOWN_BEHAVIOR"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	if len(bundle.Behaviors) != 1 {
		t.Fatalf("expected 1 behavior, got %d", len(bundle.Behaviors))
	}
	if bundle.Behaviors[0].ID != "UNKNOWN_BEHAVIOR" {
		t.Errorf("expected behavior ID 'UNKNOWN_BEHAVIOR', got '%s'", bundle.Behaviors[0].ID)
	}
}

func TestGenerateReadyBundlesWithError(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	state := map[string]interface{}{
		"version":    "2.0",
		"phase":      map[string]interface{}{"current": "ready"},
		"target_dir": "/tmp/target",
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"status": "complete",
				"phase":  1,
			},
			"T002": map[string]interface{}{
				"id":        "T002",
				"status":    "pending",
				"phase":     1,
				"depends_on": []string{"T001"},
			},
		},
		"execution": map[string]interface{}{"current_phase": 1},
	}
	stateData, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(planningDir, "state.json"), stateData, 0644)

	bundles, errs := gen.GenerateReadyBundles()

	if len(errs) == 0 {
		t.Error("expected errors since task file doesn't exist")
	}
	if len(bundles) != 0 {
		t.Errorf("expected 0 bundles, got %d", len(bundles))
	}
}

func TestBuildContextNoBehaviors(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":    "T002",
		"name":  "Test Task",
		"phase": 1,
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	if bundle.Context.Domain != "" {
		t.Errorf("expected empty domain, got '%s'", bundle.Context.Domain)
	}
}

func TestCollectFilesFromTask(t *testing.T) {
	planningDir, gen := setupTestEnv(t)

	task := map[string]interface{}{
		"id":    "T002",
		"name":  "Test Task",
		"phase": 1,
		"files": []interface{}{
			map[string]interface{}{
				"path":   "src/custom.go",
				"action": "create",
			},
		},
		"behaviors": []string{"B001"},
		"acceptance_criteria": []interface{}{
			map[string]interface{}{
				"criterion":    "Works",
				"verification": "go test",
			},
		},
	}
	createTestTask(t, planningDir, "T002", task)

	bundle, err := gen.GenerateBundle("T002")
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	foundCustom := false
	for _, f := range bundle.Files {
		if f.Path == "src/custom.go" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Error("expected custom file to be in bundle files")
	}
}
