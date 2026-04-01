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

### Archives And Logs

Doug keeps historical inspection data outside the backlog payload:

- `.doug/logs/sessions/{epic}/` stores archived `ACTIVE_TASK.md` session snapshots
- `.doug/logs/bugs/{epic}/` stores archived bug reports
- `.doug/logs/failures/{epic}/` stores archived failure reports
- `.doug/logs/output/{epic}/` stores raw agent stdout/stderr logs

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

### `doug handoff`

`doug handoff` owns deterministic backlog generation. Its responsibilities are:

- parse `.doug/plan/PLAN.md`
- read the fenced YAML payload from the `## Handoff Data` section of `PLAN.md`
- emit `.doug/plan/epics/<EPIC-ID>/`
- create `metadata.yaml` with status `PLANNED`
- generate `.doug/plan/manifest.yaml` when greenfield scaffold data is present
- refuse in-place overwrite of `ACTIVE` or `COMPLETED` backlog epics

### `doug run <EPIC-ID>`

When invoked with an epic ID, `doug run` owns epic promotion from backlog to runtime. Its responsibilities are:

- verify the target backlog epic exists and is `PLANNED`
- verify the root runtime workspace is not already executing another epic
- copy the backlog epic `PRD.md` and `tasks.yaml` into root `.doug/`
- mark the backlog epic `ACTIVE`
- continue through the existing root-level orchestration and rollover/bootstrap model

Epic promotion is a controlled checkout into the existing runtime path, not a parallel execution system.

### Runtime Completion Handler

The runtime terminal completion path owns the `ACTIVE -> COMPLETED` transition. Its responsibilities are:

- finalize the active runtime epic
- archive the executed runtime session history and related logs
- mark the backlog epic `COMPLETED`
- preserve the original handed-off payload files without rewriting them in place

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
