package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type PhaseState struct {
	Current   string   `json:"current"`
	Completed []string `json:"completed"`
}

type ArtifactRef struct {
	Path        string  `json:"path,omitempty"`
	Checksum    string  `json:"checksum,omitempty"`
	Valid       bool    `json:"valid,omitempty"`
	ValidatedAt string  `json:"validated_at,omitempty"`
	Error       *string `json:"error,omitempty"`
}

type SpecCoverage struct {
	Ratio     float64 `json:"ratio"`
	Passed    bool    `json:"passed"`
	Threshold float64 `json:"threshold"`
	Timestamp string  `json:"timestamp,omitempty"`
}

type ValidationViolation struct {
	TaskID    string `json:"task_id,omitempty"`
	Behavior  string `json:"behavior,omitempty"`
	Evidence  string `json:"evidence,omitempty"`
	MissingDep string `json:"missing_dependency,omitempty"`
	CriterionIndex int `json:"criterion_index,omitempty"`
	Issue string `json:"issue,omitempty"`
}

type ValidationResult struct {
	Passed     bool                  `json:"passed"`
	Violations []ValidationViolation `json:"violations,omitempty"`
}

type RefactorOverride struct {
	TaskID     string   `json:"task_id"`
	Supersedes []string `json:"supersedes"`
	Directive  string   `json:"directive"`
}

type ValidationResults struct {
	SpecCoverage       *SpecCoverage      `json:"spec_coverage,omitempty"`
	PhaseLeakage       *ValidationResult  `json:"phase_leakage,omitempty"`
	DependencyExistence *ValidationResult `json:"dependency_existence,omitempty"`
	AcceptanceCriteria *ValidationResult  `json:"acceptance_criteria,omitempty"`
	RefactorOverrides  []RefactorOverride `json:"refactor_overrides,omitempty"`
	ValidatedAt        string             `json:"validated_at,omitempty"`
}

type TaskValidation struct {
	Verdict     string   `json:"verdict,omitempty"`
	Valid       bool     `json:"valid,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Issues      []string `json:"issues,omitempty"`
	ValidatedAt string   `json:"validated_at,omitempty"`
	Error       *string  `json:"error,omitempty"`
}

type Artifacts struct {
	CapabilityMap     *ArtifactRef       `json:"capability_map,omitempty"`
	PhysicalMap       *ArtifactRef       `json:"physical_map,omitempty"`
	DependencyGraph   *ArtifactRef       `json:"dependency_graph,omitempty"`
	ValidationResults *ValidationResults `json:"validation_results,omitempty"`
	TaskValidation    *TaskValidation    `json:"task_validation,omitempty"`
}

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

type TaskVerification struct {
	Verdict        string                  `json:"verdict,omitempty"`
	Recommendation string                  `json:"recommendation,omitempty"`
	Criteria       []VerificationCriterion `json:"criteria,omitempty"`
	Quality        *VerificationQuality    `json:"quality,omitempty"`
	Tests          *VerificationTests      `json:"tests,omitempty"`
	VerifiedAt     string                  `json:"verified_at,omitempty"`
}

type TaskFailure struct {
	Category    string `json:"category,omitempty"`
	Subcategory string `json:"subcategory,omitempty"`
	Retryable   bool   `json:"retryable"`
}

type Task struct {
	ID            string            `json:"id"`
	Name          string            `json:"name,omitempty"`
	Status        string            `json:"status"`
	Phase         int               `json:"phase"`
	DependsOn     []string          `json:"depends_on,omitempty"`
	Blocks        []string          `json:"blocks,omitempty"`
	File          string            `json:"file,omitempty"`
	StartedAt     string            `json:"started_at,omitempty"`
	CompletedAt   string            `json:"completed_at,omitempty"`
	Error         string            `json:"error,omitempty"`
	Failure       *TaskFailure      `json:"failure,omitempty"`
	FilesCreated  []string          `json:"files_created,omitempty"`
	FilesModified []string          `json:"files_modified,omitempty"`
	Attempts      int               `json:"attempts,omitempty"`
	DurationSecs  float64           `json:"duration_seconds,omitempty"`
	Verification  *TaskVerification `json:"verification,omitempty"`
}

type Execution struct {
	CurrentPhase   int      `json:"current_phase"`
	ActiveTasks    []string `json:"active_tasks,omitempty"`
	CompletedCount int      `json:"completed_count"`
	FailedCount    int      `json:"failed_count"`
	TotalTokens    int      `json:"total_tokens"`
	TotalCostUSD   float64  `json:"total_cost_usd"`
}

type Event struct {
	Timestamp string                 `json:"timestamp"`
	Type      string                 `json:"type"`
	TaskID    string                 `json:"task_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type State struct {
	Version   string          `json:"version"`
	Phase     PhaseState      `json:"phase"`
	TargetDir string          `json:"target_dir"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	Artifacts Artifacts       `json:"artifacts"`
	Tasks     map[string]Task `json:"tasks"`
	Execution Execution       `json:"execution"`
	Halt      *HaltInfo       `json:"halt,omitempty"`
	Events    []Event         `json:"events,omitempty"`
}

type StateManager struct {
	path string
	dir  string
}

func NewStateManager(planningDir string) *StateManager {
	return &StateManager{
		path: filepath.Join(planningDir, "state.json"),
		dir:  planningDir,
	}
}

func (sm *StateManager) Path() string {
	return sm.path
}

func LoadState(path string) (*State, error) {
	lock, err := acquireFileLock(path, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire read lock: %w", err)
	}
	defer lock.Release()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("state file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	return &state, nil
}

func SaveState(path string, state *State) error {
	lock, err := acquireFileLock(path, true)
	if err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	defer lock.Release()

	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func ValidateState(state *State) []error {
	var errs []error

	if state.Version != "2.0" {
		errs = append(errs, fmt.Errorf("invalid version: expected '2.0', got '%s'", state.Version))
	}

	validPhases := map[string]bool{
		"ingestion": true, "logical": true, "physical": true,
		"definition": true, "sequencing": true, "ready": true,
		"executing": true, "complete": true, "spec_review": true,
		"validation": true,
	}
	if !validPhases[state.Phase.Current] {
		errs = append(errs, fmt.Errorf("invalid phase: '%s'", state.Phase.Current))
	}

	if state.TargetDir == "" {
		errs = append(errs, fmt.Errorf("target_dir is required"))
	}

	if state.CreatedAt == "" {
		errs = append(errs, fmt.Errorf("created_at is required"))
	}

	validStatuses := map[string]bool{
		"pending": true, "ready": true, "running": true,
		"complete": true, "failed": true, "blocked": true, "skipped": true,
	}
	for id, task := range state.Tasks {
		if task.ID == "" {
			errs = append(errs, fmt.Errorf("task %s: id is required", id))
		}
		if !validStatuses[task.Status] {
			errs = append(errs, fmt.Errorf("task %s: invalid status '%s'", id, task.Status))
		}
	}

	return errs
}

func (sm *StateManager) Load() (*State, error) {
	return LoadState(sm.path)
}

func (sm *StateManager) Save(state *State) error {
	return SaveState(sm.path, state)
}

func (sm *StateManager) Validate(state *State) []error {
	return ValidateState(state)
}
