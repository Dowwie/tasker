package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CheckpointTask struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type CheckpointTasks struct {
	Pending   []string `json:"pending"`
	Completed []string `json:"completed"`
	Failed    []string `json:"failed"`
}

type Checkpoint struct {
	Version   string          `json:"version"`
	BatchID   string          `json:"batch_id"`
	SpawnedAt string          `json:"spawned_at"`
	Status    string          `json:"status"`
	Tasks     CheckpointTasks `json:"tasks"`
	UpdatedAt string          `json:"updated_at,omitempty"`
}

func CheckpointPath(planningDir string) string {
	return filepath.Join(planningDir, "orchestrator-checkpoint.json")
}

func LoadCheckpoint(planningDir string) (*Checkpoint, error) {
	path := CheckpointPath(planningDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	return &cp, nil
}

func SaveCheckpoint(planningDir string, cp *Checkpoint) error {
	cp.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize checkpoint: %w", err)
	}

	path := CheckpointPath(planningDir)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return nil
}

func CreateCheckpoint(planningDir string, taskIDs []string) (*Checkpoint, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	batchID := fmt.Sprintf("batch-%s", now)

	cp := &Checkpoint{
		Version:   "1.0",
		BatchID:   batchID,
		SpawnedAt: now,
		Status:    "active",
		Tasks: CheckpointTasks{
			Pending:   taskIDs,
			Completed: []string{},
			Failed:    []string{},
		},
	}

	if err := SaveCheckpoint(planningDir, cp); err != nil {
		return nil, err
	}

	return cp, nil
}

func UpdateCheckpointTask(planningDir, taskID, status string) error {
	cp, err := LoadCheckpoint(planningDir)
	if err != nil {
		return err
	}
	if cp == nil {
		return fmt.Errorf("no active checkpoint")
	}

	cp.Tasks.Pending = removeString(cp.Tasks.Pending, taskID)
	cp.Tasks.Completed = removeString(cp.Tasks.Completed, taskID)
	cp.Tasks.Failed = removeString(cp.Tasks.Failed, taskID)

	switch status {
	case "success", "complete":
		cp.Tasks.Completed = append(cp.Tasks.Completed, taskID)
	case "failed", "failure":
		cp.Tasks.Failed = append(cp.Tasks.Failed, taskID)
	default:
		cp.Tasks.Pending = append(cp.Tasks.Pending, taskID)
	}

	return SaveCheckpoint(planningDir, cp)
}

func CompleteCheckpoint(planningDir string) error {
	cp, err := LoadCheckpoint(planningDir)
	if err != nil {
		return err
	}
	if cp == nil {
		return fmt.Errorf("no active checkpoint")
	}

	cp.Status = "complete"
	return SaveCheckpoint(planningDir, cp)
}

func ClearCheckpoint(planningDir string) error {
	path := CheckpointPath(planningDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove checkpoint: %w", err)
	}
	return nil
}

type RecoveryInfo struct {
	OrphanedTasks []string `json:"orphaned_tasks"`
	RecoveredTasks []RecoveredTask `json:"recovered_tasks"`
}

type RecoveredTask struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func RecoverCheckpoint(planningDir string) (*RecoveryInfo, error) {
	cp, err := LoadCheckpoint(planningDir)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, fmt.Errorf("no checkpoint to recover")
	}

	info := &RecoveryInfo{
		OrphanedTasks: []string{},
		RecoveredTasks: []RecoveredTask{},
	}

	bundlesDir := filepath.Join(planningDir, "bundles")

	for _, taskID := range cp.Tasks.Pending {
		resultPath := filepath.Join(bundlesDir, fmt.Sprintf("%s-result.json", taskID))
		if _, err := os.Stat(resultPath); err == nil {
			data, err := os.ReadFile(resultPath)
			if err != nil {
				continue
			}

			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				continue
			}

			status, _ := result["status"].(string)
			info.RecoveredTasks = append(info.RecoveredTasks, RecoveredTask{
				TaskID: taskID,
				Status: status,
			})

			if err := UpdateCheckpointTask(planningDir, taskID, status); err != nil {
				continue
			}
		} else {
			statePath := filepath.Join(planningDir, "state.json")
			state, err := LoadState(statePath)
			if err != nil {
				continue
			}

			if task, ok := state.Tasks[taskID]; ok && task.Status == "running" {
				info.OrphanedTasks = append(info.OrphanedTasks, taskID)
			}
		}
	}

	return info, nil
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
