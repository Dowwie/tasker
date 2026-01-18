package fsm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/errors"
)

// ValidationIssue represents a single validation failure with invariant reference.
type ValidationIssue struct {
	Invariant string            `json:"invariant"`
	Message   string            `json:"message"`
	Context   map[string]string `json:"context,omitempty"`
}

// ValidationWarning represents a non-fatal validation concern.
type ValidationWarning struct {
	Message string            `json:"message"`
	Context map[string]string `json:"context,omitempty"`
}

// ValidationResult contains the outcome of FSM validation.
type ValidationResult struct {
	Passed    bool                `json:"passed"`
	Issues    []ValidationIssue   `json:"issues"`
	Warnings  []ValidationWarning `json:"warnings"`
}

// NewValidationResult creates an empty validation result that starts as passed.
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Passed:   true,
		Issues:   make([]ValidationIssue, 0),
		Warnings: make([]ValidationWarning, 0),
	}
}

// AddIssue records a validation failure and marks the result as not passed.
func (r *ValidationResult) AddIssue(invariant, message string, context map[string]string) {
	r.Passed = false
	r.Issues = append(r.Issues, ValidationIssue{
		Invariant: invariant,
		Message:   message,
		Context:   context,
	})
}

// AddWarning records a non-fatal validation concern.
func (r *ValidationResult) AddWarning(message string, context map[string]string) {
	r.Warnings = append(r.Warnings, ValidationWarning{
		Message: message,
		Context: context,
	})
}

// CoverageResult contains transition coverage statistics.
type CoverageResult struct {
	TotalTransitions   int     `json:"total_transitions"`
	CoveredTransitions int     `json:"covered_transitions"`
	CoveragePercent    float64 `json:"coverage_percent"`
	UncoveredIDs       []string `json:"uncovered_ids,omitempty"`
}

// TaskCoverageResult contains task-based FSM coverage validation results.
type TaskCoverageResult struct {
	Passed               bool            `json:"passed"`
	SteelThreadCoverage  *CoverageResult `json:"steel_thread_coverage,omitempty"`
	NonSteelCoverage     *CoverageResult `json:"non_steel_coverage,omitempty"`
	Issues               []ValidationIssue `json:"issues"`
}

// ValidateInvariants validates I1-I5 FSM invariants for a complete FSM directory.
// It checks:
//   - I1: Steel Thread FSM mandatory (primary_machine exists and has level steel_thread)
//   - I3: Completeness (initial state, terminals, no dead ends, reachability)
//   - I4: Guard-Invariant linkage
func ValidateInvariants(fsmDir string) (*ValidationResult, error) {
	result := NewValidationResult()

	indexPath := filepath.Join(fsmDir, "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, errors.IOReadFailed(indexPath, err)
	}

	var index IndexFile
	if err := json.Unmarshal(indexData, &index); err != nil {
		return nil, errors.Internal("failed to parse index.json", err)
	}

	validateI1SteelThread(&index, result)

	machineMap := make(map[string]MachineEntry)
	for _, m := range index.Machines {
		machineMap[m.ID] = m
	}

	for _, machine := range index.Machines {
		statesPath := filepath.Join(fsmDir, machine.Files["states"])
		transitionsPath := filepath.Join(fsmDir, machine.Files["transitions"])

		statesData, err := os.ReadFile(statesPath)
		if err != nil {
			result.AddIssue("I3", fmt.Sprintf("Cannot load states from %s", statesPath), nil)
			continue
		}

		transitionsData, err := os.ReadFile(transitionsPath)
		if err != nil {
			result.AddIssue("I3", fmt.Sprintf("Cannot load transitions from %s", transitionsPath), nil)
			continue
		}

		var states StatesFile
		var transitions TransitionsFile
		if err := json.Unmarshal(statesData, &states); err != nil {
			result.AddIssue("I3", fmt.Sprintf("Cannot parse states file %s", statesPath), nil)
			continue
		}
		if err := json.Unmarshal(transitionsData, &transitions); err != nil {
			result.AddIssue("I3", fmt.Sprintf("Cannot parse transitions file %s", transitionsPath), nil)
			continue
		}

		validateI3Completeness(&states, &transitions, result)
		validateI4GuardLinkage(&transitions, result)
	}

	return result, nil
}

// validateI1SteelThread checks that a steel thread machine exists and is primary.
func validateI1SteelThread(index *IndexFile, result *ValidationResult) {
	if index.PrimaryMachine == "" {
		result.AddIssue("I1", "No primary_machine defined in index", nil)
		return
	}

	var primaryMachine *MachineEntry
	for i := range index.Machines {
		if index.Machines[i].ID == index.PrimaryMachine {
			primaryMachine = &index.Machines[i]
			break
		}
	}

	if primaryMachine == nil {
		result.AddIssue("I1", fmt.Sprintf("primary_machine '%s' not found in machines list", index.PrimaryMachine), nil)
		return
	}

	if primaryMachine.Level != "steel_thread" {
		result.AddIssue("I1", fmt.Sprintf("primary_machine '%s' has level '%s', expected 'steel_thread'", index.PrimaryMachine, primaryMachine.Level), map[string]string{
			"machine_id": primaryMachine.ID,
			"level":      primaryMachine.Level,
		})
	}
}

// validateI3Completeness checks model completeness constraints.
func validateI3Completeness(states *StatesFile, transitions *TransitionsFile, result *ValidationResult) {
	machineID := states.MachineID

	stateIDs := make(map[string]bool)
	stateTypes := make(map[string]string)
	for _, s := range states.States {
		stateIDs[s.ID] = true
		stateTypes[s.ID] = s.Type
	}

	if states.InitialState == "" {
		result.AddIssue("I3", fmt.Sprintf("Machine %s: No initial_state defined", machineID), nil)
	} else if !stateIDs[states.InitialState] {
		result.AddIssue("I3", fmt.Sprintf("Machine %s: initial_state '%s' not in states list", machineID, states.InitialState), nil)
	}

	if len(states.TerminalStates) == 0 {
		result.AddIssue("I3", fmt.Sprintf("Machine %s: No terminal_states defined", machineID), nil)
	}

	for _, term := range states.TerminalStates {
		if !stateIDs[term] {
			result.AddIssue("I3", fmt.Sprintf("Machine %s: terminal_state '%s' not in states list", machineID, term), nil)
		}
	}

	terminalSet := make(map[string]bool)
	for _, t := range states.TerminalStates {
		terminalSet[t] = true
	}

	hasSuccess := false
	hasFailure := false
	for _, t := range states.TerminalStates {
		if stateTypes[t] == "success" {
			hasSuccess = true
		}
		if stateTypes[t] == "failure" {
			hasFailure = true
		}
	}
	if !hasSuccess && !hasFailure {
		result.AddWarning(fmt.Sprintf("Machine %s: No success or failure terminal states", machineID), nil)
	}

	sources := make(map[string]bool)
	for _, t := range transitions.Transitions {
		sources[t.FromState] = true
	}

	for _, t := range transitions.Transitions {
		if !stateIDs[t.FromState] {
			result.AddIssue("I3", fmt.Sprintf("Machine %s: Transition %s references unknown from_state '%s'", machineID, t.ID, t.FromState), nil)
		}
		if !stateIDs[t.ToState] {
			result.AddIssue("I3", fmt.Sprintf("Machine %s: Transition %s references unknown to_state '%s'", machineID, t.ID, t.ToState), nil)
		}
	}

	for _, s := range states.States {
		if s.Type != "success" && s.Type != "failure" && !sources[s.ID] {
			result.AddIssue("I3", fmt.Sprintf("Machine %s: Non-terminal state '%s' (%s) has no outgoing transitions", machineID, s.ID, s.Name), nil)
		}
	}

	if states.InitialState != "" {
		reachable := computeReachable(states.InitialState, transitions.Transitions)
		for sid := range stateIDs {
			if !reachable[sid] {
				result.AddIssue("I3", fmt.Sprintf("Machine %s: State '%s' unreachable from initial", machineID, sid), nil)
			}
		}
	}
}

// computeReachable finds all states reachable from the initial state.
func computeReachable(initialState string, transitions []Transition) map[string]bool {
	reachable := make(map[string]bool)
	toVisit := []string{initialState}

	for len(toVisit) > 0 {
		current := toVisit[0]
		toVisit = toVisit[1:]

		if reachable[current] {
			continue
		}
		reachable[current] = true

		for _, t := range transitions {
			if t.FromState == current && !reachable[t.ToState] {
				toVisit = append(toVisit, t.ToState)
			}
		}
	}

	return reachable
}

// validateI4GuardLinkage checks that guards are linked to invariants.
func validateI4GuardLinkage(transitions *TransitionsFile, result *ValidationResult) {
	machineID := transitions.MachineID

	for _, t := range transitions.Transitions {
		for _, guard := range t.Guards {
			if guard.InvariantID == "" {
				result.AddWarning(fmt.Sprintf("Machine %s: Guard on %s has no invariant_id linkage", machineID, t.ID), map[string]string{
					"transition": t.ID,
					"condition":  guard.Condition,
				})
			}
		}
	}
}

// CheckCompleteness validates completeness for a single machine given states and transitions.
// Returns issues found as a ValidationResult.
func CheckCompleteness(states *StatesFile, transitions *TransitionsFile) *ValidationResult {
	result := NewValidationResult()
	validateI3Completeness(states, transitions, result)
	validateI4GuardLinkage(transitions, result)
	return result
}

// ComputeCoverage calculates transition coverage percentage based on behavior linkage.
// A transition is considered "covered" if it has at least one behavior linked.
func ComputeCoverage(transitions *TransitionsFile) *CoverageResult {
	total := len(transitions.Transitions)
	covered := 0
	uncovered := make([]string, 0)

	for _, t := range transitions.Transitions {
		if len(t.Behaviors) > 0 {
			covered++
		} else {
			uncovered = append(uncovered, t.ID)
		}
	}

	var percent float64
	if total > 0 {
		percent = float64(covered) / float64(total) * 100
	} else {
		percent = 100
	}

	return &CoverageResult{
		TotalTransitions:   total,
		CoveredTransitions: covered,
		CoveragePercent:    percent,
		UncoveredIDs:       uncovered,
	}
}

// TaskTransitionCoverage represents which transitions a task covers.
type TaskTransitionCoverage struct {
	TaskID              string   `json:"task_id"`
	TransitionsCovered  []string `json:"transitions_covered"`
}

// CheckTaskCoverage validates that tasks implement all required FSM transitions.
// Steel thread transitions require 100% coverage, non-steel thread require the specified threshold.
func CheckTaskCoverage(
	index *IndexFile,
	fsmDir string,
	tasksCoverage []TaskTransitionCoverage,
	steelThreadThreshold float64,
	nonSteelThreadThreshold float64,
) (*TaskCoverageResult, error) {
	result := &TaskCoverageResult{
		Passed: true,
		Issues: make([]ValidationIssue, 0),
	}

	transitionToTasks := make(map[string][]string)
	for _, tc := range tasksCoverage {
		for _, trID := range tc.TransitionsCovered {
			transitionToTasks[trID] = append(transitionToTasks[trID], tc.TaskID)
		}
	}

	var steelThreadTransitions []string
	var nonSteelTransitions []string

	for _, machine := range index.Machines {
		transitionsPath := filepath.Join(fsmDir, machine.Files["transitions"])
		transitionsData, err := os.ReadFile(transitionsPath)
		if err != nil {
			continue
		}

		var transitions TransitionsFile
		if err := json.Unmarshal(transitionsData, &transitions); err != nil {
			continue
		}

		isSteelThread := machine.ID == index.PrimaryMachine || machine.Level == "steel_thread"

		for _, t := range transitions.Transitions {
			if isSteelThread {
				steelThreadTransitions = append(steelThreadTransitions, t.ID)
			} else {
				nonSteelTransitions = append(nonSteelTransitions, t.ID)
			}
		}
	}

	steelCovered := make([]string, 0)
	steelUncovered := make([]string, 0)
	for _, trID := range steelThreadTransitions {
		if _, ok := transitionToTasks[trID]; ok {
			steelCovered = append(steelCovered, trID)
		} else {
			steelUncovered = append(steelUncovered, trID)
		}
	}

	nonSteelCovered := make([]string, 0)
	nonSteelUncovered := make([]string, 0)
	for _, trID := range nonSteelTransitions {
		if _, ok := transitionToTasks[trID]; ok {
			nonSteelCovered = append(nonSteelCovered, trID)
		} else {
			nonSteelUncovered = append(nonSteelUncovered, trID)
		}
	}

	var steelCoveragePercent float64 = 100
	if len(steelThreadTransitions) > 0 {
		steelCoveragePercent = float64(len(steelCovered)) / float64(len(steelThreadTransitions)) * 100
	}

	var nonSteelCoveragePercent float64 = 100
	if len(nonSteelTransitions) > 0 {
		nonSteelCoveragePercent = float64(len(nonSteelCovered)) / float64(len(nonSteelTransitions)) * 100
	}

	result.SteelThreadCoverage = &CoverageResult{
		TotalTransitions:   len(steelThreadTransitions),
		CoveredTransitions: len(steelCovered),
		CoveragePercent:    steelCoveragePercent,
		UncoveredIDs:       steelUncovered,
	}

	result.NonSteelCoverage = &CoverageResult{
		TotalTransitions:   len(nonSteelTransitions),
		CoveredTransitions: len(nonSteelCovered),
		CoveragePercent:    nonSteelCoveragePercent,
		UncoveredIDs:       nonSteelUncovered,
	}

	if steelCoveragePercent < steelThreadThreshold*100 {
		result.Passed = false
		result.Issues = append(result.Issues, ValidationIssue{
			Invariant: "TASK_COVERAGE",
			Message:   fmt.Sprintf("Steel thread coverage %.1f%% below required %.0f%%", steelCoveragePercent, steelThreadThreshold*100),
			Context: map[string]string{
				"required":  fmt.Sprintf("%.0f%%", steelThreadThreshold*100),
				"actual":    fmt.Sprintf("%.1f%%", steelCoveragePercent),
				"uncovered": fmt.Sprintf("%v", steelUncovered),
			},
		})
	}

	if nonSteelCoveragePercent < nonSteelThreadThreshold*100 {
		result.Passed = false
		result.Issues = append(result.Issues, ValidationIssue{
			Invariant: "TASK_COVERAGE",
			Message:   fmt.Sprintf("Non-steel-thread coverage %.1f%% below required %.0f%%", nonSteelCoveragePercent, nonSteelThreadThreshold*100),
			Context: map[string]string{
				"required":  fmt.Sprintf("%.0f%%", nonSteelThreadThreshold*100),
				"actual":    fmt.Sprintf("%.1f%%", nonSteelCoveragePercent),
				"uncovered": fmt.Sprintf("%v", nonSteelUncovered),
			},
		})
	}

	return result, nil
}
