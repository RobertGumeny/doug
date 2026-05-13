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

// ValidateTaskTypes ensures that no task in tasks.yaml uses a runtime-only type
// (scaffold). Scaffold is reserved for the doug scaffold command and must never
// appear in user-authored tasks.yaml. feature, bugfix, documentation, and
// manual_review are all valid user-authored task types.
func ValidateTaskTypes(tasks *types.Tasks) error {
	for _, t := range tasks.Epic.Tasks {
		if t.Type.IsSynthetic() {
			return fmt.Errorf(
				"task %q has type %q which is reserved for orchestrator use and cannot appear in tasks.yaml",
				t.ID, t.Type,
			)
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
