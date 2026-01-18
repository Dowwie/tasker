package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dgordon/tasker/internal/state"
)

func createDashboardTestState() *state.State {
	return &state.State{
		Version: "2.0",
		Phase:   state.PhaseState{Current: "executing", Completed: []string{"ingestion", "logical"}},
		Tasks: map[string]state.Task{
			"T001": {ID: "T001", Name: "First task", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Name: "Second task", Status: "running", Phase: 1},
			"T003": {ID: "T003", Name: "Third task", Status: "pending", Phase: 2},
			"T004": {ID: "T004", Name: "Fourth task", Status: "failed", Phase: 1},
			"T005": {ID: "T005", Name: "Fifth task", Status: "ready", Phase: 2},
		},
	}
}

func TestRenderDashboard(t *testing.T) {
	testState := createDashboardTestState()
	tasks := sortedTasks(testState.Tasks)

	view := RenderDashboard(testState, tasks, 80)

	if view == "" {
		t.Error("expected non-empty dashboard view")
	}

	expectedContent := []string{
		"Tasker Dashboard",
		"T001",
		"T002",
		"T003",
		"complete",
		"running",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(view, expected) {
			t.Errorf("dashboard should contain '%s', got:\n%s", expected, view)
		}
	}
}

func TestRenderDashboardNilState(t *testing.T) {
	view := RenderDashboard(nil, nil, 80)

	if view != "No state available" {
		t.Errorf("expected 'No state available' for nil state, got '%s'", view)
	}
}

func TestRenderDashboardStatusIndicators(t *testing.T) {
	testState := createDashboardTestState()
	tasks := sortedTasks(testState.Tasks)

	view := RenderDashboard(testState, tasks, 80)

	indicators := []string{"✓", "●", "✗", "○", "◎"}
	found := 0
	for _, indicator := range indicators {
		if strings.Contains(view, indicator) {
			found++
		}
	}

	if found == 0 {
		t.Error("dashboard should contain status indicators")
	}
}

func TestDashboardMetrics(t *testing.T) {
	testState := createDashboardTestState()
	metrics := CalculateMetrics(testState)

	if metrics.TotalTasks != 5 {
		t.Errorf("expected 5 total tasks, got %d", metrics.TotalTasks)
	}

	if metrics.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", metrics.Completed)
	}

	if metrics.Running != 1 {
		t.Errorf("expected 1 running, got %d", metrics.Running)
	}

	if metrics.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", metrics.Failed)
	}

	if metrics.Pending != 1 {
		t.Errorf("expected 1 pending, got %d", metrics.Pending)
	}

	if metrics.Ready != 1 {
		t.Errorf("expected 1 ready, got %d", metrics.Ready)
	}

	if metrics.CurrentPhase != "executing" {
		t.Errorf("expected phase 'executing', got '%s'", metrics.CurrentPhase)
	}
}

func TestDashboardMetricsPhaseProgress(t *testing.T) {
	testState := createDashboardTestState()
	metrics := CalculateMetrics(testState)

	if len(metrics.PhaseProgress) != 2 {
		t.Errorf("expected 2 phases, got %d", len(metrics.PhaseProgress))
	}

	phase1 := metrics.PhaseProgress[1]
	if phase1.Total != 3 {
		t.Errorf("expected 3 tasks in phase 1, got %d", phase1.Total)
	}
	if phase1.Completed != 1 {
		t.Errorf("expected 1 completed in phase 1, got %d", phase1.Completed)
	}
	if phase1.Running != 1 {
		t.Errorf("expected 1 running in phase 1, got %d", phase1.Running)
	}
	if phase1.Failed != 1 {
		t.Errorf("expected 1 failed in phase 1, got %d", phase1.Failed)
	}

	phase2 := metrics.PhaseProgress[2]
	if phase2.Total != 2 {
		t.Errorf("expected 2 tasks in phase 2, got %d", phase2.Total)
	}
}

func TestDashboardStyling(t *testing.T) {
	testState := createDashboardTestState()
	tasks := sortedTasks(testState.Tasks)

	view := RenderDashboard(testState, tasks, 80)

	if view == "" {
		t.Error("expected styled dashboard view")
	}

	if !strings.Contains(view, "Overall Progress") {
		t.Error("dashboard should contain 'Overall Progress' section")
	}

	if !strings.Contains(view, "Status Summary") {
		t.Error("dashboard should contain 'Status Summary' section")
	}

	if !strings.Contains(view, "Phase Progress") {
		t.Error("dashboard should contain 'Phase Progress' section")
	}

	if !strings.Contains(view, "Tasks") {
		t.Error("dashboard should contain 'Tasks' section")
	}
}

func TestRenderProgressBar(t *testing.T) {
	testCases := []struct {
		name      string
		completed int
		total     int
		width     int
		wantLen   int
	}{
		{
			name:      "50% complete",
			completed: 5,
			total:     10,
			width:     20,
			wantLen:   20,
		},
		{
			name:      "100% complete",
			completed: 10,
			total:     10,
			width:     20,
			wantLen:   20,
		},
		{
			name:      "0% complete",
			completed: 0,
			total:     10,
			width:     20,
			wantLen:   20,
		},
		{
			name:      "zero total",
			completed: 0,
			total:     0,
			width:     20,
			wantLen:   0,
		},
		{
			name:      "zero width",
			completed: 5,
			total:     10,
			width:     0,
			wantLen:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bar := RenderProgressBar(tc.completed, tc.total, tc.width)

			if tc.wantLen == 0 {
				if bar != "" {
					t.Errorf("expected empty bar, got '%s'", bar)
				}
				return
			}

			if tc.completed == tc.total && tc.total > 0 {
				if !strings.Contains(bar, "█") {
					t.Error("100% bar should contain filled characters")
				}
			}

			if tc.completed == 0 && tc.total > 0 {
				if !strings.Contains(bar, "░") {
					t.Error("0% bar should contain empty characters")
				}
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	testCases := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcdef", 3, "abc"},
		{"", 10, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := truncateString(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q",
					tc.input, tc.maxLen, result, tc.expected)
			}
		})
	}
}

func TestGetSortedPhases(t *testing.T) {
	pm := map[int]PhaseMetrics{
		3: {Phase: 3, Total: 5},
		1: {Phase: 1, Total: 10},
		2: {Phase: 2, Total: 7},
	}

	sorted := getSortedPhases(pm)

	if len(sorted) != 3 {
		t.Errorf("expected 3 phases, got %d", len(sorted))
	}

	expected := []int{1, 2, 3}
	for i, p := range sorted {
		if p != expected[i] {
			t.Errorf("position %d: expected phase %d, got %d", i, expected[i], p)
		}
	}
}

func TestDashboardWithEmptyState(t *testing.T) {
	testState := &state.State{
		Version: "2.0",
		Phase:   state.PhaseState{Current: "ready"},
		Tasks:   map[string]state.Task{},
	}

	view := RenderDashboard(testState, []state.Task{}, 80)

	if view == "" {
		t.Error("expected non-empty view for empty state")
	}

	if !strings.Contains(view, "0%") {
		t.Error("dashboard should show 0% for empty state")
	}

	if !strings.Contains(view, "ready") {
		t.Error("dashboard should show current phase")
	}
}

func TestDashboardWithAllStatuses(t *testing.T) {
	testState := &state.State{
		Version: "2.0",
		Phase:   state.PhaseState{Current: "executing"},
		Tasks: map[string]state.Task{
			"T001": {ID: "T001", Status: "complete", Phase: 1},
			"T002": {ID: "T002", Status: "running", Phase: 1},
			"T003": {ID: "T003", Status: "pending", Phase: 1},
			"T004": {ID: "T004", Status: "ready", Phase: 1},
			"T005": {ID: "T005", Status: "failed", Phase: 1},
			"T006": {ID: "T006", Status: "blocked", Phase: 1},
			"T007": {ID: "T007", Status: "skipped", Phase: 1},
		},
	}

	tasks := sortedTasks(testState.Tasks)
	view := RenderDashboard(testState, tasks, 80)

	if !strings.Contains(view, "Complete:") {
		t.Error("dashboard should show Complete metric")
	}

	if !strings.Contains(view, "Running:") {
		t.Error("dashboard should show Running metric")
	}

	if !strings.Contains(view, "Failed:") {
		t.Error("dashboard should show Failed metric")
	}

	if !strings.Contains(view, "Blocked:") {
		t.Error("dashboard should show Blocked metric")
	}

	if !strings.Contains(view, "Skipped:") {
		t.Error("dashboard should show Skipped metric")
	}
}

func TestDashboardTaskListTruncation(t *testing.T) {
	tasks := make(map[string]state.Task)
	for i := 1; i <= 15; i++ {
		id := fmt.Sprintf("T%03d", i)
		tasks[id] = state.Task{ID: id, Name: fmt.Sprintf("Task %d", i), Status: "pending", Phase: 1}
	}

	testState := &state.State{
		Version: "2.0",
		Phase:   state.PhaseState{Current: "executing"},
		Tasks:   tasks,
	}

	sortedTaskList := sortedTasks(testState.Tasks)
	view := RenderDashboard(testState, sortedTaskList, 80)

	if !strings.Contains(view, "and 5 more tasks") {
		t.Error("dashboard should indicate truncated task list")
	}
}

func TestDashboardWidthHandling(t *testing.T) {
	testState := createDashboardTestState()
	tasks := sortedTasks(testState.Tasks)

	narrowView := RenderDashboard(testState, tasks, 40)
	wideView := RenderDashboard(testState, tasks, 120)

	if narrowView == "" || wideView == "" {
		t.Error("dashboard should render at any width")
	}
}

var _ = fmt.Sprintf
