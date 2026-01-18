package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type StatusSummary struct {
	Phase         string         `json:"phase"`
	TotalTasks    int            `json:"total_tasks"`
	ByStatus      map[string]int `json:"by_status"`
	ActiveTasks   []string       `json:"active_tasks"`
	ReadyTasks    []string       `json:"ready_tasks"`
	FailedTasks   []string       `json:"failed_tasks"`
	CurrentPhase  int            `json:"current_phase"`
	PhaseProgress string         `json:"phase_progress"`
}

var phaseOrder = []string{
	"ingestion",
	"spec_review",
	"logical",
	"physical",
	"definition",
	"validation",
	"sequencing",
	"ready",
	"executing",
	"complete",
}

func GetStatus(state *State) StatusSummary {
	summary := StatusSummary{
		Phase:        state.Phase.Current,
		TotalTasks:   len(state.Tasks),
		ByStatus:     make(map[string]int),
		ActiveTasks:  []string{},
		ReadyTasks:   []string{},
		FailedTasks:  []string{},
		CurrentPhase: state.Execution.CurrentPhase,
	}

	for _, task := range state.Tasks {
		summary.ByStatus[task.Status]++
		if task.Status == "running" {
			summary.ActiveTasks = append(summary.ActiveTasks, task.ID)
		}
		if task.Status == "failed" {
			summary.FailedTasks = append(summary.FailedTasks, task.ID)
		}
	}

	ready := GetReadyTasks(state)
	for _, task := range ready {
		summary.ReadyTasks = append(summary.ReadyTasks, task.ID)
	}

	completed := summary.ByStatus["complete"] + summary.ByStatus["skipped"]
	if summary.TotalTasks > 0 {
		summary.PhaseProgress = fmt.Sprintf("%d/%d", completed, summary.TotalTasks)
	} else {
		summary.PhaseProgress = "0/0"
	}

	return summary
}

func AdvancePhase(sm *StateManager) (string, error) {
	state, err := sm.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load state: %w", err)
	}

	currentIdx := -1
	for i, p := range phaseOrder {
		if p == state.Phase.Current {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return "", fmt.Errorf("unknown current phase: %s", state.Phase.Current)
	}

	if currentIdx >= len(phaseOrder)-1 {
		return state.Phase.Current, fmt.Errorf("already at final phase: %s", state.Phase.Current)
	}

	if state.Phase.Current == "executing" {
		allDone := true
		for _, task := range state.Tasks {
			if task.Status != "complete" && task.Status != "skipped" {
				allDone = false
				break
			}
		}
		if !allDone {
			return state.Phase.Current, fmt.Errorf("cannot advance from executing: not all tasks are complete or skipped")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	previousPhase := state.Phase.Current
	nextPhase := phaseOrder[currentIdx+1]

	state.Phase.Completed = append(state.Phase.Completed, previousPhase)
	state.Phase.Current = nextPhase

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "phase_advanced",
		Details: map[string]interface{}{
			"from": previousPhase,
			"to":   nextPhase,
		},
	})

	if err := sm.Save(state); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	return nextPhase, nil
}

type ArtifactValidation struct {
	Path        string   `json:"path"`
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors,omitempty"`
	ValidatedAt string   `json:"validated_at"`
}

type SchemaProperty struct {
	Type       string                    `json:"type,omitempty"`
	Properties map[string]SchemaProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
	Items      *SchemaProperty           `json:"items,omitempty"`
}

type JSONSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]SchemaProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

func ValidateArtifact(planningDir, artifactPath, schemaPath string) (*ArtifactValidation, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	fullArtifactPath := filepath.Join(planningDir, artifactPath)
	artifactData, err := os.ReadFile(fullArtifactPath)
	if err != nil {
		return &ArtifactValidation{
			Path:        artifactPath,
			Valid:       false,
			Errors:      []string{fmt.Sprintf("failed to read artifact: %v", err)},
			ValidatedAt: now,
		}, nil
	}

	var artifact map[string]interface{}
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		return &ArtifactValidation{
			Path:        artifactPath,
			Valid:       false,
			Errors:      []string{fmt.Sprintf("invalid JSON: %v", err)},
			ValidatedAt: now,
		}, nil
	}

	if schemaPath == "" {
		return &ArtifactValidation{
			Path:        artifactPath,
			Valid:       true,
			ValidatedAt: now,
		}, nil
	}

	fullSchemaPath := filepath.Join(planningDir, schemaPath)
	schemaData, err := os.ReadFile(fullSchemaPath)
	if err != nil {
		return &ArtifactValidation{
			Path:        artifactPath,
			Valid:       false,
			Errors:      []string{fmt.Sprintf("failed to read schema: %v", err)},
			ValidatedAt: now,
		}, nil
	}

	var schema JSONSchema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return &ArtifactValidation{
			Path:        artifactPath,
			Valid:       false,
			Errors:      []string{fmt.Sprintf("invalid schema JSON: %v", err)},
			ValidatedAt: now,
		}, nil
	}

	errors := validateAgainstSchema(artifact, schema)
	return &ArtifactValidation{
		Path:        artifactPath,
		Valid:       len(errors) == 0,
		Errors:      errors,
		ValidatedAt: now,
	}, nil
}

func validateAgainstSchema(data map[string]interface{}, schema JSONSchema) []string {
	var errors []string

	for _, required := range schema.Required {
		if _, exists := data[required]; !exists {
			errors = append(errors, fmt.Sprintf("missing required field: %s", required))
		}
	}

	for key, prop := range schema.Properties {
		val, exists := data[key]
		if !exists {
			continue
		}

		if prop.Type != "" {
			if !checkType(val, prop.Type) {
				errors = append(errors, fmt.Sprintf("field '%s' has wrong type: expected %s", key, prop.Type))
			}
		}
	}

	return errors
}

func checkType(val interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		_, ok := val.(float64)
		return ok
	case "integer":
		f, ok := val.(float64)
		if !ok {
			return false
		}
		return f == float64(int(f))
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "array":
		_, ok := val.([]interface{})
		return ok
	case "object":
		_, ok := val.(map[string]interface{})
		return ok
	case "null":
		return val == nil
	}
	return true
}
