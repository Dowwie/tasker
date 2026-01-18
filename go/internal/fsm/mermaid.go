package fsm

import (
	"fmt"
	"strings"
)

// GenerateMermaid generates a Mermaid stateDiagram-v2 diagram from FSM states and transitions.
func GenerateMermaid(machineName string, states *StatesFile, transitions *TransitionsFile) string {
	if states == nil || transitions == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("stateDiagram-v2\n")
	sb.WriteString(fmt.Sprintf("    title %s\n\n", machineName))

	stateMap := make(map[string]State)
	for _, s := range states.States {
		stateMap[s.ID] = s
	}

	for _, s := range states.States {
		safeName := strings.ReplaceAll(s.Name, "\"", "'")
		switch s.Type {
		case "initial":
			sb.WriteString(fmt.Sprintf("    [*] --> %s\n", s.ID))
			sb.WriteString(fmt.Sprintf("    %s: %s\n", s.ID, safeName))
		case "success":
			sb.WriteString(fmt.Sprintf("    %s --> [*]\n", s.ID))
			sb.WriteString(fmt.Sprintf("    %s: %s\n", s.ID, safeName))
		case "failure":
			sb.WriteString(fmt.Sprintf("    %s: %s [FAILURE]\n", s.ID, safeName))
		default:
			sb.WriteString(fmt.Sprintf("    %s: %s\n", s.ID, safeName))
		}
	}

	sb.WriteString("\n")

	for _, t := range transitions.Transitions {
		safeTrigger := strings.ReplaceAll(t.Trigger, "\"", "'")
		if len(safeTrigger) > 30 {
			safeTrigger = safeTrigger[:27] + "..."
		}
		sb.WriteString(fmt.Sprintf("    %s --> %s: %s\n", t.FromState, t.ToState, safeTrigger))
	}

	return sb.String()
}

// GenerateNotes generates markdown documentation describing FSM states and transitions.
func GenerateNotes(machineName string, states *StatesFile, transitions *TransitionsFile) string {
	if states == nil || transitions == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", machineName))

	sb.WriteString("## States\n\n")

	stateMap := make(map[string]State)
	for _, s := range states.States {
		stateMap[s.ID] = s
	}

	if states.InitialState != "" {
		if initial, ok := stateMap[states.InitialState]; ok {
			sb.WriteString(fmt.Sprintf("**Initial State:** %s (%s)\n\n", initial.Name, initial.ID))
		}
	}

	if len(states.TerminalStates) > 0 {
		sb.WriteString("**Terminal States:**\n")
		for _, termID := range states.TerminalStates {
			if term, ok := stateMap[termID]; ok {
				sb.WriteString(fmt.Sprintf("- %s (%s) - %s\n", term.Name, term.ID, term.Type))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### All States\n\n")
	sb.WriteString("| ID | Name | Type | Description |\n")
	sb.WriteString("|----|------|------|-------------|\n")
	for _, s := range states.States {
		desc := s.Description
		if desc == "" && s.SpecRef != nil {
			desc = s.SpecRef.Quote
		}
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", s.ID, s.Name, s.Type, desc))
	}

	sb.WriteString("\n## Transitions\n\n")
	sb.WriteString("| ID | From | To | Trigger | Guards |\n")
	sb.WriteString("|----|------|----|---------|--------|\n")
	for _, t := range transitions.Transitions {
		fromName := t.FromState
		if from, ok := stateMap[t.FromState]; ok {
			fromName = from.Name
		}
		toName := t.ToState
		if to, ok := stateMap[t.ToState]; ok {
			toName = to.Name
		}

		guardStrs := make([]string, 0, len(t.Guards))
		for _, g := range t.Guards {
			if g.InvariantID != "" {
				guardStrs = append(guardStrs, g.InvariantID)
			} else if g.Condition != "" {
				cond := g.Condition
				if len(cond) > 20 {
					cond = cond[:17] + "..."
				}
				guardStrs = append(guardStrs, cond)
			}
		}
		guards := strings.Join(guardStrs, ", ")
		if guards == "" {
			guards = "-"
		}

		trigger := t.Trigger
		if len(trigger) > 25 {
			trigger = trigger[:22] + "..."
		}

		failureMarker := ""
		if t.IsFailurePath {
			failureMarker = " [FAILURE]"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s%s | %s |\n", t.ID, fromName, toName, trigger, failureMarker, guards))
	}

	if len(transitions.GuardsIndex) > 0 {
		sb.WriteString("\n## Guards Index\n\n")
		sb.WriteString("Mapping of invariant IDs to transitions that enforce them:\n\n")
		for invID, transIDs := range transitions.GuardsIndex {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", invID, strings.Join(transIDs, ", ")))
		}
	}

	sb.WriteString("\n## Diagram\n\n")
	sb.WriteString("```mermaid\n")
	sb.WriteString(GenerateMermaid(machineName, states, transitions))
	sb.WriteString("```\n")

	return sb.String()
}
