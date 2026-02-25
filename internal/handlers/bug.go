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

// HandleBug processes a BUG outcome reported by the agent.
//
// Sequence:
//  1. Nested bug check — if the current task is already a bugfix, return a
//     Tier 3 fatal error immediately (before any rollback). A bugfix task
//     that itself reports BUG would cause a death spiral.
//  2. Rollback uncommitted changes (non-fatal; logged as warning).
//  3. Record task metrics (non-fatal; in-memory).
//  4. Generate bug ID: "BUG-" + ctx.TaskID.
//  5. Archive bug report from logs/ACTIVE_BUG.md to
//     logs/bugs/{epic}/bug-{taskID}.md (non-fatal if ACTIVE_BUG.md is absent).
//  6. Set active_task to { type: bugfix, id: BUG-{taskID} }.
//  7. Set next_task to the interrupted task: { type: <resolved>, id: ctx.TaskID }.
//     For user-defined tasks, type is looked up in tasks.yaml.
//     For synthetic tasks (documentation, etc.), type is taken from ctx.TaskType
//     directly — this avoids a tasks.yaml lookup that would always miss (CI-5 fix).
//  8. Persist updated state.
func HandleBug(ctx *orchestrator.LoopContext) error {
	// 1. Nested bug check — must run before rollback (Tier 3; no self-correction).
	if ctx.TaskType == types.TaskTypeBugfix {
		return fmt.Errorf("nested bug detected: task %s (type %s) reported BUG; "+
			"this would cause a death spiral — manual review required",
			ctx.TaskID, ctx.TaskType)
	}

	// 2. Rollback changes. Non-fatal — log warning and continue.
	if err := git.RollbackChanges(ctx.ProjectRoot, protectedPaths); err != nil {
		log.Warning(fmt.Sprintf("rollback failed: %v", err))
	}

	// 3. Record metrics (non-fatal; in-memory only).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, "bug", duration)

	// 4. Generate bug ID.
	bugID := "BUG-" + ctx.TaskID

	// 5. Archive bug report from logs/ACTIVE_BUG.md (non-fatal).
	if err := archiveBugReport(ctx, bugID); err != nil {
		log.Warning(fmt.Sprintf("bug archive skipped: %v", err))
	}

	// 6 & 7. Schedule the bugfix task and record the interrupted task as next.
	interruptedType := resolveInterruptedType(ctx)
	ctx.State.ActiveTask = types.TaskPointer{
		Type: types.TaskTypeBugfix,
		ID:   bugID,
	}
	ctx.State.NextTask = types.TaskPointer{
		Type: interruptedType,
		ID:   ctx.TaskID,
	}

	// 8. Persist updated state.
	if err := state.SaveProjectState(ctx.StatePath, ctx.State); err != nil {
		return fmt.Errorf("save state after bug scheduling: %w", err)
	}

	log.Warning(fmt.Sprintf("task %s interrupted by bug — scheduled bugfix %s; will resume %s next",
		ctx.TaskID, bugID, ctx.TaskID))
	return nil
}

// resolveInterruptedType returns the TaskType for the task that was interrupted
// by a bug discovery. It is placed in next_task so the orchestrator can resume
// after the bugfix completes.
//
// For synthetic tasks (documentation, manual_review, bugfix): ctx.TaskType is
// returned directly, because synthetic tasks are never in tasks.yaml (CI-5 fix).
//
// For user-defined tasks: the task list is searched by ID and the stored type is
// returned. If the task is not found (should not happen for well-formed state),
// ctx.TaskType is used as a fallback with a warning.
func resolveInterruptedType(ctx *orchestrator.LoopContext) types.TaskType {
	if ctx.TaskType.IsSynthetic() {
		return ctx.TaskType
	}
	for _, t := range ctx.Tasks.Epic.Tasks {
		if t.ID == ctx.TaskID {
			return t.Type
		}
	}
	log.Warning(fmt.Sprintf("task %s not found in tasks.yaml — using type %s for next_task",
		ctx.TaskID, ctx.TaskType))
	return ctx.TaskType
}

// archiveBugReport copies logs/ACTIVE_BUG.md to
// logs/bugs/{epic}/bug-{taskID}.md.
//
// Returns a non-fatal error when:
//   - logs/ACTIVE_BUG.md does not exist (CI-1 design: flat path, not subdirectory)
//   - any I/O error occurs during the copy
func archiveBugReport(ctx *orchestrator.LoopContext, bugID string) error {
	src := filepath.Join(ctx.LogsDir, "ACTIVE_BUG.md")
	data, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("logs/ACTIVE_BUG.md not found — skipping archive")
		}
		return fmt.Errorf("read ACTIVE_BUG.md: %w", err)
	}

	epicID := ctx.State.CurrentEpic.ID
	dst := filepath.Join(ctx.LogsDir, "bugs", epicID, "bug-"+ctx.TaskID+".md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir for bug archive: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write bug archive: %w", err)
	}
	log.Info(fmt.Sprintf("bug report archived to %s (bug ID: %s)", dst, bugID))
	return nil
}
