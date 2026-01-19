# T009: Verification and calibration

## Summary

Implements verification recording and verifier calibration tracking for the tasker CLI. These functions enable tracking of task verification results and measuring verifier accuracy over time.

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
| B33 | RecordCalibration | state | Records verifier calibration data (correct, false_positive, false_negative) |
| B34 | GetCalibrationScore | output | Computes and returns calibration accuracy metrics |

## Dependencies

- T007: DAG validation (same package, provides Task type and validation patterns)

## Testing

```bash
cd go && go test ./internal/validate/... -v
```

Tests verify:
- RecordVerification validates verdicts (PASS, FAIL, CONDITIONAL) and recommendations (PROCEED, BLOCK)
- RecordCalibration tracks correct, false_positive, and false_negative outcomes
- GetCalibrationScore computes accuracy ratio correctly (correct / total)
