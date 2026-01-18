package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEvaluation_Success(t *testing.T) {
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current": "complete",
		},
		"tasks": map[string]interface{}{
			"T001": map[string]interface{}{
				"id":       "T001",
				"name":     "Task One",
				"status":   "complete",
				"attempts": 1,
				"verification": map[string]interface{}{
					"quality": map[string]interface{}{
						"types":    "PASS",
						"docs":     "PASS",
						"patterns": "PASS",
						"errors":   "PASS",
					},
					"criteria": []interface{}{
						map[string]interface{}{"name": "Criterion 1", "score": "PASS"},
						map[string]interface{}{"name": "Criterion 2", "score": "PASS"},
					},
					"tests": map[string]interface{}{
						"coverage":   "PASS",
						"assertions": "PASS",
						"edge_cases": "PASS",
					},
				},
			},
			"T002": map[string]interface{}{
				"id":       "T002",
				"name":     "Task Two",
				"status":   "complete",
				"attempts": 2,
				"verification": map[string]interface{}{
					"quality": map[string]interface{}{
						"types": "PARTIAL",
						"docs":  "PASS",
					},
					"criteria": []interface{}{
						map[string]interface{}{"name": "Criterion 1", "score": "PASS"},
						map[string]interface{}{"name": "Criterion 2", "score": "PARTIAL"},
					},
					"tests": map[string]interface{}{
						"edge_cases": "PARTIAL",
					},
				},
			},
			"T003": map[string]interface{}{
				"id":     "T003",
				"name":   "Task Three",
				"status": "failed",
				"error":  "Test failure",
			},
		},
		"execution": map[string]interface{}{
			"completed_count": 2,
			"failed_count":    1,
			"total_tokens":    10000,
			"total_cost_usd":  0.50,
		},
		"artifacts": map[string]interface{}{
			"task_validation": map[string]interface{}{
				"verdict": "PASS",
				"issues":  []interface{}{},
			},
		},
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test state: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("Failed to write test state: %v", err)
	}

	result, err := RunEvaluation(tmpDir)
	if err != nil {
		t.Fatalf("RunEvaluation failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.PlanningVerdict != "PASS" {
		t.Errorf("Expected planning verdict PASS, got %s", result.PlanningVerdict)
	}

	if result.TotalTasks != 3 {
		t.Errorf("Expected 3 total tasks, got %d", result.TotalTasks)
	}

	if result.Metrics.CompletedCount != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", result.Metrics.CompletedCount)
	}

	if result.Metrics.FailedCount != 1 {
		t.Errorf("Expected 1 failed task, got %d", result.Metrics.FailedCount)
	}

	expectedSuccessRate := 2.0 / 3.0
	if result.Metrics.TaskSuccessRate < expectedSuccessRate-0.01 || result.Metrics.TaskSuccessRate > expectedSuccessRate+0.01 {
		t.Errorf("Expected task success rate ~%.2f, got %.2f", expectedSuccessRate, result.Metrics.TaskSuccessRate)
	}

	if result.Metrics.FirstAttemptSuccessRate != 0.5 {
		t.Errorf("Expected first attempt success rate 0.5, got %.2f", result.Metrics.FirstAttemptSuccessRate)
	}

	if result.Metrics.TotalTokens != 10000 {
		t.Errorf("Expected 10000 total tokens, got %d", result.Metrics.TotalTokens)
	}

	if result.Metrics.TotalCriteria != 4 {
		t.Errorf("Expected 4 total criteria, got %d", result.Metrics.TotalCriteria)
	}

	if result.Metrics.CriteriaPass != 3 {
		t.Errorf("Expected 3 criteria pass, got %d", result.Metrics.CriteriaPass)
	}

	if len(result.FailedTasks) != 1 {
		t.Errorf("Expected 1 failed task, got %d", len(result.FailedTasks))
	}
	if len(result.FailedTasks) > 0 && result.FailedTasks[0].Error != "Test failure" {
		t.Errorf("Expected failed task error 'Test failure', got '%s'", result.FailedTasks[0].Error)
	}

	if len(result.ImprovementPatterns) == 0 {
		t.Error("Expected improvement patterns due to PARTIAL scores")
	}
}

func TestRunEvaluation_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()

	state := map[string]interface{}{
		"version": "2.0",
		"phase": map[string]interface{}{
			"current": "ready",
		},
		"tasks":     map[string]interface{}{},
		"execution": map[string]interface{}{},
		"artifacts": map[string]interface{}{},
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test state: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("Failed to write test state: %v", err)
	}

	result, err := RunEvaluation(tmpDir)
	if err != nil {
		t.Fatalf("RunEvaluation failed on empty state: %v", err)
	}

	if result.TotalTasks != 0 {
		t.Errorf("Expected 0 tasks, got %d", result.TotalTasks)
	}

	if result.Metrics.TaskSuccessRate != 0 {
		t.Errorf("Expected 0 success rate for empty state, got %.2f", result.Metrics.TaskSuccessRate)
	}
}

func TestRunEvaluation_MissingStateFile(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := RunEvaluation(tmpDir)
	if err == nil {
		t.Fatal("Expected error for missing state file")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' in error, got: %v", err)
	}
}

func TestRunEvaluation_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := RunEvaluation(tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("Expected 'parse' in error, got: %v", err)
	}
}

func TestFormatEvaluationReport(t *testing.T) {
	result := &EvaluationResult{
		PlanningVerdict:     "PASS",
		PlanningIssuesCount: 0,
		Metrics: &EvaluationMetrics{
			TaskSuccessRate:         0.9,
			FirstAttemptSuccessRate: 0.8,
			AvgAttempts:             1.2,
			TokensPerTask:           5000,
			CostPerTask:             0.25,
			QualityPassRate:         0.85,
			FunctionalPassRate:      0.95,
			TestEdgeCaseRate:        0.75,
			CompletedCount:          9,
			FailedCount:             1,
			TotalTokens:             45000,
			TotalCostUSD:            2.25,
			CriteriaPass:            18,
			CriteriaPartial:         1,
			CriteriaFail:            1,
			TotalCriteria:           20,
			QualityDimensions:       map[string]int{"types": 8, "docs": 9, "patterns": 7, "errors": 9},
			QualityTotals:           map[string]int{"types": 9, "docs": 9, "patterns": 9, "errors": 9},
		},
		FailedTasks: []FailedTask{
			{ID: "T010", Name: "Failed Task", Error: "Compile error"},
		},
		ImprovementPatterns: []ImprovementPattern{
			{Description: "2 task(s) had type annotation issues", Count: 2},
		},
		TotalTasks:   10,
		BlockedCount: 0,
		SkippedCount: 0,
	}

	output := FormatEvaluationReport(result)

	expectedContains := []string{
		"Execution Evaluation Report",
		"Planning Quality",
		"Plan Verdict: PASS",
		"Execution Summary",
		"Tasks: 10 total",
		"Completed:",
		"First-Attempt Success:",
		"Average Attempts: 1.20",
		"Verification Breakdown",
		"Functional Criteria:",
		"PASS:",
		"Code Quality:",
		"Cost Analysis",
		"Total Tokens:  45000",
		"Total Cost:    $2.25",
		"Failure Analysis",
		"T010: FAIL - Compile error",
		"Improvement Patterns",
		"type annotation issues",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain '%s', but it didn't.\nOutput:\n%s", expected, output)
		}
	}
}

func TestComputeMetrics_EdgeCases(t *testing.T) {
	state := &stateForEval{
		Tasks: map[string]stateTask{
			"T001": {
				ID:       "T001",
				Status:   "complete",
				Attempts: 0,
			},
		},
		Execution: stateExecution{
			CompletedCount: 1,
			FailedCount:    0,
			TotalTokens:    1000,
			TotalCostUSD:   0.05,
		},
	}

	metrics := computeMetrics(state)

	if metrics.AvgAttempts != 1.0 {
		t.Errorf("Expected avg attempts 1.0 for task with 0 attempts (default), got %.2f", metrics.AvgAttempts)
	}

	if metrics.TaskSuccessRate != 1.0 {
		t.Errorf("Expected task success rate 1.0, got %.2f", metrics.TaskSuccessRate)
	}

	if metrics.TokensPerTask != 1000 {
		t.Errorf("Expected tokens per task 1000, got %d", metrics.TokensPerTask)
	}
}

func TestGetImprovementPatterns(t *testing.T) {
	state := &stateForEval{
		Tasks: map[string]stateTask{
			"T001": {
				ID:     "T001",
				Status: "complete",
				Verification: map[string]interface{}{
					"tests": map[string]interface{}{
						"edge_cases": "FAIL",
					},
					"quality": map[string]interface{}{
						"types": "PARTIAL",
						"docs":  "FAIL",
					},
				},
			},
			"T002": {
				ID:     "T002",
				Status: "complete",
				Verification: map[string]interface{}{
					"tests": map[string]interface{}{
						"edge_cases": "PARTIAL",
					},
					"quality": map[string]interface{}{
						"types": "PASS",
						"docs":  "PARTIAL",
					},
				},
			},
		},
	}

	patterns := getImprovementPatterns(state)

	if len(patterns) != 3 {
		t.Errorf("Expected 3 improvement patterns, got %d", len(patterns))
	}

	foundEdgeCase := false
	foundType := false
	foundDoc := false

	for _, p := range patterns {
		if strings.Contains(p.Description, "edge case") {
			foundEdgeCase = true
			if p.Count != 2 {
				t.Errorf("Expected 2 edge case issues, got %d", p.Count)
			}
		}
		if strings.Contains(p.Description, "type annotation") {
			foundType = true
			if p.Count != 1 {
				t.Errorf("Expected 1 type issue, got %d", p.Count)
			}
		}
		if strings.Contains(p.Description, "documentation") {
			foundDoc = true
			if p.Count != 2 {
				t.Errorf("Expected 2 doc issues, got %d", p.Count)
			}
		}
	}

	if !foundEdgeCase {
		t.Error("Expected edge case pattern to be identified")
	}
	if !foundType {
		t.Error("Expected type annotation pattern to be identified")
	}
	if !foundDoc {
		t.Error("Expected documentation pattern to be identified")
	}
}
