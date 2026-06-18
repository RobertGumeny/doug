---
title: Planning And Execution Lifecycle Contract
updated: 2026-05-20
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

This coexistence is intentional but transitional in operator experience: the repository still supports both the newer planning/backlog workspace and the older direct root-runtime editing path. Docs should describe `.doug/plan/` as the structured planning path without implying that manual root-runtime authoring has been removed.

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
- `.doug/plan/history/` as the deterministic archive of handed-off planning workbooks
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
- `.doug/logs/bugs/{epic}/` stores the canonical durable archive for all bug reports, whether blocking or non-blocking
- `.doug/logs/failures/{epic}/` stores archived failure reports
- `.doug/logs/output/{epic}/` stores raw agent stdout/stderr logs
- `.doug/logs/archives/{epic}/` stores the final root `.doug/` runtime snapshot (`PRD.md`, `tasks.yaml`, `project-state.yaml`, optional `ACTIVE_TASK.md`, plus `archived_at.txt`)

`ACTIVE_TASK.md` in root `.doug/` is the canonical Doug-managed brief for agent runs. In runtime execution it is also ephemeral live state, not durable history. Handlers archive it to `.doug/logs/sessions/{epic}/` before any state-changing work, then remove the live root file after outcome handling is complete. On epic completion, runtime snapshot archival runs before that cleanup, so the final archive may still include `ACTIVE_TASK.md` when it existed at finalization time.

`ACTIVE_BUG.md` is also live runtime state, but only for blocking interruptions. It is the transient handoff file that gives a scheduled bugfix task guaranteed context. It is not the durable bug archive; all bug reports belong under `.doug/logs/bugs/{epic}/`.

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

## PLAN.md Handoff Data Structure

### Handoff Readiness Is a Confirmed State

A `## Handoff Data` section that contains parseable YAML is not automatically handoff-ready. The plan advances from draft to handoff-ready only when the user has explicitly confirmed the alignment summary produced by the planning agent. Parseable YAML is a necessary condition; explicit user confirmation is the sufficient one.

This two-stage contract is encoded in the plan skill (`## Planning Stages`), the Doug-owned planning brief (`planBriefBlock`), and the built-in `PlanPrompt`. All three surfaces use consistent phrasing so the rule holds in Doug-managed planning runs.

The `## Handoff Data` section of `PLAN.md` must contain a fenced YAML block that `doug handoff` can parse without guesswork. All fields below are required unless noted.

Unknown fields are rejected. Treat the documented YAML below as the exact supported contract for `doug handoff`; do not add extra keys under `project`, `manifest`, `epics`, or `tasks`.

```yaml
schema_version: 1
project:
  name: "My Actual Project Name"   # required; human-readable project name
  mode: "brownfield"               # required; "brownfield" or "greenfield"
manifest:                          # optional; include for greenfield scaffold output only
  schema_version: 1
  project:
    name: "My Actual Project Name"
    mode: "greenfield"
  scaffold:
    language: "typescript"
    runtime: "node"
    framework: "nextjs"
    package_manager: "pnpm"
    build_system: "npm-scripts"
  dependencies:
    runtime:
      - "next@current-stable-version"
    development:
      - "typescript@current-stable-version"
  constraints:
    - "Deploy on Vercel"
epics:
  - id: "EPIC-1"                   # required; unique identifier used for backlog directory names
    name: "My Epic Name"           # required; human-readable epic title
    prd: |                         # required; agent-authored product requirements for this epic
      # PRD

      Describe the epic scope, motivation, and constraints here. This content
      becomes the PRD.md file in the epic's backlog package and is the primary
      product brief available to the runtime agent during execution.
    tasks:
      - id: "EPIC-1-001"           # required; unique task identifier within the epic
        type: "feature"            # optional; defaults to "feature"; valid: feature, bugfix, documentation
        status: "TODO"             # optional; defaults to "TODO"
        description: "..."         # required; one-sentence task description
        acceptance_criteria:
          - "..."                  # required; at least one non-empty binary criterion
```

### Where `prd` Content Comes From

The `prd` field is agent-authored during the `doug plan` session. The planning agent writes product requirements directly into the `## Handoff Data` YAML under each epic's `prd` key. The value becomes `PRD.md` verbatim inside the generated backlog package. It should describe the epic's scope, motivation, and any constraints the runtime agent needs for execution — without requiring the agent to look elsewhere for product context.

For greenfield work, scaffold metadata belongs under `manifest`, not under `project`. The `project` object only supports `name` and `mode`.

### Placeholder-Safety Validation

`doug handoff` rejects PLAN.md documents that still contain seed-template placeholder values. The following exact values are recognized as placeholders and will cause handoff to fail with an actionable error:

| Field | Rejected placeholder value |
|-------|---------------------------|
| `project.name` | `"My Project"` |
| `epic.name` | `"Example Epic"` |
| `epic.prd` | any value containing `"Describe the epic's product requirements here."` |
| `task.description` | `"Describe the task here."` |
| `task.acceptance_criteria` item | `"First acceptance criterion."` or `"Second acceptance criterion."` |

Validation is limited to these exact known seed strings. Ordinary user-authored prose that resembles but does not exactly match a placeholder is accepted.

## Command Responsibilities

### `doug plan`

`doug plan` owns authoring and iterating on `.doug/plan/PLAN.md`. Its responsibilities are:

- create `.doug/plan/PLAN.md` when it is missing
- create or refresh root `.doug/ACTIVE_TASK.md` as the canonical brief for the planning run
- rewrite the Doug-owned planning brief in `.doug/ACTIVE_TASK.md` on each planning run so current CLI intent and unresolved bug context are authoritative
- accept explicit planning context from the CLI via positional intent text plus optional `--intent`, `--mode`, and `--epic` hints; accepted `--mode` values are `discovery`, `roadmapping`, `definition`, `feature`, `refactor`, `bugfix`, and `greenfield`
- when positional text and `--intent` are both absent, capture planning intent in a single-line interactive prompt that submits with Enter when the session is interactive; otherwise fail fast instead of silently reusing stale workbook prose
- persist the resolved planning run context into the Doug-owned brief before launching the planning agent
- surface unresolved archived bug reports from `.doug/logs/bugs/{epic}/` in the Doug-owned brief so deferred bugs re-enter planning without a second manual intake artifact
- emit the Doug planning prompt through Pi with the `plan` skill
- launch true interactive Pi for the planning conversation rather than using the RPC one-shot runtime path
- keep `PLAN.md` as the editable planning workbook described by `ACTIVE_TASK.md`
- enforce the two-stage planning model: **Draft** (workbook refinement) and **Handoff-Ready** (alignment confirmed); the planning agent must not write final machine-consumable handoff YAML before the user explicitly confirms the alignment summary
- keep planning free-form while targeting the deterministic handoff contract
- suppress heartbeat logging for planning sessions: no heartbeat interval or callback is passed to the agent, so liveness logs do not appear during `doug plan` (heartbeat remains active for `doug run` and other non-interactive paths)

`doug plan` does not activate runtime work by itself, and it does not own deterministic derivative artifacts such as backlog epic packages or `.doug/plan/manifest.yaml`.

`.doug/ACTIVE_TASK.md` remains the canonical run brief for Doug-managed planning runs. The planning intent itself is PLAN-owned run context: Doug resolves it from positional text, `--intent`, or interactive capture, then writes that resolved intent into the Doug-owned planning context in `PLAN.md` before agent launch. If older workbook prose disagrees with the current resolved intent, the planning session must reconcile the workbook to the run context instead of silently following stale content.

For greenfield work, `doug plan` is also where scaffold intent is described first. The scaffold manifest is still a derivative output generated later by `doug handoff`, rather than a second hand-maintained primary planning file.

When archived bug reports re-enter planning:

- bugs from `PLANNED` epics may update the existing planned package when the scope still matches, or become a new `PLANNED` follow-up when it does not
- bugs from `ACTIVE` epics must be planned as new follow-up work instead of reopening or mutating the active backlog package
- bugs from `COMPLETED` epics must always become new planning work; completed backlog packages remain historical and immutable

### `doug handoff`

`doug handoff` owns deterministic backlog generation. Its responsibilities are:

- parse `.doug/plan/PLAN.md`
- read the fenced YAML payload from the `## Handoff Data` section of `PLAN.md`
- emit `.doug/plan/epics/<EPIC-ID>/`
- create `metadata.yaml` with status `PLANNED`
- generate `.doug/plan/manifest.yaml` when greenfield scaffold data is present
- archive the exact pre-handoff workbook under `.doug/plan/history/`
- reseed `.doug/plan/PLAN.md` for the next planning cycle with Doug-owned post-handoff context instead of leaving handed-off epic content in place
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

### Bug Reporting During Runtime

Doug separates live interruption state from durable bug history:

- blocking bugs create or refresh `.doug/ACTIVE_BUG.md` so the follow-up bugfix task has live context
- every bug report, including blocking reports, is durably archived under `.doug/logs/bugs/{epic}/`
- non-blocking or deferred bugs skip `ACTIVE_BUG.md` and still go straight to the durable archive

This keeps the runtime handoff contract narrow while making later planning and inspection depend on the archived bug files instead of the transient live briefing.

`doug plan` is the rediscovery path for deferred bugs. It reads unresolved bug reports from the canonical archive and places them into the Doug-owned planning brief so the next planning cycle can turn them into new or updated `PLANNED` work intentionally.

### Runtime Completion Handler

The runtime terminal completion path owns the `ACTIVE -> COMPLETED` transition. Its responsibilities are:

- finalize the active runtime epic
- archive the executed root `.doug/` runtime snapshot into `.doug/logs/archives/{epic}/`
- archive the executed runtime session history and related logs
- remove the live root `.doug/ACTIVE_TASK.md` once archival/finalization is complete
- mark the backlog epic `COMPLETED`
- preserve the original handed-off payload files without rewriting them in place

Completed work is retired history. If later follow-up is required, that work becomes a new epic with a new backlog package instead of reopening or editing the completed payload in place.

When KB synthesis is enabled, Doug may then run a separate best-effort post-epic documentation pass. That pass reads the archived runtime snapshot and session logs, writes its own `POST_EPIC_KB` session artifact, and may commit KB updates, but it does not reopen runtime task state or alter the completed backlog lifecycle.

If the completed epic did not originate from backlog planning, the runtime snapshot is still archived, but no backlog metadata update is attempted because no `.doug/plan/epics/<EPIC-ID>/metadata.yaml` exists for that runtime-only path.

## Runtime Authority Boundary

Lifecycle authority changes by phase:

- before promotion, backlog `metadata.yaml` is authoritative for epic lifecycle state
- during execution, root `.doug/project-state.yaml` and root `.doug/tasks.yaml` are authoritative for runtime progress
- on terminal completion, runtime propagates the final lifecycle result back into backlog metadata

This keeps backlog planning state and active runtime state separate while still allowing deterministic promotion between them.

For backend preparation, Doug also distinguishes between agent-facing surfaces and non-agent-facing control artifacts:

- Doug-owned control and lifecycle files such as root `.doug/tasks.yaml`, `.doug/project-state.yaml`, backlog metadata, and archive directories are non-agent-facing by default
- Doug-owned agent-facing files are exposed only when the run contract names them explicitly, such as `.doug/ACTIVE_TASK.md`, root `.doug/PRD.md`, `.doug/plan/PLAN.md`, or a blocking `.doug/ACTIVE_BUG.md` handoff
- repository-owned files remain project authority rather than Doug authority, even when they are loaded into the run context

Default writable surfaces are workflow-specific:

- runtime and scaffold runs expose the project workspace plus live Doug handoff files (`ACTIVE_TASK.md`, `ACTIVE_BUG.md`, `ACTIVE_FAILURE.md`)
- planning runs expose only `.doug/ACTIVE_TASK.md` and `.doug/plan/PLAN.md`
- post-epic KB runs expose only `docs/kb/` and `.doug/ACTIVE_TASK.md`

The Pi request contract mirrors this split so Doug can pass explicit path authority, context order, and writable-surface intent without inferring behavior from path strings alone.

## Manual Root-Level Path Remains Supported

The planning lifecycle is additive, not mandatory.

Users may continue to:

- edit root `.doug/PRD.md` directly
- edit root `.doug/tasks.yaml` directly
- run doug against the root runtime workspace without creating backlog epics

That manual path remains a supported runtime contract. Planning simply provides an integrated route that produces the same root-level runtime artifacts.
