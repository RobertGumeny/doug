// Package metrics provides task metric recording and epic summary reporting
// for the doug orchestrator.
package metrics

import (
	"fmt"
	"io"
	"time"

	"github.com/robertgumeny/doug/internal/types"
)

// RecordTaskMetrics appends a TaskMetric for the completed task to
// state.Metrics.Tasks and calls UpdateMetricTotals to refresh the totals.
//
// Metric recording is non-fatal by design: if the caller encounters an error
// after this call, it should log a warning rather than failing the task.
func RecordTaskMetrics(state *types.ProjectState, taskID string, outcome string, durationSeconds int, attempts int, taskType string, agentDurationSeconds int, providerWaitMs int64, providerFailures []types.ProviderFailure) {
	metric := types.TaskMetric{
		TaskID:               taskID,
		Outcome:              outcome,
		DurationSeconds:      durationSeconds,
		CompletedAt:          time.Now().UTC().Format(time.RFC3339),
		Attempts:             attempts,
		TaskType:             taskType,
		AgentDurationSeconds: agentDurationSeconds,
		ProviderWaitMs:       providerWaitMs,
		ProviderFailures:     cloneProviderFailures(providerFailures),
	}
	state.Metrics.Tasks = append(state.Metrics.Tasks, metric)
	UpdateMetricTotals(state)
}

func cloneProviderFailures(in []types.ProviderFailure) []types.ProviderFailure {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.ProviderFailure, len(in))
	copy(out, in)
	return out
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

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// PrintEpicSummary prints a box-draw table to w summarizing the completed
// epic: total tasks, total wall time (formatted as h/m/s), and average time
// per task.
func PrintEpicSummary(w io.Writer, state *types.ProjectState) {
	total := state.Metrics.TotalTasksCompleted
	totalSec := state.Metrics.TotalDurationSeconds

	avgSec := 0
	if total > 0 {
		avgSec = totalSec / total
	}

	totalFmt := formatDuration(totalSec)
	avgFmt := fmt.Sprintf("%ds per task", avgSec)

	const line = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	writef(w, "\n%s\n", line)
	writef(w, "EPIC SUMMARY\n")
	writef(w, "%s\n", line)
	writef(w, "  %-22s %d\n", "Total Tasks:", total)
	writef(w, "  %-22s %s\n", "Total Time:", totalFmt)
	writef(w, "  %-22s %s\n", "Average Time:", avgFmt)
	writef(w, "%s\n\n", line)
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
