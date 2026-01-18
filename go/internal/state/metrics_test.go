package state

import (
	"math"
	"testing"
)

func TestLogTokens(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "running", Phase: 1},
		},
		Execution: Execution{TotalTokens: 1000, TotalCostUSD: 0.01},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := sm.LogTokens("T001", 500, 300, 0.008, "claude-3-opus")
	if err != nil {
		t.Fatalf("LogTokens failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Execution.TotalTokens != 1800 {
		t.Errorf("expected total_tokens 1800, got %d", loaded.Execution.TotalTokens)
	}
	expectedCost := 0.018
	if math.Abs(loaded.Execution.TotalCostUSD-expectedCost) > 0.0001 {
		t.Errorf("expected total_cost_usd ~0.018, got %f", loaded.Execution.TotalCostUSD)
	}

	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}

	event := loaded.Events[0]
	if event.Type != "tokens_logged" {
		t.Errorf("expected event type 'tokens_logged', got '%s'", event.Type)
	}
	if event.TaskID != "T001" {
		t.Errorf("expected task_id 'T001', got '%s'", event.TaskID)
	}
	if event.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}

	details := event.Details
	if int(details["input_tokens"].(float64)) != 500 {
		t.Errorf("expected input_tokens 500, got %v", details["input_tokens"])
	}
	if int(details["output_tokens"].(float64)) != 300 {
		t.Errorf("expected output_tokens 300, got %v", details["output_tokens"])
	}
	if int(details["total_tokens"].(float64)) != 800 {
		t.Errorf("expected total_tokens 800, got %v", details["total_tokens"])
	}
	if details["model"].(string) != "claude-3-opus" {
		t.Errorf("expected model 'claude-3-opus', got '%v'", details["model"])
	}
}

func TestLogTokensWithoutTaskID(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks:     map[string]Task{},
		Execution: Execution{TotalTokens: 0, TotalCostUSD: 0},
		Events:    []Event{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	err := sm.LogTokens("", 100, 50, 0.001, "")
	if err != nil {
		t.Fatalf("LogTokens failed: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Execution.TotalTokens != 150 {
		t.Errorf("expected total_tokens 150, got %d", loaded.Execution.TotalTokens)
	}
}

func TestGetMetrics(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1, DurationSecs: 60.0},
			"T002": {ID: "T002", Status: "complete", Phase: 1, DurationSecs: 120.0},
			"T003": {ID: "T003", Status: "failed", Phase: 1},
			"T004": {ID: "T004", Status: "skipped", Phase: 2},
			"T005": {ID: "T005", Status: "pending", Phase: 2},
			"T006": {ID: "T006", Status: "running", Phase: 2},
		},
		Execution: Execution{
			TotalTokens:  5000,
			TotalCostUSD: 0.05,
		},
	}

	metrics := GetMetrics(state)

	if metrics.TotalTasks != 6 {
		t.Errorf("expected total_tasks 6, got %d", metrics.TotalTasks)
	}
	if metrics.CompletedTasks != 2 {
		t.Errorf("expected completed_tasks 2, got %d", metrics.CompletedTasks)
	}
	if metrics.FailedTasks != 1 {
		t.Errorf("expected failed_tasks 1, got %d", metrics.FailedTasks)
	}
	if metrics.SkippedTasks != 1 {
		t.Errorf("expected skipped_tasks 1, got %d", metrics.SkippedTasks)
	}
	if metrics.PendingTasks != 1 {
		t.Errorf("expected pending_tasks 1, got %d", metrics.PendingTasks)
	}
	if metrics.RunningTasks != 1 {
		t.Errorf("expected running_tasks 1, got %d", metrics.RunningTasks)
	}

	expectedCompletionRate := (float64(2+1+1) / float64(6)) * 100
	if metrics.CompletionRate != expectedCompletionRate {
		t.Errorf("expected completion_rate %.2f, got %.2f", expectedCompletionRate, metrics.CompletionRate)
	}

	expectedSuccessRate := (float64(2) / float64(3)) * 100
	if metrics.SuccessRate != expectedSuccessRate {
		t.Errorf("expected success_rate %.2f, got %.2f", expectedSuccessRate, metrics.SuccessRate)
	}

	if metrics.TotalDurationSecs != 180.0 {
		t.Errorf("expected total_duration_seconds 180.0, got %f", metrics.TotalDurationSecs)
	}
	if metrics.AvgDurationSecs != 90.0 {
		t.Errorf("expected avg_duration_seconds 90.0, got %f", metrics.AvgDurationSecs)
	}

	if metrics.TotalTokens != 5000 {
		t.Errorf("expected total_tokens 5000, got %d", metrics.TotalTokens)
	}
	if metrics.TotalCostUSD != 0.05 {
		t.Errorf("expected total_cost_usd 0.05, got %f", metrics.TotalCostUSD)
	}
}

func TestGetMetricsEmpty(t *testing.T) {
	state := &State{
		Tasks:     map[string]Task{},
		Execution: Execution{},
	}

	metrics := GetMetrics(state)

	if metrics.TotalTasks != 0 {
		t.Errorf("expected total_tasks 0, got %d", metrics.TotalTasks)
	}
	if metrics.CompletionRate != 0 {
		t.Errorf("expected completion_rate 0, got %f", metrics.CompletionRate)
	}
	if metrics.SuccessRate != 0 {
		t.Errorf("expected success_rate 0, got %f", metrics.SuccessRate)
	}
	if metrics.AvgDurationSecs != 0 {
		t.Errorf("expected avg_duration_seconds 0, got %f", metrics.AvgDurationSecs)
	}
}

func TestGetMetricsAllComplete(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1, DurationSecs: 30.0},
			"T002": {ID: "T002", Status: "complete", Phase: 1, DurationSecs: 60.0},
		},
		Execution: Execution{},
	}

	metrics := GetMetrics(state)

	if metrics.CompletionRate != 100.0 {
		t.Errorf("expected completion_rate 100, got %f", metrics.CompletionRate)
	}
	if metrics.SuccessRate != 100.0 {
		t.Errorf("expected success_rate 100, got %f", metrics.SuccessRate)
	}
}

func TestGetPlanningMetrics(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "complete", Phase: 1},
			"T003": {ID: "T003", Status: "pending", Phase: 1, DependsOn: []string{}},
			"T004": {ID: "T004", Status: "pending", Phase: 2, DependsOn: []string{"T001"}},
			"T005": {ID: "T005", Status: "pending", Phase: 2, DependsOn: []string{"T003"}},
			"T006": {ID: "T006", Status: "blocked", Phase: 3},
		},
		Execution: Execution{CurrentPhase: 1},
	}

	metrics := GetPlanningMetrics(state)

	if metrics.TotalPhases != 3 {
		t.Errorf("expected total_phases 3, got %d", metrics.TotalPhases)
	}
	if metrics.CurrentPhase != 1 {
		t.Errorf("expected current_phase 1, got %d", metrics.CurrentPhase)
	}

	if metrics.TasksPerPhase[1] != 3 {
		t.Errorf("expected tasks_per_phase[1] = 3, got %d", metrics.TasksPerPhase[1])
	}
	if metrics.TasksPerPhase[2] != 2 {
		t.Errorf("expected tasks_per_phase[2] = 2, got %d", metrics.TasksPerPhase[2])
	}
	if metrics.TasksPerPhase[3] != 1 {
		t.Errorf("expected tasks_per_phase[3] = 1, got %d", metrics.TasksPerPhase[3])
	}

	if metrics.CompletedPerPhase[1] != 2 {
		t.Errorf("expected completed_per_phase[1] = 2, got %d", metrics.CompletedPerPhase[1])
	}
	if metrics.CompletedPerPhase[2] != 0 {
		t.Errorf("expected completed_per_phase[2] = 0, got %d", metrics.CompletedPerPhase[2])
	}

	expectedPhase1Progress := (float64(2) / float64(3)) * 100
	if metrics.PhaseProgress[1] != expectedPhase1Progress {
		t.Errorf("expected phase_progress[1] = %.2f, got %.2f", expectedPhase1Progress, metrics.PhaseProgress[1])
	}
	if metrics.PhaseProgress[2] != 0 {
		t.Errorf("expected phase_progress[2] = 0, got %.2f", metrics.PhaseProgress[2])
	}

	expectedOverall := (float64(2) / float64(6)) * 100
	if metrics.OverallProgress != expectedOverall {
		t.Errorf("expected overall_progress = %.2f, got %.2f", expectedOverall, metrics.OverallProgress)
	}

	if metrics.EstimatedRemaining != 4 {
		t.Errorf("expected estimated_remaining = 4, got %d", metrics.EstimatedRemaining)
	}
	if metrics.BlockedTasks != 1 {
		t.Errorf("expected blocked_tasks = 1, got %d", metrics.BlockedTasks)
	}
	if metrics.ReadyTasks != 2 {
		t.Errorf("expected ready_tasks = 2, got %d", metrics.ReadyTasks)
	}
}

func TestGetPlanningMetricsEmpty(t *testing.T) {
	state := &State{
		Tasks:     map[string]Task{},
		Execution: Execution{CurrentPhase: 0},
	}

	metrics := GetPlanningMetrics(state)

	if metrics.TotalPhases != 0 {
		t.Errorf("expected total_phases 0, got %d", metrics.TotalPhases)
	}
	if metrics.OverallProgress != 0 {
		t.Errorf("expected overall_progress 0, got %f", metrics.OverallProgress)
	}
	if metrics.EstimatedRemaining != 0 {
		t.Errorf("expected estimated_remaining 0, got %d", metrics.EstimatedRemaining)
	}
}

func TestGetPlanningMetricsWithSkipped(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "skipped", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1, DependsOn: []string{"T001"}},
		},
		Execution: Execution{CurrentPhase: 1},
	}

	metrics := GetPlanningMetrics(state)

	if metrics.CompletedPerPhase[1] != 1 {
		t.Errorf("expected completed_per_phase[1] = 1 (skipped counts), got %d", metrics.CompletedPerPhase[1])
	}
	if metrics.ReadyTasks != 1 {
		t.Errorf("expected ready_tasks = 1 (T002 should be ready after T001 skipped), got %d", metrics.ReadyTasks)
	}
}

func TestGetFailureMetrics(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {
				ID:       "T002",
				Name:     "Test task",
				Status:   "failed",
				Phase:    1,
				Error:    "test error",
				Attempts: 2,
				Failure:  &TaskFailure{Category: "test", Retryable: true},
			},
			"T003": {
				ID:       "T003",
				Status:   "failed",
				Phase:    1,
				Error:    "spec violation",
				Attempts: 1,
				Failure:  &TaskFailure{Category: "spec", Retryable: false},
			},
			"T004": {
				ID:       "T004",
				Status:   "failed",
				Phase:    2,
				Error:    "another test failure",
				Attempts: 3,
				Failure:  &TaskFailure{Category: "test", Retryable: true},
			},
		},
		Execution: Execution{},
	}

	breakdown := GetFailureMetrics(state)

	if breakdown.TotalFailures != 3 {
		t.Errorf("expected total_failures 3, got %d", breakdown.TotalFailures)
	}

	if breakdown.ByCategory["test"] != 2 {
		t.Errorf("expected by_category['test'] = 2, got %d", breakdown.ByCategory["test"])
	}
	if breakdown.ByCategory["spec"] != 1 {
		t.Errorf("expected by_category['spec'] = 1, got %d", breakdown.ByCategory["spec"])
	}

	if breakdown.RetryableCount != 2 {
		t.Errorf("expected retryable_count 2, got %d", breakdown.RetryableCount)
	}
	if breakdown.NonRetryableCount != 1 {
		t.Errorf("expected non_retryable_count 1, got %d", breakdown.NonRetryableCount)
	}

	if breakdown.ByRetryable[true] != 2 {
		t.Errorf("expected by_retryable[true] = 2, got %d", breakdown.ByRetryable[true])
	}
	if breakdown.ByRetryable[false] != 1 {
		t.Errorf("expected by_retryable[false] = 1, got %d", breakdown.ByRetryable[false])
	}

	if len(breakdown.FailedTasks) != 3 {
		t.Errorf("expected 3 failed_tasks entries, got %d", len(breakdown.FailedTasks))
	}

	found := false
	for _, ft := range breakdown.FailedTasks {
		if ft.TaskID == "T002" {
			found = true
			if ft.Name != "Test task" {
				t.Errorf("expected name 'Test task', got '%s'", ft.Name)
			}
			if ft.Category != "test" {
				t.Errorf("expected category 'test', got '%s'", ft.Category)
			}
			if ft.Error != "test error" {
				t.Errorf("expected error 'test error', got '%s'", ft.Error)
			}
			if !ft.Retryable {
				t.Error("expected retryable true")
			}
			if ft.Attempts != 2 {
				t.Errorf("expected attempts 2, got %d", ft.Attempts)
			}
		}
	}
	if !found {
		t.Error("T002 not found in failed_tasks")
	}
}

func TestGetFailureMetricsNoFailures(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 1},
		},
		Execution: Execution{},
	}

	breakdown := GetFailureMetrics(state)

	if breakdown.TotalFailures != 0 {
		t.Errorf("expected total_failures 0, got %d", breakdown.TotalFailures)
	}
	if len(breakdown.FailedTasks) != 0 {
		t.Errorf("expected empty failed_tasks, got %d", len(breakdown.FailedTasks))
	}
	if len(breakdown.ByCategory) != 0 {
		t.Errorf("expected empty by_category, got %v", breakdown.ByCategory)
	}
}

func TestGetFailureMetricsNoFailureInfo(t *testing.T) {
	state := &State{
		Tasks: map[string]Task{
			"T001": {
				ID:     "T001",
				Status: "failed",
				Phase:  1,
				Error:  "unknown error",
			},
		},
		Execution: Execution{},
	}

	breakdown := GetFailureMetrics(state)

	if breakdown.TotalFailures != 1 {
		t.Errorf("expected total_failures 1, got %d", breakdown.TotalFailures)
	}
	if breakdown.ByCategory["unknown"] != 1 {
		t.Errorf("expected by_category['unknown'] = 1, got %d", breakdown.ByCategory["unknown"])
	}
	if breakdown.NonRetryableCount != 1 {
		t.Errorf("expected non_retryable_count 1 (defaults to false), got %d", breakdown.NonRetryableCount)
	}
}

func TestGetMetricsViaStateManager(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1, DurationSecs: 30.0},
		},
		Execution: Execution{TotalTokens: 1000},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	metrics, err := sm.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if metrics.TotalTasks != 1 {
		t.Errorf("expected total_tasks 1, got %d", metrics.TotalTasks)
	}
	if metrics.TotalTokens != 1000 {
		t.Errorf("expected total_tokens 1000, got %d", metrics.TotalTokens)
	}
}

func TestGetPlanningMetricsViaStateManager(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "pending", Phase: 2},
		},
		Execution: Execution{CurrentPhase: 2},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	metrics, err := sm.GetPlanningMetrics()
	if err != nil {
		t.Fatalf("GetPlanningMetrics failed: %v", err)
	}

	if metrics.CurrentPhase != 2 {
		t.Errorf("expected current_phase 2, got %d", metrics.CurrentPhase)
	}
	if metrics.TotalPhases != 2 {
		t.Errorf("expected total_phases 2, got %d", metrics.TotalPhases)
	}
}

func TestGetFailureMetricsViaStateManager(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "executing"},
		TargetDir: "/target",
		CreatedAt: "2026-01-18T10:00:00Z",
		Tasks: map[string]Task{
			"T001": {
				ID:      "T001",
				Status:  "failed",
				Phase:   1,
				Failure: &TaskFailure{Category: "test", Retryable: true},
			},
		},
		Execution: Execution{},
	}
	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	breakdown, err := sm.GetFailureMetrics()
	if err != nil {
		t.Fatalf("GetFailureMetrics failed: %v", err)
	}

	if breakdown.TotalFailures != 1 {
		t.Errorf("expected total_failures 1, got %d", breakdown.TotalFailures)
	}
	if breakdown.RetryableCount != 1 {
		t.Errorf("expected retryable_count 1, got %d", breakdown.RetryableCount)
	}
}
