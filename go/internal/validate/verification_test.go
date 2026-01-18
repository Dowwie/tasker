package validate

import (
	"os"
	"path/filepath"
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

func TestValidateRollback_Valid(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	originalContent := []byte("original content")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	rollback, err := PrepareRollback("T001", tmpDir, []string{"test.txt"})
	if err != nil {
		t.Fatalf("failed to prepare rollback: %v", err)
	}

	validation := ValidateRollback(rollback, []string{}, []string{"test.txt"})
	if !validation.Valid {
		t.Errorf("expected valid rollback, got issues: %v", validation.Issues)
	}
}

func TestValidateRollback_CreatedFileNotDeleted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	createdFile := filepath.Join(tmpDir, "created.txt")
	if err := os.WriteFile(createdFile, []byte("created"), 0644); err != nil {
		t.Fatalf("failed to write created file: %v", err)
	}

	rollback := &RollbackData{
		TaskID:        "T001",
		TargetDir:     tmpDir,
		FileChecksums: make(map[string]string),
		FileExisted:   make(map[string]bool),
	}

	validation := ValidateRollback(rollback, []string{"created.txt"}, []string{})
	if validation.Valid {
		t.Error("expected invalid rollback when created file not deleted")
	}
	if len(validation.Issues) == 0 {
		t.Error("expected issues")
	}

	found := false
	for _, issue := range validation.Issues {
		if issue == "created file not deleted: created.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'created file not deleted' issue, got: %v", validation.Issues)
	}
}

func TestValidateRollback_ModifiedFileNotRestored(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	originalContent := []byte("original content")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	rollback, err := PrepareRollback("T001", tmpDir, []string{"test.txt"})
	if err != nil {
		t.Fatalf("failed to prepare rollback: %v", err)
	}

	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	validation := ValidateRollback(rollback, []string{}, []string{"test.txt"})
	if validation.Valid {
		t.Error("expected invalid rollback when file not restored")
	}

	found := false
	for _, issue := range validation.Issues {
		if len(issue) > 30 && issue[:30] == "file not restored to original:" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'file not restored to original' issue, got: %v", validation.Issues)
	}
}

func TestValidateRollback_FileShoudNotExist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rollback := &RollbackData{
		TaskID:        "T001",
		TargetDir:     tmpDir,
		FileChecksums: map[string]string{"new.txt": ""},
		FileExisted:   map[string]bool{"new.txt": false},
	}

	newFile := filepath.Join(tmpDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("should not exist"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	validation := ValidateRollback(rollback, []string{}, []string{"new.txt"})
	if validation.Valid {
		t.Error("expected invalid rollback when file should not exist")
	}

	found := false
	for _, issue := range validation.Issues {
		if issue == "file should not exist after rollback: new.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'file should not exist after rollback' issue, got: %v", validation.Issues)
	}
}

func TestValidateRollback_NilRollbackData(t *testing.T) {
	validation := ValidateRollback(nil, []string{}, []string{})
	if validation.Valid {
		t.Error("expected invalid rollback for nil data")
	}
	if len(validation.Issues) == 0 || validation.Issues[0] != "no rollback data provided" {
		t.Errorf("expected 'no rollback data provided' issue, got: %v", validation.Issues)
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

func TestPrepareRollback_NewFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rollback, err := PrepareRollback("T001", tmpDir, []string{"new.txt"})
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if rollback.FileExisted["new.txt"] {
		t.Error("expected FileExisted[new.txt] to be false")
	}
	if rollback.FileChecksums["new.txt"] != "" {
		t.Errorf("expected empty checksum for new file, got %s", rollback.FileChecksums["new.txt"])
	}
}

func TestPrepareRollback_ExistingFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	rollback, err := PrepareRollback("T001", tmpDir, []string{"existing.txt"})
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
		return
	}

	if !rollback.FileExisted["existing.txt"] {
		t.Error("expected FileExisted[existing.txt] to be true")
	}
	if rollback.FileChecksums["existing.txt"] == "" {
		t.Error("expected non-empty checksum for existing file")
	}
}

func TestPrepareRollback_EmptyTaskID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = PrepareRollback("", tmpDir, []string{"test.txt"})
	if err == nil {
		t.Error("expected error for empty task ID")
		return
	}
}

func TestPrepareRollback_EmptyTargetDir(t *testing.T) {
	_, err := PrepareRollback("T001", "", []string{"test.txt"})
	if err == nil {
		t.Error("expected error for empty target dir")
		return
	}
}
