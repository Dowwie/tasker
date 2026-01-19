package fsm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dgordon/tasker/internal/errors"
)

type SpecRef struct {
	Quote    string `json:"quote"`
	Location string `json:"location,omitempty"`
}

type Guard struct {
	Condition   string `json:"condition"`
	InvariantID string `json:"invariant_id,omitempty"`
	Negated     bool   `json:"negated,omitempty"`
}

type State struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	SpecRef     *SpecRef `json:"spec_ref,omitempty"`
	Invariants  []string `json:"invariants,omitempty"`
	Behaviors   []string `json:"behaviors,omitempty"`
}

type Transition struct {
	ID            string   `json:"id"`
	FromState     string   `json:"from_state"`
	ToState       string   `json:"to_state"`
	Trigger       string   `json:"trigger"`
	Guards        []Guard  `json:"guards,omitempty"`
	Behaviors     []string `json:"behaviors,omitempty"`
	SpecRef       *SpecRef `json:"spec_ref,omitempty"`
	IsFailurePath bool     `json:"is_failure_path,omitempty"`
}

type Machine struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Level         string       `json:"level"`
	States        []State      `json:"states,omitempty"`
	Transitions   []Transition `json:"transitions,omitempty"`
	TriggerReason string       `json:"trigger_reason,omitempty"`
	ParentMachine string       `json:"parent_machine,omitempty"`
}

type StatesFile struct {
	Version        string  `json:"version"`
	MachineID      string  `json:"machine_id"`
	InitialState   string  `json:"initial_state"`
	TerminalStates []string `json:"terminal_states"`
	States         []State `json:"states"`
}

type TransitionsFile struct {
	Version     string              `json:"version"`
	MachineID   string              `json:"machine_id"`
	Transitions []Transition        `json:"transitions"`
	GuardsIndex map[string][]string `json:"guards_index"`
}

type MachineEntry struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Level           string            `json:"level"`
	TriggerReason   string            `json:"trigger_reason,omitempty"`
	Files           map[string]string `json:"files,omitempty"`
	StatesFile      string            `json:"states_file,omitempty"`
	TransitionsFile string            `json:"transitions_file,omitempty"`
	Description     string            `json:"description,omitempty"`
}

// GetStatesFile returns the states file path, checking both formats
func (m *MachineEntry) GetStatesFile() string {
	if m.Files != nil {
		if f, ok := m.Files["states"]; ok {
			return f
		}
	}
	return m.StatesFile
}

// GetTransitionsFile returns the transitions file path, checking both formats
func (m *MachineEntry) GetTransitionsFile() string {
	if m.Files != nil {
		if f, ok := m.Files["transitions"]; ok {
			return f
		}
	}
	return m.TransitionsFile
}

// GetDiagramFile returns the diagram file path
func (m *MachineEntry) GetDiagramFile() string {
	if m.Files != nil {
		if f, ok := m.Files["diagram"]; ok {
			return f
		}
	}
	return ""
}

// GetNotesFile returns the notes file path
func (m *MachineEntry) GetNotesFile() string {
	if m.Files != nil {
		if f, ok := m.Files["notes"]; ok {
			return f
		}
	}
	return ""
}

type IndexFile struct {
	Version        string                  `json:"version"`
	SpecSlug       string                  `json:"spec_slug"`
	SpecChecksum   string                  `json:"spec_checksum"`
	CreatedAt      string                  `json:"created_at"`
	PrimaryMachine string                  `json:"primary_machine"`
	Machines       []MachineEntry          `json:"machines"`
	Hierarchy      map[string][]string     `json:"hierarchy"`
	Invariants     []map[string]interface{} `json:"invariants"`
}

type Workflow struct {
	Name           string                 `json:"name"`
	Trigger        string                 `json:"trigger,omitempty"`
	IsSteelThread  bool                   `json:"is_steel_thread,omitempty"`
	Steps          []WorkflowStep         `json:"steps"`
	Postconditions []string               `json:"postconditions,omitempty"`
	Variants       []WorkflowVariant      `json:"variants,omitempty"`
	Failures       []WorkflowFailure      `json:"failures,omitempty"`
}

type WorkflowStep struct {
	Name          string   `json:"name,omitempty"`
	Action        string   `json:"action,omitempty"`
	Description   string   `json:"description,omitempty"`
	Postcondition string   `json:"postcondition,omitempty"`
	Behaviors     []string `json:"behaviors,omitempty"`
}

type WorkflowVariant struct {
	Condition string `json:"condition,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	FromStep  *int   `json:"from_step,omitempty"`
}

type WorkflowFailure struct {
	Condition string `json:"condition,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	FromStep  *int   `json:"from_step,omitempty"`
}

type Invariant struct {
	ID   string `json:"id"`
	Rule string `json:"rule"`
}

type SpecData struct {
	Workflows  []Workflow             `json:"workflows"`
	Invariants []map[string]interface{} `json:"invariants"`
	Flows      []map[string]interface{} `json:"flows,omitempty"`
}

type CapabilityMapFlow struct {
	Name          string                   `json:"name"`
	IsSteelThread bool                     `json:"is_steel_thread,omitempty"`
	Steps         []map[string]interface{} `json:"steps"`
}

type CapabilityMapBehavior struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type CapabilityMapCapability struct {
	Behaviors  []CapabilityMapBehavior  `json:"behaviors,omitempty"`
	Invariants []map[string]interface{} `json:"invariants,omitempty"`
}

type CapabilityMapDomain struct {
	Capabilities []CapabilityMapCapability `json:"capabilities,omitempty"`
}

type CapabilityMap struct {
	SpecChecksum string                `json:"spec_checksum,omitempty"`
	Domains      []CapabilityMapDomain `json:"domains,omitempty"`
	Flows        []CapabilityMapFlow   `json:"flows,omitempty"`
}

type Compiler struct {
	stateCounter      int
	transitionCounter int
	machineCounter    int
	machines          []Machine
	invariants        []map[string]interface{}
}

func NewCompiler() *Compiler {
	return &Compiler{
		machines:   make([]Machine, 0),
		invariants: make([]map[string]interface{}, 0),
	}
}

func (c *Compiler) nextStateID() string {
	c.stateCounter++
	return fmt.Sprintf("S%d", c.stateCounter)
}

func (c *Compiler) nextTransitionID() string {
	c.transitionCounter++
	return fmt.Sprintf("TR%d", c.transitionCounter)
}

func (c *Compiler) nextMachineID() string {
	c.machineCounter++
	return fmt.Sprintf("M%d", c.machineCounter)
}

func (c *Compiler) CompileFromSpec(specData *SpecData) (*Machine, error) {
	if specData == nil {
		return nil, errors.ValidationFailed("spec data is nil")
	}

	c.invariants = specData.Invariants

	var steelThreadWorkflow *Workflow
	for i := range specData.Workflows {
		if specData.Workflows[i].IsSteelThread {
			steelThreadWorkflow = &specData.Workflows[i]
			break
		}
	}

	if steelThreadWorkflow == nil && len(specData.Workflows) > 0 {
		steelThreadWorkflow = &specData.Workflows[0]
	}

	if steelThreadWorkflow == nil {
		return nil, errors.ValidationFailed("no workflow found to compile")
	}

	machine, err := c.compileWorkflow(steelThreadWorkflow, "steel_thread")
	if err != nil {
		return nil, err
	}

	c.machines = append(c.machines, *machine)
	return machine, nil
}

func (c *Compiler) compileWorkflow(workflow *Workflow, level string) (*Machine, error) {
	if len(workflow.Steps) == 0 {
		return nil, errors.ValidationFailed(fmt.Sprintf("workflow '%s' has no steps", workflow.Name))
	}

	machineID := c.nextMachineID()
	machineName := workflow.Name
	if machineName == "" {
		machineName = "Unnamed Workflow"
	}

	states := make([]State, 0)
	transitions := make([]Transition, 0)

	trigger := workflow.Trigger
	if trigger == "" {
		trigger = "start"
	}

	initialState := State{
		ID:          c.nextStateID(),
		Name:        fmt.Sprintf("Awaiting %s", trigger),
		Type:        "initial",
		Description: "Initial state before workflow begins",
		SpecRef: &SpecRef{
			Quote:    trigger,
			Location: fmt.Sprintf("Workflow: %s", machineName),
		},
	}
	states = append(states, initialState)

	prevState := initialState
	for i, step := range workflow.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = step.Action
		}
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		postcondition := step.Postcondition
		if postcondition == "" {
			postcondition = fmt.Sprintf("Completed: %s", stepName)
		}

		state := State{
			ID:          c.nextStateID(),
			Name:        postcondition,
			Type:        "normal",
			Description: step.Description,
			SpecRef: &SpecRef{
				Quote:    coalesce(step.Description, stepName),
				Location: fmt.Sprintf("Workflow: %s, Step %d", machineName, i+1),
			},
			Behaviors: step.Behaviors,
		}
		states = append(states, state)

		transition := Transition{
			ID:        c.nextTransitionID(),
			FromState: prevState.ID,
			ToState:   state.ID,
			Trigger:   stepName,
			Behaviors: step.Behaviors,
			SpecRef: &SpecRef{
				Quote:    coalesce(step.Description, stepName),
				Location: fmt.Sprintf("Workflow: %s, Step %d", machineName, i+1),
			},
		}
		transitions = append(transitions, transition)

		prevState = state
	}

	postconditionText := "Workflow completed successfully"
	if len(workflow.Postconditions) > 0 {
		postconditionText = workflow.Postconditions[0]
	}

	successState := State{
		ID:          c.nextStateID(),
		Name:        postconditionText,
		Type:        "success",
		Description: "Workflow completed successfully",
		SpecRef: &SpecRef{
			Quote:    postconditionText,
			Location: fmt.Sprintf("Workflow: %s, Postconditions", machineName),
		},
	}
	states = append(states, successState)

	completionTransition := Transition{
		ID:        c.nextTransitionID(),
		FromState: prevState.ID,
		ToState:   successState.ID,
		Trigger:   "Complete workflow",
		SpecRef: &SpecRef{
			Quote:    "Workflow completion",
			Location: fmt.Sprintf("Workflow: %s", machineName),
		},
	}
	transitions = append(transitions, completionTransition)

	for _, variant := range workflow.Variants {
		c.addVariantTransitions(&variant, &states, &transitions, machineName)
	}

	for _, failure := range workflow.Failures {
		c.addFailureTransitions(&failure, &states, &transitions, machineName)
	}

	triggerReason := "mandatory"
	if level != "steel_thread" {
		triggerReason = "complexity"
	}

	return &Machine{
		ID:            machineID,
		Name:          machineName,
		Level:         level,
		States:        states,
		Transitions:   transitions,
		TriggerReason: triggerReason,
	}, nil
}

func (c *Compiler) addVariantTransitions(variant *WorkflowVariant, states *[]State, transitions *[]Transition, machineName string) {
	condition := variant.Condition
	outcome := variant.Outcome

	var fromState *State
	if variant.FromStep != nil && *variant.FromStep < len(*states) {
		fromState = &(*states)[*variant.FromStep]
	} else {
		for i := range *states {
			if (*states)[i].Type == "normal" {
				fromState = &(*states)[i]
				break
			}
		}
	}

	if fromState == nil {
		return
	}

	var toState *State
	for i := range *states {
		if strings.Contains(strings.ToLower((*states)[i].Name), strings.ToLower(outcome)) {
			toState = &(*states)[i]
			break
		}
	}

	if toState == nil {
		newState := State{
			ID:          c.nextStateID(),
			Name:        coalesce(outcome, fmt.Sprintf("Variant: %s", condition)),
			Type:        "normal",
			Description: fmt.Sprintf("Alternative path when: %s", condition),
			SpecRef: &SpecRef{
				Quote:    fmt.Sprintf("If %s, then %s", condition, outcome),
				Location: fmt.Sprintf("Workflow: %s, Variants", machineName),
			},
		}
		*states = append(*states, newState)
		toState = &(*states)[len(*states)-1]
	}

	guards := c.extractGuards(condition)

	transition := Transition{
		ID:        c.nextTransitionID(),
		FromState: fromState.ID,
		ToState:   toState.ID,
		Trigger:   condition,
		Guards:    guards,
		SpecRef: &SpecRef{
			Quote:    fmt.Sprintf("If %s, then %s", condition, outcome),
			Location: fmt.Sprintf("Workflow: %s, Variants", machineName),
		},
	}
	*transitions = append(*transitions, transition)
}

func (c *Compiler) addFailureTransitions(failure *WorkflowFailure, states *[]State, transitions *[]Transition, machineName string) {
	condition := failure.Condition
	outcome := failure.Outcome
	if outcome == "" {
		outcome = "Error"
	}

	var failureState *State
	for i := range *states {
		if (*states)[i].Type == "failure" && strings.Contains(strings.ToLower((*states)[i].Name), strings.ToLower(outcome)) {
			failureState = &(*states)[i]
			break
		}
	}

	if failureState == nil {
		newState := State{
			ID:          c.nextStateID(),
			Name:        outcome,
			Type:        "failure",
			Description: fmt.Sprintf("Failure state: %s", condition),
			SpecRef: &SpecRef{
				Quote:    fmt.Sprintf("If %s, then %s", condition, outcome),
				Location: fmt.Sprintf("Workflow: %s, Failures", machineName),
			},
		}
		*states = append(*states, newState)
		failureState = &(*states)[len(*states)-1]
	}

	var fromStates []*State
	if failure.FromStep != nil && *failure.FromStep < len(*states) {
		fromStates = []*State{&(*states)[*failure.FromStep]}
	} else {
		for i := range *states {
			if (*states)[i].Type == "initial" || (*states)[i].Type == "normal" {
				fromStates = append(fromStates, &(*states)[i])
			}
		}
	}

	for _, fromState := range fromStates {
		exists := false
		for _, t := range *transitions {
			if t.FromState == fromState.ID && t.ToState == failureState.ID {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		guards := c.extractGuards(condition)

		transition := Transition{
			ID:            c.nextTransitionID(),
			FromState:     fromState.ID,
			ToState:       failureState.ID,
			Trigger:       condition,
			Guards:        guards,
			IsFailurePath: true,
			SpecRef: &SpecRef{
				Quote:    fmt.Sprintf("If %s, then %s", condition, outcome),
				Location: fmt.Sprintf("Workflow: %s, Failures", machineName),
			},
		}
		*transitions = append(*transitions, transition)
	}
}

func (c *Compiler) extractGuards(condition string) []Guard {
	guards := make([]Guard, 0)
	conditionLower := strings.ToLower(condition)
	wordPattern := regexp.MustCompile(`\b\w+\b`)

	for _, inv := range c.invariants {
		ruleText, ok := inv["rule"].(string)
		if !ok || ruleText == "" {
			continue
		}
		ruleLower := strings.ToLower(ruleText)

		keywords := wordPattern.FindAllString(ruleLower, -1)
		matches := 0
		for _, kw := range keywords {
			if len(kw) > 3 && strings.Contains(conditionLower, kw) {
				matches++
			}
		}

		hasKeyword := strings.Contains(conditionLower, "must") ||
			strings.Contains(conditionLower, "valid") ||
			strings.Contains(conditionLower, "invalid") ||
			strings.Contains(conditionLower, "require")

		if matches >= 2 || hasKeyword {
			invID, _ := inv["id"].(string)
			guards = append(guards, Guard{
				Condition:   condition,
				InvariantID: invID,
			})
			break
		}
	}

	if len(guards) == 0 {
		guards = append(guards, Guard{Condition: condition})
	}

	return guards
}

func (c *Compiler) CompileFromCapabilityMap(capMap *CapabilityMap, specText string) (*Machine, error) {
	if capMap == nil {
		return nil, errors.ValidationFailed("capability map is nil")
	}

	var steelThreadFlow *CapabilityMapFlow
	for i := range capMap.Flows {
		if capMap.Flows[i].IsSteelThread {
			steelThreadFlow = &capMap.Flows[i]
			break
		}
	}

	if steelThreadFlow == nil && len(capMap.Flows) > 0 {
		steelThreadFlow = &capMap.Flows[0]
	}

	if steelThreadFlow == nil {
		return nil, errors.ValidationFailed("no flow found in capability map")
	}

	behaviorLookup := make(map[string]CapabilityMapBehavior)
	for _, domain := range capMap.Domains {
		for _, cap := range domain.Capabilities {
			for _, beh := range cap.Behaviors {
				behaviorLookup[beh.ID] = beh
			}
		}
	}

	workflow := c.flowToWorkflow(steelThreadFlow, behaviorLookup)

	var invariants []map[string]interface{}
	for _, domain := range capMap.Domains {
		for _, cap := range domain.Capabilities {
			invariants = append(invariants, cap.Invariants...)
		}
	}

	specData := &SpecData{
		Workflows:  []Workflow{*workflow},
		Invariants: invariants,
	}

	return c.CompileFromSpec(specData)
}

func (c *Compiler) flowToWorkflow(flow *CapabilityMapFlow, behaviorLookup map[string]CapabilityMapBehavior) *Workflow {
	steps := make([]WorkflowStep, 0)

	for i, stepData := range flow.Steps {
		behID, _ := stepData["behavior_id"].(string)
		desc, _ := stepData["description"].(string)

		beh, hasBeh := behaviorLookup[behID]
		stepName := desc
		if hasBeh && beh.Name != "" {
			stepName = beh.Name
		}
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		stepDesc := desc
		if hasBeh && beh.Description != "" {
			stepDesc = beh.Description
		}

		var behaviors []string
		if behID != "" {
			behaviors = []string{behID}
		}

		steps = append(steps, WorkflowStep{
			Name:          stepName,
			Action:        beh.Name,
			Description:   stepDesc,
			Postcondition: fmt.Sprintf("Completed: %s", coalesce(beh.Name, "step")),
			Behaviors:     behaviors,
		})
	}

	flowName := flow.Name
	if flowName == "" {
		flowName = "Steel Thread"
	}

	return &Workflow{
		Name:           flowName,
		Trigger:        "User initiates flow",
		IsSteelThread:  flow.IsSteelThread,
		Steps:          steps,
		Postconditions: []string{fmt.Sprintf("%s completed successfully", flowName)},
		Variants:       nil,
		Failures:       nil,
	}
}

func (c *Compiler) WriteStates(machine *Machine, outputPath string) error {
	if machine == nil {
		return errors.ValidationFailed("machine is nil")
	}

	var initialState string
	var terminalStates []string

	for _, s := range machine.States {
		if s.Type == "initial" && initialState == "" {
			initialState = s.ID
		}
		if s.Type == "success" || s.Type == "failure" {
			terminalStates = append(terminalStates, s.ID)
		}
	}

	statesFile := StatesFile{
		Version:        "1.0",
		MachineID:      machine.ID,
		InitialState:   initialState,
		TerminalStates: terminalStates,
		States:         machine.States,
	}

	data, err := json.MarshalIndent(statesFile, "", "  ")
	if err != nil {
		return errors.Internal("failed to marshal states", err)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.IOWriteFailed(outputPath, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return errors.IOWriteFailed(outputPath, err)
	}

	return nil
}

func (c *Compiler) WriteTransitions(machine *Machine, outputPath string) error {
	if machine == nil {
		return errors.ValidationFailed("machine is nil")
	}

	guardsIndex := make(map[string][]string)
	for _, trans := range machine.Transitions {
		for _, guard := range trans.Guards {
			if guard.InvariantID != "" {
				guardsIndex[guard.InvariantID] = append(guardsIndex[guard.InvariantID], trans.ID)
			}
		}
	}

	transitionsFile := TransitionsFile{
		Version:     "1.0",
		MachineID:   machine.ID,
		Transitions: machine.Transitions,
		GuardsIndex: guardsIndex,
	}

	data, err := json.MarshalIndent(transitionsFile, "", "  ")
	if err != nil {
		return errors.Internal("failed to marshal transitions", err)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.IOWriteFailed(outputPath, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return errors.IOWriteFailed(outputPath, err)
	}

	return nil
}

func (c *Compiler) Export(outputDir, specSlug, specChecksum string) (*IndexFile, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, errors.IOWriteFailed(outputDir, err)
	}

	var primaryMachine string
	for _, m := range c.machines {
		if m.Level == "steel_thread" {
			primaryMachine = m.ID
			break
		}
	}
	if primaryMachine == "" && len(c.machines) > 0 {
		primaryMachine = c.machines[0].ID
	}

	machineEntries := make([]MachineEntry, 0)
	hierarchy := make(map[string][]string)

	for _, machine := range c.machines {
		machineSlug := strings.ToLower(strings.ReplaceAll(machine.Name, " ", "-"))
		if machine.Level == "steel_thread" {
			machineSlug = "steel-thread"
		}

		files := map[string]string{
			"states":      fmt.Sprintf("%s.states.json", machineSlug),
			"transitions": fmt.Sprintf("%s.transitions.json", machineSlug),
			"diagram":     fmt.Sprintf("%s.mmd", machineSlug),
			"notes":       fmt.Sprintf("%s.notes.md", machineSlug),
		}

		statesPath := filepath.Join(outputDir, files["states"])
		if err := c.WriteStates(&machine, statesPath); err != nil {
			return nil, err
		}

		transitionsPath := filepath.Join(outputDir, files["transitions"])
		if err := c.WriteTransitions(&machine, transitionsPath); err != nil {
			return nil, err
		}

		machineEntries = append(machineEntries, MachineEntry{
			ID:            machine.ID,
			Name:          machine.Name,
			Level:         machine.Level,
			TriggerReason: machine.TriggerReason,
			Files:         files,
		})

		if machine.ParentMachine != "" {
			hierarchy[machine.ParentMachine] = append(hierarchy[machine.ParentMachine], machine.ID)
		}
	}

	index := &IndexFile{
		Version:        "1.0",
		SpecSlug:       specSlug,
		SpecChecksum:   specChecksum,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		PrimaryMachine: primaryMachine,
		Machines:       machineEntries,
		Hierarchy:      hierarchy,
		Invariants:     c.invariants,
	}

	indexData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, errors.Internal("failed to marshal index", err)
	}

	indexPath := filepath.Join(outputDir, "index.json")
	if err := os.WriteFile(indexPath, indexData, 0644); err != nil {
		return nil, errors.IOWriteFailed(indexPath, err)
	}

	return index, nil
}

func ComputeChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:16]
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
