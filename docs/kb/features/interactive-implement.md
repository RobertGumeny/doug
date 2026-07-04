---
title: Interactive Implement MCP Surface
updated: 2026-07-04
category: Features
tags: [implement, interactive, headless, mcp, lifecycle, locking, recovery]
related_articles:
  - docs/kb/features/execution-model.md
  - docs/kb/features/planning-lifecycle.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/packages/lifecycle.md
  - docs/kb/packages/mcp.md
  - docs/kb/packages/runlock.md
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
| `diagnose_lifecycle` | No | Reports pointer/status/active-brief drift without mutating state. |
| `reconcile_lifecycle` | Yes, only with `mode: "repair"` | Repairs narrow Doug-owned drift cases, such as stale or missing `.doug/ACTIVE_TASK.md` for the current active pointer and incorrect `next_task`; unsupported or ambiguous drift returns manual review without file changes. |
| `get_next_task` | Yes | Claims the next Doug-authored assignment, writes/returns `.doug/ACTIVE_TASK.md`, persists the attempt count, and returns dispatcher/worker context-hygiene guidance. |
| `report_task_complete` | Yes | Parses the `## Agent Result` block and runs the same verified success path used by headless handlers before state advances. |
| `report_task_blocked` | Yes | Records a `FAILURE` outcome through Doug-owned lifecycle failure/blockage handling. |

Interactive completion is still result-block based. The worker edits `.doug/ACTIVE_TASK.md`; the MCP tool only asks Doug to read and process that result. Normal task completion, including the final user backlog task, may be reported as `SUCCESS`. The terminal interactive completion signal is Doug's `report_task_complete` response field `success_result_kind: "epic_complete"`, not a requirement that the worker wrote `outcome: EPIC_COMPLETE`. Explicit `EPIC_COMPLETE` remains accepted when the task brief allows it. The runtime-internal `BUG` outcome is not an interactive MCP completion surface.

When `success_result_kind` is `epic_complete`, `report_task_complete` has already run shared epic finalization: runtime state is archived, active pointers are cleared, and the next `get_next_task` call can assign Doug-owned post-runtime lifecycle work, such as post-epic review or post-epic KB/changelog synthesis, through the same `.doug/ACTIVE_TASK.md` and result-block contract.

## Handshake-Surface Contract

`.doug/ACTIVE_TASK.md` is the only doug<->agent handshake surface for Doug-managed work.

- **Inbound to the agent:** Doug writes a lean assignment brief in `.doug/ACTIVE_TASK.md`. The brief names the task, acceptance criteria, build-system verifier, and pointers to context such as `.doug/PRD.md`, `docs/kb/README.md`, package docs, archives, or attempt logs. It should not inline large PRD/KB/changelog payloads merely to transfer context; fresh workers follow the pointers they need.
- **Outbound from the agent:** workflow outcome flows through exactly one report channel for the current mode. In headless Implement, the agent fills the `## Agent Result` frontmatter in `.doug/ACTIVE_TASK.md` and Doug parses it after the Pi run ends. In interactive Implement, the worker still fills the result block, but the dispatcher asks Doug to process it through `report_task_complete` or `report_task_blocked`.
- **Other agent-written files:** files such as `docs/kb/**`, `CHANGELOG.md`, `.doug/plan/PLAN.md`, post-epic review artifacts, or scaffold outputs are Doug-scaffolded work products that the agent fills in. They are not competing lifecycle outcome channels unless a mode explicitly names them as its report channel.

This contract keeps recovery deterministic: Doug can recreate or validate the live brief from lifecycle state, and operators do not have to infer lifecycle truth from arbitrary work-product edits.

## Recovery After Interrupted Interactive Sessions

After a terminal interruption, failed report attempt, stale brief suspicion, or client restart, reconnect the dispatcher to `doug mcp` and inspect before mutating. A typical recovery flow is:

```text
get_status
  -> Read current_epic, lifecycle_phase, active_assignment, brief_path,
     attempt_count, and allowed_next_actions.

# If status is unclear, a brief is missing/stale, or the client lost its place:
diagnose_lifecycle
  -> Review findings for pointer/status/active-brief drift.
  -> This is read-only and must not claim work or repair files.

# Only when diagnostics identify a supported Doug-owned repair:
reconcile_lifecycle {"mode":"repair"}
  -> Doug rewrites or corrects only narrow, explainable drift cases.
  -> Read changed_files, changed_fields, repaired/manual_review, and message.

get_status
  -> Confirm the repaired state and allowed next action before continuing.
```

If `get_status` reports an active assignment and `.doug/ACTIVE_TASK.md` matches it, hand that existing brief to a fresh worker and continue the task; do not call `get_next_task` just because the old terminal was interrupted. If diagnostics require manual review, stop and make an explicit maintenance/bugfix task rather than guessing.

Direct lifecycle YAML edits are unsupported recovery behavior. Do not repair interruptions by editing `.doug/project-state.yaml`, `.doug/tasks.yaml`, backlog `metadata.yaml`, attempts, statuses, task pointers, or `completed_at` by hand. Those files encode coupled invariants that must be changed by Doug-owned code: headless `doug run`, mutating MCP tools, or internal lifecycle helpers.

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
3. After the worker fills the result block, the dispatcher calls `report_task_complete` or `report_task_blocked` so Doug verifies and advances state. For final-task success, the dispatcher treats `success_result_kind: "epic_complete"` in the report response as the terminal completion signal.
4. Start a **fresh dispatcher per epic**. Do not carry an old dispatcher's private conversation into a new epic as hidden project memory.

Claude Code is the first targeted interactive client for this guidance: run `doug mcp` from the project, keep the Claude session connected to MCP as the dispatcher, and use Claude's normal new-chat/context-renewal workflow for each worker task and at epic boundaries. Doug does not enforce client-side resets; it provides dispatcher prompts, worker-ready brief text, and lifecycle tools so the operator can keep Claude's context clean.

Future interactive clients should adapt to the same boundary rather than changing lifecycle semantics. The client adapter is responsible for connecting to the local stdio MCP server, presenting tool responses, starting/renewing worker contexts, and passing the canonical `.doug/ACTIVE_TASK.md` brief to those workers. Doug remains responsible for claims, diagnostics, repair, verification, and state mutation.

Learning and cross-task context must flow through Doug-owned artifacts: committed code, docs, `docs/kb/`, `CHANGELOG.md`, `.doug/ACTIVE_TASK.md`, `.doug/intake/**`, and `.doug/logs/epics/**`. Do not rely on a single long-lived private agent conversation as the source of truth for later tasks.

## Related Topics

- [Interaction Model And Pi Policy Ownership](execution-model.md)
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md)
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md)
- [internal/lifecycle](../packages/lifecycle.md)
- [internal/mcp](../packages/mcp.md)
- [internal/runlock](../packages/runlock.md)
- [internal/handlers](../packages/handlers.md)
- [internal/orchestrator](../packages/orchestrator.md)
