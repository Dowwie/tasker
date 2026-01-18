package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestShowDashboard_DisplaysProgressWithStatusBreakdown(t *testing.T) {
	var buf bytes.Buffer

	data := DashboardData{
		Phase:     "executing",
		TargetDir: "/home/user/project",
		StatusCounts: StatusCounts{
			Pending:  2,
			Ready:    1,
			Running:  1,
			Complete: 5,
			Failed:   1,
			Blocked:  0,
			Skipped:  0,
			Total:    10,
		},
		CurrentPhase: 2,
		TotalPhases:  3,
		ActiveTasks:  []string{"T006"},
		RecentFailed: []string{"T003"},
	}

	ShowDashboard(&buf, data)
	output := buf.String()

	if !strings.Contains(output, "TASK DASHBOARD") {
		t.Error("output should contain TASK DASHBOARD header")
	}

	if !strings.Contains(output, "Phase:") {
		t.Error("output should contain Phase label")
	}
	if !strings.Contains(output, "executing") {
		t.Error("output should show current phase")
	}

	if !strings.Contains(output, "Target:") {
		t.Error("output should contain Target label")
	}
	if !strings.Contains(output, "/home/user/project") {
		t.Error("output should show target directory")
	}

	if !strings.Contains(output, "PROGRESS") {
		t.Error("output should contain PROGRESS section")
	}
	if !strings.Contains(output, "50.0%") {
		t.Error("output should show 50% progress (5/10 complete)")
	}

	if !strings.Contains(output, "Status Breakdown:") {
		t.Error("output should contain Status Breakdown section")
	}
	if !strings.Contains(output, "Complete:") {
		t.Error("output should show Complete count")
	}
	if !strings.Contains(output, "Running:") {
		t.Error("output should show Running count")
	}
	if !strings.Contains(output, "Failed:") {
		t.Error("output should show Failed count")
	}
	if !strings.Contains(output, "Total:") {
		t.Error("output should show Total count")
	}

	if !strings.Contains(output, "ACTIVE TASKS") {
		t.Error("output should contain ACTIVE TASKS section")
	}
	if !strings.Contains(output, "T006") {
		t.Error("output should show active task T006")
	}

	if !strings.Contains(output, "RECENT FAILURES") {
		t.Error("output should contain RECENT FAILURES section")
	}
	if !strings.Contains(output, "T003") {
		t.Error("output should show failed task T003")
	}
}

func TestShowDashboard_NoActiveOrFailedTasks(t *testing.T) {
	var buf bytes.Buffer

	data := DashboardData{
		Phase:     "ready",
		TargetDir: "/project",
		StatusCounts: StatusCounts{
			Complete: 0,
			Total:    5,
		},
		ActiveTasks:  []string{},
		RecentFailed: []string{},
	}

	ShowDashboard(&buf, data)
	output := buf.String()

	if strings.Contains(output, "ACTIVE TASKS") {
		t.Error("output should not contain ACTIVE TASKS section when empty")
	}
	if strings.Contains(output, "RECENT FAILURES") {
		t.Error("output should not contain RECENT FAILURES section when empty")
	}
}

func TestShowDashboard_ProgressBarRendering(t *testing.T) {
	var buf bytes.Buffer

	data := DashboardData{
		Phase:     "executing",
		TargetDir: "/project",
		StatusCounts: StatusCounts{
			Complete: 8,
			Total:    10,
		},
	}

	ShowDashboard(&buf, data)
	output := buf.String()

	if !strings.Contains(output, "80.0%") {
		t.Error("output should show 80% progress")
	}

	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Error("output should contain progress bar brackets")
	}
	if !strings.Contains(output, "#") {
		t.Error("output should contain filled progress bar characters")
	}
}

func TestShowStatus_DisplaysCompactSummary(t *testing.T) {
	var buf bytes.Buffer

	data := StatusData{
		Phase:    "executing",
		Complete: 5,
		Total:    10,
		Running:  2,
		Failed:   1,
		IsHalted: false,
	}

	ShowStatus(&buf, data)
	output := strings.TrimSpace(buf.String())

	expected := "EXECUTING 5/10 running=2 failed=1"
	if output != expected {
		t.Errorf("output = %q, want %q", output, expected)
	}
}

func TestShowStatus_WithHaltedState(t *testing.T) {
	var buf bytes.Buffer

	data := StatusData{
		Phase:      "executing",
		Complete:   3,
		Total:      10,
		Running:    0,
		Failed:     2,
		IsHalted:   true,
		HaltReason: "max failures exceeded",
	}

	ShowStatus(&buf, data)
	output := strings.TrimSpace(buf.String())

	expected := "EXECUTING 3/10 running=0 failed=2 [HALTED: max failures exceeded]"
	if output != expected {
		t.Errorf("output = %q, want %q", output, expected)
	}
}

func TestShowStatus_HaltedWithoutReason(t *testing.T) {
	var buf bytes.Buffer

	data := StatusData{
		Phase:    "ready",
		Complete: 0,
		Total:    5,
		Running:  0,
		Failed:   0,
		IsHalted: true,
	}

	ShowStatus(&buf, data)
	output := strings.TrimSpace(buf.String())

	expected := "READY 0/5 running=0 failed=0 [HALTED]"
	if output != expected {
		t.Errorf("output = %q, want %q", output, expected)
	}
}

func TestCountTasksByStatus(t *testing.T) {
	tasks := map[string]interface{}{
		"T001": map[string]interface{}{"id": "T001", "status": "complete"},
		"T002": map[string]interface{}{"id": "T002", "status": "complete"},
		"T003": map[string]interface{}{"id": "T003", "status": "running"},
		"T004": map[string]interface{}{"id": "T004", "status": "pending"},
		"T005": map[string]interface{}{"id": "T005", "status": "failed"},
		"T006": map[string]interface{}{"id": "T006", "status": "ready"},
		"T007": map[string]interface{}{"id": "T007", "status": "blocked"},
		"T008": map[string]interface{}{"id": "T008", "status": "skipped"},
	}

	counts := CountTasksByStatus(tasks)

	if counts.Total != 8 {
		t.Errorf("Total = %d, want 8", counts.Total)
	}
	if counts.Complete != 2 {
		t.Errorf("Complete = %d, want 2", counts.Complete)
	}
	if counts.Running != 1 {
		t.Errorf("Running = %d, want 1", counts.Running)
	}
	if counts.Pending != 1 {
		t.Errorf("Pending = %d, want 1", counts.Pending)
	}
	if counts.Failed != 1 {
		t.Errorf("Failed = %d, want 1", counts.Failed)
	}
	if counts.Ready != 1 {
		t.Errorf("Ready = %d, want 1", counts.Ready)
	}
	if counts.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", counts.Blocked)
	}
	if counts.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", counts.Skipped)
	}
}

func TestCountTasksByStatus_EmptyTasks(t *testing.T) {
	tasks := map[string]interface{}{}

	counts := CountTasksByStatus(tasks)

	if counts.Total != 0 {
		t.Errorf("Total = %d, want 0", counts.Total)
	}
}

func TestCountTasksByStatus_InvalidTaskFormat(t *testing.T) {
	tasks := map[string]interface{}{
		"T001": "not a map",
		"T002": map[string]interface{}{"id": "T002"},
		"T003": map[string]interface{}{"id": "T003", "status": 123},
	}

	counts := CountTasksByStatus(tasks)

	if counts.Total != 3 {
		t.Errorf("Total = %d, want 3", counts.Total)
	}
	if counts.Complete != 0 && counts.Running != 0 && counts.Pending != 0 {
		t.Error("no statuses should be counted for invalid tasks")
	}
}

func TestExtractActiveTasks(t *testing.T) {
	tasks := map[string]interface{}{
		"T001": map[string]interface{}{"id": "T001", "status": "complete"},
		"T002": map[string]interface{}{"id": "T002", "status": "running"},
		"T003": map[string]interface{}{"id": "T003", "status": "running"},
		"T004": map[string]interface{}{"id": "T004", "status": "pending"},
	}

	active := ExtractActiveTasks(tasks)

	if len(active) != 2 {
		t.Fatalf("len(active) = %d, want 2", len(active))
	}

	if active[0] != "T002" || active[1] != "T003" {
		t.Errorf("active = %v, want [T002 T003]", active)
	}
}

func TestExtractFailedTasks(t *testing.T) {
	tasks := map[string]interface{}{
		"T001": map[string]interface{}{"id": "T001", "status": "complete"},
		"T002": map[string]interface{}{"id": "T002", "status": "failed"},
		"T003": map[string]interface{}{"id": "T003", "status": "running"},
		"T004": map[string]interface{}{"id": "T004", "status": "failed"},
	}

	failed := ExtractFailedTasks(tasks)

	if len(failed) != 2 {
		t.Fatalf("len(failed) = %d, want 2", len(failed))
	}

	if failed[0] != "T002" || failed[1] != "T004" {
		t.Errorf("failed = %v, want [T002 T004]", failed)
	}
}

func TestBuildProgressBar(t *testing.T) {
	tests := []struct {
		pct      int
		width    int
		expected string
	}{
		{0, 10, "[----------]"},
		{50, 10, "[#####-----]"},
		{100, 10, "[##########]"},
		{25, 20, "[#####---------------]"},
		{-10, 10, "[----------]"},
		{150, 10, "[##########]"},
	}

	for _, tc := range tests {
		result := buildProgressBar(tc.pct, tc.width)
		if result != tc.expected {
			t.Errorf("buildProgressBar(%d, %d) = %q, want %q", tc.pct, tc.width, result, tc.expected)
		}
	}
}
