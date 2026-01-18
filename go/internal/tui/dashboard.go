package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dgordon/tasker/internal/state"
)

var (
	dashboardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				MarginBottom(1)

	dashboardSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")).
				MarginTop(1)

	metricLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	metricValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255"))

	phaseStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))

	progressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	progressBarFillStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46"))

	statusIndicators = map[string]string{
		"pending":  "○",
		"ready":    "◎",
		"running":  "●",
		"complete": "✓",
		"failed":   "✗",
		"blocked":  "⊘",
		"skipped":  "⊖",
	}
)

type DashboardMetrics struct {
	TotalTasks    int
	Completed     int
	Running       int
	Failed        int
	Pending       int
	Ready         int
	Blocked       int
	Skipped       int
	CurrentPhase  string
	PhaseProgress map[int]PhaseMetrics
}

type PhaseMetrics struct {
	Phase     int
	Total     int
	Completed int
	Running   int
	Failed    int
}

// CalculateMetrics computes dashboard metrics from state
func CalculateMetrics(s *state.State) DashboardMetrics {
	metrics := DashboardMetrics{
		TotalTasks:    len(s.Tasks),
		CurrentPhase:  s.Phase.Current,
		PhaseProgress: make(map[int]PhaseMetrics),
	}

	for _, task := range s.Tasks {
		switch task.Status {
		case "complete":
			metrics.Completed++
		case "running":
			metrics.Running++
		case "failed":
			metrics.Failed++
		case "pending":
			metrics.Pending++
		case "ready":
			metrics.Ready++
		case "blocked":
			metrics.Blocked++
		case "skipped":
			metrics.Skipped++
		}

		pm := metrics.PhaseProgress[task.Phase]
		pm.Phase = task.Phase
		pm.Total++
		switch task.Status {
		case "complete":
			pm.Completed++
		case "running":
			pm.Running++
		case "failed":
			pm.Failed++
		}
		metrics.PhaseProgress[task.Phase] = pm
	}

	return metrics
}

// RenderProgressBar creates a visual progress bar
func RenderProgressBar(completed, total, width int) string {
	if total == 0 || width <= 0 {
		return ""
	}

	fillWidth := (completed * width) / total
	emptyWidth := width - fillWidth

	filled := progressBarFillStyle.Render(strings.Repeat("█", fillWidth))
	empty := progressBarEmptyStyle.Render(strings.Repeat("░", emptyWidth))

	return filled + empty
}

// RenderDashboard creates the dashboard view (behavior B63)
func RenderDashboard(s *state.State, tasks []state.Task, width int) string {
	if s == nil {
		return "No state available"
	}

	var b strings.Builder
	metrics := CalculateMetrics(s)

	b.WriteString(dashboardTitleStyle.Render("Tasker Dashboard"))
	b.WriteString("\n\n")

	b.WriteString(dashboardSectionStyle.Render("Overall Progress"))
	b.WriteString("\n")

	completionPct := 0
	if metrics.TotalTasks > 0 {
		completionPct = (metrics.Completed * 100) / metrics.TotalTasks
	}

	barWidth := 30
	if width > 60 {
		barWidth = 40
	}
	progressBar := RenderProgressBar(metrics.Completed, metrics.TotalTasks, barWidth)
	b.WriteString(fmt.Sprintf("%s %d%% (%d/%d)\n",
		progressBar,
		completionPct,
		metrics.Completed,
		metrics.TotalTasks))

	b.WriteString("\n")
	b.WriteString(metricLabelStyle.Render("Phase: "))
	b.WriteString(phaseStyle.Render(metrics.CurrentPhase))
	b.WriteString("\n\n")

	b.WriteString(dashboardSectionStyle.Render("Status Summary"))
	b.WriteString("\n")

	statusLine := fmt.Sprintf("%s %s %d  %s %s %d  %s %s %d  %s %s %d",
		statusStyles["complete"].Render(statusIndicators["complete"]),
		metricLabelStyle.Render("Complete:"),
		metrics.Completed,
		statusStyles["running"].Render(statusIndicators["running"]),
		metricLabelStyle.Render("Running:"),
		metrics.Running,
		statusStyles["failed"].Render(statusIndicators["failed"]),
		metricLabelStyle.Render("Failed:"),
		metrics.Failed,
		statusStyles["ready"].Render(statusIndicators["ready"]),
		metricLabelStyle.Render("Ready:"),
		metrics.Ready,
	)
	b.WriteString(statusLine)
	b.WriteString("\n")

	if metrics.Pending > 0 || metrics.Blocked > 0 || metrics.Skipped > 0 {
		statusLine2 := fmt.Sprintf("%s %s %d  %s %s %d  %s %s %d",
			statusStyles["pending"].Render(statusIndicators["pending"]),
			metricLabelStyle.Render("Pending:"),
			metrics.Pending,
			statusStyles["blocked"].Render(statusIndicators["blocked"]),
			metricLabelStyle.Render("Blocked:"),
			metrics.Blocked,
			statusStyles["skipped"].Render(statusIndicators["skipped"]),
			metricLabelStyle.Render("Skipped:"),
			metrics.Skipped,
		)
		b.WriteString(statusLine2)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dashboardSectionStyle.Render("Phase Progress"))
	b.WriteString("\n")

	phases := getSortedPhases(metrics.PhaseProgress)
	for _, phase := range phases {
		pm := metrics.PhaseProgress[phase]
		phaseBar := RenderProgressBar(pm.Completed, pm.Total, 20)
		phasePct := 0
		if pm.Total > 0 {
			phasePct = (pm.Completed * 100) / pm.Total
		}

		phaseLabel := fmt.Sprintf("Phase %d", phase)
		b.WriteString(fmt.Sprintf("  %s  %s %3d%% (%d/%d)",
			metricLabelStyle.Render(fmt.Sprintf("%-8s", phaseLabel)),
			phaseBar,
			phasePct,
			pm.Completed,
			pm.Total))

		if pm.Running > 0 {
			b.WriteString(fmt.Sprintf("  %s %d active",
				statusStyles["running"].Render(statusIndicators["running"]),
				pm.Running))
		}
		if pm.Failed > 0 {
			b.WriteString(fmt.Sprintf("  %s %d failed",
				statusStyles["failed"].Render(statusIndicators["failed"]),
				pm.Failed))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dashboardSectionStyle.Render("Tasks"))
	b.WriteString("\n")

	maxTasks := 10
	if len(tasks) < maxTasks {
		maxTasks = len(tasks)
	}

	for i := 0; i < maxTasks; i++ {
		task := tasks[i]
		indicator := statusIndicators[task.Status]
		if indicator == "" {
			indicator = "?"
		}

		statusStyle, ok := statusStyles[task.Status]
		if !ok {
			statusStyle = normalStyle
		}

		line := fmt.Sprintf("  %s %s [%s] %s",
			statusStyle.Render(indicator),
			normalStyle.Render(task.ID),
			statusStyle.Render(task.Status),
			normalStyle.Render(truncateString(task.Name, 40)),
		)
		b.WriteString(line + "\n")
	}

	if len(tasks) > maxTasks {
		b.WriteString(fmt.Sprintf("  ... and %d more tasks\n", len(tasks)-maxTasks))
	}

	return b.String()
}

// getSortedPhases returns phase numbers in sorted order
func getSortedPhases(pm map[int]PhaseMetrics) []int {
	phases := make([]int, 0, len(pm))
	for p := range pm {
		phases = append(phases, p)
	}

	for i := 0; i < len(phases)-1; i++ {
		for j := i + 1; j < len(phases); j++ {
			if phases[i] > phases[j] {
				phases[i], phases[j] = phases[j], phases[i]
			}
		}
	}

	return phases
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
