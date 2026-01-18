package state

import (
	"fmt"
	"time"
)

type TokenEntry struct {
	Timestamp   string  `json:"timestamp"`
	TaskID      string  `json:"task_id,omitempty"`
	InputTokens int     `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens int     `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd,omitempty"`
	Model       string  `json:"model,omitempty"`
}

type PerformanceMetrics struct {
	TotalTasks      int     `json:"total_tasks"`
	CompletedTasks  int     `json:"completed_tasks"`
	FailedTasks     int     `json:"failed_tasks"`
	SkippedTasks    int     `json:"skipped_tasks"`
	PendingTasks    int     `json:"pending_tasks"`
	RunningTasks    int     `json:"running_tasks"`
	CompletionRate  float64 `json:"completion_rate"`
	SuccessRate     float64 `json:"success_rate"`
	AvgDurationSecs float64 `json:"avg_duration_seconds"`
	TotalDurationSecs float64 `json:"total_duration_seconds"`
	TotalTokens     int     `json:"total_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
}

type PlanningMetrics struct {
	TotalPhases       int     `json:"total_phases"`
	CurrentPhase      int     `json:"current_phase"`
	TasksPerPhase     map[int]int `json:"tasks_per_phase"`
	CompletedPerPhase map[int]int `json:"completed_per_phase"`
	PhaseProgress     map[int]float64 `json:"phase_progress"`
	OverallProgress   float64 `json:"overall_progress"`
	EstimatedRemaining int    `json:"estimated_remaining_tasks"`
	BlockedTasks      int     `json:"blocked_tasks"`
	ReadyTasks        int     `json:"ready_tasks"`
}

type FailureBreakdown struct {
	ByCategory    map[string]int `json:"by_category"`
	ByRetryable   map[bool]int   `json:"by_retryable"`
	TotalFailures int            `json:"total_failures"`
	RetryableCount int           `json:"retryable_count"`
	NonRetryableCount int        `json:"non_retryable_count"`
	FailedTasks   []FailedTaskInfo `json:"failed_tasks,omitempty"`
}

type FailedTaskInfo struct {
	TaskID      string `json:"task_id"`
	Name        string `json:"name,omitempty"`
	Category    string `json:"category,omitempty"`
	Error       string `json:"error,omitempty"`
	Retryable   bool   `json:"retryable"`
	Attempts    int    `json:"attempts"`
}

func LogTokens(sm *StateManager, taskID string, inputTokens, outputTokens int, costUSD float64, model string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	totalTokens := inputTokens + outputTokens

	state.Execution.TotalTokens += totalTokens
	state.Execution.TotalCostUSD += costUSD

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "tokens_logged",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  totalTokens,
			"cost_usd":      costUSD,
			"model":         model,
		},
	})

	if err := sm.Save(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func GetMetrics(state *State) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		TotalTokens:  state.Execution.TotalTokens,
		TotalCostUSD: state.Execution.TotalCostUSD,
	}

	var totalDuration float64
	var completedWithDuration int

	for _, task := range state.Tasks {
		metrics.TotalTasks++
		switch task.Status {
		case "complete":
			metrics.CompletedTasks++
			if task.DurationSecs > 0 {
				totalDuration += task.DurationSecs
				completedWithDuration++
			}
		case "failed":
			metrics.FailedTasks++
		case "skipped":
			metrics.SkippedTasks++
		case "pending", "ready", "blocked":
			metrics.PendingTasks++
		case "running":
			metrics.RunningTasks++
		}
	}

	if metrics.TotalTasks > 0 {
		finished := metrics.CompletedTasks + metrics.FailedTasks + metrics.SkippedTasks
		metrics.CompletionRate = float64(finished) / float64(metrics.TotalTasks) * 100
	}

	finishedNonSkipped := metrics.CompletedTasks + metrics.FailedTasks
	if finishedNonSkipped > 0 {
		metrics.SuccessRate = float64(metrics.CompletedTasks) / float64(finishedNonSkipped) * 100
	}

	metrics.TotalDurationSecs = totalDuration
	if completedWithDuration > 0 {
		metrics.AvgDurationSecs = totalDuration / float64(completedWithDuration)
	}

	return metrics
}

func GetPlanningMetrics(state *State) *PlanningMetrics {
	metrics := &PlanningMetrics{
		TasksPerPhase:     make(map[int]int),
		CompletedPerPhase: make(map[int]int),
		PhaseProgress:     make(map[int]float64),
		CurrentPhase:      state.Execution.CurrentPhase,
	}

	maxPhase := 0
	completedOrSkipped := make(map[string]bool)

	for _, task := range state.Tasks {
		metrics.TasksPerPhase[task.Phase]++
		if task.Phase > maxPhase {
			maxPhase = task.Phase
		}

		if task.Status == "complete" || task.Status == "skipped" {
			metrics.CompletedPerPhase[task.Phase]++
			completedOrSkipped[task.ID] = true
		}

		if task.Status == "blocked" {
			metrics.BlockedTasks++
		}
	}

	for _, task := range state.Tasks {
		if task.Status != "pending" {
			continue
		}
		allDepsSatisfied := true
		for _, dep := range task.DependsOn {
			if !completedOrSkipped[dep] {
				allDepsSatisfied = false
				break
			}
		}
		if allDepsSatisfied {
			metrics.ReadyTasks++
		}
	}

	metrics.TotalPhases = maxPhase

	for phase, total := range metrics.TasksPerPhase {
		if total > 0 {
			completed := metrics.CompletedPerPhase[phase]
			metrics.PhaseProgress[phase] = float64(completed) / float64(total) * 100
		}
	}

	totalTasks := len(state.Tasks)
	if totalTasks > 0 {
		totalCompleted := 0
		for _, count := range metrics.CompletedPerPhase {
			totalCompleted += count
		}
		metrics.OverallProgress = float64(totalCompleted) / float64(totalTasks) * 100
		metrics.EstimatedRemaining = totalTasks - totalCompleted
	}

	return metrics
}

func GetFailureMetrics(state *State) *FailureBreakdown {
	breakdown := &FailureBreakdown{
		ByCategory:  make(map[string]int),
		ByRetryable: make(map[bool]int),
		FailedTasks: []FailedTaskInfo{},
	}

	for _, task := range state.Tasks {
		if task.Status != "failed" {
			continue
		}

		breakdown.TotalFailures++

		category := "unknown"
		retryable := false

		if task.Failure != nil {
			if task.Failure.Category != "" {
				category = task.Failure.Category
			}
			retryable = task.Failure.Retryable
		}

		breakdown.ByCategory[category]++
		breakdown.ByRetryable[retryable]++

		if retryable {
			breakdown.RetryableCount++
		} else {
			breakdown.NonRetryableCount++
		}

		breakdown.FailedTasks = append(breakdown.FailedTasks, FailedTaskInfo{
			TaskID:    task.ID,
			Name:      task.Name,
			Category:  category,
			Error:     task.Error,
			Retryable: retryable,
			Attempts:  task.Attempts,
		})
	}

	return breakdown
}

func (sm *StateManager) LogTokens(taskID string, inputTokens, outputTokens int, costUSD float64, model string) error {
	return LogTokens(sm, taskID, inputTokens, outputTokens, costUSD, model)
}

func (sm *StateManager) GetMetrics() (*PerformanceMetrics, error) {
	state, err := sm.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	return GetMetrics(state), nil
}

func (sm *StateManager) GetPlanningMetrics() (*PlanningMetrics, error) {
	state, err := sm.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	return GetPlanningMetrics(state), nil
}

func (sm *StateManager) GetFailureMetrics() (*FailureBreakdown, error) {
	state, err := sm.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	return GetFailureMetrics(state), nil
}
