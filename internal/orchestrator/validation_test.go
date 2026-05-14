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
// ValidateTaskTypes
// ---------------------------------------------------------------------------

func TestValidateTaskTypes_AllFeature(t *testing.T) {
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-7-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
				{ID: "EPIC-7-002", Type: types.TaskTypeFeature, Status: types.StatusTODO},
			},
		},
	}
	if err := orchestrator.ValidateTaskTypes(tasks); err != nil {
		t.Errorf("ValidateTaskTypes: unexpected error for all-feature tasks: %v", err)
	}
}

func TestValidateTaskTypes_BugfixAllowed(t *testing.T) {
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-7-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
				{ID: "EPIC-7-007", Type: types.TaskTypeBugfix, Status: types.StatusTODO},
			},
		},
	}
	if err := orchestrator.ValidateTaskTypes(tasks); err != nil {
		t.Errorf("ValidateTaskTypes: unexpected error for bugfix task type: %v", err)
	}
}

func TestValidateTaskTypes_DocumentationAllowed(t *testing.T) {
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-7-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
				{ID: "EPIC-7-002", Type: types.TaskTypeDocumentation, Status: types.StatusTODO},
			},
		},
	}
	if err := orchestrator.ValidateTaskTypes(tasks); err != nil {
		t.Errorf("ValidateTaskTypes: unexpected error for documentation task type: %v", err)
	}
}

func TestValidateTaskTypes_ManualReviewReturnsError(t *testing.T) {
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{{ID: "EPIC-7-009", Type: types.TaskType("manual_review"), Status: types.StatusTODO}},
		},
	}
	if err := orchestrator.ValidateTaskTypes(tasks); err == nil {
		t.Error("ValidateTaskTypes: expected error for removed manual_review task type, got nil")
	}
}

func TestValidateTaskTypes_CustomTypeAllowed(t *testing.T) {
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{{ID: "EPIC-7-010", Type: types.TaskType("refactor"), Status: types.StatusTODO}},
		},
	}
	if err := orchestrator.ValidateTaskTypes(tasks); err != nil {
		t.Errorf("ValidateTaskTypes: unexpected error for custom task type: %v", err)
	}
}

func TestValidateTaskTypes_ScaffoldTypeReturnsError(t *testing.T) {
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-7-001", Type: types.TaskTypeFeature, Status: types.StatusTODO},
				{ID: "EPIC-7-002", Type: types.TaskTypeScaffold, Status: types.StatusTODO},
			},
		},
	}
	if err := orchestrator.ValidateTaskTypes(tasks); err == nil {
		t.Error("ValidateTaskTypes: expected error for scaffold task type, got nil")
	}
}

// ---------------------------------------------------------------------------
// NormalizeLegacyManualReviewState / ValidateActiveTaskIsRunnable
// ---------------------------------------------------------------------------

func TestNormalizeLegacyManualReviewState_BacklogTask(t *testing.T) {
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskType("manual_review"), ID: "EPIC-3-002", Attempts: 5, ConsecutiveTestFailures: 2, TestFailureOutput: "boom"},
		NextTask:   types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-003"},
	}
	tasks := &types.Tasks{Epic: types.EpicDefinition{Tasks: []types.Task{{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusDone}, {ID: "EPIC-3-002", Type: types.TaskTypeDocumentation, Status: types.StatusInProgress}, {ID: "EPIC-3-003", Type: types.TaskTypeFeature, Status: types.StatusTODO}}}}

	normalized, err := orchestrator.NormalizeLegacyManualReviewState(state, tasks)
	if err != nil {
		t.Fatalf("NormalizeLegacyManualReviewState: %v", err)
	}
	if !normalized {
		t.Fatal("expected normalization to report changed=true")
	}
	if state.ActiveTask.Type != types.TaskTypeDocumentation || state.ActiveTask.ID != "EPIC-3-002" {
		t.Fatalf("active task not normalized: %+v", state.ActiveTask)
	}
	if state.ActiveTask.Attempts != 0 || state.ActiveTask.ConsecutiveTestFailures != 0 || state.ActiveTask.TestFailureOutput != "" {
		t.Fatalf("active task transient fields not cleared: %+v", state.ActiveTask)
	}
	if state.NextTask.ID != "" {
		t.Fatalf("next task should be cleared, got %+v", state.NextTask)
	}
	if tasks.Epic.Tasks[1].Status != types.StatusBlocked {
		t.Fatalf("backlog task status: got %q, want BLOCKED", tasks.Epic.Tasks[1].Status)
	}
}

func TestNormalizeLegacyManualReviewState_FailedSyntheticBugfix(t *testing.T) {
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskType("manual_review"), ID: "BUG-EPIC-3-002", Attempts: 5},
		NextTask:   types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-002"},
	}
	tasks := &types.Tasks{Epic: types.EpicDefinition{Tasks: []types.Task{{ID: "EPIC-3-002", Type: types.TaskTypeFeature, Status: types.StatusInProgress}}}}

	normalized, err := orchestrator.NormalizeLegacyManualReviewState(state, tasks)
	if err != nil {
		t.Fatalf("NormalizeLegacyManualReviewState: %v", err)
	}
	if !normalized {
		t.Fatal("expected normalization to report changed=true")
	}
	if state.ActiveTask.Type != types.TaskTypeFeature || state.ActiveTask.ID != "EPIC-3-002" {
		t.Fatalf("active task not promoted to interrupted backlog task: %+v", state.ActiveTask)
	}
	if state.NextTask.ID != "" {
		t.Fatalf("next task should be cleared, got %+v", state.NextTask)
	}
	if tasks.Epic.Tasks[0].Status != types.StatusBlocked {
		t.Fatalf("backlog task status: got %q, want BLOCKED", tasks.Epic.Tasks[0].Status)
	}
}

func TestNormalizeLegacyManualReviewState_AmbiguousReturnsError(t *testing.T) {
	state := &types.ProjectState{ActiveTask: types.TaskPointer{Type: types.TaskType("manual_review"), ID: "MISSING"}}
	tasks := &types.Tasks{Epic: types.EpicDefinition{Tasks: []types.Task{{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusTODO}}}}

	if _, err := orchestrator.NormalizeLegacyManualReviewState(state, tasks); err == nil {
		t.Fatal("expected ambiguous legacy state to return an error")
	}
}

func TestValidateActiveTaskIsRunnable_BlockedActiveTaskReturnsError(t *testing.T) {
	state := &types.ProjectState{ActiveTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-3-002"}}
	tasks := &types.Tasks{Epic: types.EpicDefinition{Tasks: []types.Task{{ID: "EPIC-3-002", Type: types.TaskTypeFeature, Status: types.StatusBlocked}}}}

	if err := orchestrator.ValidateActiveTaskIsRunnable(state, tasks); err == nil {
		t.Fatal("expected blocked active task to be rejected")
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

func TestValidateStateSync_AutoCorrect_HandlerInjectedBugfix(t *testing.T) {
	// Handler-injected bugfix tasks (BUG-xxx IDs) are not in tasks.yaml.
	// The run loop skips ValidateStateSync for such tasks; if called directly,
	// a single TODO candidate triggers auto-correction (bugfix is user-authorable).
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
	if err != nil {
		t.Fatalf("ValidateStateSync: unexpected error for handler-injected bugfix: %v", err)
	}
	if result.Kind != orchestrator.ValidationAutoCorrected {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationAutoCorrected", result.Kind)
	}
}

func TestValidateStateSync_AutoCorrect_HandlerInjectedDocumentation(t *testing.T) {
	// Documentation tasks not in tasks.yaml auto-correct with a single candidate.
	// The run loop skips ValidateStateSync for such tasks (IDs not in backlog).
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
	if err != nil {
		t.Fatalf("ValidateStateSync: unexpected error for handler-injected documentation: %v", err)
	}
	if result.Kind != orchestrator.ValidationAutoCorrected {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationAutoCorrected", result.Kind)
	}
}

func TestValidateStateSync_Fatal_ScaffoldTask(t *testing.T) {
	// Scaffold is runtime-only and can never be in tasks.yaml — must return Fatal.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{
			Type: types.TaskTypeScaffold,
			ID:   "SCAFFOLD-001",
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
		t.Fatal("ValidateStateSync: expected error for scaffold active task, got nil")
	}
	if result.Kind != orchestrator.ValidationFatal {
		t.Errorf("ValidateStateSync: kind: got %v, want ValidationFatal", result.Kind)
	}
}

func TestValidateStateSync_OK_UserAuthoredBugfix(t *testing.T) {
	// User-authored bugfix tasks in tasks.yaml are found and return OK.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeBugfix, ID: "EPIC-3-002"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusDone},
				{ID: "EPIC-3-002", Type: types.TaskTypeBugfix, Status: types.StatusInProgress},
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
}

func TestValidateStateSync_OK_UserAuthoredDocumentation(t *testing.T) {
	// User-authored documentation tasks in tasks.yaml are found and return OK.
	state := &types.ProjectState{
		ActiveTask: types.TaskPointer{Type: types.TaskTypeDocumentation, ID: "EPIC-3-003"},
	}
	tasks := &types.Tasks{
		Epic: types.EpicDefinition{
			Tasks: []types.Task{
				{ID: "EPIC-3-001", Type: types.TaskTypeFeature, Status: types.StatusDone},
				{ID: "EPIC-3-003", Type: types.TaskTypeDocumentation, Status: types.StatusTODO},
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
