---
title: internal/mcp — Interactive Implement Tool Handlers
updated: 2026-07-05
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
| `diagnose_lifecycle` | `DiagnoseLifecycle` | No | Calls `lifecycle.DiagnoseLifecycle` and returns status plus diagnostic findings for drift/manual-review cases. |
| `get_next_task` | `GetNextTask` | Yes | Acquires `.doug/run.lock`, calls `lifecycle.ClaimNext`, writes/returns the canonical `.doug/ACTIVE_TASK.md`, and includes dispatcher/worker context guidance. |
| `reconcile_lifecycle` | `ReconcileLifecycle` | Yes, only with `mode: "repair"` | Acquires `.doug/run.lock`, applies only supported Doug-owned repairs, and reports every changed file and lifecycle field; unsupported or ambiguous drift returns manual review without changing lifecycle files. |
| `report_task_complete` | `ReportTaskComplete` | Yes | Acquires `.doug/run.lock`, parses `ACTIVE_TASK.md`, accepts only `SUCCESS` or `EPIC_COMPLETE`, builds a `LoopContext`, delegates to verified success handling, and preserves the returned `success_result_kind`. |
| `report_task_blocked` | `ReportTaskBlocked` | Yes | Acquires `.doug/run.lock`, parses `ACTIVE_TASK.md`, requires `FAILURE`, and records retry/blockage through `lifecycle.RecordTaskFailure`. |

`get_status` is intentionally lock-free and read-only. All mutating tool methods use `internal/runlock` so they cannot race with `doug run` or another mutating MCP request.

## Response Shapes

`tools/list` metadata is generated from `ToolDefinitions()`, which keeps tool names, descriptions, and JSON object input schemas colocated with the handler package. No-argument tools expose an object schema with no properties; `reconcile_lifecycle` requires `mode: "repair"`; report tools accept optional `task_id`.

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

`allowed_next_actions` stays backward-compatible as an array of strings rather than a structured object. Each entry uses the same action-token grammar: snake_case action names, with optional parenthesized key/value guidance for required arguments (for example `reconcile_lifecycle(mode=repair)`).

`DiagnosticsResponse` embeds status plus `findings[]` entries with `code`, `severity`, `message`, optional `path`, and `requires_manual_review`. `ReconcileResponse` embeds diagnostics plus `repaired`, `manual_review`, `changed_files`, `changed_fields`, and `message`.

`NextTaskResponse` embeds status plus the active brief text, `assignment_brief_path`, dispatcher/worker guidance, and `already_active`/`claimed` flags. The brief is deliberately bounded assignment material plus context pointers (for example `.doug/PRD.md`, `docs/kb/README.md`, and the Build System section); it must not inline PRD/KB/changelog payloads. `ReportResponse` embeds status plus the reported outcome, the verified `success_result_kind` when completing a task, a human-readable message, and terminal guidance to stop or renew context before requesting more work. `success_result_kind` distinguishes ordinary advancement (`continue`) from terminal epic completion (`epic_complete`) without overwriting the agent-reported outcome. Dispatchers should treat `success_result_kind: "epic_complete"` as the interactive terminal completion signal, even when the worker's result outcome was ordinary `SUCCESS` on the final backlog task.

## Post-Epic Lifecycle Work

When backlog user tasks are drained, `get_next_task` can assign Doug-owned lifecycle work through the same active-task/result-block contract:

- `POST_EPIC_REVIEW` when advisory review is enabled
- `POST_EPIC_KB` when KB/changelog synthesis is enabled and review is disabled

The generated brief remains a Doug-owned `.doug/ACTIVE_TASK.md` assignment and includes context-hygiene guidance. Completed interactive tasks clear the live brief before the next claim; `get_next_task` then writes a fresh assignment rather than returning a stale completed brief. When verified completion returns `EpicComplete`, `report_task_complete` invokes the shared epic finalization handler so runtime snapshots are archived and active pointers are cleared before the response is returned. The response carries `success_result_kind: "epic_complete"`; callers must use that field, not only the parsed `outcome`, to decide whether runtime finalization has completed.

## Dispatcher/Worker Hygiene

`get_next_task` returns guidance instructing the MCP-connected session to act as a thin dispatcher: claim work, hand the canonical brief to a fresh worker context, then report the worker's completed result through Doug. Operators should start a fresh dispatcher per epic and preserve learning through Doug-owned artifacts, not hidden private conversation state.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [internal/lifecycle](lifecycle.md)
- [internal/runlock](runlock.md)
- [internal/handlers](handlers.md)
- [internal/agent](agent.md)
