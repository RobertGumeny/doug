---
title: Planning And Execution Lifecycle Contract
updated: 2026-04-01
category: Features
tags: [planning, handoff, lifecycle, epics, backlog, run, archives]
related_articles:
  - docs/kb/features/scaffold.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/types.md
---

# Planning And Execution Lifecycle Contract

## Overview

Doug uses two distinct workspaces under `.doug/`:

- root `.doug/` is the single active runtime workspace
- `.doug/plan/` stores planning artifacts and deterministic backlog packages

Planning is an optional path into the existing runtime model. Users may still manage root `.doug/PRD.md` and root `.doug/tasks.yaml` manually and run doug without using backlog planning at all.

## Ownership Model

### Root `.doug/`

Root `.doug/` is reserved for the currently active execution context only. It owns:

- root runtime inputs such as `.doug/PRD.md` and `.doug/tasks.yaml`
- root runtime state such as `.doug/project-state.yaml`
- the active agent briefing files such as `.doug/ACTIVE_TASK.md`, `.doug/ACTIVE_BUG.md`, and `.doug/ACTIVE_FAILURE.md`
- runtime logs under `.doug/logs/`

Only one epic may be active in the root `.doug/` runtime workspace at a time.

### `.doug/plan/`

`.doug/plan/` is the planning and backlog namespace. It owns:

- `.doug/plan/PLAN.md` as the editable planning document produced by `doug plan`
- `.doug/plan/manifest.yaml` when greenfield planning needs scaffold input
- `.doug/plan/epics/` as the backlog package root

Files under `.doug/plan/` are not the live runtime working copy. They are planning inputs or deterministic handoff outputs.

### `.doug/plan/epics/<EPIC-ID>/`

Each backlog epic directory is a durable package created by handoff. It contains:

- `PRD.md`
- `tasks.yaml`
- `metadata.yaml`

`PRD.md` and `tasks.yaml` are the handed-off payload. They are not revised in place after the epic has completed. Follow-up work must become a new epic with a new backlog package.

`metadata.yaml` is the lifecycle wrapper around that payload. It tracks whether the package is waiting in backlog, currently promoted into runtime, or retired as completed history.

The metadata file also carries the deterministic provenance and lifecycle timestamps used by the implementation:

- `epic_id`
- `status`
- `created_at`
- `source_plan_path`
- `activated_at` when promotion has occurred
- `completed_at` after terminal completion

### Archives And Logs

Doug keeps historical inspection data outside the backlog payload:

- `.doug/logs/sessions/{epic}/` stores archived `ACTIVE_TASK.md` session snapshots
- `.doug/logs/bugs/{epic}/` stores archived bug reports
- `.doug/logs/failures/{epic}/` stores archived failure reports
- `.doug/logs/output/{epic}/` stores raw agent stdout/stderr logs
- `.doug/logs/archives/{epic}/` stores the final root `.doug/` runtime snapshot (`PRD.md`, `tasks.yaml`, `project-state.yaml`, optional `ACTIVE_TASK.md`, plus `archived_at.txt`)

Completed execution history is archived for inspection, but the backlog payload for a completed epic remains immutable.

## Lifecycle States

Backlog epic metadata supports exactly three lifecycle states:

- `PLANNED`
- `ACTIVE`
- `COMPLETED`

These states apply to backlog epics in `.doug/plan/epics/<EPIC-ID>/metadata.yaml`.

## Allowed State Transitions

| From | To | Owner | Meaning |
|------|----|-------|---------|
| none | `PLANNED` | `doug handoff` | Create a new backlog epic package from `PLAN.md` |
| `PLANNED` | `ACTIVE` | `doug run <EPIC-ID>` promotion flow | Promote one planned epic into root `.doug/` |
| `ACTIVE` | `COMPLETED` | runtime terminal completion handler | Propagate terminal completion back to backlog metadata |

No other transitions are part of the v1 lifecycle contract.

In particular:

- `doug handoff` must not overwrite backlog epics already marked `ACTIVE` or `COMPLETED`
- completed epics are never moved back to `PLANNED` or `ACTIVE`
- follow-up work after completion becomes a new epic instead of revising the completed package in place

## Command Responsibilities

### `doug plan`

`doug plan` owns authoring and iterating on `.doug/plan/PLAN.md`. Its responsibilities are:

- create `.doug/plan/PLAN.md` when it is missing
- launch the configured provider with the `plan` skill
- keep `PLAN.md` as the single primary planning artifact
- keep planning free-form while targeting the deterministic handoff contract

`doug plan` does not activate runtime work by itself, and it does not own deterministic derivative artifacts such as backlog epic packages or `.doug/plan/manifest.yaml`.

For greenfield work, `doug plan` is also where scaffold intent is described first. The scaffold manifest is still a derivative output generated later by `doug handoff`, rather than a second hand-maintained primary planning file.

### `doug handoff`

`doug handoff` owns deterministic backlog generation. Its responsibilities are:

- parse `.doug/plan/PLAN.md`
- read the fenced YAML payload from the `## Handoff Data` section of `PLAN.md`
- emit `.doug/plan/epics/<EPIC-ID>/`
- create `metadata.yaml` with status `PLANNED`
- generate `.doug/plan/manifest.yaml` when greenfield scaffold data is present
- preserve parser-safe quoting when rendering `tasks.yaml`
- refuse in-place overwrite of `ACTIVE` or `COMPLETED` backlog epics

The backlog package written by handoff is the durable planning output:

- `PRD.md` captures the epic-level product brief
- `tasks.yaml` captures the runtime-compatible task list
- `metadata.yaml` captures lifecycle state such as `PLANNED`, `ACTIVE`, or `COMPLETED`

The `tasks.yaml` renderer must quote `description` and `acceptance_criteria` string values. This is parser-sensitive and especially important for deterministic handoff output, where the generated file must continue to round-trip cleanly through the existing loader.

### `doug run <EPIC-ID>`

When invoked with an epic ID, `doug run` owns epic promotion from backlog to runtime. Its responsibilities are:

- verify the target backlog epic exists and is `PLANNED`
- verify the root runtime workspace is not already executing another epic
- copy the backlog epic `PRD.md` and `tasks.yaml` into root `.doug/`
- mark the backlog epic `ACTIVE`
- continue through the existing root-level orchestration and rollover/bootstrap model

Epic promotion is a controlled checkout into the existing runtime path, not a parallel execution system.

After promotion, root `.doug/project-state.yaml`, root `.doug/tasks.yaml`, and the active briefing/log files are the authoritative execution state. The backlog package remains the original handed-off artifact rather than becoming a mutable working copy.

### Runtime Completion Handler

The runtime terminal completion path owns the `ACTIVE -> COMPLETED` transition. Its responsibilities are:

- finalize the active runtime epic
- archive the executed root `.doug/` runtime snapshot into `.doug/logs/archives/{epic}/`
- archive the executed runtime session history and related logs
- mark the backlog epic `COMPLETED`
- preserve the original handed-off payload files without rewriting them in place

Completed work is retired history. If later follow-up is required, that work becomes a new epic with a new backlog package instead of reopening or editing the completed payload in place.

If the completed epic did not originate from backlog planning, the runtime snapshot is still archived, but no backlog metadata update is attempted because no `.doug/plan/epics/<EPIC-ID>/metadata.yaml` exists for that runtime-only path.

## Runtime Authority Boundary

Lifecycle authority changes by phase:

- before promotion, backlog `metadata.yaml` is authoritative for epic lifecycle state
- during execution, root `.doug/project-state.yaml` and root `.doug/tasks.yaml` are authoritative for runtime progress
- on terminal completion, runtime propagates the final lifecycle result back into backlog metadata

This keeps backlog planning state and active runtime state separate while still allowing deterministic promotion between them.

## Manual Root-Level Path Remains Supported

The planning lifecycle is additive, not mandatory.

Users may continue to:

- edit root `.doug/PRD.md` directly
- edit root `.doug/tasks.yaml` directly
- run doug against the root runtime workspace without creating backlog epics

That manual path remains a supported runtime contract. Planning simply provides an integrated route that produces the same root-level runtime artifacts.
