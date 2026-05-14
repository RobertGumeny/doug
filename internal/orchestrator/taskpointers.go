package orchestrator

import (
	"github.com/robertgumeny/doug/internal/types"
)

// InitializeTaskPointers sets state.ActiveTask and state.NextTask based on the
// current status of user-defined tasks.
//
// Selection order for active task:
//  1. First IN_PROGRESS task (orchestrator was interrupted mid-task)
//  2. First TODO task (normal forward progress)
//
// next_task is set to the first TODO task that appears after the selected
// active task in the list.
func InitializeTaskPointers(state *types.ProjectState, tasks *types.Tasks) {
	// Don't re-initialize when the active task is not in the backlog.
	// Handler-injected tasks (e.g., BUG-xxx bugfix tasks) are not in tasks.yaml;
	// scanning the task list would clobber the in-progress pointer.
	if state.ActiveTask.ID != "" {
		activeInBacklog := false
		for _, t := range tasks.Epic.Tasks {
			if t.ID != state.ActiveTask.ID {
				continue
			}
			activeInBacklog = true
			if t.Status == types.StatusBlocked {
				state.NextTask = types.TaskPointer{}
				return
			}
			break
		}
		if !activeInBacklog {
			return
		}
	}

	// 1. Find active: prefer IN_PROGRESS over TODO in a single pass.
	// IN_PROGRESS always wins; we keep the first TODO we see in case no
	// IN_PROGRESS task is found, but continue scanning to avoid missing one.
	var activeTask *types.Task
	for i := range tasks.Epic.Tasks {
		t := &tasks.Epic.Tasks[i]
		if t.Status == types.StatusInProgress {
			activeTask = t
			break
		}
		if t.Status == types.StatusTODO && activeTask == nil {
			activeTask = t
		}
	}

	// 2. No user tasks remain — clear runtime task pointers. Post-epic KB
	// synthesis runs outside the main task loop and is not represented here.
	if activeTask == nil {
		state.ActiveTask = types.TaskPointer{}
		state.NextTask = types.TaskPointer{}
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
// the new NextTask from the remaining TODO tasks. Forwarded to types.AdvanceToNextTask.
func AdvanceToNextTask(state *types.ProjectState, tasks *types.Tasks) bool {
	return types.AdvanceToNextTask(state, tasks)
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
// Status field in memory. Forwarded to types.UpdateTaskStatus.
func UpdateTaskStatus(tasks *types.Tasks, id string, status types.Status) error {
	return types.UpdateTaskStatus(tasks, id, status)
}
