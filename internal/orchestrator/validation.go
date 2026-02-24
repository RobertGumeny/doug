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
		return fmt.Errorf("project-state.yaml: current_epic.id is required but empty")
	}
	if state.ActiveTask.Type == "" {
		return fmt.Errorf("project-state.yaml: active_task.type is required but empty")
	}
	if state.ActiveTask.ID == "" {
		return fmt.Errorf("project-state.yaml: active_task.id is required but empty")
	}

	validStatuses := map[types.Status]bool{
		types.StatusTODO:       true,
		types.StatusInProgress: true,
		types.StatusDone:       true,
		types.StatusBlocked:    true,
	}
	for _, t := range tasks.Epic.Tasks {
		if !validStatuses[t.Status] {
			return fmt.Errorf("tasks.yaml: task %q has invalid status %q (must be TODO, IN_PROGRESS, DONE, or BLOCKED)", t.ID, t.Status)
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
//   - Tier 3 (ambiguous or synthetic): active task is synthetic (bugfix or
//     documentation), or there are zero or multiple candidate tasks → return
//     Fatal error. The caller must exit.
//
//   - No mismatch: return OK.
//
// Note: synthetic tasks (bugfix, documentation) are intentionally absent from
// tasks.yaml. Encountering a synthetic active_task.ID that somehow triggers
// the not-found path indicates an ambiguous state that requires manual review.
func ValidateStateSync(state *types.ProjectState, tasks *types.Tasks) (ValidationResult, error) {
	// Check if active task ID is present in tasks.yaml.
	for _, t := range tasks.Epic.Tasks {
		if t.ID == state.ActiveTask.ID {
			return ValidationResult{Kind: ValidationOK}, nil
		}
	}

	// Active task ID not found in tasks.yaml.

	// Synthetic tasks are never in tasks.yaml by design; a mismatch here
	// means the state is ambiguous — do not attempt auto-correction.
	if state.ActiveTask.Type.IsSynthetic() {
		return ValidationResult{Kind: ValidationFatal},
			fmt.Errorf(
				"active synthetic task %q (type %q) not found in tasks.yaml; state is ambiguous — manual correction required",
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
			"active_task.id %q not found in tasks.yaml and %d candidate tasks remain (need exactly 1 for auto-correction)",
			state.ActiveTask.ID, len(candidates),
		)
}
