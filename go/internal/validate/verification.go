package validate

import (
	"fmt"
	"time"
)

type VerificationCriterion struct {
	Name     string `json:"name"`
	Score    string `json:"score"`
	Evidence string `json:"evidence,omitempty"`
}

type VerificationQuality struct {
	Types    string `json:"types,omitempty"`
	Docs     string `json:"docs,omitempty"`
	Patterns string `json:"patterns,omitempty"`
	Errors   string `json:"errors,omitempty"`
}

type VerificationTests struct {
	Coverage   string `json:"coverage,omitempty"`
	Assertions string `json:"assertions,omitempty"`
	EdgeCases  string `json:"edge_cases,omitempty"`
}

type VerificationResult struct {
	TaskID         string                  `json:"task_id"`
	Verdict        string                  `json:"verdict"`
	Recommendation string                  `json:"recommendation"`
	Criteria       []VerificationCriterion `json:"criteria,omitempty"`
	Quality        *VerificationQuality    `json:"quality,omitempty"`
	Tests          *VerificationTests      `json:"tests,omitempty"`
	VerifiedAt     string                  `json:"verified_at"`
}

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
	FalsePositives []string           `json:"false_positives"`
	FalseNegatives []string           `json:"false_negatives"`
	History        []CalibrationEntry `json:"history"`
}

type CalibrationScore struct {
	Score          float64 `json:"score"`
	TotalVerified  int     `json:"total_verified"`
	CorrectCount   int     `json:"correct_count"`
	FalsePositives int     `json:"false_positives"`
	FalseNegatives int     `json:"false_negatives"`
	ComputedAt     string  `json:"computed_at"`
}

var validVerdicts = map[string]bool{
	"PASS":        true,
	"FAIL":        true,
	"CONDITIONAL": true,
}

var validRecommendations = map[string]bool{
	"PROCEED": true,
	"BLOCK":   true,
}

var validCalibrationOutcomes = map[string]bool{
	"correct":        true,
	"false_positive": true,
	"false_negative": true,
}

func RecordVerification(
	taskID string,
	verdict string,
	recommendation string,
	criteria []VerificationCriterion,
	quality *VerificationQuality,
	tests *VerificationTests,
) (*VerificationResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	if !validVerdicts[verdict] {
		return nil, fmt.Errorf("invalid verdict: %s. Must be one of PASS, FAIL, CONDITIONAL", verdict)
	}

	if !validRecommendations[recommendation] {
		return nil, fmt.Errorf("invalid recommendation: %s. Must be one of PROCEED, BLOCK", recommendation)
	}

	result := &VerificationResult{
		TaskID:         taskID,
		Verdict:        verdict,
		Recommendation: recommendation,
		Criteria:       criteria,
		Quality:        quality,
		Tests:          tests,
		VerifiedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	return result, nil
}

func NewCalibrationData() *CalibrationData {
	return &CalibrationData{
		TotalVerified:  0,
		Correct:        0,
		FalsePositives: []string{},
		FalseNegatives: []string{},
		History:        []CalibrationEntry{},
	}
}

func RecordCalibration(
	calibration *CalibrationData,
	taskID string,
	verdict string,
	recommendation string,
	actualOutcome string,
	notes string,
) (*CalibrationData, error) {
	if calibration == nil {
		calibration = NewCalibrationData()
	}

	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	if !validCalibrationOutcomes[actualOutcome] {
		return nil, fmt.Errorf("invalid outcome: %s. Must be one of correct, false_positive, false_negative", actualOutcome)
	}

	entry := CalibrationEntry{
		TaskID:         taskID,
		Verdict:        verdict,
		Recommendation: recommendation,
		ActualOutcome:  actualOutcome,
		Notes:          notes,
		RecordedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	calibration.History = append(calibration.History, entry)
	calibration.TotalVerified++

	switch actualOutcome {
	case "correct":
		calibration.Correct++
	case "false_positive":
		calibration.FalsePositives = append(calibration.FalsePositives, taskID)
	case "false_negative":
		calibration.FalseNegatives = append(calibration.FalseNegatives, taskID)
	}

	return calibration, nil
}

func GetCalibrationScore(calibration *CalibrationData) *CalibrationScore {
	score := &CalibrationScore{
		Score:          1.0,
		TotalVerified:  0,
		CorrectCount:   0,
		FalsePositives: 0,
		FalseNegatives: 0,
		ComputedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	if calibration == nil {
		return score
	}

	score.TotalVerified = calibration.TotalVerified
	score.CorrectCount = calibration.Correct
	score.FalsePositives = len(calibration.FalsePositives)
	score.FalseNegatives = len(calibration.FalseNegatives)

	if score.TotalVerified > 0 {
		score.Score = float64(score.CorrectCount) / float64(score.TotalVerified)
	}

	return score
}
