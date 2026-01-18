package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dgordon/tasker/internal/state"
)

type StateProvider interface {
	Load() (*state.State, error)
}

type Model struct {
	planningDir   string
	stateProvider StateProvider
	state         *state.State
	tasks         []state.Task
	cursor        int
	err           error
	width         int
	height        int
	quitting      bool
}

type stateMsg struct {
	state *state.State
	err   error
}

type errMsg struct {
	err error
}

func NewModel(planningDir string) Model {
	return Model{
		planningDir:   planningDir,
		stateProvider: state.NewStateManager(planningDir),
		cursor:        0,
	}
}

func NewModelWithProvider(planningDir string, provider StateProvider) Model {
	return Model{
		planningDir:   planningDir,
		stateProvider: provider,
		cursor:        0,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchStateCmd()
}

func (m Model) fetchStateCmd() tea.Cmd {
	return func() tea.Msg {
		s, err := m.stateProvider.Load()
		return stateMsg{state: s, err: err}
	}
}

// FetchState retrieves the current state for display (behavior B66)
func FetchState(provider StateProvider) (*state.State, error) {
	return provider.Load()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case stateMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.state = msg.state
		m.tasks = sortedTasks(msg.state.Tasks)
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyboard(msg)
	}

	return m, nil
}

// HandleKeyboard processes keyboard input for navigation (behavior B65)
func (m Model) handleKeyboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if len(m.tasks) > 0 {
			m.cursor = min(m.cursor+1, len(m.tasks)-1)
		}
		return m, nil

	case "k", "up":
		if len(m.tasks) > 0 {
			m.cursor = max(m.cursor-1, 0)
		}
		return m, nil

	case "enter":
		return m, nil

	case "r":
		return m, m.fetchStateCmd()
	}

	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	statusStyles = map[string]lipgloss.Style{
		"pending":  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		"ready":    lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		"running":  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		"complete": lipgloss.NewStyle().Foreground(lipgloss.Color("46")),
		"failed":   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		"blocked":  lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
		"skipped":  lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.state == nil {
		return "Loading state..."
	}

	var s string

	s += titleStyle.Render("Tasker TUI") + "\n"
	s += fmt.Sprintf("Phase: %s | Tasks: %d\n\n", m.state.Phase.Current, len(m.tasks))

	if len(m.tasks) == 0 {
		s += "No tasks found.\n"
	} else {
		for i, task := range m.tasks {
			cursor := "  "
			style := normalStyle
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			statusStyle, ok := statusStyles[task.Status]
			if !ok {
				statusStyle = normalStyle
			}

			line := fmt.Sprintf("%s%s [%s] %s",
				cursor,
				style.Render(task.ID),
				statusStyle.Render(task.Status),
				task.Name,
			)
			s += line + "\n"
		}
	}

	s += "\n" + helpStyle.Render("j/k: navigate | enter: select | r: refresh | q: quit")

	return s
}

func sortedTasks(tasks map[string]state.Task) []state.Task {
	result := make([]state.Task, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, t)
	}

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Phase > result[j].Phase ||
				(result[i].Phase == result[j].Phase && result[i].ID > result[j].ID) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

func Run(planningDir string) error {
	m := NewModel(planningDir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
