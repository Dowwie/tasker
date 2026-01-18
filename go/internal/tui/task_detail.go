package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dgordon/tasker/internal/state"
)

type AcceptanceCriterion struct {
	Criterion    string `json:"criterion"`
	Verification string `json:"verification"`
}

type TaskFile struct {
	Path    string `json:"path"`
	Action  string `json:"action"`
	Purpose string `json:"purpose"`
}

type TaskDefinition struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
	Files              []TaskFile            `json:"files"`
}

type TaskDetail struct {
	Task               state.Task
	AcceptanceCriteria []AcceptanceCriterion
	Files              []TaskFile
}

var (
	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				MarginBottom(1)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")).
				MarginTop(1)

	detailLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	detailValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	criterionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	verificationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true)

	filePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	actionCreateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46"))

	actionModifyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	actionDeleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))
)

func LoadTaskDefinition(filePath string) (*TaskDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var def TaskDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse task definition: %w", err)
	}

	return &def, nil
}

func BuildTaskDetail(task state.Task) TaskDetail {
	detail := TaskDetail{
		Task:               task,
		AcceptanceCriteria: nil,
		Files:              nil,
	}

	if task.File != "" {
		def, err := LoadTaskDefinition(task.File)
		if err == nil && def != nil {
			detail.AcceptanceCriteria = def.AcceptanceCriteria
			detail.Files = def.Files
		}
	}

	return detail
}

func RenderTaskDetail(detail TaskDetail, width int) string {
	var b strings.Builder

	b.WriteString(detailTitleStyle.Render(fmt.Sprintf("%s: %s", detail.Task.ID, detail.Task.Name)))
	b.WriteString("\n\n")

	b.WriteString(sectionHeaderStyle.Render("Status"))
	b.WriteString("\n")
	statusStyle, ok := statusStyles[detail.Task.Status]
	if !ok {
		statusStyle = normalStyle
	}
	b.WriteString(fmt.Sprintf("  %s %s\n",
		detailLabelStyle.Render("Status:"),
		statusStyle.Render(detail.Task.Status)))
	b.WriteString(fmt.Sprintf("  %s %d\n",
		detailLabelStyle.Render("Phase:"),
		detail.Task.Phase))

	if len(detail.Task.DependsOn) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionHeaderStyle.Render("Dependencies"))
		b.WriteString("\n")
		for _, dep := range detail.Task.DependsOn {
			b.WriteString(fmt.Sprintf("  - %s\n", detailValueStyle.Render(dep)))
		}
	}

	if len(detail.Task.Blocks) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionHeaderStyle.Render("Blocks"))
		b.WriteString("\n")
		for _, blocked := range detail.Task.Blocks {
			b.WriteString(fmt.Sprintf("  - %s\n", detailValueStyle.Render(blocked)))
		}
	}

	if len(detail.AcceptanceCriteria) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionHeaderStyle.Render("Acceptance Criteria"))
		b.WriteString("\n")
		for i, ac := range detail.AcceptanceCriteria {
			status := renderCriterionStatus(detail.Task, i)
			b.WriteString(fmt.Sprintf("  %s %s\n",
				status,
				criterionStyle.Render(ac.Criterion)))
			if ac.Verification != "" {
				b.WriteString(fmt.Sprintf("      %s\n",
					verificationStyle.Render(ac.Verification)))
			}
		}
	}

	if len(detail.Files) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionHeaderStyle.Render("Files"))
		b.WriteString("\n")
		for _, f := range detail.Files {
			actionStyled := renderActionType(f.Action)
			b.WriteString(fmt.Sprintf("  [%s] %s\n",
				actionStyled,
				filePathStyle.Render(f.Path)))
		}
	}

	if detail.Task.Error != "" {
		b.WriteString("\n")
		b.WriteString(sectionHeaderStyle.Render("Error"))
		b.WriteString("\n")
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		b.WriteString(fmt.Sprintf("  %s\n", errorStyle.Render(detail.Task.Error)))
	}

	return b.String()
}

func renderCriterionStatus(task state.Task, index int) string {
	if task.Verification != nil && len(task.Verification.Criteria) > index {
		crit := task.Verification.Criteria[index]
		switch crit.Score {
		case "PASS":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("[PASS]")
		case "FAIL":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[FAIL]")
		case "PARTIAL":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[PART]")
		}
	}

	switch task.Status {
	case "complete":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("[PASS]")
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[FAIL]")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("[----]")
	}
}

func renderActionType(action string) string {
	switch action {
	case "create":
		return actionCreateStyle.Render("create")
	case "modify":
		return actionModifyStyle.Render("modify")
	case "delete":
		return actionDeleteStyle.Render("delete")
	default:
		return detailValueStyle.Render(action)
	}
}
