package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/metrics"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// HandleFailure processes a FAILURE outcome reported by the agent. It rolls
// back uncommitted changes, records metrics, and either schedules a retry or
// blocks the task and switches the active task to manual review after the
// retry limit is reached.
func HandleFailure(ctx *types.LoopContext, agentDurationSeconds int) error {
	// 0. Archive ACTIVE_TASK.md unconditionally before any state change.
	if err := agent.ArchiveActiveTask(ctx.DougDir, ctx.LogsDir, ctx.CurrentEpic.ID, ctx.TaskID, ctx.Attempts); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("session archive failed: %v", err))
	}

	// 1. Rollback changes. Non-fatal — log warning and continue.
	if err := git.RollbackChanges(ctx.ProjectRoot, git.DefaultProtectedPaths); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("rollback failed: %v", err))
	}

	// 2. Record metrics (non-fatal; in-memory only).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, string(types.OutcomeFailure), duration, ctx.Attempts, string(ctx.TaskType), agentDurationSeconds)

	// 3a. Below max_retries — persist metrics and schedule a retry.
	if ctx.Attempts < ctx.Config.MaxRetries {
		ctx.Logger.Warning(fmt.Sprintf("task %s failed (attempt %d/%d) — will retry",
			ctx.TaskID, ctx.Attempts, ctx.Config.MaxRetries))
		if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("could not persist failure metrics for task %s: %v", ctx.TaskID, err))
		}
		return nil
	}

	// 3b. MAX_RETRIES reached — block the task.
	ctx.Logger.Error(fmt.Sprintf("task %s has failed %d/%d times — marking BLOCKED",
		ctx.TaskID, ctx.Attempts, ctx.Config.MaxRetries))

	// Archive failure report from logs/ACTIVE_FAILURE.md (non-fatal).
	if err := archiveFailureReport(ctx); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("failure archive skipped: %v", err))
	}

	// Mark task BLOCKED in tasks.yaml (skipped for synthetic tasks).
	if !ctx.TaskType.IsSynthetic() {
		if err := types.UpdateTaskStatus(ctx.Tasks, ctx.TaskID, types.StatusBlocked); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("could not mark task %s blocked: %v", ctx.TaskID, err))
		} else if err := state.SaveTasks(ctx.TasksPath, ctx.Tasks); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("could not save tasks after blocking task %s: %v", ctx.TaskID, err))
		}
	}

	// Set active_task to manual_review and persist state.
	ctx.State.ActiveTask = types.TaskPointer{
		Type: types.TaskTypeManualReview,
		ID:   ctx.TaskID,
	}
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("could not save state after setting manual review: %v", err))
	}

	return fmt.Errorf("task %s blocked after %d attempts: requires manual review",
		ctx.TaskID, ctx.Attempts)
}

// archiveFailureReport copies .doug/ACTIVE_FAILURE.md to
// .doug/logs/failures/{epic}/failure-{taskID}.md. Missing source files and I/O
// failures are returned as non-fatal errors.
func archiveFailureReport(ctx *types.LoopContext) error {
	src := filepath.Join(ctx.DougDir, "ACTIVE_FAILURE.md")
	data, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(".doug/ACTIVE_FAILURE.md not found — skipping archive")
		}
		return fmt.Errorf("read ACTIVE_FAILURE.md: %w", err)
	}

	epicID := ctx.State.CurrentEpic.ID
	dst := filepath.Join(ctx.LogsDir, "failures", epicID, "failure-"+ctx.TaskID+".md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir for failure archive: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write failure archive: %w", err)
	}
	ctx.Logger.Info(fmt.Sprintf("failure report archived to %s", dst))
	return nil
}
