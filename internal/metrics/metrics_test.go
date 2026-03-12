package metrics_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/metrics"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func emptyState() *types.ProjectState {
	return &types.ProjectState{}
}

// ---------------------------------------------------------------------------
// RecordTaskMetrics
// ---------------------------------------------------------------------------

func TestRecordTaskMetrics_AppendsEntry(t *testing.T) {
	state := emptyState()

	metrics.RecordTaskMetrics(state, "EPIC-1-001", "SUCCESS", 120, 2, "feature", 95)

	if len(state.Metrics.Tasks) != 1 {
		t.Fatalf("Tasks len: got %d, want 1", len(state.Metrics.Tasks))
	}
	m := state.Metrics.Tasks[0]
	if m.TaskID != "EPIC-1-001" {
		t.Errorf("TaskID: got %q, want %q", m.TaskID, "EPIC-1-001")
	}
	if m.Outcome != "SUCCESS" {
		t.Errorf("Outcome: got %q, want %q", m.Outcome, "SUCCESS")
	}
	if m.DurationSeconds != 120 {
		t.Errorf("DurationSeconds: got %d, want 120", m.DurationSeconds)
	}
	if m.CompletedAt == "" {
		t.Error("CompletedAt should not be empty")
	}
	if !strings.Contains(m.CompletedAt, "T") {
		t.Errorf("CompletedAt does not look like RFC3339: %q", m.CompletedAt)
	}
	if m.Attempts != 2 {
		t.Errorf("Attempts: got %d, want 2", m.Attempts)
	}
	if m.TaskType != "feature" {
		t.Errorf("TaskType: got %q, want %q", m.TaskType, "feature")
	}
	if m.AgentDurationSeconds != 95 {
		t.Errorf("AgentDurationSeconds: got %d, want 95", m.AgentDurationSeconds)
	}
}

func TestRecordTaskMetrics_CallsUpdateMetricTotals(t *testing.T) {
	state := emptyState()

	metrics.RecordTaskMetrics(state, "T1", "SUCCESS", 100, 1, "feature", 90)
	metrics.RecordTaskMetrics(state, "T2", "SUCCESS", 200, 1, "feature", 180)

	if state.Metrics.TotalTasksCompleted != 2 {
		t.Errorf("TotalTasksCompleted: got %d, want 2", state.Metrics.TotalTasksCompleted)
	}
	if state.Metrics.TotalDurationSeconds != 300 {
		t.Errorf("TotalDurationSeconds: got %d, want 300", state.Metrics.TotalDurationSeconds)
	}
}

func TestRecordTaskMetrics_MultipleAppends(t *testing.T) {
	state := emptyState()

	metrics.RecordTaskMetrics(state, "T1", "SUCCESS", 60, 1, "feature", 55)
	metrics.RecordTaskMetrics(state, "T2", "FAILURE", 90, 2, "feature", 85)
	metrics.RecordTaskMetrics(state, "T3", "SUCCESS", 30, 3, "bugfix", 28)

	if len(state.Metrics.Tasks) != 3 {
		t.Fatalf("Tasks len: got %d, want 3", len(state.Metrics.Tasks))
	}
	if state.Metrics.Tasks[1].Outcome != "FAILURE" {
		t.Errorf("Tasks[1].Outcome: got %q, want %q", state.Metrics.Tasks[1].Outcome, "FAILURE")
	}
}

// ---------------------------------------------------------------------------
// UpdateMetricTotals
// ---------------------------------------------------------------------------

func TestUpdateMetricTotals_EmptyTasks(t *testing.T) {
	state := emptyState()

	metrics.UpdateMetricTotals(state)

	if state.Metrics.TotalTasksCompleted != 0 {
		t.Errorf("TotalTasksCompleted: got %d, want 0", state.Metrics.TotalTasksCompleted)
	}
	if state.Metrics.TotalDurationSeconds != 0 {
		t.Errorf("TotalDurationSeconds: got %d, want 0", state.Metrics.TotalDurationSeconds)
	}
}

func TestUpdateMetricTotals_SumsCorrectly(t *testing.T) {
	state := &types.ProjectState{
		Metrics: types.Metrics{
			Tasks: []types.TaskMetric{
				{TaskID: "T1", DurationSeconds: 100},
				{TaskID: "T2", DurationSeconds: 200},
				{TaskID: "T3", DurationSeconds: 50},
			},
		},
	}

	metrics.UpdateMetricTotals(state)

	if state.Metrics.TotalTasksCompleted != 3 {
		t.Errorf("TotalTasksCompleted: got %d, want 3", state.Metrics.TotalTasksCompleted)
	}
	if state.Metrics.TotalDurationSeconds != 350 {
		t.Errorf("TotalDurationSeconds: got %d, want 350", state.Metrics.TotalDurationSeconds)
	}
}

func TestUpdateMetricTotals_OverwritesPreviousTotals(t *testing.T) {
	state := &types.ProjectState{
		Metrics: types.Metrics{
			TotalTasksCompleted:  99,
			TotalDurationSeconds: 9999,
			Tasks: []types.TaskMetric{
				{TaskID: "T1", DurationSeconds: 10},
			},
		},
	}

	metrics.UpdateMetricTotals(state)

	if state.Metrics.TotalTasksCompleted != 1 {
		t.Errorf("TotalTasksCompleted: got %d, want 1", state.Metrics.TotalTasksCompleted)
	}
	if state.Metrics.TotalDurationSeconds != 10 {
		t.Errorf("TotalDurationSeconds: got %d, want 10", state.Metrics.TotalDurationSeconds)
	}
}

// ---------------------------------------------------------------------------
// PrintEpicSummary (smoke test — just ensure no panic)
// ---------------------------------------------------------------------------

func TestPrintEpicSummary_NoTasks(t *testing.T) {
	state := emptyState()
	// Should not panic when there are no tasks (zero-division guard).
	metrics.PrintEpicSummary(state)
}

func TestPrintEpicSummary_WithTasks(t *testing.T) {
	state := &types.ProjectState{
		Metrics: types.Metrics{
			TotalTasksCompleted:  3,
			TotalDurationSeconds: 3661, // 1h 1m 1s
			Tasks: []types.TaskMetric{
				{TaskID: "T1", DurationSeconds: 1000},
				{TaskID: "T2", DurationSeconds: 1000},
				{TaskID: "T3", DurationSeconds: 1661},
			},
		},
	}
	// Should not panic with non-zero totals.
	metrics.PrintEpicSummary(state)
}
