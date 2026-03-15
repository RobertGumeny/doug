// Package handlers implements the outcome handlers for the orchestration loop.
// Each handler receives a *types.LoopContext and performs the full
// response sequence for one of the four agent outcomes: SUCCESS, FAILURE,
// BUG, or EPIC_COMPLETE.
package handlers

import (
	"fmt"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/changelog"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/metrics"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// SuccessResultKind classifies the outcome of HandleSuccess.
type SuccessResultKind int

const (
	// Continue means the task completed normally and the main loop should
	// proceed to the next iteration with the updated task pointers.
	Continue SuccessResultKind = iota

	// Retry means a non-fatal issue occurred (git commit failure). The main
	// loop should continue to the next iteration and allow the state machine
	// to recover naturally.
	Retry

	// EpicComplete means the KB synthesis documentation task completed
	// successfully. The caller should invoke HandleEpicComplete next.
	EpicComplete

	// BuildFailure means build or test verification failed after the agent
	// reported SUCCESS. The project state is set to PAUSED, the working tree
	// is preserved, and the main loop should exit cleanly without retrying.
	BuildFailure
)

// SuccessResult is returned by HandleSuccess to direct the main loop.
type SuccessResult struct {
	Kind SuccessResultKind
}

// protectedPaths are state-tracking files that must be preserved across a git
// rollback so the orchestrator does not lose its place after a bad agent run.
var protectedPaths = []string{
	".doug/project-state.yaml",
	".doug/tasks.yaml",
}

// HandleSuccess processes a SUCCESS outcome reported by the agent. It installs
// any new dependencies, verifies the build and tests, records task metadata,
// updates task state, commits the result, and tells the main loop whether to
// continue, retry, or finish the epic.
func HandleSuccess(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (SuccessResult, error) {
	// 0. Archive ACTIVE_TASK.md unconditionally before any state change.
	if err := agent.ArchiveActiveTask(ctx.DougDir, ctx.LogsDir, ctx.CurrentEpic.ID, ctx.TaskID, ctx.Attempts); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("session archive failed: %v", err))
	}

	// 1. Install new dependencies if any were added by the agent.
	if len(result.DependenciesAdded) > 0 {
		ctx.Logger.Info(fmt.Sprintf("installing new dependencies: %v", result.DependenciesAdded))
		if err := ctx.BuildSystem.Install(); err != nil {
			ctx.Logger.Error(fmt.Sprintf("dependency install failed: %v", err))
			return pauseProject(ctx, fmt.Sprintf("dependency install failed: %v", err))
		}
	}

	// 1b. Ensure dependencies are present for verification even when the
	// agent's sandboxed install did not persist into the orchestrator workspace.
	if !ctx.BuildSystem.IsInitialized() {
		ctx.Logger.Info("build system not initialized; installing dependencies before verification")
		if err := ctx.BuildSystem.Install(); err != nil {
			ctx.Logger.Error(fmt.Sprintf("dependency install failed: %v", err))
			return pauseProject(ctx, fmt.Sprintf("dependency install failed: %v", err))
		}
	}

	// 2. Verify build.
	ctx.Logger.Info("verifying build")
	if err := ctx.BuildSystem.Build(); err != nil {
		ctx.Logger.Error(fmt.Sprintf("build verification failed:\n%v", err))
		return pauseProject(ctx, fmt.Sprintf("build verification failed: %v", err))
	}
	ctx.Logger.Success("build passed")

	// 3. Verify tests.
	ctx.Logger.Info("verifying tests")
	if err := ctx.BuildSystem.Test(); err != nil {
		ctx.Logger.Error(fmt.Sprintf("test verification failed:\n%v", err))
		if ctx.State.ActiveTask.ConsecutiveTestFailures >= 1 {
			// Second consecutive test failure after SUCCESS — pause the project.
			return pauseProject(ctx, fmt.Sprintf("test verification failed: %v", err))
		}
		// First test failure — inject output into next briefing and retry.
		// Retry counter increments normally (no decrement unlike BUILD_FAILURE).
		ctx.State.ActiveTask.ConsecutiveTestFailures++
		ctx.State.ActiveTask.TestFailureOutput = err.Error()
		if saveErr := state.SaveProjectState(ctx.StatePath, ctx.State); saveErr != nil {
			return SuccessResult{Kind: BuildFailure}, fmt.Errorf("save state after test failure: %w", saveErr)
		}
		ctx.Logger.Warning(fmt.Sprintf("test failure on attempt %d for task %s — retrying with test output injected", ctx.Attempts, ctx.TaskID))
		return SuccessResult{Kind: Retry}, nil
	}
	ctx.Logger.Success("tests passed")
	// Reset consecutive test failure tracking now that tests are passing.
	ctx.State.ActiveTask.ConsecutiveTestFailures = 0
	ctx.State.ActiveTask.TestFailureOutput = ""

	// 4. Record task metrics (in-memory; non-fatal if the task ID is odd).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, string(types.OutcomeSuccess), duration, ctx.Attempts, string(ctx.TaskType), agentDurationSeconds)

	// 5. Update CHANGELOG.md (non-fatal).
	if result.ChangelogEntry != "" {
		if err := changelog.UpdateChangelog(
			ctx.ChangelogPath,
			result.ChangelogEntry,
			string(ctx.TaskType),
		); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("changelog update skipped: %v", err))
		}
	}

	// 6. Mark user-defined task as DONE (synthetic tasks are never in tasks.yaml).
	if !ctx.TaskType.IsSynthetic() {
		if err := types.UpdateTaskStatus(ctx.Tasks, ctx.TaskID, types.StatusDone); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("could not mark task %s done: %v", ctx.TaskID, err))
		}
		if err := state.SaveTasks(ctx.TasksPath, ctx.Tasks); err != nil {
			return SuccessResult{Kind: Retry}, fmt.Errorf("save tasks after marking DONE: %w", err)
		}
	}

	// 7. Documentation (KB synthesis) task: set completed_at, commit, return EpicComplete.
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

	// 8. Advance task pointers or inject KB synthesis.
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

	// 9. Persist updated state.
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		return SuccessResult{Kind: Retry}, fmt.Errorf("save state: %w", err)
	}

	// 10. Commit all changes for this task.
	commitMsg := taskCommitMessage(ctx.TaskType, ctx.TaskID)
	if err := git.Commit(commitMsg, ctx.ProjectRoot); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("git commit failed for task %s: %v", ctx.TaskID, err))
		return SuccessResult{Kind: Retry}, nil
	}

	// 11. Backfill commit SHA into the last metrics entry and persist.
	backfillCommitSHA(ctx)

	ctx.Logger.Success(fmt.Sprintf("task %s committed", ctx.TaskID))
	return SuccessResult{Kind: Continue}, nil
}

// backfillCommitSHA reads the current HEAD SHA and writes it into the last
// metrics entry, then re-persists project state. Both steps are non-fatal:
// failures are logged as warnings so that a SHA lookup error never blocks the
// orchestration loop.
func backfillCommitSHA(ctx *types.LoopContext) {
	if len(ctx.State.Metrics.Tasks) == 0 {
		return
	}
	sha, err := git.CurrentSHA(ctx.ProjectRoot)
	if err != nil {
		ctx.Logger.Warning(fmt.Sprintf("could not read commit SHA: %v", err))
		return
	}
	ctx.State.Metrics.Tasks[len(ctx.State.Metrics.Tasks)-1].CommitSHA = sha
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("could not persist commit SHA: %v", err))
	}
}

// pauseProject sets the project status to PAUSED, decrements the attempt
// counter to undo the increment that happened at the start of this iteration,
// and persists state. The working tree is left intact so the user can inspect
// and fix the problem. Returns (BuildFailure, nil) on success or
// (BuildFailure, err) if state cannot be saved.
func pauseProject(ctx *types.LoopContext, reason string) (SuccessResult, error) {
	// Undo the attempt increment: BUILD_FAILURE must not consume a retry.
	if ctx.State.ActiveTask.Attempts > 0 {
		ctx.State.ActiveTask.Attempts--
	}
	ctx.State.Status = types.ProjectStatusPaused
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		return SuccessResult{Kind: BuildFailure}, fmt.Errorf("save state after build failure: %w", err)
	}
	ctx.Logger.Error(fmt.Sprintf(
		"project PAUSED for task %s: %s\n"+
			"Inspect the working tree, fix the issue, clear 'status' in .doug/project-state.yaml, then run `doug run` to resume.",
		ctx.TaskID, reason,
	))
	return SuccessResult{Kind: BuildFailure}, nil
}

// taskCommitMessage returns a conventional commit message for the given task type.
func taskCommitMessage(taskType types.TaskType, taskID string) string {
	switch taskType {
	case types.TaskTypeBugfix:
		return "fix: " + taskID
	case types.TaskTypeDocumentation:
		return "docs: " + taskID
	default:
		return "feat: " + taskID
	}
}
