# T009: Verification and calibration

## Summary

Implements verification recording, rollback validation, and verifier calibration tracking for the tasker CLI. These functions enable tracking of task verification results, validating that rollbacks restore files correctly, and measuring verifier accuracy over time.

## Components

- `go/internal/validate/verification.go` - Core verification and calibration functions
- `go/internal/validate/verification_test.go` - Unit tests for all verification operations

## API / Interface

### Types

```go
type VerificationResult struct {
    TaskID         string
    Verdict        string                  // PASS, FAIL, CONDITIONAL
    Recommendation string                  // PROCEED, BLOCK
    Criteria       []VerificationCriterion
    Quality        *VerificationQuality
    Tests          *VerificationTests
    VerifiedAt     string
}

type RollbackData struct {
    TaskID        string
    PreparedAt    string
    TargetDir     string
    FileChecksums map[string]string
    FileExisted   map[string]bool
}

type RollbackValidation struct {
    Valid       bool
    Issues      []string
    ValidatedAt string
}

type CalibrationData struct {
    TotalVerified  int
    Correct        int
    FalsePositives []string
    FalseNegatives []string
    History        []CalibrationEntry
}

type CalibrationScore struct {
    Score          float64
    TotalVerified  int
    CorrectCount   int
    FalsePositives int
    FalseNegatives int
    ComputedAt     string
}
```

### Functions

```go
func RecordVerification(
    taskID string,
    verdict string,
    recommendation string,
    criteria []VerificationCriterion,
    quality *VerificationQuality,
    tests *VerificationTests,
) (*VerificationResult, error)

func PrepareRollback(taskID string, targetDir string, filesToModify []string) (*RollbackData, error)

func ValidateRollback(
    rollback *RollbackData,
    filesCreated []string,
    filesModified []string,
) *RollbackValidation

func NewCalibrationData() *CalibrationData

func RecordCalibration(
    calibration *CalibrationData,
    taskID string,
    verdict string,
    recommendation string,
    actualOutcome string,  // correct, false_positive, false_negative
    notes string,
) (*CalibrationData, error)

func GetCalibrationScore(calibration *CalibrationData) *CalibrationScore
```

## Behaviors Implemented

| ID | Name | Type | Description |
|----|------|------|-------------|
| B31 | RecordVerification | state | Records verification results with verdict, recommendation, and criteria |
| B32 | ValidateRollback | process | Validates that rollback restored files to original state using checksums |
| B33 | RecordCalibration | state | Records verifier calibration data (correct, false_positive, false_negative) |
| B34 | GetCalibrationScore | output | Computes and returns calibration accuracy metrics |

## Dependencies

- T007: DAG validation (same package, provides Task type and validation patterns)
- crypto/sha256: File checksumming for rollback validation

## Testing

```bash
cd go && go test ./internal/validate/... -v
```

Tests verify:
- RecordVerification validates verdicts (PASS, FAIL, CONDITIONAL) and recommendations (PROCEED, BLOCK)
- ValidateRollback detects undeleted created files, unrestored modified files, and checksum mismatches
- RecordCalibration tracks correct, false_positive, and false_negative outcomes
- GetCalibrationScore computes accuracy ratio correctly (correct / total)
