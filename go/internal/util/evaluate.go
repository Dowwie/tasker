package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EvaluationMetrics contains computed performance metrics from task execution.
type EvaluationMetrics struct {
	TaskSuccessRate        float64 `json:"task_success_rate"`
	FirstAttemptSuccessRate float64 `json:"first_attempt_success_rate"`
	AvgAttempts            float64 `json:"avg_attempts"`
	TokensPerTask          int     `json:"tokens_per_task"`
	CostPerTask            float64 `json:"cost_per_task"`
	QualityPassRate        float64 `json:"quality_pass_rate"`
	FunctionalPassRate     float64 `json:"functional_pass_rate"`
	TestEdgeCaseRate       float64 `json:"test_edge_case_rate"`
	CompletedCount         int     `json:"completed_count"`
	FailedCount            int     `json:"failed_count"`
	TotalTokens            int     `json:"total_tokens"`
	TotalCostUSD           float64 `json:"total_cost_usd"`
	CriteriaPass           int     `json:"criteria_pass"`
	CriteriaPartial        int     `json:"criteria_partial"`
	CriteriaFail           int     `json:"criteria_fail"`
	TotalCriteria          int     `json:"total_criteria"`
	QualityDimensions      map[string]int `json:"quality_dimensions"`
	QualityTotals          map[string]int `json:"quality_totals"`
}

// FailedTask represents a task that failed during execution.
type FailedTask struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

// ImprovementPattern describes a pattern identified for improvement.
type ImprovementPattern struct {
	Description string `json:"description"`
	Count       int    `json:"count"`
}

// EvaluationResult contains the full evaluation report data.
type EvaluationResult struct {
	PlanningVerdict      string               `json:"planning_verdict"`
	PlanningIssuesCount  int                  `json:"planning_issues_count"`
	Metrics              *EvaluationMetrics   `json:"metrics"`
	FailedTasks          []FailedTask         `json:"failed_tasks"`
	ImprovementPatterns  []ImprovementPattern `json:"improvement_patterns"`
	TotalTasks           int                  `json:"total_tasks"`
	BlockedCount         int                  `json:"blocked_count"`
	SkippedCount         int                  `json:"skipped_count"`
}

// stateTask matches the task structure in state.json for evaluation purposes.
type stateTask struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Status       string                 `json:"status"`
	Attempts     int                    `json:"attempts"`
	Error        string                 `json:"error"`
	Verification map[string]interface{} `json:"verification"`
}

// stateExecution matches the execution structure in state.json.
type stateExecution struct {
	CompletedCount int     `json:"completed_count"`
	FailedCount    int     `json:"failed_count"`
	TotalTokens    int     `json:"total_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
}

// stateForEval is a minimal state structure for evaluation.
type stateForEval struct {
	Tasks     map[string]stateTask   `json:"tasks"`
	Execution stateExecution         `json:"execution"`
	Artifacts map[string]interface{} `json:"artifacts"`
}

// RunEvaluation runs a full evaluation on the state file in the planning directory
// and returns the evaluation result with computed metrics.
func RunEvaluation(planningDir string) (*EvaluationResult, error) {
	statePath := filepath.Join(planningDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("state file not found: %s", statePath)
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state stateForEval
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	metrics := computeMetrics(&state)
	failedTasks := getFailedTasks(&state)
	patterns := getImprovementPatterns(&state)

	blockedCount := 0
	skippedCount := 0
	for _, task := range state.Tasks {
		if task.Status == "blocked" {
			blockedCount++
		}
		if task.Status == "skipped" {
			skippedCount++
		}
	}

	planningVerdict := "N/A"
	planningIssuesCount := 0
	if artifacts, ok := state.Artifacts["task_validation"].(map[string]interface{}); ok {
		if verdict, ok := artifacts["verdict"].(string); ok {
			planningVerdict = verdict
		}
		if issues, ok := artifacts["issues"].([]interface{}); ok {
			planningIssuesCount = len(issues)
		}
	}

	return &EvaluationResult{
		PlanningVerdict:     planningVerdict,
		PlanningIssuesCount: planningIssuesCount,
		Metrics:             metrics,
		FailedTasks:         failedTasks,
		ImprovementPatterns: patterns,
		TotalTasks:          len(state.Tasks),
		BlockedCount:        blockedCount,
		SkippedCount:        skippedCount,
	}, nil
}

func computeMetrics(state *stateForEval) *EvaluationMetrics {
	completed := state.Execution.CompletedCount
	failed := state.Execution.FailedCount
	totalTokens := state.Execution.TotalTokens
	totalCost := state.Execution.TotalCostUSD
	totalFinished := completed + failed

	firstAttemptSuccesses := 0
	qualityFullPass := 0
	totalCriteria := 0
	criteriaPass := 0
	criteriaPartial := 0
	criteriaFail := 0
	tasksWithTests := 0
	edgeCasesPass := 0
	totalAttempts := 0

	qualityDimensions := map[string]int{"types": 0, "docs": 0, "patterns": 0, "errors": 0}
	qualityTotals := map[string]int{"types": 0, "docs": 0, "patterns": 0, "errors": 0}

	for _, task := range state.Tasks {
		if task.Status == "complete" {
			attempts := task.Attempts
			if attempts == 0 {
				attempts = 1
			}
			totalAttempts += attempts
			if attempts == 1 {
				firstAttemptSuccesses++
			}

			if task.Verification != nil {
				allQualityPass := true
				if quality, ok := task.Verification["quality"].(map[string]interface{}); ok {
					for _, dim := range []string{"types", "docs", "patterns", "errors"} {
						if score, ok := quality[dim].(string); ok {
							qualityTotals[dim]++
							if score == "PASS" {
								qualityDimensions[dim]++
							} else {
								allQualityPass = false
							}
						}
					}
				}
				if allQualityPass {
					qualityFullPass++
				}

				if criteria, ok := task.Verification["criteria"].([]interface{}); ok {
					for _, c := range criteria {
						if crit, ok := c.(map[string]interface{}); ok {
							totalCriteria++
							if score, ok := crit["score"].(string); ok {
								switch score {
								case "PASS":
									criteriaPass++
								case "PARTIAL":
									criteriaPartial++
								default:
									criteriaFail++
								}
							}
						}
					}
				}

				if tests, ok := task.Verification["tests"].(map[string]interface{}); ok {
					tasksWithTests++
					if edgeCases, ok := tests["edge_cases"].(string); ok && edgeCases == "PASS" {
						edgeCasesPass++
					}
				}
			}
		}
	}

	metrics := &EvaluationMetrics{
		CompletedCount:    completed,
		FailedCount:       failed,
		TotalTokens:       totalTokens,
		TotalCostUSD:      totalCost,
		CriteriaPass:      criteriaPass,
		CriteriaPartial:   criteriaPartial,
		CriteriaFail:      criteriaFail,
		TotalCriteria:     totalCriteria,
		QualityDimensions: qualityDimensions,
		QualityTotals:     qualityTotals,
	}

	if totalFinished > 0 {
		metrics.TaskSuccessRate = float64(completed) / float64(totalFinished)
	}
	if completed > 0 {
		metrics.FirstAttemptSuccessRate = float64(firstAttemptSuccesses) / float64(completed)
		metrics.AvgAttempts = float64(totalAttempts) / float64(completed)
		metrics.TokensPerTask = totalTokens / completed
		metrics.CostPerTask = totalCost / float64(completed)
		metrics.QualityPassRate = float64(qualityFullPass) / float64(completed)
	}
	if totalCriteria > 0 {
		metrics.FunctionalPassRate = float64(criteriaPass) / float64(totalCriteria)
	}
	if tasksWithTests > 0 {
		metrics.TestEdgeCaseRate = float64(edgeCasesPass) / float64(tasksWithTests)
	}

	return metrics
}

func getFailedTasks(state *stateForEval) []FailedTask {
	var failed []FailedTask
	for id, task := range state.Tasks {
		if task.Status == "failed" {
			errorMsg := task.Error
			if errorMsg == "" {
				errorMsg = "Unknown error"
			}
			failed = append(failed, FailedTask{
				ID:    id,
				Name:  task.Name,
				Error: errorMsg,
			})
		}
	}
	return failed
}

func getImprovementPatterns(state *stateForEval) []ImprovementPattern {
	var patterns []ImprovementPattern
	edgeCaseIssues := 0
	typeIssues := 0
	docIssues := 0

	for _, task := range state.Tasks {
		if task.Verification != nil {
			if tests, ok := task.Verification["tests"].(map[string]interface{}); ok {
				if edgeCases, ok := tests["edge_cases"].(string); ok {
					if edgeCases == "PARTIAL" || edgeCases == "FAIL" {
						edgeCaseIssues++
					}
				}
			}

			if quality, ok := task.Verification["quality"].(map[string]interface{}); ok {
				if types, ok := quality["types"].(string); ok {
					if types == "PARTIAL" || types == "FAIL" {
						typeIssues++
					}
				}
				if docs, ok := quality["docs"].(string); ok {
					if docs == "PARTIAL" || docs == "FAIL" {
						docIssues++
					}
				}
			}
		}
	}

	if edgeCaseIssues > 0 {
		patterns = append(patterns, ImprovementPattern{
			Description: fmt.Sprintf("%d task(s) had issues with edge case testing", edgeCaseIssues),
			Count:       edgeCaseIssues,
		})
	}
	if typeIssues > 0 {
		patterns = append(patterns, ImprovementPattern{
			Description: fmt.Sprintf("%d task(s) had type annotation issues", typeIssues),
			Count:       typeIssues,
		})
	}
	if docIssues > 0 {
		patterns = append(patterns, ImprovementPattern{
			Description: fmt.Sprintf("%d task(s) had documentation issues", docIssues),
			Count:       docIssues,
		})
	}

	return patterns
}

// FormatEvaluationReport formats an EvaluationResult as a human-readable string.
func FormatEvaluationReport(result *EvaluationResult) string {
	var output string

	output += "Execution Evaluation Report\n"
	output += "============================================================\n\n"

	output += "Planning Quality\n"
	output += "----------------------------------------\n"
	output += fmt.Sprintf("Plan Verdict: %s\n", result.PlanningVerdict)
	output += fmt.Sprintf("Issues at Planning: %d\n\n", result.PlanningIssuesCount)

	output += "Execution Summary\n"
	output += "----------------------------------------\n"
	output += fmt.Sprintf("Tasks: %d total\n", result.TotalTasks)
	if result.Metrics.CompletedCount > 0 {
		pct := float64(result.Metrics.CompletedCount) / float64(result.TotalTasks) * 100
		output += fmt.Sprintf("  Completed:     %d (%.0f%%)\n", result.Metrics.CompletedCount, pct)
	}
	if result.Metrics.FailedCount > 0 {
		pct := float64(result.Metrics.FailedCount) / float64(result.TotalTasks) * 100
		output += fmt.Sprintf("  Failed:        %d (%.0f%%)\n", result.Metrics.FailedCount, pct)
	}
	if result.BlockedCount > 0 {
		pct := float64(result.BlockedCount) / float64(result.TotalTasks) * 100
		output += fmt.Sprintf("  Blocked:       %d (%.0f%%)\n", result.BlockedCount, pct)
	}
	if result.SkippedCount > 0 {
		pct := float64(result.SkippedCount) / float64(result.TotalTasks) * 100
		output += fmt.Sprintf("  Skipped:       %d (%.0f%%)\n", result.SkippedCount, pct)
	}
	output += "\n"

	firstAttemptCount := int(result.Metrics.FirstAttemptSuccessRate * float64(result.Metrics.CompletedCount))
	output += fmt.Sprintf("First-Attempt Success: %d/%d (%.0f%%)\n",
		firstAttemptCount, result.Metrics.CompletedCount,
		result.Metrics.FirstAttemptSuccessRate*100)
	output += fmt.Sprintf("Average Attempts: %.2f\n\n", result.Metrics.AvgAttempts)

	output += "Verification Breakdown\n"
	output += "----------------------------------------\n"
	output += "Functional Criteria:\n"
	if result.Metrics.TotalCriteria > 0 {
		passRate := float64(result.Metrics.CriteriaPass) / float64(result.Metrics.TotalCriteria) * 100
		partialRate := float64(result.Metrics.CriteriaPartial) / float64(result.Metrics.TotalCriteria) * 100
		failRate := float64(result.Metrics.CriteriaFail) / float64(result.Metrics.TotalCriteria) * 100
		output += fmt.Sprintf("  PASS:     %d/%d (%.0f%%)\n", result.Metrics.CriteriaPass, result.Metrics.TotalCriteria, passRate)
		output += fmt.Sprintf("  PARTIAL:  %d/%d (%.0f%%)\n", result.Metrics.CriteriaPartial, result.Metrics.TotalCriteria, partialRate)
		output += fmt.Sprintf("  FAIL:     %d/%d (%.0f%%)\n", result.Metrics.CriteriaFail, result.Metrics.TotalCriteria, failRate)
	} else {
		output += "  No criteria data\n"
	}
	output += "\n"

	output += "Code Quality:\n"
	for _, dim := range []string{"types", "docs", "patterns", "errors"} {
		total := result.Metrics.QualityTotals[dim]
		passed := result.Metrics.QualityDimensions[dim]
		if total > 0 {
			dimTitle := dim
			if len(dim) > 0 {
				dimTitle = string(dim[0]-32) + dim[1:]
			}
			output += fmt.Sprintf("  %-10s %d/%d PASS\n", dimTitle, passed, total)
		}
	}
	output += "\n"

	output += "Cost Analysis\n"
	output += "----------------------------------------\n"
	output += fmt.Sprintf("Total Tokens:  %d\n", result.Metrics.TotalTokens)
	output += fmt.Sprintf("Total Cost:    $%.2f\n", result.Metrics.TotalCostUSD)
	if result.Metrics.CompletedCount > 0 {
		output += fmt.Sprintf("Per Task:      $%.4f\n", result.Metrics.CostPerTask)
	}
	output += "\n"

	if len(result.FailedTasks) > 0 {
		output += "Failure Analysis\n"
		output += "----------------------------------------\n"
		for _, ft := range result.FailedTasks {
			output += fmt.Sprintf("%s: FAIL - %s\n", ft.ID, ft.Error)
		}
		output += "\n"
	}

	if len(result.ImprovementPatterns) > 0 {
		output += "Improvement Patterns\n"
		output += "----------------------------------------\n"
		for _, p := range result.ImprovementPatterns {
			output += fmt.Sprintf("- %s\n", p.Description)
		}
		output += "\n"
	}

	return output
}
