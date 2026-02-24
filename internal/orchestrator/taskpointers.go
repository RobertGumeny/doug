package orchestrator

import (
	"fmt"

	"github.com/robertgumeny/doug/internal/types"
)

// InitializeTaskPointers sets state.ActiveTask and state.NextTask based on the
// current status of user-defined tasks.
//
// Selection order for active task:
//  1. First IN_PROGRESS task (orchestrator was interrupted mid-task)
//  2. First TODO task (normal forward progress)
//
// If no user tasks remain (all DONE or BLOCKED) and kb_enabled is true,
// a synthetic KB_UPDATE documentation task is injected as the active task.
//
// next_task is set to the first TODO task that appears after the selected
// active task in the list.
func InitializeTaskPointers(state *types.ProjectState, tasks *types.Tasks) {
	// 1. Find active: prefer IN_PROGRESS, then first TODO.
	var activeTask *types.Task
	for i := range tasks.Epic.Tasks {
		if tasks.Epic.Tasks[i].Status == types.StatusInProgress {
			activeTask = &tasks.Epic.Tasks[i]
			break
		}
	}
	if activeTask == nil {
		for i := range tasks.Epic.Tasks {
			if tasks.Epic.Tasks[i].Status == types.StatusTODO {
				activeTask = &tasks.Epic.Tasks[i]
				break
			}
		}
	}

	// 2. No user tasks remain — inject KB_UPDATE if enabled.
	if activeTask == nil {
		state.NextTask = types.TaskPointer{}
		if state.KBEnabled {
			state.ActiveTask = types.TaskPointer{
				Type: types.TaskTypeDocumentation,
				ID:   "KB_UPDATE",
			}
		}
		return
	}

	state.ActiveTask = types.TaskPointer{
		Type: activeTask.Type,
		ID:   activeTask.ID,
	}

	// 3. Find next: first TODO task that appears after the active task.
	foundActive := false
	state.NextTask = types.TaskPointer{}
	for _, t := range tasks.Epic.Tasks {
		if foundActive && t.Status == types.StatusTODO {
			state.NextTask = types.TaskPointer{Type: t.Type, ID: t.ID}
			break
		}
		if t.ID == activeTask.ID {
			foundActive = true
		}
	}
}

// AdvanceToNextTask promotes state.NextTask to state.ActiveTask and locates
// the new NextTask from the remaining TODO tasks.
//
// Returns false immediately (without modifying state) if NextTask is empty —
// meaning there is no task to advance to. Returns true after a successful
// promotion, even if no further next task exists after the new active.
//
// Attempts on the newly promoted active task is reset to 0; the caller must
// call IncrementAttempts at the start of the next iteration.
func AdvanceToNextTask(state *types.ProjectState, tasks *types.Tasks) bool {
	if state.NextTask.ID == "" {
		return false
	}

	// Promote next → active; reset attempt counter.
	state.ActiveTask = types.TaskPointer{
		Type:     state.NextTask.Type,
		ID:       state.NextTask.ID,
		Attempts: 0,
	}

	// Find new next: first TODO task that appears after the newly active task.
	foundActive := false
	state.NextTask = types.TaskPointer{}
	for _, t := range tasks.Epic.Tasks {
		if foundActive && t.Status == types.StatusTODO {
			state.NextTask = types.TaskPointer{Type: t.Type, ID: t.ID}
			break
		}
		if t.ID == state.ActiveTask.ID {
			foundActive = true
		}
	}

	return true
}

// FindNextActiveTask returns the ID and TaskType of the next task that should
// become active, scanning the task list in order. IN_PROGRESS tasks are
// preferred over TODO tasks (supporting orchestrator-restart recovery).
//
// Returns empty strings when no active task candidates remain.
func FindNextActiveTask(tasks *types.Tasks) (string, types.TaskType) {
	for _, t := range tasks.Epic.Tasks {
		if t.Status == types.StatusInProgress {
			return t.ID, t.Type
		}
	}
	for _, t := range tasks.Epic.Tasks {
		if t.Status == types.StatusTODO {
			return t.ID, t.Type
		}
	}
	return "", ""
}

// IncrementAttempts increments the Attempts counter on state.ActiveTask in
// memory. The caller is responsible for persisting the updated state via
// SaveState.
func IncrementAttempts(state *types.ProjectState) {
	state.ActiveTask.Attempts++
}

// UpdateTaskStatus finds the task with the given ID in tasks and updates its
// Status field in memory. Returns an error if no task with that ID is found.
// The caller is responsible for persisting the updated tasks via SaveTasks.
func UpdateTaskStatus(tasks *types.Tasks, id string, status types.Status) error {
	for i := range tasks.Epic.Tasks {
		if tasks.Epic.Tasks[i].ID == id {
			tasks.Epic.Tasks[i].Status = status
			return nil
		}
	}
	return fmt.Errorf("task %q not found in tasks", id)
}
