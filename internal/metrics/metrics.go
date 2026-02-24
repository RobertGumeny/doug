// Package metrics provides task metric recording and epic summary reporting
// for the doug orchestrator.
package metrics

import (
	"fmt"
	"time"

	"github.com/robertgumeny/doug/internal/types"
)

// RecordTaskMetrics appends a TaskMetric for the completed task to
// state.Metrics.Tasks and calls UpdateMetricTotals to refresh the totals.
//
// Metric recording is non-fatal by design: if the caller encounters an error
// after this call, it should log a warning rather than failing the task.
func RecordTaskMetrics(state *types.ProjectState, taskID string, outcome string, durationSeconds int) {
	metric := types.TaskMetric{
		TaskID:          taskID,
		Outcome:         outcome,
		DurationSeconds: durationSeconds,
		CompletedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	state.Metrics.Tasks = append(state.Metrics.Tasks, metric)
	UpdateMetricTotals(state)
}

// UpdateMetricTotals recalculates TotalTasksCompleted and TotalDurationSeconds
// from the full Tasks slice in state.Metrics. It overwrites any previously
// stored totals, making it safe to call multiple times.
func UpdateMetricTotals(state *types.ProjectState) {
	total := 0
	for _, t := range state.Metrics.Tasks {
		total += t.DurationSeconds
	}
	state.Metrics.TotalTasksCompleted = len(state.Metrics.Tasks)
	state.Metrics.TotalDurationSeconds = total
}

// PrintEpicSummary prints a box-draw table to stdout summarizing the completed
// epic: total tasks, total wall time (formatted as h/m/s), and average time
// per task.
func PrintEpicSummary(state *types.ProjectState) {
	total := state.Metrics.TotalTasksCompleted
	totalSec := state.Metrics.TotalDurationSeconds

	avgSec := 0
	if total > 0 {
		avgSec = totalSec / total
	}

	totalFmt := formatDuration(totalSec)
	avgFmt := fmt.Sprintf("%ds per task", avgSec)

	const line = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	fmt.Printf("\n%s\n", line)
	fmt.Println("EPIC SUMMARY")
	fmt.Printf("%s\n", line)
	fmt.Printf("  %-22s %d\n", "Total Tasks:", total)
	fmt.Printf("  %-22s %s\n", "Total Time:", totalFmt)
	fmt.Printf("  %-22s %s\n", "Average Time:", avgFmt)
	fmt.Printf("%s\n\n", line)
}

// formatDuration converts a duration in seconds to a human-readable string.
// Examples: "0s", "45s", "3m 15s", "1h 2m 30s".
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "0s"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
