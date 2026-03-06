package orchestrator_test

import (
	"testing"

	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

func TestPrepareForEpicRollover_NoCurrentEpic_NoOp(t *testing.T) {
	state := &types.ProjectState{}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{ID: "EPIC-2"},
	}

	rolled, err := orchestrator.PrepareForEpicRollover(state, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rolled {
		t.Fatal("expected no rollover on first run")
	}
}

func TestPrepareForEpicRollover_SameEpic_NoOp(t *testing.T) {
	completedAt := "2026-03-06T00:00:00Z"
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:          "EPIC-1",
			CompletedAt: &completedAt,
		},
		ActiveTask: types.TaskPointer{Type: types.TaskTypeDocumentation, ID: "KB_UPDATE"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{ID: "EPIC-1"},
	}

	rolled, err := orchestrator.PrepareForEpicRollover(state, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rolled {
		t.Fatal("expected no rollover for same epic ID")
	}
	if state.ActiveTask.ID != "KB_UPDATE" {
		t.Fatalf("state should be unchanged; ActiveTask.ID=%q", state.ActiveTask.ID)
	}
}

func TestPrepareForEpicRollover_NewEpicBlockedWhenIncomplete(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID: "EPIC-1",
		},
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-1-001", Attempts: 2},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{ID: "EPIC-2"},
	}

	rolled, err := orchestrator.PrepareForEpicRollover(state, tasks)
	if err == nil {
		t.Fatal("expected error when previous epic is incomplete")
	}
	if rolled {
		t.Fatal("rollover should not occur when blocked")
	}
}

func TestPrepareForEpicRollover_NewEpicResetsRuntimeState(t *testing.T) {
	completedAt := "2026-03-06T00:00:00Z"
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:          "EPIC-1",
			Name:        "Old Epic",
			BranchName:  "feature/EPIC-1",
			StartedAt:   "2026-03-01T00:00:00Z",
			CompletedAt: &completedAt,
		},
		ActiveTask: types.TaskPointer{Type: types.TaskTypeDocumentation, ID: "KB_UPDATE", Attempts: 1},
		NextTask:   types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-1-999"},
		Metrics: types.Metrics{
			TotalTasksCompleted: 3,
			Tasks: []types.TaskMetric{
				{TaskID: "EPIC-1-001", Outcome: "success"},
			},
		},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{ID: "EPIC-2"},
	}

	rolled, err := orchestrator.PrepareForEpicRollover(state, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rolled {
		t.Fatal("expected rollover reset")
	}
	if state.CurrentEpic.ID != "" {
		t.Fatalf("CurrentEpic should be reset; got ID=%q", state.CurrentEpic.ID)
	}
	if state.ActiveTask.ID != "" || state.NextTask.ID != "" {
		t.Fatalf("task pointers should be reset; active=%q next=%q", state.ActiveTask.ID, state.NextTask.ID)
	}
	if state.Metrics.TotalTasksCompleted != 0 || len(state.Metrics.Tasks) != 0 {
		t.Fatalf("metrics should be reset; got total=%d len=%d", state.Metrics.TotalTasksCompleted, len(state.Metrics.Tasks))
	}
}
