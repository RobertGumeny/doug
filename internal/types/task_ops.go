package types

import "fmt"

// UpdateTaskStatus finds the task with the given ID in tasks and updates its
// Status field in memory. Returns an error if no task with that ID is found.
// The caller is responsible for persisting the updated tasks via SaveTasks.
func UpdateTaskStatus(tasks *Tasks, id string, status Status) error {
	for i := range tasks.Epic.Tasks {
		if tasks.Epic.Tasks[i].ID == id {
			tasks.Epic.Tasks[i].Status = status
			return nil
		}
	}
	return fmt.Errorf("task %q not found in tasks", id)
}

// AreAllUserTasksComplete reports whether every user-defined task in tasks is
// terminal. In the current orchestrator model, completion means every task in
// tasks.yaml is DONE; post-epic KB synthesis is not part of this predicate.
func AreAllUserTasksComplete(tasks *Tasks) bool {
	for _, t := range tasks.Epic.Tasks {
		if t.Status != StatusDone {
			return false
		}
	}
	return true
}

// AdvanceToNextTask promotes state.NextTask to state.ActiveTask and locates
// the new NextTask from the remaining TODO tasks.
//
// Returns false immediately (without modifying state) if NextTask is empty.
// Returns true after a successful promotion, even if no further next task exists.
//
// Attempts on the newly promoted active task is reset to 0; the caller must
// call IncrementAttempts at the start of the next iteration.
func AdvanceToNextTask(state *ProjectState, tasks *Tasks) bool {
	if state.NextTask.ID == "" {
		return false
	}

	// Promote next → active; reset attempt counter.
	state.ActiveTask = TaskPointer{
		Type:     state.NextTask.Type,
		ID:       state.NextTask.ID,
		Attempts: 0,
	}

	// Find new next: first TODO task that appears after the newly active task.
	foundActive := false
	state.NextTask = TaskPointer{}
	for _, t := range tasks.Epic.Tasks {
		if foundActive && t.Status == StatusTODO {
			state.NextTask = TaskPointer{Type: t.Type, ID: t.ID}
			break
		}
		if t.ID == state.ActiveTask.ID {
			foundActive = true
		}
	}

	return true
}
