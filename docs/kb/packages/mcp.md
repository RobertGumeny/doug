---
title: internal/mcp — Interactive Implement Tool Handlers
updated: 2026-07-03
category: Packages
tags: [mcp, interactive, implement, lifecycle, locking]
related_articles:
  - docs/kb/features/interactive-implement.md
  - docs/kb/packages/lifecycle.md
  - docs/kb/packages/runlock.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
---

# internal/mcp — Interactive Implement Tool Handlers

## Overview

`internal/mcp` owns Doug's testable interactive Implement tool semantics. The package backs the local stdio MCP server exposed by `doug mcp`, while `cmd/mcp.go` stays thin JSON-RPC/framing glue.

The package routes interactive work through Doug-owned lifecycle control points instead of letting an MCP-connected agent edit `.doug/project-state.yaml` or `.doug/tasks.yaml` directly.

## ToolHandler

```go
type ToolHandler struct {
    ProjectRoot   string
    Config        *config.OrchestratorConfig
    Logger        log.Logger
    BuildSystem   build.BuildSystem
    HandleSuccess func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error)
}
```

`ToolHandler` is the seam used by tests and the command layer. It resolves default paths/config/build system when fields are omitted. `HandleSuccess` is injectable for tests; production defaults to `handlers.HandleSuccess` so interactive completion uses the same verified success path as headless runtime.

## Tools

| Tool | Method | Mutates? | Behavior |
|------|--------|----------|----------|
| `get_status` | `GetStatus` | No | Calls `lifecycle.DiscoverStatus` and returns current epic, lifecycle phase, assignment pointers, brief path, attempt count, blocked/completed state, and allowed next actions. |
| `get_next_task` | `GetNextTask` | Yes | Acquires `.doug/run.lock`, calls `lifecycle.ClaimNext`, writes/returns the canonical `.doug/ACTIVE_TASK.md`, and includes dispatcher/worker context guidance. |
| `report_task_complete` | `ReportTaskComplete` | Yes | Acquires `.doug/run.lock`, parses `ACTIVE_TASK.md`, accepts only `SUCCESS` or `EPIC_COMPLETE`, builds a `LoopContext`, and delegates to verified success handling. |
| `report_task_blocked` | `ReportTaskBlocked` | Yes | Acquires `.doug/run.lock`, parses `ACTIVE_TASK.md`, requires `FAILURE`, and records retry/blockage through `lifecycle.RecordTaskFailure`. |

`get_status` is intentionally lock-free and read-only. All mutating tool methods use `internal/runlock` so they cannot race with `doug run` or another mutating MCP request.

## Response Shapes

`StatusResponse` includes:

- `current_epic`
- `lifecycle_phase`
- optional `active_assignment`
- optional `next_assignment`
- `brief_path`
- `attempt_count`
- `blocked`
- `completed`
- `allowed_next_actions`

`NextTaskResponse` embeds status plus the active brief text, dispatcher/worker guidance, and `already_active`/`claimed` flags. `ReportResponse` embeds status plus the reported outcome and a human-readable message.

## Post-Epic Lifecycle Work

When backlog user tasks are drained, `get_next_task` can assign Doug-owned lifecycle work through the same active-task/result-block contract:

- `POST_EPIC_REVIEW` when advisory review is enabled
- `POST_EPIC_KB` when KB/changelog synthesis is enabled and review is disabled or already out of scope for the claim path

The generated brief remains a Doug-owned `.doug/ACTIVE_TASK.md` assignment and includes context-hygiene guidance.

## Dispatcher/Worker Hygiene

`get_next_task` returns guidance instructing the MCP-connected session to act as a thin dispatcher: claim work, hand the canonical brief to a fresh worker context, then report the worker's completed result through Doug. Operators should start a fresh dispatcher per epic and preserve learning through Doug-owned artifacts, not hidden private conversation state.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [internal/lifecycle](lifecycle.md)
- [internal/runlock](runlock.md)
- [internal/handlers](handlers.md)
- [internal/agent](agent.md)
