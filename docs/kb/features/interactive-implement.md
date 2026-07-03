---
title: Interactive Implement MCP Surface
updated: 2026-07-03
category: Features
tags: [implement, interactive, headless, mcp, lifecycle, locking]
related_articles:
  - docs/kb/features/execution-model.md
  - docs/kb/features/planning-lifecycle.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/orchestrator.md
---

# Interactive Implement MCP Surface

## Overview

Doug supports two Implement entry points with one lifecycle authority model:

- **Headless Implement**: `doug run` writes `.doug/ACTIVE_TASK.md`, launches a fresh Pi RPC one-shot worker for the current task, parses `## Agent Result`, verifies success, commits/archives, and advances lifecycle state.
- **Interactive Implement**: an operator who is already inside a long-lived interactive agent session runs Doug's local stdio MCP server and uses Doug-owned MCP tools to claim and report work. Doug still writes the canonical brief and owns lifecycle mutation.

Use **headless** and **interactive** for this distinction. Do not revive older PUSH/PULL terminology in code, docs, operator copy, or task briefs.

The interactive surface is MCP-first. Doug exposes lifecycle controls through `doug mcp`; it does not expose a parallel CLI command set for claiming or completing individual interactive tasks.

## MCP Tools

`doug mcp` serves a local stdio MCP endpoint with these tools:

| Tool | Mutates lifecycle? | Purpose |
|------|--------------------|---------|
| `get_status` | No | Reports current epic, lifecycle phase, active assignment, brief path, attempt count, blocked/completed state, and allowed next actions. |
| `get_next_task` | Yes | Claims the next Doug-authored assignment, writes/returns `.doug/ACTIVE_TASK.md`, persists the attempt count, and returns dispatcher/worker context-hygiene guidance. |
| `report_task_complete` | Yes | Parses the `## Agent Result` block and runs the same verified success path used by headless handlers before state advances. |
| `report_task_blocked` | Yes | Records a `FAILURE` outcome through Doug-owned lifecycle failure/blockage handling. |

Interactive completion is still result-block based. The worker edits `.doug/ACTIVE_TASK.md` and reports `SUCCESS`, `FAILURE`, or `EPIC_COMPLETE` as the task brief allows; the MCP tool only asks Doug to read and process that result. The runtime-internal `BUG` outcome is not an interactive MCP completion surface.

When user backlog tasks are drained, `get_next_task` can assign Doug-owned post-runtime lifecycle work, such as post-epic review or post-epic KB/changelog synthesis, through the same `.doug/ACTIVE_TASK.md` and result-block contract.

## Lifecycle Authority Boundary

`.doug/project-state.yaml` and `.doug/tasks.yaml` are Doug-owned lifecycle state files. They are **not** an external write API.

Operators and agents must not manually edit those files to claim a task, increment attempts, mark a task `DONE`/`BLOCKED`, advance task pointers, stamp `completed_at`, or finalize an epic. Those transitions are coupled invariants and must flow through Doug-owned code: headless `doug run`, mutating MCP tools, or internal handlers/lifecycle helpers.

Manual authoring of root `.doug/PRD.md` and initial task content remains supported before runtime, but once Doug is driving Implement, lifecycle changes belong to Doug. If state looks wrong, use Doug commands/tools or stop and repair through an explicit maintenance task rather than ad hoc YAML edits.

## Locking

Headless `doug run` and mutating MCP tools share the advisory `.doug/run.lock` lock. A second mutating driver fails fast while the lock is held, so two clients do not claim or advance lifecycle state concurrently.

`get_status` is intentionally read-only and lock-free. It must not claim work, write `.doug/ACTIVE_TASK.md`, or mutate project state.

## Dispatcher/Worker Context Hygiene

Interactive Implement is designed for deterministic context boundaries:

1. Keep the MCP-connected session as a **thin dispatcher**. It should inspect status, claim assignments, and report results; it should not accumulate private implementation context across many tasks.
2. Hand each claimed `.doug/ACTIVE_TASK.md` brief to a **fresh worker context per task**. The worker reads the task brief, relevant repo docs/code, and writes the task result block.
3. After the worker fills `## Agent Result`, the dispatcher calls `report_task_complete` or `report_task_blocked` so Doug verifies and advances state.
4. Start a **fresh dispatcher per epic**. Do not carry an old dispatcher's private conversation into a new epic as hidden project memory.

Learning and cross-task context must flow through Doug-owned artifacts: committed code, docs, `docs/kb/`, `CHANGELOG.md`, `.doug/ACTIVE_TASK.md`, `.doug/intake/**`, and `.doug/logs/epics/**`. Do not rely on a single long-lived private agent conversation as the source of truth for later tasks.

## Related Topics

- [Interaction Model And Pi Policy Ownership](execution-model.md)
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md)
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md)
- [internal/handlers](../packages/handlers.md)
- [internal/orchestrator](../packages/orchestrator.md)
