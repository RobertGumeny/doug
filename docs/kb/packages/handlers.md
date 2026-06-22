---
title: internal/handlers — Outcome Handlers & LoopContext
updated: 2026-06-21
category: Packages
tags: [handlers, success, failure, bug, epic, resume, paused, build-failure, loop-context, orchestration, logger, provider-stall]
related_articles:
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/types.md
  - docs/kb/packages/types-loop-context.md
  - docs/kb/packages/state.md
  - docs/kb/packages/git.md
  - docs/kb/packages/metrics.md
  - docs/kb/packages/changelog.md
  - docs/kb/packages/agent.md
  - docs/kb/features/run-ux-provider-visibility.md
  - docs/kb/infrastructure/go.md
---

# internal/handlers — Outcome Handlers

## Overview

`internal/handlers` implements the five outcome handlers for the orchestration loop. Each handler receives a `*types.LoopContext` and performs the full response sequence for one agent outcome: SUCCESS, FAILURE, BUG, EPIC_COMPLETE, or RESUME (PAUSED project).

Handlers that call `git.RollbackChanges` pass `git.DefaultProtectedPaths` (defined in `internal/git`) — the single source of truth for orchestrator state files that must survive a rollback.

**Handlers that process an agent-written `ACTIVE_TASK.md` call `agent.ArchiveActiveTask` first** — `HandleSuccess`, `HandleFailure`, `HandleBug`, and `HandleEpicComplete` archive the session file before any state mutation. This is non-fatal; a missing ACTIVE_TASK.md logs a warning and processing continues. They also clean up the live root file after handling so stale briefings do not linger. `HandleResume` does not archive because the resume path skips agent invocation entirely.

---

## LoopContext

`LoopContext` is defined in `internal/types/loop_context.go` and carries all per-iteration state. Every handler receives exactly one `*LoopContext` parameter. See [internal/types](types.md) for the full field reference.

Key fields for handler authors:

| Field | Purpose |
|-------|---------|
| `TaskID`, `TaskType`, `Attempts` | Per-iteration identity, snapshotted after `IncrementAttempts` |
| `CurrentEpic` | Snapshot of epic state for logging/commit messages |
| `Config` | `OrchestratorConfig` — `MaxRetries`, `BuildSystem`, etc. |
| `BuildSystem` | Build/test/install interface |
| `State`, `Tasks` | Mutable in-memory state — mutations persist by calling `state.Save*` |
| `StatePath`, `TasksPath`, `DougDir`, `LogsDir`, `ChangelogPath` | Resolved file paths |
| `ProviderWaitMs`, `ProviderFailures` | Runtime observability copied from the backend for metric persistence |
| `Logger` | Structured output — use `ctx.Logger.Info/Warning/Error/...` instead of package-level `log.*` |

`LoopContext` is constructed fresh each iteration in `Orchestrator.Run` after `IncrementAttempts`. Provider observability is filled after the backend returns and before outcome dispatch. `SessionResult` and `AgentDurationSeconds` are **not** on `LoopContext`; they are passed as explicit parameters to `HandleSuccess`.

---

## HandleSuccess

```go
func HandleSuccess(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (SuccessResult, error)
```

### SuccessResultKind

```go
type SuccessResultKind int

const (
    Continue     SuccessResultKind = iota  // normal forward progress
    Retry                                   // non-fatal; loop retries next iteration
    EpicComplete                            // user-task execution is done; caller runs HandleEpicComplete
    BuildFailure                            // build/test verification failed; project paused
)
```

### Sequence

0. **Archive** — `agent.ArchiveActiveTask(...)`. Non-fatal.
0a. **Reject blocking bugs on SUCCESS** — if any `result.Bugs` entry has `severity: blocking`, return a fatal error before any state advances or commits. Blocking bugs must be surfaced through a `BUG` outcome.
0b. **Archive non-blocking bugs** — for each `result.Bugs` entry with `severity: non-blocking`, write a durable archive via `agent.WriteBugArchive(...)` under `.doug/logs/bugs/{epic}/` (bug ID `NB-BUG-{taskID}-{n}`, severity `low`, status `open`). Non-fatal: a failed archive logs a warning and processing continues. This runs before task pointers advance.
1. **Install dependencies** — if `SessionResult.DependenciesAdded` is non-empty, call `BuildSystem.Install()`. If the build system is still uninitialized after the agent run, install as well. On failure: `pauseProject` → return `BuildFailure`.
2. **Build** — `BuildSystem.Build()`. On failure: `pauseProject` → return `BuildFailure`.
3. **Test** — `BuildSystem.Test()`. On failure: see **Test Failure Retry** below.
3c. **Bugfix archive writeback** — when `ctx.TaskType == TaskTypeBugfix` and the active task carries a `BugArchivePath`, call `agent.UpdateBugArchiveResolved(archivePath, ctx.TaskID)` to flip the matching archive to `fixed` with resolver metadata. Non-fatal: a missing/unreadable/malformed archive logs a warning and never blocks the bugfix outcome or the interrupted task's resumption.
3b. **Lint** — only when `ctx.Config.LintEnabled` is true. Calls `runLint(ctx)` which dispatches to `build.RunLint(projectRoot, LintCommand)` when `LintCommand` is set, or `BuildSystem.Lint()` when it is empty and the build system has a default. On failure: `pauseProject` → return `BuildFailure`. See [config.md](config.md) for `LintEnabled`/`LintCommand` semantics.
4. **Record metrics** — `metrics.RecordTaskMetrics(...)`, including provider wait/failure diagnostics from `LoopContext`. Non-fatal.
5. **Changelog** — `changelog.UpdateChangelog(...)` if `ChangelogEntry != ""`. Resolves category via `result.ChangelogCategory` with fallback to `taskTypeToCategory(ctx.TaskType)`. Non-fatal.
6. **Mark task DONE** — `types.UpdateTaskStatus(...)` + `state.SaveTasks(...)`. Skipped for synthetic tasks.
7. **Terminal-task branch** — if `types.AreAllUserTasksComplete(ctx.Tasks)`: set `CurrentEpic.CompletedAt`, save state, commit the terminal task, call `backfillCommitSHA`, return `EpicComplete`.
8. **Advance** — otherwise `AdvanceToNextTask()`. If no next task exists even though the epic is not terminal, return `Retry` with an error.
9. **Save state** — `state.SaveProjectState(...)`.
10. **Commit** — `git.Commit(commitMsg, ...)`. On failure: log warning, return `Retry` (non-fatal).
11. **Backfill commit SHA** — `backfillCommitSHA(ctx)`. Non-fatal; only writes when a metrics entry exists.
12. **Cleanup live briefing** — remove root `.doug/ACTIVE_TASK.md` before returning, except when returning `EpicComplete`
13. Return `Continue`.

### Test Failure Retry

On step 3 test failure, the handler checks `ctx.State.ActiveTask.ConsecutiveTestFailures`:

- **First failure** (`< 1`): store test output in `TestFailureOutput`, increment `ConsecutiveTestFailures` to 1, save state, return `Retry`. The next iteration's `WriteActiveTask` injects the stored output into the briefing as `## Previous Test Failure Output`. Attempt counter increments normally (this is a real retry, not a pause).
- **Second consecutive failure** (`>= 1`): call `pauseProject` → return `BuildFailure`. Attempt counter is decremented.
- **Tests pass**: reset `ConsecutiveTestFailures = 0` and `TestFailureOutput = ""` before continuing.

### pauseProject helper

```go
func pauseProject(ctx *types.LoopContext, reason string) (SuccessResult, error)
```

1. Decrements `ctx.State.ActiveTask.Attempts` (so BUILD_FAILURE never consumes a retry slot)
2. Sets `ctx.State.Status = types.ProjectStatusPaused`
3. Saves state
4. Logs a user-actionable message: which field to clear and what command to run to resume

### Commit message convention

| Task type | Prefix |
|-----------|--------|
| `feature` | `feat:` |
| `bugfix`  | `fix:`  |
| `documentation` | `docs:` |

### Key decisions

- **Build/dep install failure → pause, not retry**: After an agent reports SUCCESS, rollback-and-retry is wrong — the working tree changes are likely correct. Pausing preserves the work and prompts the developer to fix the environment.
- **First test failure → retry with output**: Test output gives the agent context to fix failures. Second consecutive failure pauses instead of spinning forever.
- **Attempt counter decremented in `pauseProject`**: Increment happens at the top of the loop before the agent runs; decrement ensures BUILD_FAILURE never consumes a retry slot.
- **Git commit failure is non-fatal**: Returns `Retry`. State/tasks writes already persisted; the state machine recovers on the next iteration.

---

## HandleResume

```go
func HandleResume(ctx *types.LoopContext) (SuccessResult, error)
```

Called when `doug run` detects a paused project (`ctx.State.Status == ProjectStatusPaused`) instead of running an agent. Runs build/test verification against the current working tree (no agent invocation).

### Sequence

0. **Clear status**: `ctx.State.Status = ""` so a passing build leaves status empty.
1. **Install** (if uninitialized build system) → on failure: `pauseProject` → return `BuildFailure`.
2. **Build** → on failure: `pauseProject` → return `BuildFailure`.
3. **Test** → on failure: `pauseProject` → return `BuildFailure`.
3b. **Lint** — only when `ctx.Config.LintEnabled` is true. Same `runLint(ctx)` helper as `HandleSuccess`. On failure: `pauseProject` → return `BuildFailure`.
4. **Mark task DONE** (skip for synthetic tasks).
5. **Terminal-task branch** — if `types.AreAllUserTasksComplete(ctx.Tasks)`: set `CurrentEpic.CompletedAt`, save state, commit the terminal task, call `backfillCommitSHA`, return `EpicComplete`.
6. **Advance** — otherwise promote `NextTask`; if none exists while the epic is not terminal, return `Retry`.
7. **Save state**.
8. **Commit**.
9. **Backfill commit SHA** — non-fatal; usually a no-op because resume does not record a fresh metrics entry.
10. Return `Continue`.

### Key decisions

- **Attempt counter NOT incremented on resume**: The resume iteration is not a new agent attempt. The increment at the top of the loop is skipped in `internal/orchestrator/run.go` for resume iterations.
- **`EnsureProjectReady` skipped on resume**: Pre-flight build/test is skipped to avoid a double run and to ensure PAUSED status is re-set correctly on failure.
- **`pauseProject` called on resume failure**: Consistent behavior — any build/test failure sets PAUSED status with decremented attempts.
- **Resume is narrower than success**: `HandleResume` verifies and commits the preserved working tree, but it does not record task metrics or update `CHANGELOG.md`.

---

## HandleFailure

```go
func HandleFailure(ctx *types.LoopContext, agentDurationSeconds int) error
```

### Sequence

0. **Archive** — `agent.ArchiveActiveTask(...)`. Non-fatal.
1. **Rollback** — `git.RollbackChanges(...)`. Non-fatal.
2. **Record metrics** — non-fatal.
3. **Check retry count**:
   - `ctx.Attempts < cfg.MaxRetries` → `SaveProjectState` (persists failure metric), log warning, return `nil` (loop retries).
   - `ctx.Attempts >= cfg.MaxRetries` → block the task:
     - Mark the originating backlog task `BLOCKED`.
     - Leave `active_task` pointing at that blocked backlog task, clear transient retry/test-failure fields, clear `next_task`, and save.
     - For failed synthetic bugfix tasks, fold the blocked state back onto the interrupted backlog task from `next_task`.
     - Return `fmt.Errorf("task %s blocked after %d attempts: requires manual review", ...)`.
4. **Cleanup live briefing** — remove root `.doug/ACTIVE_TASK.md` before returning on both retry and blocked paths.

`HandleFailure` does not read or archive a separate failure handoff file; the structured `ACTIVE_TASK.md` result and archived session are the task-failure record.

---

## HandleBug

```go
func HandleBug(ctx *types.LoopContext, agentDurationSeconds int) error
```

### Sequence

0. **Archive** — `agent.ArchiveActiveTask(...)`. Non-fatal.
1. **Nested bug check** — if `TaskType == TaskTypeBugfix`, return fatal error immediately (Tier 3). A bugfix task reporting BUG would create a death spiral.
2. **Validate blocking bug payload** — exactly one `SessionBug` with `severity: blocking` must be in `result.Bugs`. Zero or multiple blocking bugs is a fatal error before any state mutation.
3. **Rollback** — non-fatal.
4. **Record metrics** — non-fatal.
5. **Generate bug ID** — `"BUG-" + ctx.TaskID`.
6. **Archive blocking bug payload** — write the blocking bug to `logs/bugs/{epic}/bug-{taskID}.md` (or a versioned sibling on repeats). Returns the absolute archive path.
7. **Schedule bugfix with full payload** — set `active_task = { type: bugfix, id: BUG-{taskID}, bug_id: ..., bug_severity: ..., bug_source_task: ..., bug_body: ..., bug_archive_path: ... }`. The bug payload fields on `TaskPointer` survive crash/restart so the bugfix brief can be rendered without any separate file.
8. **Preserve interrupted task** — set `next_task = { type: resolveInterruptedType(), id: ctx.TaskID }`.
9. **Save state**.
10. **Cleanup live briefing** — remove root `.doug/ACTIVE_TASK.md` before returning.

### resolveInterruptedType

`scaffold` (runtime-only) returns `ctx.TaskType` directly — scaffold tasks are never in `tasks.yaml`. For all other types (`feature`, `bugfix`, `documentation`): looks up the task by ID in `ctx.Tasks.Epic.Tasks` and returns the stored type. Fallback to `ctx.TaskType` with a warning log if the ID is not found (e.g., a handler-injected task with a non-backlog ID).

---

## HandleEpicComplete

```go
func HandleEpicComplete(ctx *types.LoopContext) error
```

### Sequence

0. **Archive** — `agent.ArchiveActiveTask(...)`. Non-fatal.
1. **Ensure completion timestamp** — if `current_epic.completed_at` is nil/empty, set it to now and save state.
2. **Finalize backlog/runtime completion** — `plan.FinalizeEpicCompletion(...)`:
   - archives the executed root `.doug/` snapshot into `.doug/logs/archives/{epic}/`
   - updates `.doug/plan/epics/{epic}/metadata.yaml` from `ACTIVE` to `COMPLETED` when backlog metadata exists
   - returns an error if backlog metadata exists but is not `ACTIVE`
   - still archives the runtime snapshot when the epic was run from the direct root-level path with no backlog package
3. **Print summary** — `metrics.PrintEpicSummary(os.Stderr, ctx.State)`.
4. **Commit finalization** — `git.Commit("chore: finalize {epicID}", ctx.ProjectRoot)`:
   - `git.ErrNothingToCommit` → non-fatal; log info and continue.
   - Any other error → return explicit error (Tier 3; CI-6 fix).
5. **Print completion banner** — `log.Section("EPIC {epicID} COMPLETE")`.
6. **Cleanup live briefing** — remove root `.doug/ACTIVE_TASK.md` when the handler returns, after the runtime snapshot archival/finalization work has completed.

---

## Run Loop Integration (Orchestrator.Run)

The loop is now in `internal/orchestrator/run.go` (`Orchestrator.Run`). `cmd/run.go` is reduced to flag parsing + `orchestrator.New(cfg, paths).Run(cmd.Context())`.

### Pre-loop (Orchestrator.Run)

```
CheckDependencies → fatal on missing binary
LoadProjectState + LoadTasks
Detect PAUSED → set resumeFromPause=true (skip EnsureProjectReady)
PrepareForEpicRollover → BootstrapFromTasks
IsEpicAlreadyComplete → return nil if done
EnsureProjectReady (skipped on resume) → fatal on build/test failure
ValidateYAMLStructure + ValidateTaskTypes → fatal on error
EnsureEpicBranch
InitializeTaskPointers
ValidateStateSync (skipped when active task ID is not in tasks.yaml)
SaveProjectState
```

### Main loop

```
for iteration < MaxIterations:
    check ctx.Done() → return ctx.Err() if cancelled

    if resumeFromPause:
        HandleResume → [BuildFailure→return nil | Continue | EpicComplete→HandleEpicComplete→return nil | Retry]
        resumeFromPause = false
        continue

    IncrementAttempts → SaveProjectState (persist before agent)
    WriteActiveTask (injects TestFailureOutput if non-empty)
    backend.Run(ctx, ...) → outputLog file (non-zero exit is non-fatal)
    ParseSessionResult (contract/parse failure → archive + surface explicit error; restore attempt count; no HandleFailure retry path)

    switch outcome:
      SUCCESS      → HandleSuccess(ctx, result, durationSecs)
                     → [BuildFailure→return nil | EpicComplete→HandleEpicComplete→runPostEpicKB→return nil | Continue | Retry]
      FAILURE      → HandleFailure(ctx, durationSecs) → [fatal error→return err | nil→retry]
      BUG          → HandleBug(ctx, durationSecs) → [fatal error→return err | nil→continue]
      EPIC_COMPLETE→ HandleEpicComplete(ctx) → runPostEpicKB → [error→return err | nil→return nil]

max iterations reached → return nil (exit 0)
```

### Exit code policy

| Condition | Exit code |
|-----------|-----------|
| All tasks DONE | 0 |
| Max iterations reached | 0 |
| `HandleEpicComplete` returns nil | 0 |
| `BuildFailure` (project paused) | 0 |
| Context cancelled | non-zero (ctx.Err()) |
| Nested bug detected | 1 |
| Task blocked (max retries) | 1 |
| `HandleEpicComplete` returns error | 1 |
| Fatal state/git error | 1 |

### CLI flags

All flags are applied only when explicitly set via `cmd.Flags().Changed("flag-name")`:

| Flag | Config field |
|------|-------------|
| `--agent` | `AgentCommand` |
| `--build-system` | `BuildSystem` |
| `--max-retries` | `MaxRetries` |
| `--max-iterations` | `MaxIterations` |
| `--kb-enabled` | `KBEnabled` |
| `--agent-heartbeat-seconds` | `AgentHeartbeatSeconds` |

---

## Related Topics

- [internal/orchestrator](orchestrator.md) — BootstrapFromTasks, task pointers, ValidateStateSync, PrepareForEpicRollover
- [internal/agent](agent.md) — ArchiveActiveTask, WriteActiveTask (TestFailureOutput injection)
- [internal/types](types.md) — TaskType, Outcome constants, TaskPointer, ProjectStatus
- [internal/state](state.md) — SaveProjectState, SaveTasks (called by every handler)
- [internal/git](git.md) — RollbackChanges, Commit, ErrNothingToCommit
- [internal/metrics](metrics.md) — RecordTaskMetrics, PrintEpicSummary
- [Run UX + Provider Stall Visibility](../features/run-ux-provider-visibility.md) — provider observability fields consumed by handlers
- [internal/changelog](changelog.md) — UpdateChangelog
- [Go Infrastructure](../infrastructure/go.md) — three failure tiers, exec/atomic conventions
