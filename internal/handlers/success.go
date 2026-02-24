// Package handlers implements the outcome handlers for the orchestration loop.
// Each handler receives a *orchestrator.LoopContext and performs the full
// response sequence for one of the four agent outcomes: SUCCESS, FAILURE,
// BUG, or EPIC_COMPLETE.
package handlers

import (
	"fmt"
	"time"

	"github.com/robertgumeny/doug/internal/changelog"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/metrics"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// SuccessResultKind classifies the outcome of HandleSuccess.
type SuccessResultKind int

const (
	// Continue means the task completed normally and the main loop should
	// proceed to the next iteration with the updated task pointers.
	Continue SuccessResultKind = iota

	// Retry means a non-fatal issue occurred (build/test failure or git
	// commit failure). The main loop should continue to the next iteration
	// and allow the state machine to recover naturally.
	Retry

	// EpicComplete means the KB synthesis documentation task completed
	// successfully. The caller should invoke HandleEpicComplete next.
	EpicComplete
)

// SuccessResult is returned by HandleSuccess to direct the main loop.
type SuccessResult struct {
	Kind SuccessResultKind
}

// protectedPaths are state-tracking files that must be preserved across a git
// rollback so the orchestrator does not lose its place after a bad agent run.
var protectedPaths = []string{
	"project-state.yaml",
	"tasks.yaml",
}

// HandleSuccess processes a SUCCESS outcome reported by the agent.
//
// Sequence:
//  1. Install new dependencies if the session result lists any.
//  2. Verify build — on failure: rollback, return Retry.
//  3. Verify tests  — on failure: rollback, return Retry.
//  4. Record task metrics in state (non-fatal, in-memory).
//  5. Update CHANGELOG.md (non-fatal; logs warning on error).
//  6. Mark user-defined task DONE in tasks.yaml.
//  7. For documentation tasks: set current_epic.completed_at, save state,
//     commit, return EpicComplete.
//  8. For feature/bugfix tasks: inject KB_UPDATE or advance task pointers.
//  9. Persist state.
// 10. Commit — on failure: log warning, return Retry (non-fatal).
// 11. Return Continue.
func HandleSuccess(ctx *orchestrator.LoopContext) (SuccessResult, error) {
	// 1. Install new dependencies if any were added by the agent.
	if len(ctx.SessionResult.DependenciesAdded) > 0 {
		log.Info(fmt.Sprintf("installing new dependencies: %v", ctx.SessionResult.DependenciesAdded))
		if err := ctx.BuildSystem.Install(); err != nil {
			log.Error(fmt.Sprintf("dependency install failed: %v", err))
			if rbErr := git.RollbackChanges(ctx.ProjectRoot, protectedPaths); rbErr != nil {
				return SuccessResult{Kind: Retry}, fmt.Errorf("rollback after dependency install failure: %w", rbErr)
			}
			return SuccessResult{Kind: Retry}, nil
		}
	}

	// 2. Verify build.
	log.Info("verifying build")
	if err := ctx.BuildSystem.Build(); err != nil {
		log.Error(fmt.Sprintf("build verification failed:\n%v", err))
		if rbErr := git.RollbackChanges(ctx.ProjectRoot, protectedPaths); rbErr != nil {
			return SuccessResult{Kind: Retry}, fmt.Errorf("rollback after build failure: %w", rbErr)
		}
		return SuccessResult{Kind: Retry}, nil
	}
	log.Success("build passed")

	// 3. Verify tests.
	log.Info("verifying tests")
	if err := ctx.BuildSystem.Test(); err != nil {
		log.Error(fmt.Sprintf("test verification failed:\n%v", err))
		if rbErr := git.RollbackChanges(ctx.ProjectRoot, protectedPaths); rbErr != nil {
			return SuccessResult{Kind: Retry}, fmt.Errorf("rollback after test failure: %w", rbErr)
		}
		return SuccessResult{Kind: Retry}, nil
	}
	log.Success("tests passed")

	// 4. Record task metrics (in-memory; non-fatal if the task ID is odd).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, "success", duration)

	// 5. Update CHANGELOG.md (non-fatal).
	if ctx.SessionResult.ChangelogEntry != "" {
		if err := changelog.UpdateChangelog(
			ctx.ChangelogPath,
			ctx.SessionResult.ChangelogEntry,
			string(ctx.TaskType),
		); err != nil {
			log.Warning(fmt.Sprintf("changelog update skipped: %v", err))
		}
	}

	// 6. Mark user-defined task as DONE (synthetic tasks are never in tasks.yaml).
	if !ctx.TaskType.IsSynthetic() {
		if err := orchestrator.UpdateTaskStatus(ctx.Tasks, ctx.TaskID, types.StatusDone); err != nil {
			log.Warning(fmt.Sprintf("could not mark task %s done: %v", ctx.TaskID, err))
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
			log.Warning(fmt.Sprintf("git commit failed for docs task %s: %v", ctx.TaskID, err))
			return SuccessResult{Kind: Retry}, nil
		}
		return SuccessResult{Kind: EpicComplete}, nil
	}

	// 8. Advance task pointers or inject KB synthesis.
	if orchestrator.NeedsKBSynthesis(ctx.State, ctx.Tasks) {
		log.Info("all feature tasks complete — scheduling KB synthesis")
		ctx.State.ActiveTask = types.TaskPointer{
			Type: types.TaskTypeDocumentation,
			ID:   "KB_UPDATE",
		}
		ctx.State.NextTask = types.TaskPointer{}
	} else {
		orchestrator.AdvanceToNextTask(ctx.State, ctx.Tasks)
	}

	// 9. Persist updated state.
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		return SuccessResult{Kind: Retry}, fmt.Errorf("save state: %w", err)
	}

	// 10. Commit all changes for this task.
	commitMsg := taskCommitMessage(ctx.TaskType, ctx.TaskID)
	if err := git.Commit(commitMsg, ctx.ProjectRoot); err != nil {
		log.Warning(fmt.Sprintf("git commit failed for task %s: %v", ctx.TaskID, err))
		return SuccessResult{Kind: Retry}, nil
	}

	log.Success(fmt.Sprintf("task %s committed", ctx.TaskID))
	return SuccessResult{Kind: Continue}, nil
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
