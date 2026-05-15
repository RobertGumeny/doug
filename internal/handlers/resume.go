package handlers

import (
	"fmt"
	"time"

	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// HandleResume processes the resume path when the project was PAUSED after a
// build failure. It skips agent invocation and runs build and test verification
// directly against the current working tree.
//
// On success: marks the task DONE, advances the task pointer, commits, and
// returns (Continue, nil) so the main loop proceeds with the next task.
//
// On failure: re-sets the project status to PAUSED, persists state, and
// returns (BuildFailure, nil) so the caller exits cleanly.
func HandleResume(ctx *types.LoopContext) (SuccessResult, error) {
	// 0. Clear PAUSED status — if verification fails, pauseProject will re-set it.
	ctx.State.Status = ""

	// 1. Ensure deps are present for verification.
	if !ctx.BuildSystem.IsInitialized() {
		ctx.Logger.Info("build system not initialized; installing dependencies before verification")
		if err := ctx.BuildSystem.Install(); err != nil {
			return pauseProject(ctx, fmt.Sprintf("dependency install failed: %v", err))
		}
	}

	// 2. Verify build.
	ctx.Logger.Info("verifying build")
	if err := ctx.BuildSystem.Build(); err != nil {
		return pauseProject(ctx, fmt.Sprintf("build verification failed: %v", err))
	}
	ctx.Logger.Success("build passed")

	// 3. Verify tests.
	ctx.Logger.Info("verifying tests")
	if err := ctx.BuildSystem.Test(); err != nil {
		return pauseProject(ctx, fmt.Sprintf("test verification failed: %v", err))
	}
	ctx.Logger.Success("tests passed")

	// 3b. Run lint validation if enabled.
	if ctx.Config.LintEnabled {
		if err := runLint(ctx); err != nil {
			return pauseProject(ctx, fmt.Sprintf("lint verification failed: %v", err))
		}
	}

	// 4. Mark task as DONE in tasks.yaml. For handler-injected tasks whose IDs
	// are not in tasks.yaml, UpdateTaskStatus logs a warning and SaveTasks
	// persists the unchanged task list harmlessly.
	if err := types.UpdateTaskStatus(ctx.Tasks, ctx.TaskID, types.StatusDone); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("could not mark task %s done: %v", ctx.TaskID, err))
	}
	if err := state.SaveTasks(ctx.TasksPath, ctx.Tasks); err != nil {
		return SuccessResult{Kind: Retry}, fmt.Errorf("save tasks after resume: %w", err)
	}

	// 5. Advance task pointers or complete the epic when no user tasks remain.
	if types.AreAllUserTasksComplete(ctx.Tasks) {
		now := time.Now().UTC().Format(time.RFC3339)
		ctx.State.CurrentEpic.CompletedAt = &now
		if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
			return SuccessResult{Kind: Retry}, fmt.Errorf("save state after terminal resume completion: %w", err)
		}
		if err := git.Commit(taskCommitMessage(ctx.TaskType, ctx.TaskID), ctx.ProjectRoot); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("git commit failed after resume for terminal task %s: %v", ctx.TaskID, err))
			return SuccessResult{Kind: Retry}, nil
		}
		backfillCommitSHA(ctx)
		return SuccessResult{Kind: EpicComplete}, nil
	} else {
		advanced := types.AdvanceToNextTask(ctx.State, ctx.Tasks)
		if !advanced {
			return SuccessResult{Kind: Retry}, fmt.Errorf("advance task pointers after resume for %s: no next task but epic is not terminal", ctx.TaskID)
		}
	}

	// 6. Persist updated state.
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		return SuccessResult{Kind: Retry}, fmt.Errorf("save state after resume: %w", err)
	}

	// 7. Commit all changes for this task.
	commitMsg := taskCommitMessage(ctx.TaskType, ctx.TaskID)
	if err := git.Commit(commitMsg, ctx.ProjectRoot); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("git commit failed after resume for task %s: %v", ctx.TaskID, err))
		return SuccessResult{Kind: Retry}, nil
	}

	// 8. Backfill commit SHA into metrics.
	backfillCommitSHA(ctx)

	ctx.Logger.Success(fmt.Sprintf("task %s committed after resume", ctx.TaskID))
	return SuccessResult{Kind: Continue}, nil
}
