package orchestrator_test

import (
	"testing"

	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func threeTaskTasks(statuses ...types.Status) *types.Tasks {
	tasks := []types.Task{
		{ID: "T1", Type: types.TaskTypeFeature, Status: statuses[0]},
		{ID: "T2", Type: types.TaskTypeFeature, Status: statuses[1]},
		{ID: "T3", Type: types.TaskTypeFeature, Status: statuses[2]},
	}
	return &types.Tasks{
		Epic: types.EpicDefinition{
			ID:    "EPIC-X",
			Name:  "Test Epic",
			Tasks: tasks,
		},
	}
}

func allDoneTasks() *types.Tasks {
	return &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "T1", Type: types.TaskTypeFeature, Status: types.StatusDone},
				{ID: "T2", Type: types.TaskTypeFeature, Status: types.StatusDone},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// InitializeTaskPointers
// ---------------------------------------------------------------------------

func TestInitializeTaskPointers_PrefersInProgress(t *testing.T) {
	state := &types.ProjectState{KBEnabled: true}
	tasks := threeTaskTasks(types.StatusDone, types.StatusInProgress, types.StatusTODO)

	orchestrator.InitializeTaskPointers(state, tasks)

	if state.ActiveTask.ID != "T2" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "T2")
	}
	if state.ActiveTask.Type != types.TaskTypeFeature {
		t.Errorf("ActiveTask.Type: got %q, want %q", state.ActiveTask.Type, types.TaskTypeFeature)
	}
	if state.NextTask.ID != "T3" {
		t.Errorf("NextTask.ID: got %q, want %q", state.NextTask.ID, "T3")
	}
}

func TestInitializeTaskPointers_FallsBackToTODO(t *testing.T) {
	state := &types.ProjectState{KBEnabled: true}
	tasks := threeTaskTasks(types.StatusDone, types.StatusTODO, types.StatusTODO)

	orchestrator.InitializeTaskPointers(state, tasks)

	if state.ActiveTask.ID != "T2" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "T2")
	}
	if state.NextTask.ID != "T3" {
		t.Errorf("NextTask.ID: got %q, want %q", state.NextTask.ID, "T3")
	}
}

func TestInitializeTaskPointers_FirstTaskActive(t *testing.T) {
	state := &types.ProjectState{KBEnabled: true}
	tasks := threeTaskTasks(types.StatusTODO, types.StatusTODO, types.StatusTODO)

	orchestrator.InitializeTaskPointers(state, tasks)

	if state.ActiveTask.ID != "T1" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "T1")
	}
	if state.NextTask.ID != "T2" {
		t.Errorf("NextTask.ID: got %q, want %q", state.NextTask.ID, "T2")
	}
}

func TestInitializeTaskPointers_LastTask_NoNext(t *testing.T) {
	state := &types.ProjectState{KBEnabled: true}
	tasks := threeTaskTasks(types.StatusDone, types.StatusDone, types.StatusTODO)

	orchestrator.InitializeTaskPointers(state, tasks)

	if state.ActiveTask.ID != "T3" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "T3")
	}
	if state.NextTask.ID != "" {
		t.Errorf("NextTask.ID: got %q, want empty (last task)", state.NextTask.ID)
	}
}

func TestInitializeTaskPointers_KBSynthesisTrigger(t *testing.T) {
	state := &types.ProjectState{KBEnabled: true}
	tasks := allDoneTasks()

	orchestrator.InitializeTaskPointers(state, tasks)

	if state.ActiveTask.ID != "KB_UPDATE" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "KB_UPDATE")
	}
	if state.ActiveTask.Type != types.TaskTypeDocumentation {
		t.Errorf("ActiveTask.Type: got %q, want %q", state.ActiveTask.Type, types.TaskTypeDocumentation)
	}
	if state.NextTask.ID != "" {
		t.Errorf("NextTask.ID: got %q, want empty after KB_UPDATE injection", state.NextTask.ID)
	}
}

func TestInitializeTaskPointers_SyntheticActiveTask_NotClobbered(t *testing.T) {
	// A state with a synthetic active task (bugfix) should be returned unchanged.
	// Scanning tasks.yaml for IN_PROGRESS/TODO would find a user task and
	// overwrite the synthetic pointer — the guard prevents that.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeBugfix,
			ID:       "BUG-EPIC-1-001",
			Attempts: 2,
		},
		NextTask: types.TaskPointer{
			Type: types.TaskTypeFeature,
			ID:   "T2",
		},
	}
	tasks := threeTaskTasks(types.StatusInProgress, types.StatusTODO, types.StatusTODO)

	orchestrator.InitializeTaskPointers(state, tasks)

	// Active task must be unchanged.
	if state.ActiveTask.ID != "BUG-EPIC-1-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "BUG-EPIC-1-001")
	}
	if state.ActiveTask.Type != types.TaskTypeBugfix {
		t.Errorf("ActiveTask.Type: got %q, want %q", state.ActiveTask.Type, types.TaskTypeBugfix)
	}
	if state.ActiveTask.Attempts != 2 {
		t.Errorf("ActiveTask.Attempts: got %d, want 2", state.ActiveTask.Attempts)
	}
	// NextTask must also be unchanged.
	if state.NextTask.ID != "T2" {
		t.Errorf("NextTask.ID: got %q, want %q", state.NextTask.ID, "T2")
	}
}

func TestInitializeTaskPointers_KBDisabled_NoSynthetic(t *testing.T) {
	state := &types.ProjectState{KBEnabled: false}
	tasks := allDoneTasks()

	orchestrator.InitializeTaskPointers(state, tasks)

	// No KB synthesis when disabled; active task should remain zero value.
	if state.ActiveTask.ID == "KB_UPDATE" {
		t.Error("InitializeTaskPointers: should not inject KB_UPDATE when kb_enabled=false")
	}
}

// ---------------------------------------------------------------------------
// AdvanceToNextTask
// ---------------------------------------------------------------------------

func TestAdvanceToNextTask_ThroughThreeTasks(t *testing.T) {
	// Tasks: T1=IN_PROGRESS, T2=TODO, T3=TODO
	// State: active=T1, next=T2
	tasks := threeTaskTasks(types.StatusInProgress, types.StatusTODO, types.StatusTODO)
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "T1", Attempts: 3},
		NextTask:   types.TaskPointer{Type: types.TaskTypeFeature, ID: "T2"},
	}

	// Advance 1: T1 → T2, T3 becomes next.
	ok := orchestrator.AdvanceToNextTask(state, tasks)
	if !ok {
		t.Fatal("AdvanceToNextTask: expected true on first advance, got false")
	}
	if state.ActiveTask.ID != "T2" {
		t.Errorf("after advance 1, ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "T2")
	}
	if state.ActiveTask.Attempts != 0 {
		t.Errorf("after advance 1, Attempts: got %d, want 0 (reset on promotion)", state.ActiveTask.Attempts)
	}
	if state.NextTask.ID != "T3" {
		t.Errorf("after advance 1, NextTask.ID: got %q, want %q", state.NextTask.ID, "T3")
	}

	// Advance 2: T2 → T3, no next.
	ok = orchestrator.AdvanceToNextTask(state, tasks)
	if !ok {
		t.Fatal("AdvanceToNextTask: expected true on second advance, got false")
	}
	if state.ActiveTask.ID != "T3" {
		t.Errorf("after advance 2, ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "T3")
	}
	if state.NextTask.ID != "" {
		t.Errorf("after advance 2, NextTask.ID: got %q, want empty (T3 is last)", state.NextTask.ID)
	}
}

func TestAdvanceToNextTask_FromLastTask_NoNext(t *testing.T) {
	// State where active is the last task and next is empty.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "T3"},
		NextTask:   types.TaskPointer{}, // empty — no next task
	}
	tasks := threeTaskTasks(types.StatusDone, types.StatusDone, types.StatusInProgress)

	ok := orchestrator.AdvanceToNextTask(state, tasks)
	if ok {
		t.Error("AdvanceToNextTask: expected false when next_task is empty, got true")
	}
	// State should be unchanged.
	if state.ActiveTask.ID != "T3" {
		t.Errorf("ActiveTask.ID should be unchanged: got %q, want %q", state.ActiveTask.ID, "T3")
	}
}

func TestAdvanceToNextTask_ResetsAttempts(t *testing.T) {
	tasks := threeTaskTasks(types.StatusInProgress, types.StatusTODO, types.StatusTODO)
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "T1", Attempts: 5},
		NextTask:   types.TaskPointer{Type: types.TaskTypeFeature, ID: "T2"},
	}

	orchestrator.AdvanceToNextTask(state, tasks)

	if state.ActiveTask.Attempts != 0 {
		t.Errorf("Attempts after advance: got %d, want 0", state.ActiveTask.Attempts)
	}
}

// ---------------------------------------------------------------------------
// FindNextActiveTask
// ---------------------------------------------------------------------------

func TestFindNextActiveTask_PrefersInProgress(t *testing.T) {
	tasks := threeTaskTasks(types.StatusDone, types.StatusInProgress, types.StatusTODO)

	id, taskType := orchestrator.FindNextActiveTask(tasks)

	if id != "T2" {
		t.Errorf("FindNextActiveTask: id: got %q, want %q", id, "T2")
	}
	if taskType != types.TaskTypeFeature {
		t.Errorf("FindNextActiveTask: type: got %q, want %q", taskType, types.TaskTypeFeature)
	}
}

func TestFindNextActiveTask_FirstTODOWhenNoneInProgress(t *testing.T) {
	tasks := threeTaskTasks(types.StatusDone, types.StatusTODO, types.StatusTODO)

	id, taskType := orchestrator.FindNextActiveTask(tasks)

	if id != "T2" {
		t.Errorf("FindNextActiveTask: id: got %q, want %q", id, "T2")
	}
	if taskType != types.TaskTypeFeature {
		t.Errorf("FindNextActiveTask: type: got %q, want %q", taskType, types.TaskTypeFeature)
	}
}

func TestFindNextActiveTask_NoneAvailable(t *testing.T) {
	tasks := allDoneTasks()

	id, taskType := orchestrator.FindNextActiveTask(tasks)

	if id != "" {
		t.Errorf("FindNextActiveTask: id: got %q, want empty", id)
	}
	if taskType != "" {
		t.Errorf("FindNextActiveTask: type: got %q, want empty", taskType)
	}
}

// ---------------------------------------------------------------------------
// IncrementAttempts
// ---------------------------------------------------------------------------

func TestIncrementAttempts(t *testing.T) {
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Attempts: 2},
	}

	orchestrator.IncrementAttempts(state)

	if state.ActiveTask.Attempts != 3 {
		t.Errorf("Attempts: got %d, want 3", state.ActiveTask.Attempts)
	}
}

func TestIncrementAttempts_FromZero(t *testing.T) {
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Attempts: 0},
	}

	orchestrator.IncrementAttempts(state)

	if state.ActiveTask.Attempts != 1 {
		t.Errorf("Attempts: got %d, want 1", state.ActiveTask.Attempts)
	}
}

// ---------------------------------------------------------------------------
// UpdateTaskStatus
// ---------------------------------------------------------------------------

func TestUpdateTaskStatus_Success(t *testing.T) {
	tasks := threeTaskTasks(types.StatusTODO, types.StatusTODO, types.StatusTODO)

	err := orchestrator.UpdateTaskStatus(tasks, "T2", types.StatusInProgress)
	if err != nil {
		t.Fatalf("UpdateTaskStatus: unexpected error: %v", err)
	}

	if tasks.Epic.Tasks[1].Status != types.StatusInProgress {
		t.Errorf("T2 status: got %q, want %q", tasks.Epic.Tasks[1].Status, types.StatusInProgress)
	}
	// Other tasks should be unchanged.
	if tasks.Epic.Tasks[0].Status != types.StatusTODO {
		t.Errorf("T1 status: got %q, want TODO (should be unchanged)", tasks.Epic.Tasks[0].Status)
	}
	if tasks.Epic.Tasks[2].Status != types.StatusTODO {
		t.Errorf("T3 status: got %q, want TODO (should be unchanged)", tasks.Epic.Tasks[2].Status)
	}
}

func TestUpdateTaskStatus_UnknownID(t *testing.T) {
	tasks := threeTaskTasks(types.StatusTODO, types.StatusTODO, types.StatusTODO)

	err := orchestrator.UpdateTaskStatus(tasks, "DOES_NOT_EXIST", types.StatusDone)
	if err == nil {
		t.Fatal("UpdateTaskStatus: expected error for unknown ID, got nil")
	}
}

func TestUpdateTaskStatus_ToDone(t *testing.T) {
	tasks := threeTaskTasks(types.StatusInProgress, types.StatusTODO, types.StatusTODO)

	err := orchestrator.UpdateTaskStatus(tasks, "T1", types.StatusDone)
	if err != nil {
		t.Fatalf("UpdateTaskStatus: unexpected error: %v", err)
	}

	if tasks.Epic.Tasks[0].Status != types.StatusDone {
		t.Errorf("T1 status: got %q, want DONE", tasks.Epic.Tasks[0].Status)
	}
}
