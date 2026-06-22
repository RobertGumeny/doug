package handlers

import (
	"fmt"
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
//  1. Archive ACTIVE_TASK.md before any state change (non-fatal).
//  2. Nested bug check — if the current task is already a bugfix, return a
//     Tier 3 fatal error immediately. A bugfix task that itself reports BUG
//     would cause a death spiral.
//  3. Validate the result contains exactly one severity: blocking bug payload.
//     Zero blocking bugs, or more than one, is a fatal error returned before
//     any rollback or state mutation.
//  4. Rollback uncommitted changes (non-fatal; logged as warning).
//  5. Record task metrics (non-fatal; in-memory).
//  6. Generate bug ID: "BUG-" + ctx.TaskID.
//  7. Archive the blocking bug payload to
//     logs/bugs/{epic}/bug-{taskID}.md (or a versioned sibling on repeats).
//  8. Set active_task to { type: bugfix, id: BUG-{taskID} }.
//  9. Set next_task to the interrupted task: { type: <resolved>, id: ctx.TaskID }.
//     For user-defined tasks, type is looked up in tasks.yaml.
//     For tasks not in tasks.yaml (e.g., handler-injected tasks), type falls
//     back to ctx.TaskType after a failed lookup.
// 10. Persist updated state.
func HandleBug(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) error {
	defer func() {
		if err := agent.CleanupActiveTask(ctx.DougDir); err != nil {
			ctx.Logger.Warning(fmt.Sprintf("active task cleanup failed: %v", err))
		}
	}()

	// 1. Archive ACTIVE_TASK.md unconditionally before any state change.
	if err := agent.ArchiveActiveTask(ctx.DougDir, ctx.LogsDir, ctx.CurrentEpic.ID, ctx.TaskID, ctx.Attempts); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("session archive failed: %v", err))
	}

	// 2. Nested bug check — must run before any other processing (Tier 3; no self-correction).
	if ctx.TaskType == types.TaskTypeBugfix {
		return fmt.Errorf("nested bug detected: task %s (type %s) reported BUG; "+
			"this would cause a death spiral — manual review required",
			ctx.TaskID, ctx.TaskType)
	}

	// 3. Validate exactly one blocking bug payload in the result.
	var blockingBugs []types.SessionBug
	for _, b := range result.Bugs {
		if b.Severity == types.SessionBugSeverityBlocking {
			blockingBugs = append(blockingBugs, b)
		}
	}
	if len(blockingBugs) == 0 {
		return fmt.Errorf("task %s reported BUG with no blocking bug payload in result: "+
			"include exactly one bugs entry with severity: blocking",
			ctx.TaskID)
	}
	if len(blockingBugs) > 1 {
		return fmt.Errorf("task %s reported BUG with %d blocking bug payloads: "+
			"exactly one bugs entry with severity: blocking is required",
			ctx.TaskID, len(blockingBugs))
	}

	// 4. Rollback changes. Non-fatal — log warning and continue.
	if err := git.RollbackChanges(ctx.ProjectRoot, git.DefaultProtectedPaths); err != nil {
		ctx.Logger.Warning(fmt.Sprintf("rollback failed: %v", err))
	}

	// 5. Record metrics (non-fatal; in-memory only).
	duration := int(time.Since(ctx.TaskStartTime).Seconds())
	metrics.RecordTaskMetrics(ctx.State, ctx.TaskID, string(types.OutcomeBug), duration, ctx.Attempts, string(ctx.TaskType), agentDurationSeconds, ctx.ProviderWaitMs, ctx.ProviderFailures)

	// 6. Generate bug ID.
	bugID := types.BugTaskIDPrefix + ctx.TaskID

	// 7. Archive the blocking bug payload before scheduling the bugfix.
	archivePath, err := archiveBlockingBug(ctx, bugID, blockingBugs[0])
	if err != nil {
		return fmt.Errorf("archive blocking bug report: %w", err)
	}

	// Compute relative archive path for the brief (best-effort; full path used on failure).
	relArchivePath := archivePath
	if rel, relErr := filepath.Rel(ctx.ProjectRoot, archivePath); relErr == nil {
		relArchivePath = rel
	}

	// 8 & 9. Schedule the bugfix task and record the interrupted task as next.
	// The bug payload fields are persisted on the TaskPointer so that a crash or
	// restart before the next dispatch does not lose the bug context.
	interruptedType := resolveInterruptedType(ctx)
	ctx.State.ActiveTask = types.TaskPointer{
		Type:           types.TaskTypeBugfix,
		ID:             bugID,
		BugID:          bugID,
		BugSeverity:    string(types.BugSeverityHigh),
		BugSourceTask:  ctx.TaskID,
		BugBody:        blockingBugs[0].Body,
		BugArchivePath: relArchivePath,
	}
	ctx.State.NextTask = types.TaskPointer{
		Type: interruptedType,
		ID:   ctx.TaskID,
	}

	// 10. Persist updated state.
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

// archiveBlockingBug archives the single blocking bug payload from the session
// result under logs/bugs/{epic}/bug-{taskID}.md (or a versioned sibling).
// It returns the absolute path of the written archive file.
func archiveBlockingBug(ctx *types.LoopContext, bugID string, bug types.SessionBug) (string, error) {
	payload := types.BugPayload{
		BugID:            bugID,
		DiscoveredByTask: ctx.TaskID,
		Severity:         types.BugSeverityHigh,
		Status:           types.BugStatusOpen,
		Body:             bug.Body,
	}
	epicID := ctx.State.CurrentEpic.ID
	archivePath, err := agent.WriteBugArchive(ctx.LogsDir, epicID, payload)
	if err != nil {
		return "", fmt.Errorf("write bug archive: %w", err)
	}
	ctx.Logger.Info(fmt.Sprintf("blocking bug archived to .doug/logs/bugs/%s (bug ID: %s)", epicID, bugID))
	return archivePath, nil
}
