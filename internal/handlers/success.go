// Package handlers implements the outcome handlers for the orchestration loop.
// Each handler receives a *types.LoopContext and performs the full
// response sequence for one of the four agent outcomes: SUCCESS, FAILURE,
// BUG, or EPIC_COMPLETE.
package handlers

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/changelog"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/lifecycle"
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

	// EpicComplete means epic execution is complete and the caller should invoke
	// HandleEpicComplete next.
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

// HandleSuccess processes a SUCCESS outcome reported by the agent. It installs
// any new dependencies, verifies the build and tests, records task metadata,
// updates task state, commits the result, and tells the main loop whether to
// continue, retry, or finish the epic.
func HandleSuccess(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (SuccessResult, error) {
	var successResult SuccessResult
	var retErr error
	defer func() {
		if successResult.Kind == EpicComplete {
			return
		}
		if err := agent.CleanupActiveTask(ctx.DougDir); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("active task cleanup failed: %v", err))
		}
	}()

	// 0. Archive ACTIVE_TASK.md unconditionally before any state change.
	if err := agent.ArchiveActiveTask(ctx.DougDir, ctx.LogsDir, ctx.CurrentEpic.ID, ctx.TaskID, ctx.Attempts); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("session archive failed: %v", err))
	}

	// 0a. Reject SUCCESS results that include any blocking bug payload.
	// A blocking bug must be surfaced through a BUG outcome, not SUCCESS.
	for _, b := range result.Bugs {
		if b.Severity == types.SessionBugSeverityBlocking {
			return SuccessResult{}, fmt.Errorf(
				"task %s: SUCCESS result contains a blocking bug payload; "+
					"use BUG outcome to surface blocking bugs",
				ctx.TaskID)
		}
	}

	// 0b. Archive non-blocking bugs before advancing task pointers (non-fatal).
	epicID := ctx.State.CurrentEpic.ID
	for i, b := range result.Bugs {
		if b.Severity != types.SessionBugSeverityNonBlocking {
			continue
		}
		bugID := fmt.Sprintf("NB-BUG-%s-%d", ctx.TaskID, i+1)
		payload := types.BugPayload{
			BugID:            bugID,
			DiscoveredByTask: ctx.TaskID,
			Severity:         types.BugSeverityLow,
			Status:           types.BugStatusOpen,
			Body:             b.Body,
		}
		if _, err := agent.WriteBugArchive(ctx.LogsDir, epicID, payload); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("non-blocking bug archive failed for %s: %v", bugID, err))
		}
	}

	// 1. Install new dependencies if any were added by the agent.
	if len(result.DependenciesAdded) > 0 {
		ctx.Logger.Info(fmt.Sprintf("installing new dependencies: %v", result.DependenciesAdded))
		if err := ctx.BuildSystem.Install(); err != nil {
			successResult, retErr = pauseProject(ctx, fmt.Sprintf("dependency install failed: %v", err))
			return successResult, retErr
		}
	}

	// 1b. Ensure dependencies are present for verification even when the
	// agent's sandboxed install did not persist into the orchestrator workspace.
	if !ctx.BuildSystem.IsInitialized() {
		ctx.Logger.Info("build system not initialized; installing dependencies before verification")
		if err := ctx.BuildSystem.Install(); err != nil {
			successResult, retErr = pauseProject(ctx, fmt.Sprintf("dependency install failed: %v", err))
			return successResult, retErr
		}
	}

	// 2. Verify build.
	ctx.Logger.Info("verifying build")
	if err := ctx.BuildSystem.Build(); err != nil {
		successResult, retErr = pauseProject(ctx, fmt.Sprintf("build verification failed: %v", err))
		return successResult, retErr
	}
	ctx.Logger.Success("build passed")

	// 3. Verify tests.
	ctx.Logger.Info("verifying tests")
	if err := ctx.BuildSystem.Test(); err != nil {
		if ctx.State.ActiveTask.ConsecutiveTestFailures >= 1 {
			// Second consecutive test failure after SUCCESS — pause the project.
			successResult, retErr = pauseProject(ctx, fmt.Sprintf("test verification failed: %v", err))
			return successResult, retErr
		}
		// First test failure — inject output into next briefing and retry.
		// Retry counter increments normally (no decrement unlike BUILD_FAILURE).
		ctx.State.ActiveTask.ConsecutiveTestFailures++
		ctx.State.ActiveTask.TestFailureOutput = err.Error()
		if saveErr := state.SaveProjectState(ctx.StatePath, ctx.State); saveErr != nil {
			successResult = SuccessResult{Kind: BuildFailure}
			retErr = fmt.Errorf("save state after test failure: %w", saveErr)
			return successResult, retErr
		}
		ctx.Logger.Warning(fmt.Sprintf("test failure on attempt %d for task %s — retrying with test output injected", ctx.Attempts, ctx.TaskID))
		successResult = SuccessResult{Kind: Retry}
		return successResult, nil
	}
	ctx.Logger.Success("tests passed")
	// Reset consecutive test failure tracking now that tests are passing.
	ctx.State.ActiveTask.ConsecutiveTestFailures = 0
	ctx.State.ActiveTask.TestFailureOutput = ""

	// 3b. Run lint validation if enabled.
	if ctx.Config.LintEnabled {
		if err := runLint(ctx); err != nil {
			successResult, retErr = pauseProject(ctx, fmt.Sprintf("lint verification failed: %v", err))
			return successResult, retErr
		}
	}

	// 3c. For unambiguous Doug-scheduled BUG-* tasks, update the corresponding
	// reported bug file to "fixed" with resolver metadata. Non-fatal: an
	// ambiguous, missing, unreadable, or malformed archive is logged as a warning
	// and never blocks the bugfix outcome or the interrupted task's resumption.
	if archivePath, ok := bugfixArchiveWritebackPath(ctx); ok {
		if err := agent.UpdateBugArchiveResolved(archivePath, ctx.TaskID); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("bug archive writeback failed for %s: %v", ctx.TaskID, err))
		}
	}

	// 4. Record task metrics (in-memory; non-fatal if the task ID is odd).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, string(types.OutcomeSuccess), duration, ctx.Attempts, string(ctx.TaskType), agentDurationSeconds, ctx.ProviderWaitMs, ctx.ProviderFailures)

	// 5. Update CHANGELOG.md (non-fatal).
	if result.ChangelogEntry != "" {
		category := result.ChangelogCategory
		if category == "" {
			category = taskTypeToCategory(ctx.TaskType)
		}
		if err := changelog.UpdateChangelog(
			ctx.ChangelogPath,
			result.ChangelogEntry,
			category,
		); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("changelog update skipped: %v", err))
		}
	}

	// 6. Apply the shared verified-completion lifecycle transition. User backlog
	// tasks are marked DONE before advancing; handler-injected synthetic tasks
	// reuse the same pointer advancement without requiring a tasks.yaml entry.
	markDone := taskExists(ctx.Tasks, ctx.TaskID)
	if !markDone {
		ctx.Logger.Warning(fmt.Sprintf("could not mark task %s done: task not found in backlog", ctx.TaskID))
	}
	completion, err := lifecycle.ApplyVerifiedCompletion(ctx.State, ctx.Tasks, ctx.TaskID, markDone, time.Now())
	if err != nil {
		successResult = SuccessResult{Kind: Retry}
		retErr = err
		return successResult, retErr
	}
	if err := state.SaveTasks(ctx.TasksPath, ctx.Tasks); err != nil {
		successResult = SuccessResult{Kind: Retry}
		retErr = fmt.Errorf("save tasks after marking DONE: %w", err)
		return successResult, retErr
	}

	// 7. Persist updated state.
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		successResult = SuccessResult{Kind: Retry}
		if completion.Terminal {
			retErr = fmt.Errorf("save state after terminal task completion: %w", err)
		} else {
			retErr = fmt.Errorf("save state: %w", err)
		}
		return successResult, retErr
	}

	if completion.Terminal {
		if err := git.Commit(taskCommitMessage(ctx.TaskType, ctx.TaskID), ctx.ProjectRoot); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("git commit failed for terminal task %s: %v", ctx.TaskID, err))
			successResult = SuccessResult{Kind: Retry}
			return successResult, nil
		}
		backfillCommitSHA(ctx)
		successResult = SuccessResult{Kind: EpicComplete}
		return successResult, nil
	}

	// 8. Commit all changes for this task.
	commitMsg := taskCommitMessage(ctx.TaskType, ctx.TaskID)
	if err := git.Commit(commitMsg, ctx.ProjectRoot); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("git commit failed for task %s: %v", ctx.TaskID, err))
		successResult = SuccessResult{Kind: Retry}
		return successResult, nil
	}

	// 10. Backfill commit SHA into the last metrics entry and persist.
	backfillCommitSHA(ctx)

	ctx.Logger.Success(fmt.Sprintf("task %s committed", ctx.TaskID))
	successResult = SuccessResult{Kind: Continue}
	return successResult, nil
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

// runLint executes lint validation for the current context. When LintCommand is
// explicitly set, it is parsed and run via build.RunLint (no sh -c). When
// LintCommand is empty, the build-system default from BuildSystem.Lint() is used.
func runLint(ctx *types.LoopContext) error {
	if ctx.Config.LintCommand != "" {
		ctx.Logger.Info(fmt.Sprintf("verifying lint: %s", ctx.Config.LintCommand))
		if err := build.RunLint(ctx.ProjectRoot, ctx.Config.LintCommand); err != nil {
			return err
		}
		ctx.Logger.Success("lint passed")
		return nil
	}
	// Use build-system default only if one is registered.
	if bs, ok := config.BuildSystems[ctx.Config.BuildSystem]; ok && bs.LintCmd != "" {
		ctx.Logger.Info("verifying lint")
		if err := ctx.BuildSystem.Lint(); err != nil {
			return err
		}
		ctx.Logger.Success("lint passed")
	}
	return nil
}

func bugfixArchiveWritebackPath(ctx *types.LoopContext) (string, bool) {
	if ctx.TaskType != types.TaskTypeBugfix {
		return "", false
	}
	if !strings.HasPrefix(ctx.TaskID, types.BugTaskIDPrefix) {
		ctx.Logger.Warning(fmt.Sprintf("bug archive writeback skipped for %s: bugfix task ID is not a %s task", ctx.TaskID, types.BugTaskIDPrefix))
		return "", false
	}
	if ctx.State.ActiveTask.BugID != "" && ctx.State.ActiveTask.BugID != ctx.TaskID {
		ctx.Logger.Warning(fmt.Sprintf("bug archive writeback skipped for %s: active bug ID %q does not match task ID", ctx.TaskID, ctx.State.ActiveTask.BugID))
		return "", false
	}
	archivePath := ctx.State.ActiveTask.BugArchivePath
	if archivePath == "" {
		ctx.Logger.Warning(fmt.Sprintf("bug archive writeback skipped for %s: no bug archive path recorded", ctx.TaskID))
		return "", false
	}
	if !filepath.IsAbs(archivePath) {
		archivePath = filepath.Join(ctx.ProjectRoot, archivePath)
	}
	return archivePath, true
}

func taskExists(tasks *types.Tasks, taskID string) bool {
	for _, task := range tasks.Epic.Tasks {
		if task.ID == taskID {
			return true
		}
	}
	return false
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

// taskTypeToCategory maps a TaskType to its corresponding ChangelogCategory.
// Unknown task types return a ChangelogCategory equal to the raw task type
// string, which UpdateChangelog will reject as an unknown category (non-fatal).
func taskTypeToCategory(t types.TaskType) types.ChangelogCategory {
	switch t {
	case types.TaskTypeFeature:
		return types.CategoryAdded
	case types.TaskTypeBugfix:
		return types.CategoryFixed
	case types.TaskTypeDocumentation:
		return types.CategoryChanged
	default:
		return types.ChangelogCategory(t)
	}
}
