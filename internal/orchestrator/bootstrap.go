// Package orchestrator contains the core orchestration logic for the doug
// binary: bootstrapping state from tasks, managing task pointers, and
// validating state consistency.
package orchestrator

import (
	"time"

	"github.com/robertgumeny/doug/internal/types"
)

// BootstrapFromTasks initializes project state from tasks on the first run.
// It is a no-op if state.CurrentEpic.ID is already set (already bootstrapped).
//
// On first run it populates:
//   - current_epic: id, name, branch_name, started_at
//   - active_task: first task in tasks.yaml
//   - next_task: second task, or zero value if only one task exists
func BootstrapFromTasks(state *types.ProjectState, tasks *types.Tasks) {
	if state.CurrentEpic.ID != "" {
		return
	}

	state.CurrentEpic.ID = tasks.Epic.ID
	state.CurrentEpic.Name = tasks.Epic.Name
	state.CurrentEpic.BranchName = "feature/" + tasks.Epic.ID
	state.CurrentEpic.StartedAt = time.Now().UTC().Format(time.RFC3339)

	if len(tasks.Epic.Tasks) > 0 {
		first := tasks.Epic.Tasks[0]
		state.ActiveTask = types.TaskPointer{
			Type: first.Type,
			ID:   first.ID,
		}
	}

	if len(tasks.Epic.Tasks) > 1 {
		second := tasks.Epic.Tasks[1]
		state.NextTask = types.TaskPointer{
			Type: second.Type,
			ID:   second.ID,
		}
	}
}

// NeedsKBSynthesis reports whether a KB synthesis (documentation) task should
// be injected as the next active task.
//
// Returns false when:
//   - kb_enabled is false
//   - active task is already a documentation type (KB synthesis already running)
//   - any user-defined task remains TODO or IN_PROGRESS
//
// Returns true only when all user-defined tasks are DONE and KB synthesis
// has not yet been started.
func NeedsKBSynthesis(state *types.ProjectState, tasks *types.Tasks) bool {
	if !state.KBEnabled {
		return false
	}
	if state.ActiveTask.Type == types.TaskTypeDocumentation {
		return false
	}
	for _, t := range tasks.Epic.Tasks {
		if t.Status == types.StatusTODO || t.Status == types.StatusInProgress {
			return false
		}
	}
	return true
}

// IsEpicAlreadyComplete reports whether the current epic has no remaining work.
//
// Returns true when all user-defined tasks are DONE and either:
//   - kb_enabled is false (no KB synthesis required), or
//   - active_task is a documentation type (KB synthesis was already run in
//     a previous iteration and completed)
func IsEpicAlreadyComplete(state *types.ProjectState, tasks *types.Tasks) bool {
	for _, t := range tasks.Epic.Tasks {
		if t.Status != types.StatusDone {
			return false
		}
	}
	if !state.KBEnabled {
		return true
	}
	return state.ActiveTask.Type == types.TaskTypeDocumentation
}
