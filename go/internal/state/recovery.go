package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RecoveryResult struct {
	Recovered     bool     `json:"recovered"`
	TasksRecovered int     `json:"tasks_recovered"`
	DataLost      []string `json:"data_lost,omitempty"`
	BackupPath    string   `json:"backup_path,omitempty"`
	Error         string   `json:"error,omitempty"`
}

func RecoverState(planningDir string) (*State, *RecoveryResult, error) {
	statePath := filepath.Join(planningDir, "state.json")
	result := &RecoveryResult{}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("state file not found: %s", statePath)
		}
		return nil, nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err == nil {
		errs := ValidateState(&state)
		if len(errs) == 0 {
			result.Recovered = false
			return &state, result, nil
		}
	}

	backupPath := statePath + ".corrupted." + time.Now().Format("20060102-150405")
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return nil, nil, fmt.Errorf("failed to create backup: %w", err)
	}
	result.BackupPath = backupPath

	recovered, dataLost := attemptPartialRecovery(data, planningDir)
	result.TasksRecovered = len(recovered.Tasks)
	result.DataLost = dataLost
	result.Recovered = true

	if err := SaveState(statePath, recovered); err != nil {
		return nil, nil, fmt.Errorf("failed to save recovered state: %w", err)
	}

	return recovered, result, nil
}

func attemptPartialRecovery(data []byte, planningDir string) (*State, []string) {
	var dataLost []string
	now := time.Now().UTC().Format(time.RFC3339Nano)

	recovered := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion", Completed: []string{}},
		TargetDir: "",
		CreatedAt: now,
		Tasks:     make(map[string]Task),
		Artifacts: Artifacts{},
		Execution: Execution{
			CurrentPhase:   0,
			ActiveTasks:    []string{},
			CompletedCount: 0,
			FailedCount:    0,
		},
		Events: []Event{
			{
				Timestamp: now,
				Type:      "state_recovered",
				Details: map[string]interface{}{
					"reason": "corrupted state file",
				},
			},
		},
	}

	var partial map[string]interface{}
	if err := json.Unmarshal(data, &partial); err == nil {
		if version, ok := partial["version"].(string); ok && version == "2.0" {
			recovered.Version = version
		} else {
			dataLost = append(dataLost, "version (invalid or missing)")
		}

		if targetDir, ok := partial["target_dir"].(string); ok && targetDir != "" {
			recovered.TargetDir = targetDir
		} else {
			dataLost = append(dataLost, "target_dir")
		}

		if createdAt, ok := partial["created_at"].(string); ok && createdAt != "" {
			recovered.CreatedAt = createdAt
		} else {
			dataLost = append(dataLost, "created_at (using current time)")
		}

		if phase, ok := partial["phase"].(map[string]interface{}); ok {
			if current, ok := phase["current"].(string); ok && isValidPhase(current) {
				recovered.Phase.Current = current
			} else {
				dataLost = append(dataLost, "phase.current (invalid)")
			}
			if completed, ok := phase["completed"].([]interface{}); ok {
				for _, c := range completed {
					if cs, ok := c.(string); ok {
						recovered.Phase.Completed = append(recovered.Phase.Completed, cs)
					}
				}
			}
		} else {
			dataLost = append(dataLost, "phase")
		}

		if tasks, ok := partial["tasks"].(map[string]interface{}); ok {
			for id, taskData := range tasks {
				if taskMap, ok := taskData.(map[string]interface{}); ok {
					task, lost := recoverTask(id, taskMap)
					if task != nil {
						recovered.Tasks[id] = *task
					}
					dataLost = append(dataLost, lost...)
				}
			}
		} else {
			dataLost = append(dataLost, "tasks")
		}

		if execution, ok := partial["execution"].(map[string]interface{}); ok {
			if currentPhase, ok := execution["current_phase"].(float64); ok {
				recovered.Execution.CurrentPhase = int(currentPhase)
			}
			if completedCount, ok := execution["completed_count"].(float64); ok {
				recovered.Execution.CompletedCount = int(completedCount)
			}
			if failedCount, ok := execution["failed_count"].(float64); ok {
				recovered.Execution.FailedCount = int(failedCount)
			}
		} else {
			dataLost = append(dataLost, "execution")
		}
	} else {
		dataLost = append(dataLost, "entire state (JSON parse failed)")
	}

	tasks, err := LoadTasks(planningDir)
	if err == nil {
		for id, task := range tasks {
			if _, exists := recovered.Tasks[id]; !exists {
				recovered.Tasks[id] = task
			}
		}
	}

	recalculateExecution(recovered)

	return recovered, dataLost
}

func isValidPhase(phase string) bool {
	validPhases := map[string]bool{
		"ingestion": true, "logical": true, "physical": true,
		"definition": true, "sequencing": true, "ready": true,
		"executing": true, "complete": true, "spec_review": true,
		"validation": true,
	}
	return validPhases[phase]
}

func recoverTask(id string, data map[string]interface{}) (*Task, []string) {
	var lost []string

	task := &Task{
		ID:     id,
		Status: "pending",
		Phase:  0,
	}

	if name, ok := data["name"].(string); ok {
		task.Name = name
	}

	if status, ok := data["status"].(string); ok && isValidStatus(status) {
		task.Status = status
	} else if _, exists := data["status"]; exists {
		lost = append(lost, fmt.Sprintf("task %s: invalid status", id))
	}

	if phase, ok := data["phase"].(float64); ok {
		task.Phase = int(phase)
	}

	if dependsOn, ok := data["depends_on"].([]interface{}); ok {
		for _, dep := range dependsOn {
			if ds, ok := dep.(string); ok {
				task.DependsOn = append(task.DependsOn, ds)
			}
		}
	}

	if blocks, ok := data["blocks"].([]interface{}); ok {
		for _, b := range blocks {
			if bs, ok := b.(string); ok {
				task.Blocks = append(task.Blocks, bs)
			}
		}
	}

	if startedAt, ok := data["started_at"].(string); ok {
		task.StartedAt = startedAt
	}

	if completedAt, ok := data["completed_at"].(string); ok {
		task.CompletedAt = completedAt
	}

	if errorMsg, ok := data["error"].(string); ok {
		task.Error = errorMsg
	}

	if attempts, ok := data["attempts"].(float64); ok {
		task.Attempts = int(attempts)
	}

	if filesCreated, ok := data["files_created"].([]interface{}); ok {
		for _, f := range filesCreated {
			if fs, ok := f.(string); ok {
				task.FilesCreated = append(task.FilesCreated, fs)
			}
		}
	}

	if filesModified, ok := data["files_modified"].([]interface{}); ok {
		for _, f := range filesModified {
			if fs, ok := f.(string); ok {
				task.FilesModified = append(task.FilesModified, fs)
			}
		}
	}

	return task, lost
}

func isValidStatus(status string) bool {
	validStatuses := map[string]bool{
		"pending": true, "ready": true, "running": true,
		"complete": true, "failed": true, "blocked": true, "skipped": true,
	}
	return validStatuses[status]
}

func recalculateExecution(state *State) {
	state.Execution.CompletedCount = 0
	state.Execution.FailedCount = 0
	state.Execution.ActiveTasks = []string{}

	for _, task := range state.Tasks {
		switch task.Status {
		case "complete", "skipped":
			state.Execution.CompletedCount++
		case "failed":
			state.Execution.FailedCount++
		case "running":
			state.Execution.ActiveTasks = append(state.Execution.ActiveTasks, task.ID)
		}
	}
}
