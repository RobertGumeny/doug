package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/metrics"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// HandleFailure processes a FAILURE outcome reported by the agent.
//
// Sequence:
//  1. Rollback uncommitted changes (rollback error is non-fatal; logged as warning).
//  2. Record task metrics (non-fatal; in-memory).
//  3. Check attempt count against config.MaxRetries.
//     - Below max_retries: log retry warning, return nil (main loop continues).
//     - At or above max_retries: archive failure report from logs/ACTIVE_FAILURE.md
//       (missing file is non-fatal), mark task BLOCKED in tasks.yaml, set
//       active_task to manual_review in project-state.yaml, persist state, and
//       return a fatal error that includes the task ID and retry count.
func HandleFailure(ctx *orchestrator.LoopContext) error {
	// 1. Rollback changes. Non-fatal — log warning and continue.
	if err := git.RollbackChanges(ctx.ProjectRoot, protectedPaths); err != nil {
		log.Warning(fmt.Sprintf("rollback failed: %v", err))
	}

	// 2. Record metrics (non-fatal; in-memory only).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, "failure", duration)

	// 3a. Below max_retries — schedule a retry.
	if ctx.Attempts < ctx.Config.MaxRetries {
		log.Warning(fmt.Sprintf("task %s failed (attempt %d/%d) — will retry",
			ctx.TaskID, ctx.Attempts, ctx.Config.MaxRetries))
		return nil
	}

	// 3b. MAX_RETRIES reached — block the task.
	log.Error(fmt.Sprintf("task %s has failed %d/%d times — marking BLOCKED",
		ctx.TaskID, ctx.Attempts, ctx.Config.MaxRetries))

	// Archive failure report from logs/ACTIVE_FAILURE.md (non-fatal).
	if err := archiveFailureReport(ctx); err != nil {
		log.Warning(fmt.Sprintf("failure archive skipped: %v", err))
	}

	// Mark task BLOCKED in tasks.yaml (skipped for synthetic tasks).
	if !ctx.TaskType.IsSynthetic() {
		if err := orchestrator.UpdateTaskStatus(ctx.Tasks, ctx.TaskID, types.StatusBlocked); err != nil {
			log.Warning(fmt.Sprintf("could not mark task %s blocked: %v", ctx.TaskID, err))
		} else if err := state.SaveTasks(ctx.TasksPath, ctx.Tasks); err != nil {
			log.Warning(fmt.Sprintf("could not save tasks after blocking task %s: %v", ctx.TaskID, err))
		}
	}

	// Set active_task to manual_review and persist state.
	ctx.State.ActiveTask = types.TaskPointer{
		Type: types.TaskTypeManualReview,
		ID:   ctx.TaskID,
	}
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		log.Warning(fmt.Sprintf("could not save state after setting manual review: %v", err))
	}

	return fmt.Errorf("task %s blocked after %d attempts: requires manual review",
		ctx.TaskID, ctx.Attempts)
}

// archiveFailureReport copies logs/ACTIVE_FAILURE.md to
// logs/failures/{epic}/failure-{taskID}.md.
//
// Returns a non-fatal error when:
//   - logs/ACTIVE_FAILURE.md does not exist (CI-2 design: flat path, not subdirectory)
//   - any I/O error occurs during copy
func archiveFailureReport(ctx *orchestrator.LoopContext) error {
	src := filepath.Join(ctx.LogsDir, "ACTIVE_FAILURE.md")
	data, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("logs/ACTIVE_FAILURE.md not found — skipping archive")
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
	log.Info(fmt.Sprintf("failure report archived to %s", dst))
	return nil
}
