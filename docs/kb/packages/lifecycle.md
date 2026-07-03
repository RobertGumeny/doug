---
title: internal/lifecycle — Shared Lifecycle Core
updated: 2026-07-03
category: Packages
tags: [lifecycle, interactive, mcp, state, tasks, finalization]
related_articles:
  - docs/kb/features/interactive-implement.md
  - docs/kb/features/planning-lifecycle.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/mcp.md
  - docs/kb/packages/state.md
  - docs/kb/packages/types.md
---

# internal/lifecycle — Shared Lifecycle Core

## Overview

`internal/lifecycle` centralizes Doug-owned task lifecycle transitions for interactive Implement and shared handler reuse. It is an internal package, not a public write API. Callers use it to discover lifecycle status, claim assignments, apply verified completion, record failure/blockage, and finalize epics while preserving coupled `project-state.yaml` and `tasks.yaml` invariants.

The package is the core behind the MCP Implement surface. Headless `doug run` still owns verification, commits, rollback, metrics, and post-epic phases through `internal/handlers` and `internal/orchestrator`.

## Status Discovery And Claiming

```go
func DiscoverStatus(opts Options) (Status, error)
func ClaimNext(opts Options) (ClaimResult, error)
```

`DiscoverStatus` is read-only. It loads Doug state/tasks and reports one of:

| StatusKind | Meaning |
|------------|---------|
| `NO_ACTIVE_TASK` | No live assignment is currently claimed and user tasks remain. |
| `ACTIVE_TASK` | `project-state.yaml` has an active task and `.doug/ACTIVE_TASK.md` exists. |
| `COMPLETE` | All user-defined tasks are `DONE`. |

`ClaimNext` is mutating and intended for interactive assignment. It:

1. returns an already-active result without advancing if status is `ACTIVE_TASK`
2. chooses the first `TODO` backlog task when no active assignment exists
3. writes `active_task` and `next_task` in `project-state.yaml`
4. increments and persists the attempt count
5. writes the canonical `.doug/ACTIVE_TASK.md` via `agent.WriteActiveTask`

Claiming deliberately does **not** write `IN_PROGRESS` into `tasks.yaml`; that preserves the existing headless runtime semantics where task completion, not claim, changes backlog status.

## Verified Completion

```go
func ApplyVerifiedCompletion(projectState *types.ProjectState, tasks *types.Tasks, taskID string, markTaskDone bool, now time.Time) (CompletionResult, error)
func CompleteVerifiedTask(opts Options, taskID string) (CompletionResult, error)
```

Completion helpers are only for after Doug has independently verified a successful result. They must not be used to bypass build/test/lint verification.

`ApplyVerifiedCompletion` mutates in-memory state/tasks. When `markTaskDone` is true, it marks the backlog task `DONE`; when all user tasks are complete, it stamps `current_epic.completed_at`; otherwise it advances `active_task`/`next_task` together.

`CompleteVerifiedTask` loads state/tasks, applies the verified completion transition, then persists both files.

## Failure And Blockage

```go
func RecordTaskFailure(opts Options, taskID string) (FailureResult, error)
func ApplyFailedTaskBlock(projectState *types.ProjectState, tasks *types.Tasks, blockedTask types.TaskPointer) error
```

`RecordTaskFailure` preserves retry state until max retries is reached. At the retry cap it marks the backlog task `BLOCKED`, leaves that blocked task as `active_task`, clears `next_task`, and persists state/tasks.

`ApplyFailedTaskBlock` is the in-memory transition used by handlers and tests. It must leave the blocked assignment visible for manual review rather than silently advancing.

## Epic Finalization

```go
func ApplyEpicFinalized(projectState *types.ProjectState, now time.Time)
func FinalizeEpic(opts Options) (FinalizationResult, error)
```

Finalization requires all user tasks to be `DONE`. `FinalizeEpic` calls the shared plan finalizer to archive the runtime snapshot and update backlog metadata, then clears runtime task pointers in `project-state.yaml`. `ApplyEpicFinalized` is the in-memory helper that guarantees `completed_at` and clears `active_task`/`next_task` together.

## Invariants

- `.doug/project-state.yaml` and `.doug/tasks.yaml` are Doug-owned lifecycle files, not an external write API.
- Interactive callers must claim/complete/block through Doug-owned tools rather than editing YAML directly.
- Claiming writes `.doug/ACTIVE_TASK.md` and increments attempts, but it does not mark a task `IN_PROGRESS`.
- Completion helpers assume verification already happened; verification remains outside this package.
- Terminal completion and epic finalization are separate: task completion may stamp `completed_at`, while finalization archives runtime state and clears pointers.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [Planning And Execution Lifecycle Contract](../features/planning-lifecycle.md)
- [internal/mcp](mcp.md)
- [internal/handlers](handlers.md)
- [internal/state](state.md)
- [internal/types](types.md)
