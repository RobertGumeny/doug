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

// IsEpicAlreadyComplete reports whether the current epic has already been
// finalized. Finalized epics have all user-defined tasks DONE, a populated
// completion timestamp, and empty runtime task pointers.
func IsEpicAlreadyComplete(state *types.ProjectState, tasks *types.Tasks) bool {
	if !types.AreAllUserTasksComplete(tasks) {
		return false
	}
	if state.CurrentEpic.CompletedAt == nil || *state.CurrentEpic.CompletedAt == "" {
		return false
	}
	return state.ActiveTask.ID == "" && state.NextTask.ID == ""
}
