package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupBundleTestEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current":   "execution",
			"completed": []string{},
		},
		"target_dir": tmpDir,
		"created_at": "2026-01-18T10:00:00Z",
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":     "T001",
				"name":   "Test Task",
				"status": "pending",
				"phase":  0,
				"file":   "tasks/T001.json",
			},
		},
		"execution": map[string]interface{}{},
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644)

	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	task := map[string]interface{}{
		"id":          "T001",
		"name":        "Test Task",
		"description": "A test task",
		"phase":       0,
		"behaviors":   []string{"B001"},
	}
	taskData, _ := json.MarshalIndent(task, "", "  ")
	os.WriteFile(filepath.Join(tasksDir, "T001.json"), taskData, 0644)

	bundlesDir := filepath.Join(tmpDir, "bundles")
	os.MkdirAll(bundlesDir, 0755)

	artifactsDir := filepath.Join(tmpDir, "artifacts")
	os.MkdirAll(artifactsDir, 0755)

	capMap := map[string]interface{}{
		"spec_checksum": "abc123",
		"domains": []interface{}{
			map[string]interface{}{
				"name": "Test Domain",
				"capabilities": []interface{}{
					map[string]interface{}{
						"name": "Test Capability",
						"behaviors": []interface{}{
							map[string]interface{}{
								"id":          "B001",
								"name":        "Test Behavior",
								"description": "A test behavior",
							},
						},
					},
				},
			},
		},
		"flows": []interface{}{},
	}
	capMapData, _ := json.MarshalIndent(capMap, "", "  ")
	os.WriteFile(filepath.Join(artifactsDir, "capability-map.json"), capMapData, 0644)

	physMap := map[string]interface{}{
		"spec_checksum": "abc123",
		"behaviors": map[string]interface{}{
			"B001": map[string]interface{}{
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
	os.WriteFile(filepath.Join(artifactsDir, "physical-map.json"), physMapData, 0644)

	return tmpDir
}

func withPlanningDir(tmpDir string, fn func()) {
	original := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = original }()
	fn()
}

func TestGenerateCmd(t *testing.T) {
	t.Run("fails for nonexistent task", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			err := generateCmd.RunE(generateCmd, []string{"TXXX"})
			if err == nil {
				t.Error("generateCmd should fail for nonexistent task")
			}
		})
	})

	t.Run("fails for missing artifacts", func(t *testing.T) {
		tmpDir := t.TempDir()

		state := map[string]interface{}{
			"version":    "2.0",
			"phase":      map[string]interface{}{"current": "execution"},
			"target_dir": tmpDir,
			"created_at": "2026-01-18T10:00:00Z",
			"tasks": map[string]interface{}{
				"T001": map[string]interface{}{
					"id": "T001", "status": "pending", "phase": 0,
				},
			},
			"execution": map[string]interface{}{},
		}
		data, _ := json.MarshalIndent(state, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644)

		tasksDir := filepath.Join(tmpDir, "tasks")
		os.MkdirAll(tasksDir, 0755)
		task := map[string]interface{}{"id": "T001", "phase": 0}
		taskData, _ := json.MarshalIndent(task, "", "  ")
		os.WriteFile(filepath.Join(tasksDir, "T001.json"), taskData, 0644)

		withPlanningDir(tmpDir, func() {
			err := generateCmd.RunE(generateCmd, []string{"T001"})
			if err == nil {
				t.Error("generateCmd should fail when artifacts are missing")
			}
		})
	})
}

func TestGenerateReadyCmd(t *testing.T) {
	t.Run("generates bundles for ready tasks", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			err := generateReadyCmd.RunE(generateReadyCmd, []string{})
			if err != nil {
				t.Errorf("generateReadyCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("handles no ready tasks", func(t *testing.T) {
		tmpDir := t.TempDir()

		state := map[string]interface{}{
			"version": "2.0",
			"phase": map[string]interface{}{
				"current":   "execution",
				"completed": []string{},
			},
			"target_dir": tmpDir,
			"created_at": "2026-01-18T10:00:00Z",
			"tasks":      map[string]interface{}{},
			"execution":  map[string]interface{}{},
		}

		data, _ := json.MarshalIndent(state, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644)

		withPlanningDir(tmpDir, func() {
			err := generateReadyCmd.RunE(generateReadyCmd, []string{})
			if err != nil {
				t.Errorf("generateReadyCmd should handle no ready tasks, got: %v", err)
			}
		})
	})
}

func TestListCmd(t *testing.T) {
	t.Run("lists bundles", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			_ = generateCmd.RunE(generateCmd, []string{"T001"})

			err := listCmd.RunE(listCmd, []string{})
			if err != nil {
				t.Errorf("listCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("handles no bundles", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			err := listCmd.RunE(listCmd, []string{})
			if err != nil {
				t.Errorf("listCmd should handle no bundles, got: %v", err)
			}
		})
	})
}

func TestCleanCmd(t *testing.T) {
	t.Run("removes bundles", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			_ = generateCmd.RunE(generateCmd, []string{"T001"})

			err := cleanCmd.RunE(cleanCmd, []string{})
			if err != nil {
				t.Errorf("cleanCmd should succeed, got: %v", err)
			}
		})
	})
}

func TestValidateCmd(t *testing.T) {
	t.Run("fails for nonexistent bundle", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			err := validateCmd.RunE(validateCmd, []string{"TXXX"})
			if err == nil {
				t.Error("validateCmd should fail for nonexistent bundle")
			}
		})
	})
}

func TestValidateIntegrityCmd(t *testing.T) {
	t.Run("validates bundle integrity", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			_ = generateCmd.RunE(generateCmd, []string{"T001"})

			err := validateIntegrityCmd.RunE(validateIntegrityCmd, []string{"T001"})
			if err != nil {
				t.Errorf("validateIntegrityCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for nonexistent bundle", func(t *testing.T) {
		tmpDir := setupBundleTestEnv(t)

		withPlanningDir(tmpDir, func() {
			err := validateIntegrityCmd.RunE(validateIntegrityCmd, []string{"TXXX"})
			if err == nil {
				t.Error("validateIntegrityCmd should fail for nonexistent bundle")
			}
		})
	})
}
