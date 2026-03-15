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

	// 4. Documentation (KB synthesis) task: set completed_at, commit, return EpicComplete.
	if ctx.TaskType == types.TaskTypeDocumentation {
		now := time.Now().UTC().Format(time.RFC3339)
		ctx.State.CurrentEpic.CompletedAt = &now
		if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
			return SuccessResult{Kind: Retry}, fmt.Errorf("save state after docs completion: %w", err)
		}
		if err := git.Commit("docs: "+ctx.TaskID, ctx.ProjectRoot); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("git commit failed for docs task %s: %v", ctx.TaskID, err))
			return SuccessResult{Kind: Retry}, nil
		}
		backfillCommitSHA(ctx)
		return SuccessResult{Kind: EpicComplete}, nil
	}

	// 5. Mark user-defined task as DONE.
	if !ctx.TaskType.IsSynthetic() {
		if err := types.UpdateTaskStatus(ctx.Tasks, ctx.TaskID, types.StatusDone); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("could not mark task %s done: %v", ctx.TaskID, err))
		}
		if err := state.SaveTasks(ctx.TasksPath, ctx.Tasks); err != nil {
			return SuccessResult{Kind: Retry}, fmt.Errorf("save tasks after resume: %w", err)
		}
	}

	// 6. Advance task pointers or inject KB synthesis.
	if types.NeedsKBSynthesis(ctx.State, ctx.Tasks, ctx.Config.KBEnabled) {
		ctx.Logger.Info("all feature tasks complete — scheduling KB synthesis")
		ctx.State.ActiveTask = types.TaskPointer{
			Type: types.TaskTypeDocumentation,
			ID:   "KB_UPDATE",
		}
		ctx.State.NextTask = types.TaskPointer{}
	} else {
		types.AdvanceToNextTask(ctx.State, ctx.Tasks)
	}

	// 7. Persist updated state.
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		return SuccessResult{Kind: Retry}, fmt.Errorf("save state after resume: %w", err)
	}

	// 8. Commit all changes for this task.
	commitMsg := taskCommitMessage(ctx.TaskType, ctx.TaskID)
	if err := git.Commit(commitMsg, ctx.ProjectRoot); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("git commit failed after resume for task %s: %v", ctx.TaskID, err))
		return SuccessResult{Kind: Retry}, nil
	}

	// 9. Backfill commit SHA into metrics.
	backfillCommitSHA(ctx)

	ctx.Logger.Success(fmt.Sprintf("task %s committed after resume", ctx.TaskID))
	return SuccessResult{Kind: Continue}, nil
}
