package orchestrator

import (
	"fmt"

	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// ValidationResult
// ---------------------------------------------------------------------------

// ValidationKind classifies the outcome of a state-sync validation check.
type ValidationKind int

const (
	// ValidationOK means state and tasks are consistent; no action was taken.
	ValidationOK ValidationKind = iota

	// ValidationAutoCorrected means the orchestrator silently redirected
	// active_task to the only available TODO/IN_PROGRESS task. The caller
	// should log the Description as a warning.
	ValidationAutoCorrected

	// ValidationFatal means the inconsistency cannot be resolved
	// automatically. The caller must exit with the Description as the error.
	ValidationFatal
)

// ValidationResult is returned by ValidateStateSync to report whether state
// was corrected and, if so, why.
type ValidationResult struct {
	Kind        ValidationKind
	Description string
}

// ---------------------------------------------------------------------------
// ValidateYAMLStructure
// ---------------------------------------------------------------------------

// ValidateYAMLStructure performs a structural sanity check on the loaded YAML
// files before any orchestration logic runs.
//
// It returns an error when:
//   - state.CurrentEpic.ID is empty (required field)
//   - state.ActiveTask.Type is empty (required field)
//   - state.ActiveTask.ID is empty (required field)
//   - any task in tasks.Epic.Tasks has an unrecognized Status value
func ValidateYAMLStructure(state *types.ProjectState, tasks *types.Tasks) error {
	if state.CurrentEpic.ID == "" {
		return fmt.Errorf("project-state.yaml: current_epic.id is required but empty — set current_epic.id in .doug/project-state.yaml")
	}
	if state.ActiveTask.Type == "" {
		return fmt.Errorf("project-state.yaml: active_task.type is required but empty — set active_task.type in .doug/project-state.yaml (e.g. feature)")
	}
	if state.ActiveTask.ID == "" {
		return fmt.Errorf("project-state.yaml: active_task.id is required but empty — set active_task.id in .doug/project-state.yaml")
	}

	validStatuses := map[types.Status]bool{
		types.StatusTODO:       true,
		types.StatusInProgress: true,
		types.StatusDone:       true,
		types.StatusBlocked:    true,
	}
	for _, t := range tasks.Epic.Tasks {
		if !validStatuses[t.Status] {
			return fmt.Errorf("tasks.yaml: task %q has invalid status %q (must be TODO, IN_PROGRESS, DONE, or BLOCKED) — edit .doug/tasks.yaml to correct the status field", t.ID, t.Status)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// ValidateTaskTypes
// ---------------------------------------------------------------------------

// ValidateTaskTypes ensures that no task in tasks.yaml uses a removed or
// runtime-only task type. scaffold is reserved for the doug scaffold command
// and manual_review is a removed legacy type; all other task types remain
// available for backlog authoring; skill routing is source-owned in agent code.
func ValidateTaskTypes(tasks *types.Tasks) error {
	for _, t := range tasks.Epic.Tasks {
		switch t.Type {
		case types.TaskTypeScaffold:
			return fmt.Errorf(
				"task %q has type %q which is reserved for orchestrator use and cannot appear in tasks.yaml",
				t.ID, t.Type,
			)
		case types.TaskType("manual_review"):
			return fmt.Errorf(
				"task %q has removed legacy type %q; use the task's real type and mark it BLOCKED when human intervention is required",
				t.ID, t.Type,
			)
		}
	}
	return nil
}

// NormalizeLegacyManualReviewState rewrites legacy project-state.yaml state
// that still uses active_task.type = manual_review into the current blocked-task
// model.
//
// Supported legacy rewrites:
//   - If active_task.id exists in the backlog, switch active_task.type to the
//     backlog task's real type and ensure that task is BLOCKED.
//   - If active_task.id is not in the backlog but next_task points to a backlog
//     task, treat this as a failed synthetic bugfix state: mark next_task
//     BLOCKED, promote it to active_task, and clear next_task.
//
// The returned bool reports whether state or task data changed in memory.
func NormalizeLegacyManualReviewState(state *types.ProjectState, tasks *types.Tasks) (bool, error) {
	if state.ActiveTask.Type != types.TaskType("manual_review") {
		return false, nil
	}

	for i := range tasks.Epic.Tasks {
		if tasks.Epic.Tasks[i].ID != state.ActiveTask.ID {
			continue
		}
		if tasks.Epic.Tasks[i].Type == types.TaskType("manual_review") {
			return false, fmt.Errorf(
				"legacy manual_review state is ambiguous: backlog task %q still has type manual_review in tasks.yaml — rewrite the task to its real type and mark it BLOCKED",
				tasks.Epic.Tasks[i].ID,
			)
		}
		if tasks.Epic.Tasks[i].Status != types.StatusBlocked {
			tasks.Epic.Tasks[i].Status = types.StatusBlocked
		}
		state.ActiveTask = types.TaskPointer{Type: tasks.Epic.Tasks[i].Type, ID: tasks.Epic.Tasks[i].ID}
		state.NextTask = types.TaskPointer{}
		return true, nil
	}

	if state.NextTask.ID != "" {
		for i := range tasks.Epic.Tasks {
			if tasks.Epic.Tasks[i].ID != state.NextTask.ID {
				continue
			}
			if tasks.Epic.Tasks[i].Type == types.TaskType("manual_review") {
				return false, fmt.Errorf(
					"legacy manual_review state is ambiguous: next_task %q still has type manual_review in tasks.yaml — rewrite the task to its real type and mark it BLOCKED",
					tasks.Epic.Tasks[i].ID,
				)
			}
			if tasks.Epic.Tasks[i].Status != types.StatusBlocked {
				tasks.Epic.Tasks[i].Status = types.StatusBlocked
			}
			state.ActiveTask = types.TaskPointer{Type: tasks.Epic.Tasks[i].Type, ID: tasks.Epic.Tasks[i].ID}
			state.NextTask = types.TaskPointer{}
			return true, nil
		}
	}

	return false, fmt.Errorf(
		"legacy manual_review state is ambiguous: active_task.id %q is not in tasks.yaml and next_task does not point to a backlog task — manually rewrite .doug/project-state.yaml to the blocked backlog task and remove manual_review",
		state.ActiveTask.ID,
	)
}

// ValidateActiveTaskIsRunnable rejects a blocked active backlog task so doug
// halts for human intervention instead of retrying or auto-advancing.
func ValidateActiveTaskIsRunnable(state *types.ProjectState, tasks *types.Tasks) error {
	for _, t := range tasks.Epic.Tasks {
		if t.ID == state.ActiveTask.ID {
			if t.Status == types.StatusBlocked {
				return fmt.Errorf(
					"active task %q is BLOCKED in tasks.yaml — resolve or unblock the task before running `doug run` again",
					state.ActiveTask.ID,
				)
			}
			break
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// ValidateStateSync
// ---------------------------------------------------------------------------

// ValidateStateSync checks whether state.ActiveTask.ID refers to a real task
// in tasks.yaml and applies the tiered recovery philosophy:
//
//   - Tier 1 (unambiguous recovery): active task ID not found, there is
//     exactly one TODO/IN_PROGRESS task → redirect silently, return
//     AutoCorrected (caller should log as warning).
//
//   - Tier 3 (ambiguous or runtime-only): active task type is scaffold
//     (runtime-only, never in tasks.yaml), or there are zero or multiple
//     candidate tasks → return Fatal error. The caller must exit.
//
//   - No mismatch: return OK.
//
// Note: callers must skip this function for active tasks not in tasks.yaml
// (e.g., handler-injected BUG-xxx bugfix tasks). The scaffold type check is a
// safety net for corrupt state; it should not be reached in normal operation.
func ValidateStateSync(state *types.ProjectState, tasks *types.Tasks) (ValidationResult, error) {
	// Check if active task ID is present in tasks.yaml.
	for _, t := range tasks.Epic.Tasks {
		if t.ID == state.ActiveTask.ID {
			return ValidationResult{Kind: ValidationOK}, nil
		}
	}

	// Active task ID not found in tasks.yaml.

	// Scaffold is runtime-only and can never be in tasks.yaml; encountering it
	// here means state is corrupt — do not attempt auto-correction.
	if state.ActiveTask.Type.IsSynthetic() {
		return ValidationResult{Kind: ValidationFatal},
			fmt.Errorf(
				"active task %q (type %q) not found in tasks.yaml; state is ambiguous — "+
					"manually set active_task in .doug/project-state.yaml to a valid task ID",
				state.ActiveTask.ID, state.ActiveTask.Type,
			)
	}

	// Count TODO/IN_PROGRESS candidates for potential redirection.
	var candidates []types.Task
	for _, t := range tasks.Epic.Tasks {
		if t.Status == types.StatusTODO || t.Status == types.StatusInProgress {
			candidates = append(candidates, t)
		}
	}

	if len(candidates) == 1 {
		// Unambiguous recovery: redirect to the single available task.
		old := state.ActiveTask.ID
		state.ActiveTask = types.TaskPointer{
			Type:     candidates[0].Type,
			ID:       candidates[0].ID,
			Attempts: state.ActiveTask.Attempts,
		}
		return ValidationResult{
			Kind: ValidationAutoCorrected,
			Description: fmt.Sprintf(
				"active_task.id %q not found in tasks.yaml; redirected to %q (only available task)",
				old, state.ActiveTask.ID,
			),
		}, nil
	}

	// Zero or multiple candidates: ambiguous — cannot safely auto-correct.
	return ValidationResult{Kind: ValidationFatal},
		fmt.Errorf(
			"active_task.id %q not found in tasks.yaml and %d candidate tasks remain (need exactly 1 for auto-correction) — "+
				"set active_task.id in .doug/project-state.yaml to the correct task ID",
			state.ActiveTask.ID, len(candidates),
		)
}
