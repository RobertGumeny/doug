package handlers

import (
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

// HandleBug processes a BUG outcome reported by the agent.
//
// Sequence:
//  1. Nested bug check — if the current task is already a bugfix, return a
//     Tier 3 fatal error immediately (before any rollback). A bugfix task
//     that itself reports BUG would cause a death spiral.
//  2. Rollback uncommitted changes (non-fatal; logged as warning).
//  3. Record task metrics (non-fatal; in-memory).
//  4. Generate bug ID: "BUG-" + ctx.TaskID.
//  5. Archive bug report from .doug/ACTIVE_BUG.md to
//     logs/bugs/{epic}/bug-{taskID}.md (or a versioned sibling on repeats).
//     Missing ACTIVE_BUG.md is fatal because the bugfix task cannot run blind.
//  6. Set active_task to { type: bugfix, id: BUG-{taskID} }.
//  7. Set next_task to the interrupted task: { type: <resolved>, id: ctx.TaskID }.
//     For user-defined tasks, type is looked up in tasks.yaml.
//     For tasks not in tasks.yaml (e.g., handler-injected tasks), type falls
//     back to ctx.TaskType after a failed lookup.
//  8. Persist updated state.
func HandleBug(ctx *types.LoopContext, agentDurationSeconds int) error {
	defer func() {
		if err := agent.CleanupActiveTask(ctx.DougDir); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("active task cleanup failed: %v", err))
		}
	}()

	// 0. Archive ACTIVE_TASK.md unconditionally before any state change.
	if err := agent.ArchiveActiveTask(ctx.DougDir, ctx.LogsDir, ctx.CurrentEpic.ID, ctx.TaskID, ctx.Attempts); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("session archive failed: %v", err))
	}

	// 1. Nested bug check — must run before rollback (Tier 3; no self-correction).
	if ctx.TaskType == types.TaskTypeBugfix {
		return fmt.Errorf("nested bug detected: task %s (type %s) reported BUG; "+
			"this would cause a death spiral — manual review required",
			ctx.TaskID, ctx.TaskType)
	}

	// 2. Rollback changes. Non-fatal — log warning and continue.
	if err := git.RollbackChanges(ctx.ProjectRoot, git.DefaultProtectedPaths); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("rollback failed: %v", err))
	}

	// 3. Record metrics (non-fatal; in-memory only).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, string(types.OutcomeBug), duration, ctx.Attempts, string(ctx.TaskType), agentDurationSeconds, ctx.ProviderWaitMs, ctx.ProviderFailures)

	// 4. Generate bug ID.
	bugID := "BUG-" + ctx.TaskID

	// 5. Archive the blocking bug report before scheduling the bugfix.
	if err := archiveBugReport(ctx, bugID); err != nil {
		return fmt.Errorf("archive blocking bug report: %w", err)
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

	ctx.Logger.Warning(fmt.Sprintf("task %s interrupted by bug — scheduled bugfix %s; will resume %s next",
		ctx.TaskID, bugID, ctx.TaskID))
	return nil
}

// resolveInterruptedType returns the TaskType for the task that was interrupted
// by a bug discovery. It is placed in next_task so the orchestrator can resume
// after the bugfix completes.
//
// For scaffold (runtime-only): ctx.TaskType is returned directly since scaffold
// tasks are never in tasks.yaml.
//
// For all other types (feature, bugfix, documentation): the task list is
// searched by ID and the stored type is returned. If not found (e.g., a
// handler-injected task with a non-backlog ID), ctx.TaskType is used as fallback.
func resolveInterruptedType(ctx *types.LoopContext) types.TaskType {
	if ctx.TaskType.IsSynthetic() {
		return ctx.TaskType
	}
	for _, t := range ctx.Tasks.Epic.Tasks {
		if t.ID == ctx.TaskID {
			return t.Type
		}
	}
	ctx.Logger.Warning(fmt.Sprintf("task %s not found in tasks.yaml — using type %s for next_task",
		ctx.TaskID, ctx.TaskType))
	return ctx.TaskType
}

// archiveBugReport reads .doug/ACTIVE_BUG.md and delegates to
// agent.WriteBugArchive, which stamps required frontmatter and handles
// versioned archive filenames.
//
// Returns an error when:
//   - .doug/ACTIVE_BUG.md does not exist
//   - any I/O error occurs during the write
func archiveBugReport(ctx *types.LoopContext, bugID string) error {
	src := filepath.Join(ctx.DougDir, "ACTIVE_BUG.md")
	body, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".doug/ACTIVE_BUG.md not found")
		}
		return fmt.Errorf("read ACTIVE_BUG.md: %w", err)
	}

	payload := types.BugPayload{
		BugID:            bugID,
		DiscoveredByTask: ctx.TaskID,
		Severity:         types.BugSeverityMedium,
		Status:           types.BugStatusOpen,
		Body:             string(body),
	}

	epicID := ctx.State.CurrentEpic.ID
	if err := agent.WriteBugArchive(ctx.LogsDir, epicID, payload); err != nil {
		return fmt.Errorf("write bug archive: %w", err)
	}

	ctx.Logger.Info(fmt.Sprintf("bug report archived to .doug/logs/bugs/%s (bug ID: %s)", epicID, bugID))
	return nil
}
