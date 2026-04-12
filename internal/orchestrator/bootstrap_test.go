package orchestrator_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func twoTaskTasks() *types.Tasks {
	return &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-3",
			Name: "State Management",
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
				{ID: "EPIC-3-002", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}
}

func singleTaskTasks() *types.Tasks {
	return &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-1",
			Name: "Scaffold",
			Tasks: []types.Task{
				{ID: "EPIC-1-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}
}

func freshState() *types.ProjectState {
	return &types.ProjectState{}
}

// ---------------------------------------------------------------------------
// BootstrapFromTasks
// ---------------------------------------------------------------------------

func TestBootstrapFromTasks_FreshState(t *testing.T) {
	state := freshState()
	tasks := twoTaskTasks()

	orchestrator.BootstrapFromTasks(state, tasks)

	if state.CurrentEpic.ID != "EPIC-3" {
		t.Errorf("CurrentEpic.ID: got %q, want %q", state.CurrentEpic.ID, "EPIC-3")
	}
	if state.CurrentEpic.Name != "State Management" {
		t.Errorf("CurrentEpic.Name: got %q, want %q", state.CurrentEpic.Name, "State Management")
	}
	if state.CurrentEpic.BranchName != "feature/EPIC-3" {
		t.Errorf("CurrentEpic.BranchName: got %q, want %q", state.CurrentEpic.BranchName, "feature/EPIC-3")
	}
	if state.CurrentEpic.StartedAt == "" {
		t.Error("CurrentEpic.StartedAt should not be empty after bootstrap")
	}
	// StartedAt must be in RFC3339 format (contains "T" and "Z" or offset)
	if !strings.Contains(state.CurrentEpic.StartedAt, "T") {
		t.Errorf("CurrentEpic.StartedAt does not look like RFC3339: %q", state.CurrentEpic.StartedAt)
	}

	if state.ActiveTask.ID != "EPIC-3-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "EPIC-3-001")
	}
	if state.ActiveTask.Type != types.TaskTypeFeature {
		t.Errorf("ActiveTask.Type: got %q, want %q", state.ActiveTask.Type, types.TaskTypeFeature)
	}
	if state.ActiveTask.Attempts != 0 {
		t.Errorf("ActiveTask.Attempts: got %d, want 0", state.ActiveTask.Attempts)
	}

	if state.NextTask.ID != "EPIC-3-002" {
		t.Errorf("NextTask.ID: got %q, want %q", state.NextTask.ID, "EPIC-3-002")
	}
	if state.NextTask.Type != types.TaskTypeFeature {
		t.Errorf("NextTask.Type: got %q, want %q", state.NextTask.Type, types.TaskTypeFeature)
	}
}

func TestBootstrapFromTasks_AlreadyBootstrapped(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:         "EPIC-3",
			Name:       "State Management",
			BranchName: "feature/EPIC-3",
			StartedAt:  "2026-02-24T20:01:28Z",
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeFeature,
			ID:       "EPIC-3-001",
			Attempts: 2,
		},
		NextTask: types.TaskPointer{
			Type: types.TaskTypeFeature,
			ID:   "EPIC-3-002",
		},
	}

	tasks := twoTaskTasks()

	// Capture snapshot before call
	origID := state.CurrentEpic.ID
	origName := state.CurrentEpic.Name
	origBranch := state.CurrentEpic.BranchName
	origStarted := state.CurrentEpic.StartedAt
	origActiveID := state.ActiveTask.ID
	origAttempts := state.ActiveTask.Attempts

	orchestrator.BootstrapFromTasks(state, tasks)

	if state.CurrentEpic.ID != origID {
		t.Errorf("CurrentEpic.ID changed: got %q, want %q", state.CurrentEpic.ID, origID)
	}
	if state.CurrentEpic.Name != origName {
		t.Errorf("CurrentEpic.Name changed: got %q, want %q", state.CurrentEpic.Name, origName)
	}
	if state.CurrentEpic.BranchName != origBranch {
		t.Errorf("CurrentEpic.BranchName changed: got %q, want %q", state.CurrentEpic.BranchName, origBranch)
	}
	if state.CurrentEpic.StartedAt != origStarted {
		t.Errorf("CurrentEpic.StartedAt changed: got %q, want %q", state.CurrentEpic.StartedAt, origStarted)
	}
	if state.ActiveTask.ID != origActiveID {
		t.Errorf("ActiveTask.ID changed: got %q, want %q", state.ActiveTask.ID, origActiveID)
	}
	if state.ActiveTask.Attempts != origAttempts {
		t.Errorf("ActiveTask.Attempts changed: got %d, want %d", state.ActiveTask.Attempts, origAttempts)
	}
}

func TestBootstrapFromTasks_SingleTaskEpic(t *testing.T) {
	state := freshState()
	tasks := singleTaskTasks()

	orchestrator.BootstrapFromTasks(state, tasks)

	if state.ActiveTask.ID != "EPIC-1-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "EPIC-1-001")
	}
	// next_task should be zero value (no second task)
	if state.NextTask.ID != "" {
		t.Errorf("NextTask.ID should be empty for single-task epic, got %q", state.NextTask.ID)
	}
	if state.NextTask.Type != "" {
		t.Errorf("NextTask.Type should be empty for single-task epic, got %q", state.NextTask.Type)
	}
}

// ---------------------------------------------------------------------------
// IsEpicAlreadyComplete
// ---------------------------------------------------------------------------

func TestIsEpicAlreadyComplete_FinalizedEpic(t *testing.T) {
	completedAt := "2026-03-10T00:00:00Z"
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{CompletedAt: &completedAt},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusDone},
			},
		},
	}
	if !orchestrator.IsEpicAlreadyComplete(state, tasks) {
		t.Error("IsEpicAlreadyComplete: want true when tasks are done, completed_at is set, and task pointers are empty")
	}
}

func TestIsEpicAlreadyComplete_NotAllDone(t *testing.T) {
	completedAt := "2026-03-10T00:00:00Z"
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{CompletedAt: &completedAt},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusInProgress},
			},
		},
	}
	if orchestrator.IsEpicAlreadyComplete(state, tasks) {
		t.Error("IsEpicAlreadyComplete: want false when not all tasks done")
	}
}

func TestIsEpicAlreadyComplete_CompletionTimestampMissing(t *testing.T) {
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusDone},
			},
		},
	}
	if orchestrator.IsEpicAlreadyComplete(state, tasks) {
		t.Error("IsEpicAlreadyComplete: want false when completed_at is missing")
	}
}

func TestIsEpicAlreadyComplete_RuntimePointersStillSet(t *testing.T) {
	completedAt := "2026-03-10T00:00:00Z"
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{CompletedAt: &completedAt},
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-002"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusDone},
				{ID: "EPIC-3-002", Status: types.StatusDone},
			},
		},
	}
	if orchestrator.IsEpicAlreadyComplete(state, tasks) {
		t.Error("IsEpicAlreadyComplete: want false when finalization has not cleared runtime pointers")
	}
}

func TestIsEpicAlreadyComplete_TasksStillPending(t *testing.T) {
	completedAt := "2026-03-10T00:00:00Z"
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{CompletedAt: &completedAt},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusTODO},
				{ID: "EPIC-3-002", Status: types.StatusTODO},
			},
		},
	}
	if orchestrator.IsEpicAlreadyComplete(state, tasks) {
		t.Error("IsEpicAlreadyComplete: want false when tasks still TODO")
	}
}
