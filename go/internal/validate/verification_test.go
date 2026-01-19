package validate

import (
	"testing"
)

func TestRecordVerification_Valid(t *testing.T) {
	criteria := []VerificationCriterion{
		{Name: "test passes", Score: "PASS", Evidence: "pytest returned 0"},
	}
	quality := &VerificationQuality{
		Types:    "PASS",
		Docs:     "PASS",
		Patterns: "PARTIAL",
		Errors:   "PASS",
	}
	tests := &VerificationTests{
		Coverage:   "PASS",
		Assertions: "PASS",
		EdgeCases:  "PARTIAL",
	}

	result, err := RecordVerification("T001", "PASS", "PROCEED", criteria, quality, tests)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if result.TaskID != "T001" {
		t.Errorf("expected TaskID T001, got %s", result.TaskID)
	}
	if result.Verdict != "PASS" {
		t.Errorf("expected Verdict PASS, got %s", result.Verdict)
	}
	if result.Recommendation != "PROCEED" {
		t.Errorf("expected Recommendation PROCEED, got %s", result.Recommendation)
	}
	if len(result.Criteria) != 1 {
		t.Errorf("expected 1 criterion, got %d", len(result.Criteria))
	}
	if result.Quality == nil {
		t.Error("expected Quality to be set")
	}
	if result.Tests == nil {
		t.Error("expected Tests to be set")
	}
	if result.VerifiedAt == "" {
		t.Error("expected VerifiedAt to be set")
	}
}

func TestRecordVerification_InvalidVerdict(t *testing.T) {
	_, err := RecordVerification("T001", "INVALID", "PROCEED", nil, nil, nil)
	if err == nil {
		t.Error("expected error for invalid verdict")
		return
	}
	expected := "invalid verdict: INVALID"
	if err.Error()[:len(expected)] != expected {
		t.Errorf("expected error starting with '%s', got: %v", expected, err)
	}
}

func TestRecordVerification_InvalidRecommendation(t *testing.T) {
	_, err := RecordVerification("T001", "PASS", "INVALID", nil, nil, nil)
	if err == nil {
		t.Error("expected error for invalid recommendation")
		return
	}
	expected := "invalid recommendation: INVALID"
	if err.Error()[:len(expected)] != expected {
		t.Errorf("expected error starting with '%s', got: %v", expected, err)
	}
}

func TestRecordVerification_EmptyTaskID(t *testing.T) {
	_, err := RecordVerification("", "PASS", "PROCEED", nil, nil, nil)
	if err == nil {
		t.Error("expected error for empty task ID")
		return
	}
	if err.Error() != "task ID is required" {
		t.Errorf("expected 'task ID is required', got: %v", err)
	}
}

func TestRecordVerification_AllVerdicts(t *testing.T) {
	verdicts := []string{"PASS", "FAIL", "CONDITIONAL"}
	for _, verdict := range verdicts {
		result, err := RecordVerification("T001", verdict, "PROCEED", nil, nil, nil)
		if err != nil {
			t.Errorf("expected no error for verdict %s, got: %v", verdict, err)
			continue
		}
		if result.Verdict != verdict {
			t.Errorf("expected verdict %s, got %s", verdict, result.Verdict)
		}
	}
}

func TestRecordVerification_AllRecommendations(t *testing.T) {
	recommendations := []string{"PROCEED", "BLOCK"}
	for _, rec := range recommendations {
		result, err := RecordVerification("T001", "PASS", rec, nil, nil, nil)
		if err != nil {
			t.Errorf("expected no error for recommendation %s, got: %v", rec, err)
			continue
		}
		if result.Recommendation != rec {
			t.Errorf("expected recommendation %s, got %s", rec, result.Recommendation)
		}
	}
}

func TestRecordCalibration_Correct(t *testing.T) {
	calibration := NewCalibrationData()

	result, err := RecordCalibration(calibration, "T001", "PASS", "PROCEED", "correct", "worked fine")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if result.TotalVerified != 1 {
		t.Errorf("expected TotalVerified 1, got %d", result.TotalVerified)
	}
	if result.Correct != 1 {
		t.Errorf("expected Correct 1, got %d", result.Correct)
	}
	if len(result.FalsePositives) != 0 {
		t.Errorf("expected 0 false positives, got %d", len(result.FalsePositives))
	}
	if len(result.FalseNegatives) != 0 {
		t.Errorf("expected 0 false negatives, got %d", len(result.FalseNegatives))
	}
	if len(result.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(result.History))
	}
}

func TestRecordCalibration_FalsePositive(t *testing.T) {
	calibration := NewCalibrationData()

	result, err := RecordCalibration(calibration, "T001", "PASS", "PROCEED", "false_positive", "failed in prod")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if result.TotalVerified != 1 {
		t.Errorf("expected TotalVerified 1, got %d", result.TotalVerified)
	}
	if result.Correct != 0 {
		t.Errorf("expected Correct 0, got %d", result.Correct)
	}
	if len(result.FalsePositives) != 1 {
		t.Errorf("expected 1 false positive, got %d", len(result.FalsePositives))
	}
	if result.FalsePositives[0] != "T001" {
		t.Errorf("expected T001 in false positives, got %s", result.FalsePositives[0])
	}
}

func TestRecordCalibration_FalseNegative(t *testing.T) {
	calibration := NewCalibrationData()

	result, err := RecordCalibration(calibration, "T001", "FAIL", "BLOCK", "false_negative", "would have worked")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if result.TotalVerified != 1 {
		t.Errorf("expected TotalVerified 1, got %d", result.TotalVerified)
	}
	if len(result.FalseNegatives) != 1 {
		t.Errorf("expected 1 false negative, got %d", len(result.FalseNegatives))
	}
	if result.FalseNegatives[0] != "T001" {
		t.Errorf("expected T001 in false negatives, got %s", result.FalseNegatives[0])
	}
}

func TestRecordCalibration_InvalidOutcome(t *testing.T) {
	calibration := NewCalibrationData()

	_, err := RecordCalibration(calibration, "T001", "PASS", "PROCEED", "invalid", "")
	if err == nil {
		t.Error("expected error for invalid outcome")
		return
	}
	expected := "invalid outcome: invalid"
	if err.Error()[:len(expected)] != expected {
		t.Errorf("expected error starting with '%s', got: %v", expected, err)
	}
}

func TestRecordCalibration_EmptyTaskID(t *testing.T) {
	calibration := NewCalibrationData()

	_, err := RecordCalibration(calibration, "", "PASS", "PROCEED", "correct", "")
	if err == nil {
		t.Error("expected error for empty task ID")
		return
	}
	if err.Error() != "task ID is required" {
		t.Errorf("expected 'task ID is required', got: %v", err)
	}
}

func TestRecordCalibration_NilCalibration(t *testing.T) {
	result, err := RecordCalibration(nil, "T001", "PASS", "PROCEED", "correct", "")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if result == nil {
		t.Error("expected non-nil result")
		return
	}
	if result.TotalVerified != 1 {
		t.Errorf("expected TotalVerified 1, got %d", result.TotalVerified)
	}
}

func TestGetCalibrationScore_Perfect(t *testing.T) {
	calibration := NewCalibrationData()

	for i := 1; i <= 5; i++ {
		calibration, _ = RecordCalibration(calibration, "T00"+string(rune('0'+i)), "PASS", "PROCEED", "correct", "")
	}

	score := GetCalibrationScore(calibration)
	if score.Score != 1.0 {
		t.Errorf("expected Score 1.0, got %f", score.Score)
	}
	if score.TotalVerified != 5 {
		t.Errorf("expected TotalVerified 5, got %d", score.TotalVerified)
	}
	if score.CorrectCount != 5 {
		t.Errorf("expected CorrectCount 5, got %d", score.CorrectCount)
	}
}

func TestGetCalibrationScore_Mixed(t *testing.T) {
	calibration := NewCalibrationData()

	calibration, _ = RecordCalibration(calibration, "T001", "PASS", "PROCEED", "correct", "")
	calibration, _ = RecordCalibration(calibration, "T002", "PASS", "PROCEED", "correct", "")
	calibration, _ = RecordCalibration(calibration, "T003", "PASS", "PROCEED", "false_positive", "")
	calibration, _ = RecordCalibration(calibration, "T004", "FAIL", "BLOCK", "false_negative", "")

	score := GetCalibrationScore(calibration)
	if score.Score != 0.5 {
		t.Errorf("expected Score 0.5 (2/4), got %f", score.Score)
	}
	if score.TotalVerified != 4 {
		t.Errorf("expected TotalVerified 4, got %d", score.TotalVerified)
	}
	if score.CorrectCount != 2 {
		t.Errorf("expected CorrectCount 2, got %d", score.CorrectCount)
	}
	if score.FalsePositives != 1 {
		t.Errorf("expected FalsePositives 1, got %d", score.FalsePositives)
	}
	if score.FalseNegatives != 1 {
		t.Errorf("expected FalseNegatives 1, got %d", score.FalseNegatives)
	}
}

func TestGetCalibrationScore_Empty(t *testing.T) {
	calibration := NewCalibrationData()

	score := GetCalibrationScore(calibration)
	if score.Score != 1.0 {
		t.Errorf("expected Score 1.0 for empty calibration, got %f", score.Score)
	}
	if score.TotalVerified != 0 {
		t.Errorf("expected TotalVerified 0, got %d", score.TotalVerified)
	}
}

func TestGetCalibrationScore_Nil(t *testing.T) {
	score := GetCalibrationScore(nil)
	if score.Score != 1.0 {
		t.Errorf("expected Score 1.0 for nil calibration, got %f", score.Score)
	}
	if score.ComputedAt == "" {
		t.Error("expected ComputedAt to be set")
	}
}

