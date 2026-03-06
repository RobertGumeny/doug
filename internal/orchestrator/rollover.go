package orchestrator

import (
	"fmt"

	"github.com/robertgumeny/doug/internal/types"
)

// PrepareForEpicRollover resets runtime state when tasks.yaml declares a new
// epic ID. The previous epic must be completed first (CompletedAt set).
//
// Returns true when a rollover reset was performed, false when no rollover is
// needed. Returns an error if rollover is requested while the previous epic is
// still incomplete.
func PrepareForEpicRollover(state *types.ProjectState, tasks *types.Tasks) (bool, error) {
	// First run or missing tasks epic ID: nothing to compare.
	if state.CurrentEpic.ID == "" || tasks.Epic.ID == "" {
		return false, nil
	}

	// Same epic ID: no rollover needed.
	if state.CurrentEpic.ID == tasks.Epic.ID {
		return false, nil
	}

	// Guardrail: never roll over unless previous epic is explicitly completed.
	if state.CurrentEpic.CompletedAt == nil || *state.CurrentEpic.CompletedAt == "" {
		return false, fmt.Errorf(
			"tasks.yaml references epic %q but current state epic %q is not completed (current_epic.completed_at is empty)",
			tasks.Epic.ID,
			state.CurrentEpic.ID,
		)
	}

	// Reset per-epic runtime state. BootstrapFromTasks repopulates these.
	state.CurrentEpic = types.EpicState{}
	state.ActiveTask = types.TaskPointer{}
	state.NextTask = types.TaskPointer{}
	state.Metrics = types.Metrics{}

	return true, nil
}
