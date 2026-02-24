package orchestrator_test

import (
	"testing"

	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// ValidateYAMLStructure
// ---------------------------------------------------------------------------

func TestValidateYAMLStructure_Valid(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-3"},
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-001"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusTODO},
				{ID: "EPIC-3-002", Status: types.StatusInProgress},
				{ID: "EPIC-3-003", Status: types.StatusDone},
				{ID: "EPIC-3-004", Status: types.StatusBlocked},
			},
		},
	}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err != nil {
		t.Errorf("ValidateYAMLStructure: unexpected error for valid input: %v", err)
	}
}

func TestValidateYAMLStructure_MissingEpicID(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: ""}, // missing
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-001"},
	}
	tasks := &types.Tasks{}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err == nil {
		t.Error("ValidateYAMLStructure: expected error for missing current_epic.id, got nil")
	}
}

func TestValidateYAMLStructure_MissingActiveTaskType(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-3"},
		ActiveTask:  types.TaskPointer{Type: "", ID: "EPIC-3-001"}, // missing type
	}
	tasks := &types.Tasks{}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err == nil {
		t.Error("ValidateYAMLStructure: expected error for missing active_task.type, got nil")
	}
}

func TestValidateYAMLStructure_MissingActiveTaskID(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-3"},
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: ""}, // missing id
	}
	tasks := &types.Tasks{}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err == nil {
		t.Error("ValidateYAMLStructure: expected error for missing active_task.id, got nil")
	}
}

func TestValidateYAMLStructure_InvalidTaskStatus(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-3"},
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-001"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: "INVALID_STATUS"},
			},
		},
	}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err == nil {
		t.Error("ValidateYAMLStructure: expected error for invalid task status, got nil")
	}
}

func TestValidateYAMLStructure_EmptyTaskStatus(t *testing.T) {
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-3"},
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-001"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: ""}, // empty status
			},
		},
	}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err == nil {
		t.Error("ValidateYAMLStructure: expected error for empty task status, got nil")
	}
}

func TestValidateYAMLStructure_NoTasks(t *testing.T) {
	// No tasks in tasks.yaml is valid for structure purposes.
	state := &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-3"},
		ActiveTask:  types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-001"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{Tasks: []types.Task{}},
	}

	if err := orchestrator.ValidateYAMLStructure(state, tasks); err != nil {
		t.Errorf("ValidateYAMLStructure: unexpected error for empty task list: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateStateSync
// ---------------------------------------------------------------------------

func TestValidateStateSync_OK(t *testing.T) {
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-002"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Status: types.StatusDone},
				{ID: "EPIC-3-002", Status: types.StatusInProgress},
				{ID: "EPIC-3-003", Status: types.StatusTODO},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err != nil {
		t.Fatalf("ValidateStateSync: unexpected error: %v", err)
	}
	if result.Kind != orchestrator.ValidationOK {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationOK", result.Kind)
	}
	// State should be unchanged.
	if state.ActiveTask.ID != "EPIC-3-002" {
		t.Errorf("ActiveTask.ID should be unchanged: got %q", state.ActiveTask.ID)
	}
}

func TestValidateStateSync_AutoCorrect_SingleCandidate(t *testing.T) {
	// active_task.id not in tasks.yaml, exactly one TODO task available.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeFeature,
			ID:       "STALE-ID",
			Attempts: 2,
		},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusDone},
				{ID: "EPIC-3-002", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err != nil {
		t.Fatalf("ValidateStateSync: unexpected error on auto-correct: %v", err)
	}
	if result.Kind != orchestrator.ValidationAutoCorrected {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationAutoCorrected", result.Kind)
	}
	if result.Description == "" {
		t.Error("ValidateStateSync: AutoCorrected result should include a description")
	}
	// State should be redirected to the single candidate.
	if state.ActiveTask.ID != "EPIC-3-002" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "EPIC-3-002")
	}
	if state.ActiveTask.Type != types.TaskTypeFeature {
		t.Errorf("ActiveTask.Type: got %q, want feature", state.ActiveTask.Type)
	}
	// Attempts are preserved during auto-correction.
	if state.ActiveTask.Attempts != 2 {
		t.Errorf("ActiveTask.Attempts: got %d, want 2 (preserved during auto-correct)", state.ActiveTask.Attempts)
	}
}

func TestValidateStateSync_AutoCorrect_InProgressCandidate(t *testing.T) {
	// The single candidate is IN_PROGRESS (also qualifies for redirect).
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "STALE-ID"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusInProgress},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err != nil {
		t.Fatalf("ValidateStateSync: unexpected error: %v", err)
	}
	if result.Kind != orchestrator.ValidationAutoCorrected {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationAutoCorrected", result.Kind)
	}
	if state.ActiveTask.ID != "EPIC-3-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", state.ActiveTask.ID, "EPIC-3-001")
	}
}

func TestValidateStateSync_Fatal_SyntheticTask(t *testing.T) {
	// Synthetic task (bugfix) is never in tasks.yaml — must return error.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{
			Type: types.TaskTypeBugfix,
			ID:   "BUG-EPIC-3-002",
		},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err == nil {
		t.Fatal("ValidateStateSync: expected error for synthetic active task, got nil")
	}
	if result.Kind != orchestrator.ValidationFatal {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationFatal", result.Kind)
	}
}

func TestValidateStateSync_Fatal_DocumentationTask(t *testing.T) {
	// Documentation (KB_UPDATE) is also synthetic.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{
			Type: types.TaskTypeDocumentation,
			ID:   "KB_UPDATE",
		},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err == nil {
		t.Fatal("ValidateStateSync: expected error for synthetic documentation task, got nil")
	}
	if result.Kind != orchestrator.ValidationFatal {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationFatal", result.Kind)
	}
}

func TestValidateStateSync_Fatal_MultipleCandidates(t *testing.T) {
	// ID not found, multiple TODO tasks — ambiguous, cannot auto-correct.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "STALE-ID"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
				{ID: "EPIC-3-002", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err == nil {
		t.Fatal("ValidateStateSync: expected error for multiple candidates, got nil")
	}
	if result.Kind != orchestrator.ValidationFatal {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationFatal", result.Kind)
	}
}

func TestValidateStateSync_Fatal_NoCandidates(t *testing.T) {
	// ID not found, zero TODO/IN_PROGRESS tasks — cannot auto-correct.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "STALE-ID"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusDone},
				{ID: "EPIC-3-002", Type: types.TaskTypeFeature, Status: types.StatusBlocked},
			},
		},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err == nil {
		t.Fatal("ValidateStateSync: expected error when no candidates, got nil")
	}
	if result.Kind != orchestrator.ValidationFatal {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationFatal", result.Kind)
	}
}

func TestValidateStateSync_EmptyTaskList(t *testing.T) {
	// No tasks in tasks.yaml at all — ID not found, zero candidates.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-001"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{Tasks: []types.Task{}},
	}

	result, err := orchestrator.ValidateStateSync(state, tasks)
	if err == nil {
		t.Fatal("ValidateStateSync: expected error for empty task list, got nil")
	}
	if result.Kind != orchestrator.ValidationFatal {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationFatal", result.Kind)
	}
}
