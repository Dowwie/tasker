package validate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
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

type RollbackData struct {
	TaskID        string            `json:"task_id"`
	PreparedAt    string            `json:"prepared_at"`
	TargetDir     string            `json:"target_dir"`
	FileChecksums map[string]string `json:"file_checksums"`
	FileExisted   map[string]bool   `json:"file_existed"`
}

type RollbackValidation struct {
	Valid       bool     `json:"valid"`
	Issues      []string `json:"issues,omitempty"`
	ValidatedAt string   `json:"validated_at"`
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

func PrepareRollback(taskID string, targetDir string, filesToModify []string) (*RollbackData, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	if targetDir == "" {
		return nil, fmt.Errorf("target directory is required")
	}

	rollback := &RollbackData{
		TaskID:        taskID,
		PreparedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		TargetDir:     targetDir,
		FileChecksums: make(map[string]string),
		FileExisted:   make(map[string]bool),
	}

	for _, filePath := range filesToModify {
		fullPath := filepath.Join(targetDir, filePath)

		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				rollback.FileChecksums[filePath] = ""
				rollback.FileExisted[filePath] = false
				continue
			}
			return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("path is a directory, not a file: %s", filePath)
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		hash := sha256.Sum256(data)
		rollback.FileChecksums[filePath] = hex.EncodeToString(hash[:])
		rollback.FileExisted[filePath] = true
	}

	return rollback, nil
}

func ValidateRollback(
	rollback *RollbackData,
	filesCreated []string,
	filesModified []string,
) *RollbackValidation {
	validation := &RollbackValidation{
		Valid:       true,
		Issues:      []string{},
		ValidatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	if rollback == nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "no rollback data provided")
		return validation
	}

	targetDir := rollback.TargetDir
	if targetDir == "" {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "rollback data missing target directory")
		return validation
	}

	for _, filePath := range filesCreated {
		fullPath := filepath.Join(targetDir, filePath)
		if _, err := os.Stat(fullPath); err == nil {
			validation.Valid = false
			validation.Issues = append(validation.Issues,
				fmt.Sprintf("created file not deleted: %s", filePath))
		}
	}

	for _, filePath := range filesModified {
		fullPath := filepath.Join(targetDir, filePath)
		originalChecksum := rollback.FileChecksums[filePath]
		existed := rollback.FileExisted[filePath]

		if !existed {
			if _, err := os.Stat(fullPath); err == nil {
				validation.Valid = false
				validation.Issues = append(validation.Issues,
					fmt.Sprintf("file should not exist after rollback: %s", filePath))
			}
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				validation.Valid = false
				validation.Issues = append(validation.Issues,
					fmt.Sprintf("file should exist after rollback: %s", filePath))
			} else {
				validation.Valid = false
				validation.Issues = append(validation.Issues,
					fmt.Sprintf("failed to check file %s: %s", filePath, err.Error()))
			}
			continue
		}

		if info.IsDir() {
			validation.Valid = false
			validation.Issues = append(validation.Issues,
				fmt.Sprintf("path is a directory after rollback: %s", filePath))
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			validation.Valid = false
			validation.Issues = append(validation.Issues,
				fmt.Sprintf("failed to read file %s: %s", filePath, err.Error()))
			continue
		}

		hash := sha256.Sum256(data)
		currentChecksum := hex.EncodeToString(hash[:])

		if currentChecksum != originalChecksum {
			validation.Valid = false
			validation.Issues = append(validation.Issues,
				fmt.Sprintf("file not restored to original: %s (expected %s..., got %s...)",
					filePath, originalChecksum[:8], currentChecksum[:8]))
		}
	}

	return validation
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
