package state

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TaskDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Phase       int      `json:"phase"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Blocks      []string `json:"blocks,omitempty"`
	Behaviors   []string `json:"behaviors,omitempty"`
	Description string   `json:"description,omitempty"`
}

func InitDecomposition(planningDir, targetDir string) (*State, error) {
	statePath := filepath.Join(planningDir, "state.json")

	if _, err := os.Stat(statePath); err == nil {
		return nil, fmt.Errorf("state.json already exists at %s", statePath)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	state := &State{
		Version:   "2.0",
		Phase:     PhaseState{Current: "ingestion", Completed: []string{}},
		TargetDir: targetDir,
		CreatedAt: now,
		UpdatedAt: now,
		Artifacts: Artifacts{},
		Tasks:     make(map[string]Task),
		Execution: Execution{
			CurrentPhase:   0,
			ActiveTasks:    []string{},
			CompletedCount: 0,
			FailedCount:    0,
			TotalTokens:    0,
			TotalCostUSD:   0.0,
		},
		Events: []Event{
			{
				Timestamp: now,
				Type:      "decomposition_initialized",
				Details: map[string]interface{}{
					"target_dir": targetDir,
				},
			},
		},
	}

	if err := SaveState(statePath, state); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}

	return state, nil
}

func LoadTasks(planningDir string) (map[string]Task, error) {
	tasksDir := filepath.Join(planningDir, "tasks")

	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("tasks directory not found: %s", tasksDir)
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	tasks := make(map[string]Task)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		taskPath := filepath.Join(tasksDir, entry.Name())
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read task file %s: %w", entry.Name(), err)
		}

		var def TaskDefinition
		if err := json.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("failed to parse task file %s: %w", entry.Name(), err)
		}

		if def.ID == "" {
			return nil, fmt.Errorf("task file %s missing required field 'id'", entry.Name())
		}

		task := Task{
			ID:        def.ID,
			Name:      def.Name,
			Status:    "pending",
			Phase:     def.Phase,
			DependsOn: def.DependsOn,
			Blocks:    def.Blocks,
			File:      entry.Name(),
		}

		tasks[def.ID] = task
	}

	return tasks, nil
}

func GetReadyTasks(state *State) []Task {
	var ready []Task

	completedOrSkipped := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == "complete" || task.Status == "skipped" {
			completedOrSkipped[task.ID] = true
		}
	}

	for _, task := range state.Tasks {
		if task.Status != "pending" {
			continue
		}

		allDepsSatisfied := true
		for _, dep := range task.DependsOn {
			if !completedOrSkipped[dep] {
				allDepsSatisfied = false
				break
			}
		}

		if allDepsSatisfied {
			ready = append(ready, task)
		}
	}

	sort.Slice(ready, func(i, j int) bool {
		if ready[i].Phase != ready[j].Phase {
			return ready[i].Phase < ready[j].Phase
		}
		return ready[i].ID < ready[j].ID
	})

	return ready
}

func StartTask(sm *StateManager, taskID string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "pending" && task.Status != "ready" {
		return fmt.Errorf("task %s is %s, can only start pending or ready tasks", taskID, task.Status)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	task.Status = "running"
	task.StartedAt = now
	task.Attempts++
	state.Tasks[taskID] = task

	state.Execution.ActiveTasks = append(state.Execution.ActiveTasks, taskID)

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "task_started",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"attempt": task.Attempts,
		},
	})

	if err := sm.Save(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func CompleteTask(sm *StateManager, taskID string, filesCreated, filesModified []string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "running" {
		return fmt.Errorf("task %s is %s, can only complete running tasks", taskID, task.Status)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	task.Status = "complete"
	task.CompletedAt = now
	task.FilesCreated = filesCreated
	task.FilesModified = filesModified

	if task.StartedAt != "" {
		startTime, err := time.Parse(time.RFC3339Nano, task.StartedAt)
		if err == nil {
			endTime, err := time.Parse(time.RFC3339Nano, now)
			if err == nil {
				task.DurationSecs = endTime.Sub(startTime).Seconds()
			}
		}
	}

	state.Tasks[taskID] = task

	state.Execution.ActiveTasks = removeFromSlice(state.Execution.ActiveTasks, taskID)
	state.Execution.CompletedCount++

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "task_completed",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"files_created":    len(filesCreated),
			"files_modified":   len(filesModified),
			"duration_seconds": task.DurationSecs,
		},
	})

	if err := sm.Save(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func FailTask(sm *StateManager, taskID, errorMsg, category string, retryable bool) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "running" {
		return fmt.Errorf("task %s is %s, can only fail running tasks", taskID, task.Status)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	task.Status = "failed"
	task.CompletedAt = now
	task.Error = errorMsg
	task.Failure = &TaskFailure{
		Category:  category,
		Retryable: retryable,
	}

	if task.StartedAt != "" {
		startTime, err := time.Parse(time.RFC3339Nano, task.StartedAt)
		if err == nil {
			endTime, err := time.Parse(time.RFC3339Nano, now)
			if err == nil {
				task.DurationSecs = endTime.Sub(startTime).Seconds()
			}
		}
	}

	state.Tasks[taskID] = task

	state.Execution.ActiveTasks = removeFromSlice(state.Execution.ActiveTasks, taskID)
	state.Execution.FailedCount++

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "task_failed",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"error":     errorMsg,
			"category":  category,
			"retryable": retryable,
		},
	})

	if err := sm.Save(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func RetryTask(sm *StateManager, taskID string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "failed" {
		return fmt.Errorf("task %s is %s, can only retry failed tasks", taskID, task.Status)
	}

	if task.Failure != nil && !task.Failure.Retryable {
		return fmt.Errorf("task %s is not retryable", taskID)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	previousAttempts := task.Attempts
	task.Status = "pending"
	task.StartedAt = ""
	task.CompletedAt = ""
	task.Error = ""
	task.Failure = nil
	task.DurationSecs = 0
	task.FilesCreated = nil
	task.FilesModified = nil
	task.Verification = nil

	state.Tasks[taskID] = task

	state.Execution.FailedCount--
	if state.Execution.FailedCount < 0 {
		state.Execution.FailedCount = 0
	}

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "task_retried",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"previous_attempts": previousAttempts,
		},
	})

	if err := sm.Save(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func SkipTask(sm *StateManager, taskID, reason string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "pending" && task.Status != "ready" && task.Status != "blocked" {
		return fmt.Errorf("task %s is %s, can only skip pending, ready, or blocked tasks", taskID, task.Status)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	task.Status = "skipped"
	task.CompletedAt = now
	if reason != "" {
		task.Error = reason
	}

	state.Tasks[taskID] = task

	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "task_skipped",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"reason": reason,
		},
	})

	if err := sm.Save(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// CommitTask commits files changed by a completed task to git.
func CommitTask(sm *StateManager, taskID string) (string, error) {
	state, err := sm.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load state: %w", err)
	}

	task, exists := state.Tasks[taskID]
	if !exists {
		return "", fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "complete" {
		return "", fmt.Errorf("task %s is %s, can only commit complete tasks", taskID, task.Status)
	}

	allFiles := append(task.FilesCreated, task.FilesModified...)
	if len(allFiles) == 0 {
		return "", fmt.Errorf("no files to commit for task %s", taskID)
	}

	targetDir := state.TargetDir
	if targetDir == "" {
		targetDir = "."
	}

	// Stage files that exist
	stagedFiles := []string{}
	for _, f := range allFiles {
		filePath := filepath.Join(targetDir, f)
		if _, err := os.Stat(filePath); err == nil {
			cmd := exec.Command("git", "add", filePath)
			cmd.Dir = targetDir
			if output, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("git add failed for %s: %s", f, string(output))
			}
			stagedFiles = append(stagedFiles, f)
		}
	}

	if len(stagedFiles) == 0 {
		return "no files to commit (all files missing)", nil
	}

	// Build commit message
	commitMsg := fmt.Sprintf("%s: %s", taskID, task.Name)

	// Commit
	cmd := exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = targetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "nothing to commit") {
			return "no changes to commit", nil
		}
		return "", fmt.Errorf("git commit failed: %s", outputStr)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	state.Events = append(state.Events, Event{
		Timestamp: now,
		Type:      "task_committed",
		TaskID:    taskID,
		Details: map[string]interface{}{
			"files":      stagedFiles,
			"commit_msg": commitMsg,
		},
	})

	if err := sm.Save(state); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	return fmt.Sprintf("committed %d file(s)", len(stagedFiles)), nil
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
