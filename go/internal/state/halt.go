package state

import (
	"fmt"
	"time"
)

type HaltInfo struct {
	Requested   bool   `json:"requested"`
	Reason      string `json:"reason,omitempty"`
	RequestedAt string `json:"requested_at,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
	HaltedAt    string `json:"halted_at,omitempty"`
	ActiveTask  string `json:"active_task,omitempty"`
}

type HaltStatus struct {
	Halted      bool   `json:"halted"`
	Reason      string `json:"reason,omitempty"`
	RequestedAt string `json:"requested_at,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
}

func RequestHalt(path string, reason string, requestedBy string) error {
	state, err := LoadState(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if state.Halt == nil {
		state.Halt = &HaltInfo{}
	}

	state.Halt.Requested = true
	state.Halt.Reason = reason
	state.Halt.RequestedAt = time.Now().UTC().Format(time.RFC3339Nano)
	state.Halt.RequestedBy = requestedBy

	state.Events = append(state.Events, Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      "halt_requested",
		Details: map[string]interface{}{
			"reason":       reason,
			"requested_by": requestedBy,
		},
	})

	if err := SaveState(path, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func CheckHalt(path string) (bool, error) {
	state, err := LoadState(path)
	if err != nil {
		return false, fmt.Errorf("failed to load state: %w", err)
	}

	if state.Halt == nil {
		return false, nil
	}

	return state.Halt.Requested, nil
}

func ConfirmHalt(path string) ([]string, error) {
	state, err := LoadState(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if state.Halt == nil || !state.Halt.Requested {
		return nil, fmt.Errorf("no halt was requested")
	}

	state.Halt.HaltedAt = time.Now().UTC().Format(time.RFC3339Nano)

	var runningTasks []string
	for id, task := range state.Tasks {
		if task.Status == "running" {
			runningTasks = append(runningTasks, id)
		}
	}

	if len(runningTasks) > 0 {
		state.Halt.ActiveTask = runningTasks[0]
	}

	state.Events = append(state.Events, Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      "halt_confirmed",
		Details: map[string]interface{}{
			"running_tasks": runningTasks,
		},
	})

	if err := SaveState(path, state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	return runningTasks, nil
}

func ResumeExecution(path string) error {
	state, err := LoadState(path)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	previousHalt := state.Halt
	state.Halt = &HaltInfo{
		Requested: false,
	}

	details := map[string]interface{}{}
	if previousHalt != nil && previousHalt.Reason != "" {
		details["previous_reason"] = previousHalt.Reason
	}

	state.Events = append(state.Events, Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      "execution_resumed",
		Details:   details,
	})

	if err := SaveState(path, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func GetHaltStatus(path string) (*HaltStatus, error) {
	state, err := LoadState(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	status := &HaltStatus{
		Halted: false,
	}

	if state.Halt != nil && state.Halt.Requested {
		status.Halted = true
		status.Reason = state.Halt.Reason
		status.RequestedAt = state.Halt.RequestedAt
		status.RequestedBy = state.Halt.RequestedBy
	}

	return status, nil
}

func (sm *StateManager) RequestHalt(reason string, requestedBy string) error {
	return RequestHalt(sm.path, reason, requestedBy)
}

func (sm *StateManager) CheckHalt() (bool, error) {
	return CheckHalt(sm.path)
}

func (sm *StateManager) ResumeExecution() error {
	return ResumeExecution(sm.path)
}

func (sm *StateManager) ConfirmHalt() ([]string, error) {
	return ConfirmHalt(sm.path)
}

func (sm *StateManager) GetHaltStatus() (*HaltStatus, error) {
	return GetHaltStatus(sm.path)
}
