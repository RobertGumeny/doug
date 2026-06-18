---
title: internal/types — LoopContext & Task Ops
updated: 2026-06-17
category: Packages
tags: [types, loop-context, task-ops, handlers, orchestrator, per-iteration, provider-stall]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/state.md
  - docs/kb/features/run-ux-provider-visibility.md
---

# internal/types — LoopContext & Task Ops

## Overview

This article covers the per-iteration execution layer in `internal/types`: the `LoopContext` struct passed to every handler, and the task operation functions in `task_ops.go`. These are relevant when working on the orchestration loop or outcome handlers. For data model structs and typed constants, see [internal/types](types.md).

---

## LoopContext (loop_context.go)

`LoopContext` carries all per-iteration state for the orchestration main loop. It is constructed once per iteration in `Orchestrator.Run` and passed to every handler.

```go
type LoopContext struct {
    // Per-iteration identity (snapshotted after IncrementAttempts)
    TaskID      string
    TaskType    TaskType
    Attempts    int
    CurrentEpic EpicState

    // Orchestrator configuration + infrastructure
    Config      *config.OrchestratorConfig
    BuildSystem build.BuildSystem
    ProjectRoot string
    TaskStartTime time.Time

    // Mutable shared state — mutated in memory and persisted by handlers
    State *ProjectState
    Tasks *Tasks

    // File system paths used by handlers
    StatePath     string // .doug/project-state.yaml
    TasksPath     string // .doug/tasks.yaml
    DougDir       string // .doug/
    LogsDir       string // .doug/logs/
    ChangelogPath string // CHANGELOG.md

    // Provider observability captured from the backend run.
    ProviderWaitMs   int64
    ProviderFailures []ProviderFailure

    // Logger is the structured output writer for this iteration.
    Logger log.Logger
}
```

`orchestrator.LoopContext` is a type alias for `types.LoopContext`; both names refer to the same type.

`ProviderWaitMs` and `ProviderFailures` are populated by `Orchestrator.Run` after the backend returns and before handler dispatch. Success/failure/bug handlers pass them to `metrics.RecordTaskMetrics` for persistence.

Do not add `SessionResult` or `AgentDurationSeconds` to `LoopContext` — they are passed as explicit parameters to `HandleSuccess`.

---

## task_ops.go — Task Operations

These functions operate on `*ProjectState` and `*Tasks` in memory. Callers persist via `state.SaveProjectState`/`state.SaveTasks`.

```go
func UpdateTaskStatus(tasks *Tasks, id string, status Status) error
func AdvanceToNextTask(state *ProjectState, tasks *Tasks) bool
func AreAllUserTasksComplete(tasks *Tasks) bool
```

The task-op surface is owned directly by `internal/types`; prefer calling `types.*` directly in new code.

See [internal/orchestrator](orchestrator.md) for the full behavioral spec of each function.

---

## Related Topics

- [internal/types](types.md) — data model structs, typed constants, UserDefined/Synthetic distinction
- [internal/orchestrator](orchestrator.md) — full behavioral spec for task ops; call order in Orchestrator.Run
- [internal/handlers](handlers.md) — all handlers receive LoopContext as their first parameter
- [internal/state](state.md) — SaveProjectState, SaveTasks (callers must persist after mutations)
- [Run UX + Provider Stall Visibility](../features/run-ux-provider-visibility.md) — provider observability flow into LoopContext and metrics
