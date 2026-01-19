package state

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type CalibrationEntry struct {
	TaskID         string `json:"task_id"`
	Verdict        string `json:"verdict"`
	Recommendation string `json:"recommendation"`
	ActualOutcome  string `json:"actual_outcome"`
	Notes          string `json:"notes,omitempty"`
	RecordedAt     string `json:"recorded_at"`
}

type CalibrationData struct {
	TotalVerified  int                `json:"total_verified"`
	Correct        int                `json:"correct"`
	FalsePositives []string           `json:"false_positives,omitempty"`
	FalseNegatives []string           `json:"false_negatives,omitempty"`
	History        []CalibrationEntry `json:"history,omitempty"`
}

func RecordVerification(sm *StateManager, taskID, verdict, recommendation string, criteria []VerificationCriterion, quality *VerificationQuality, tests *VerificationTests) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	validVerdicts := map[string]bool{"PASS": true, "FAIL": true, "CONDITIONAL": true}
	if !validVerdicts[verdict] {
		return fmt.Errorf("invalid verdict: %s (must be PASS, FAIL, or CONDITIONAL)", verdict)
	}

	validRecommendations := map[string]bool{"PROCEED": true, "BLOCK": true}
	if !validRecommendations[recommendation] {
		return fmt.Errorf("invalid recommendation: %s (must be PROCEED or BLOCK)", recommendation)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	task.Verification = &TaskVerification{
		Verdict:        verdict,
		Recommendation: recommendation,
		Criteria:       criteria,
		Quality:        quality,
		Tests:          tests,
		VerifiedAt:     now,
	}

	state.Tasks[taskID] = task

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "verification_recorded",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"verdict":        verdict,
			"recommendation": recommendation,
		},
	})

	if recommendation == "BLOCK" {
		blockedCount := 0
		for _, blockedID := range task.Blocks {
			if blockedTask, ok := state.Tasks[blockedID]; ok && blockedTask.Status == "pending" {
				blockedTask.Status = "blocked"
				blockedTask.Error = fmt.Sprintf("Blocked by verification failure of %s", taskID)
				state.Tasks[blockedID] = blockedTask
				blockedCount++
			}
		}
		if blockedCount > 0 {
			state.Events = append(state.Events, Event{
				Timestamp: now,
				Type:      "tasks_blocked_by_verification",
				TaskID:    taskID,
				Details: map[string]interface{}{
					"blocked_count": blockedCount,
				},
			})
		}
	}

	return sm.Save(state)
}

func RecordCalibration(sm *StateManager, taskID, actualOutcome, notes string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Verification == nil {
		return fmt.Errorf("no verification data for task %s", taskID)
	}

	validOutcomes := map[string]bool{"correct": true, "false_positive": true, "false_negative": true}
	if !validOutcomes[actualOutcome] {
		return fmt.Errorf("invalid outcome: %s (must be correct, false_positive, or false_negative)", actualOutcome)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	entry := CalibrationEntry{
		TaskID:         taskID,
		Verdict:        task.Verification.Verdict,
		Recommendation: task.Verification.Recommendation,
		ActualOutcome:  actualOutcome,
		Notes:          notes,
		RecordedAt:     now,
	}

	calibrationFile := sm.dir + "/calibration.json"
	var cal CalibrationData

	if data, err := LoadCalibrationData(calibrationFile); err == nil && data != nil {
		cal = *data
	}

	cal.History = append(cal.History, entry)
	cal.TotalVerified++

	switch actualOutcome {
	case "correct":
		cal.Correct++
	case "false_positive":
		cal.FalsePositives = append(cal.FalsePositives, taskID)
	case "false_negative":
		cal.FalseNegatives = append(cal.FalseNegatives, taskID)
	}

	if err := SaveCalibrationData(calibrationFile, &cal); err != nil {
		return err
	}

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "calibration_recorded",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"actual_outcome": actualOutcome,
			"verdict":        task.Verification.Verdict,
		},
	})

	return sm.Save(state)
}

func GetCalibrationScore(sm *StateManager) (float64, *CalibrationData, error) {
	calibrationFile := sm.dir + "/calibration.json"
	cal, err := LoadCalibrationData(calibrationFile)
	if err != nil {
		return 1.0, nil, nil
	}
	if cal == nil || cal.TotalVerified == 0 {
		return 1.0, cal, nil
	}

	score := float64(cal.Correct) / float64(cal.TotalVerified)
	return score, cal, nil
}

func LoadCalibrationData(path string) (*CalibrationData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var cal CalibrationData
	if err := json.Unmarshal(data, &cal); err != nil {
		return nil, err
	}

	return &cal, nil
}

func SaveCalibrationData(path string, cal *CalibrationData) error {
	data, err := json.MarshalIndent(cal, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
