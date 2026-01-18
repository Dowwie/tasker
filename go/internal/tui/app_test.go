package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dgordon/tasker/internal/state"
)

type mockStateProvider struct {
	state *state.State
	err   error
}

func (m *mockStateProvider) Load() (*state.State, error) {
	return m.state, m.err
}

func createTestState() *state.State {
	return &state.State{
		Version: "2.0",
		Phase:   state.PhaseState{Current: "executing", Completed: []string{}},
		Tasks: map[string]state.Task{
			"T001": {ID: "T001", Name: "First task", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Name: "Second task", Status: "running", Phase: 1},
			"T003": {ID: "T003", Name: "Third task", Status: "pending", Phase: 2},
		},
	}
}

func TestAppStart(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)

	if m.planningDir != "test-planning-dir" {
		t.Errorf("expected planningDir to be 'test-planning-dir', got '%s'", m.planningDir)
	}

	if m.cursor != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.cursor)
	}

	cmd := m.Init()
	if cmd == nil {
		t.Error("expected Init to return a command, got nil")
	}

	msg := cmd()
	stMsg, ok := msg.(stateMsg)
	if !ok {
		t.Errorf("expected stateMsg, got %T", msg)
	}

	if stMsg.err != nil {
		t.Errorf("unexpected error: %v", stMsg.err)
	}

	if stMsg.state == nil {
		t.Error("expected state to be loaded, got nil")
	}
}

func TestHandleKeyboard(t *testing.T) {
	testCases := []struct {
		name           string
		key            string
		initialCursor  int
		expectedCursor int
		expectedQuit   bool
	}{
		{
			name:           "j moves cursor down",
			key:            "j",
			initialCursor:  0,
			expectedCursor: 1,
			expectedQuit:   false,
		},
		{
			name:           "down moves cursor down",
			key:            "down",
			initialCursor:  0,
			expectedCursor: 1,
			expectedQuit:   false,
		},
		{
			name:           "k moves cursor up",
			key:            "k",
			initialCursor:  1,
			expectedCursor: 0,
			expectedQuit:   false,
		},
		{
			name:           "up moves cursor up",
			key:            "up",
			initialCursor:  1,
			expectedCursor: 0,
			expectedQuit:   false,
		},
		{
			name:           "j at bottom stays at bottom",
			key:            "j",
			initialCursor:  2,
			expectedCursor: 2,
			expectedQuit:   false,
		},
		{
			name:           "k at top stays at top",
			key:            "k",
			initialCursor:  0,
			expectedCursor: 0,
			expectedQuit:   false,
		},
		{
			name:           "q quits",
			key:            "q",
			initialCursor:  0,
			expectedCursor: 0,
			expectedQuit:   true,
		},
		{
			name:           "enter does nothing",
			key:            "enter",
			initialCursor:  1,
			expectedCursor: 1,
			expectedQuit:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockStateProvider{state: createTestState()}
			m := NewModelWithProvider("test-planning-dir", provider)
			m.state = provider.state
			m.tasks = sortedTasks(provider.state.Tasks)
			m.cursor = tc.initialCursor

			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			if tc.key == "up" {
				keyMsg = tea.KeyMsg{Type: tea.KeyUp}
			} else if tc.key == "down" {
				keyMsg = tea.KeyMsg{Type: tea.KeyDown}
			} else if tc.key == "enter" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
			}

			newModel, cmd := m.Update(keyMsg)
			updatedModel := newModel.(Model)

			if updatedModel.cursor != tc.expectedCursor {
				t.Errorf("expected cursor %d, got %d", tc.expectedCursor, updatedModel.cursor)
			}

			if tc.expectedQuit {
				if cmd == nil {
					t.Error("expected quit command, got nil")
				}
				if !updatedModel.quitting {
					t.Error("expected quitting to be true")
				}
			}
		})
	}
}

func TestHandleKeyboardCtrlC(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)
	m.state = provider.state
	m.tasks = sortedTasks(provider.state.Tasks)

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := m.Update(keyMsg)
	updatedModel := newModel.(Model)

	if !updatedModel.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}

	if cmd == nil {
		t.Error("expected quit command, got nil")
	}
}

func TestHandleKeyboardRefresh(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)
	m.state = provider.state
	m.tasks = sortedTasks(provider.state.Tasks)

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	_, cmd := m.Update(keyMsg)

	if cmd == nil {
		t.Error("expected refresh command, got nil")
	}
}

func TestFetchState(t *testing.T) {
	testState := createTestState()
	provider := &mockStateProvider{state: testState}

	result, err := FetchState(provider)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected state, got nil")
	}

	if result.Version != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", result.Version)
	}

	if len(result.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(result.Tasks))
	}
}

func TestFetchStateError(t *testing.T) {
	provider := &mockStateProvider{err: state.ErrStateLocked}

	result, err := FetchState(provider)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if result != nil {
		t.Error("expected nil state on error")
	}
}

func TestView(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)
	m.state = provider.state
	m.tasks = sortedTasks(provider.state.Tasks)

	view := m.View()

	if view == "" {
		t.Error("expected non-empty view")
	}

	expectedContent := []string{
		"Tasker TUI",
		"T001",
		"T002",
		"T003",
		"j/k: navigate",
	}

	for _, expected := range expectedContent {
		if !containsString(view, expected) {
			t.Errorf("view should contain '%s'", expected)
		}
	}
}

func TestViewError(t *testing.T) {
	provider := &mockStateProvider{err: state.ErrStateLocked}
	m := NewModelWithProvider("test-planning-dir", provider)
	m.err = state.ErrStateLocked

	view := m.View()

	if !containsString(view, "Error:") {
		t.Error("view should show error message")
	}
}

func TestViewLoading(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)

	view := m.View()

	if view != "Loading state..." {
		t.Errorf("expected 'Loading state...', got '%s'", view)
	}
}

func TestViewQuitting(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)
	m.quitting = true

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when quitting, got '%s'", view)
	}
}

func TestSortedTasks(t *testing.T) {
	tasks := map[string]state.Task{
		"T003": {ID: "T003", Name: "Third", Phase: 2},
		"T001": {ID: "T001", Name: "First", Phase: 1},
		"T002": {ID: "T002", Name: "Second", Phase: 1},
	}

	sorted := sortedTasks(tasks)

	if len(sorted) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(sorted))
	}

	expectedOrder := []string{"T001", "T002", "T003"}
	for i, expected := range expectedOrder {
		if sorted[i].ID != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, sorted[i].ID)
		}
	}
}

func TestWindowSizeUpdate(t *testing.T) {
	provider := &mockStateProvider{state: createTestState()}
	m := NewModelWithProvider("test-planning-dir", provider)

	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	newModel, _ := m.Update(sizeMsg)
	updatedModel := newModel.(Model)

	if updatedModel.width != 80 {
		t.Errorf("expected width 80, got %d", updatedModel.width)
	}

	if updatedModel.height != 24 {
		t.Errorf("expected height 24, got %d", updatedModel.height)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
