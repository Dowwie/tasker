package util

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// StatusCounts represents task counts by status.
type StatusCounts struct {
	Pending   int
	Ready     int
	Running   int
	Complete  int
	Failed    int
	Blocked   int
	Skipped   int
	Total     int
}

// DashboardData contains data needed for dashboard display.
type DashboardData struct {
	Phase         string
	TargetDir     string
	StatusCounts  StatusCounts
	CurrentPhase  int
	TotalPhases   int
	ActiveTasks   []string
	RecentFailed  []string
}

// StatusData contains data for compact status output.
type StatusData struct {
	Phase        string
	Complete     int
	Total        int
	Running      int
	Failed       int
	IsHalted     bool
	HaltReason   string
}

// ShowDashboard writes a dashboard display of task progress with status breakdown.
func ShowDashboard(w io.Writer, data DashboardData) {
	fmt.Fprintln(w, strings.Repeat("=", 60))
	fmt.Fprintln(w, "                    TASK DASHBOARD")
	fmt.Fprintln(w, strings.Repeat("=", 60))
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Phase:      %s\n", data.Phase)
	fmt.Fprintf(w, "Target:     %s\n", data.TargetDir)
	fmt.Fprintln(w)

	fmt.Fprintln(w, strings.Repeat("-", 60))
	fmt.Fprintln(w, "PROGRESS")
	fmt.Fprintln(w, strings.Repeat("-", 60))

	sc := data.StatusCounts
	if sc.Total > 0 {
		pct := float64(sc.Complete) / float64(sc.Total) * 100
		progressBar := buildProgressBar(int(pct), 40)
		fmt.Fprintf(w, "%s %.1f%%\n", progressBar, pct)
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Status Breakdown:")
	fmt.Fprintf(w, "  Complete:  %3d\n", sc.Complete)
	fmt.Fprintf(w, "  Running:   %3d\n", sc.Running)
	fmt.Fprintf(w, "  Ready:     %3d\n", sc.Ready)
	fmt.Fprintf(w, "  Pending:   %3d\n", sc.Pending)
	fmt.Fprintf(w, "  Failed:    %3d\n", sc.Failed)
	fmt.Fprintf(w, "  Blocked:   %3d\n", sc.Blocked)
	fmt.Fprintf(w, "  Skipped:   %3d\n", sc.Skipped)
	fmt.Fprintf(w, "  ---------\n")
	fmt.Fprintf(w, "  Total:     %3d\n", sc.Total)
	fmt.Fprintln(w)

	if len(data.ActiveTasks) > 0 {
		fmt.Fprintln(w, strings.Repeat("-", 60))
		fmt.Fprintln(w, "ACTIVE TASKS")
		fmt.Fprintln(w, strings.Repeat("-", 60))
		for _, task := range data.ActiveTasks {
			fmt.Fprintf(w, "  - %s\n", task)
		}
		fmt.Fprintln(w)
	}

	if len(data.RecentFailed) > 0 {
		fmt.Fprintln(w, strings.Repeat("-", 60))
		fmt.Fprintln(w, "RECENT FAILURES")
		fmt.Fprintln(w, strings.Repeat("-", 60))
		for _, task := range data.RecentFailed {
			fmt.Fprintf(w, "  - %s\n", task)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, strings.Repeat("=", 60))
}

func buildProgressBar(pct int, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	empty := width - filled
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", empty) + "]"
}

// ShowStatus writes a compact status summary suitable for scripts.
// Format: PHASE complete/total running=N failed=N [HALTED: reason]
func ShowStatus(w io.Writer, data StatusData) {
	var sb strings.Builder

	sb.WriteString(strings.ToUpper(data.Phase))
	sb.WriteString(fmt.Sprintf(" %d/%d", data.Complete, data.Total))
	sb.WriteString(fmt.Sprintf(" running=%d", data.Running))
	sb.WriteString(fmt.Sprintf(" failed=%d", data.Failed))

	if data.IsHalted {
		sb.WriteString(" [HALTED")
		if data.HaltReason != "" {
			sb.WriteString(": ")
			sb.WriteString(data.HaltReason)
		}
		sb.WriteString("]")
	}

	fmt.Fprintln(w, sb.String())
}

// CountTasksByStatus counts tasks from a task map, where each task is a map with a "status" key.
func CountTasksByStatus(tasks map[string]interface{}) StatusCounts {
	counts := StatusCounts{}

	for _, task := range tasks {
		counts.Total++
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		status, ok := taskMap["status"].(string)
		if !ok {
			continue
		}

		switch status {
		case "pending":
			counts.Pending++
		case "ready":
			counts.Ready++
		case "running":
			counts.Running++
		case "complete":
			counts.Complete++
		case "failed":
			counts.Failed++
		case "blocked":
			counts.Blocked++
		case "skipped":
			counts.Skipped++
		}
	}

	return counts
}

// ExtractActiveTasks returns task IDs with "running" status from a task map.
func ExtractActiveTasks(tasks map[string]interface{}) []string {
	var active []string
	for id, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}
		if status, ok := taskMap["status"].(string); ok && status == "running" {
			active = append(active, id)
		}
	}
	sort.Strings(active)
	return active
}

// ExtractFailedTasks returns task IDs with "failed" status from a task map.
func ExtractFailedTasks(tasks map[string]interface{}) []string {
	var failed []string
	for id, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}
		if status, ok := taskMap["status"].(string); ok && status == "failed" {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}
