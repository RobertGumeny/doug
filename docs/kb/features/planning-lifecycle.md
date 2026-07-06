---
title: Planning And Execution Lifecycle Contract
updated: 2026-07-06
category: Features
tags: [planning, handoff, lifecycle, epics, backlog, run, archives]
related_articles:
  - docs/kb/features/scaffold.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/types.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/plan.md
  - docs/kb/packages/dougpath.md
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
- active agent briefing state such as `.doug/ACTIVE_TASK.md` and payload fields persisted in `.doug/project-state.yaml`
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

Doug separates planning intake from forensic logs:

- `.doug/intake/bugs/{epic}/` stores reported-bug intake for later planning, including blocking and non-blocking bug reports
- `.doug/intake/research/` stores research reports that should be surfaced as recent-research planning candidates
- `.doug/logs/epics/{epic}/PRD.md`, `tasks.yaml`, and `project-state.yaml` store the finalized root runtime snapshot for the epic
- `.doug/logs/epics/{epic}/epic-review.md` (or a versioned sibling) stores advisory post-epic review output when automatic or explicit review runs
- `.doug/logs/epics/{epic}/{taskID}/attempt-N/` stores attempt-scoped forensics such as archived `ACTIVE_TASK.md` session snapshots (`session.md`), `attempt-start.json`, `stats.json`, retained Pi-native transcripts, and infra-failure records

Pi-backed runtime and finalization paths do not create default raw output mirrors; `.doug/logs/output/` is absent by default and should only exist if an operator enables a future/debug capture path.

`ACTIVE_TASK.md` in root `.doug/` is the canonical Doug-managed brief for agent runs. In runtime execution it is also ephemeral live state, not durable history. Handlers archive it to `.doug/logs/epics/{epic}/{taskID}/attempt-N/session.md` before any state-changing work, then remove the live root file after outcome handling is complete. On epic completion, the runtime snapshot is archived under `.doug/logs/epics/{epic}/` before that cleanup, so the final archive may still include `ACTIVE_TASK.md` when it existed at finalization time.

Blocking bug context is live runtime state carried on the synthetic bugfix task pointer in `.doug/project-state.yaml`. It gives the scheduled bugfix task guaranteed context without a separate active handoff file. It is not the durable bug intake; all bug reports belong under `.doug/intake/bugs/{epic}/`.

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
manifest:                          # required for greenfield scaffold output; omit for brownfield-only plans
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

For greenfield work, scaffold metadata belongs under `manifest`, not under `project`. The `project` object only supports `name` and `mode`. Greenfield handoff-ready output must include the `manifest` block, and dependency entries should be explicit `package@version` values rather than bare package names.

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
- rewrite the Doug-owned planning brief in `.doug/ACTIVE_TASK.md` on each planning run so current CLI intent is authoritative and reported-bug/recent-research intake is visible
- accept explicit planning context from the CLI via positional intent text plus optional `--intent`, `--mode`, and `--epic` hints; accepted `--mode` values are `discovery`, `roadmapping`, `definition`, `feature`, `refactor`, `bugfix`, and `greenfield`
- when `--mode` is omitted, auto-detect `greenfield` mode only for near-empty repositories with no recognized build marker, shallow or absent git history, and at most three non-`.doug`/non-`.git` files; explicit `--mode` always takes precedence
- when positional text and `--intent` are both absent, capture planning intent in the shared wrapped multiline composer when the session is interactive; otherwise fail fast instead of silently reusing stale workbook prose
- persist the resolved planning run context into the Doug-owned brief before launching the planning agent
- surface unresolved reported bugs from `.doug/intake/bugs/{epic}/` in the Doug-owned brief so deferred bugs re-enter planning without a second manual intake artifact
- surface top-level markdown reports from `.doug/intake/research/` as advisory recent-research planning candidates without creating tasks or mutating handoff data
- emit the Doug planning prompt through Pi with the `plan` skill
- launch true interactive Pi for the planning conversation rather than using the RPC one-shot runtime path
- keep `PLAN.md` as the editable planning workbook described by `ACTIVE_TASK.md`
- enforce the two-stage planning model: **Draft** (workbook refinement) and **Handoff-Ready** (alignment confirmed); the planning agent must not write final machine-consumable handoff YAML before the user explicitly confirms the alignment summary
- keep planning free-form while targeting the deterministic handoff contract
- suppress heartbeat logging for planning sessions: no heartbeat interval or callback is passed to the agent, so liveness logs do not appear during `doug plan` (heartbeat remains active for `doug run` and other non-interactive paths)

`doug plan` does not activate runtime work by itself, and it does not own deterministic derivative artifacts such as backlog epic packages or `.doug/plan/manifest.yaml`.

`.doug/ACTIVE_TASK.md` remains the canonical run brief for Doug-managed planning runs. The planning intent itself is PLAN-owned run context: Doug resolves it from positional text, `--intent`, or interactive capture, then writes that resolved intent into the Doug-owned planning context in `PLAN.md` before agent launch. If older workbook prose disagrees with the current resolved intent, the planning session must reconcile the workbook to the run context instead of silently following stale content.

For greenfield work, `doug plan` is also where scaffold intent is described first. Greenfield mode adds a hard brief directive that the `manifest` block is required in `## Handoff Data`; the initial workbook seed also uses `project.mode: "greenfield"` rather than the brownfield default. The scaffold manifest is still a derivative output generated later by `doug handoff`, rather than a second hand-maintained primary planning file.

When reported bugs re-enter planning:

- bugs from `PLANNED` epics may update the existing planned package when the scope still matches, or become a new `PLANNED` follow-up when it does not
- bugs from `ACTIVE` epics must be planned as new follow-up work instead of reopening or mutating the active backlog package
- bugs from `COMPLETED` epics must always become new planning work; completed backlog packages remain historical and immutable

### `doug handoff`

`doug handoff` owns deterministic backlog generation. Its responsibilities are:

- parse `.doug/plan/PLAN.md`
- read the fenced YAML payload from the `## Handoff Data` section of `PLAN.md`
- allocate concrete, gap-free epic identifiers for every submitted epic in document order, then emit `.doug/plan/epics/<EPIC-ID>/`
- create `metadata.yaml` with status `PLANNED`
- generate `.doug/plan/manifest.yaml` when greenfield scaffold data is present
- archive the exact pre-handoff workbook under `.doug/plan/history/`
- reseed `.doug/plan/PLAN.md` for the next planning cycle with Doug-owned post-handoff context instead of leaving handed-off epic content in place
- preserve parser-safe quoting when rendering `tasks.yaml`
- refuse in-place overwrite of `ACTIVE` or `COMPLETED` backlog epics

#### Epic ID Allocation

Before normalization, handoff validates the shape of every submitted epic and task identifier so malformed payloads are rejected before any backlog package is written:

- each epic ID must be a concrete `EPIC-<N>` or a placeholder token such as `EPIC-<X>`
- each task ID must reuse its epic's submitted ID as a prefix followed by a numeric suffix (`<epic-id>-NNN`)
- a task whose prefix does not match its epic's submitted ID is rejected with an error naming the offending task ID

Submitted epic identifiers are normalization inputs, not final IDs. Whether the planning agent wrote placeholder tokens (e.g. `EPIC-<X>`) or concrete numbers (e.g. `EPIC-42`), handoff allocates each submitted epic the next available concrete number in document order:

- the allocation floor is the highest existing numeric `EPIC-N` across `.doug/plan/epics/`, plus one
- numeric gaps are never filled; allocation only ever moves forward from the maximum
- multiple submitted epics receive consecutive numbers following their order in the YAML payload
- task identifiers are rewritten to the allocated epic prefix (`EPIC-<allocated>-NNN`), preserving the submitted numeric suffix when present
- exact references to the submitted epic/task identifiers and placeholder tokens inside `prd`, task `description`, and `acceptance_criteria` are rewritten to the allocated concrete identifiers

Because allocation always lands above every existing numeric epic, the `ACTIVE`/`COMPLETED` overwrite guard is a backstop that does not fire in normal flow; it remains as a safety net against ever clobbering an occupied slot.

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

- blocking bugs persist their payload on the synthetic bugfix task state so the follow-up bugfix task has live context
- every bug report, including blocking reports, is durably archived under `.doug/intake/bugs/{epic}/`
- non-blocking or deferred bugs skip synthetic interruption state and still go straight to the durable archive

This keeps the runtime handoff contract narrow while making later planning and inspection depend on the reported bug files instead of the transient live briefing.

#### Deterministic reported-bug intake

Doug-owned runtime paths write reported bugs through the shared `agent.WriteBugArchive(...)` writer rather than hand-authoring markdown files. The writer stamps the required frontmatter (`bug_id`, `discovered_by_task`, `timestamp`, `severity`, `status`), validates the writer vocabulary (`critical`/`high`/`medium`/`low` severity and `open`/`investigating`/`fixed`/`wont_fix` status), and allocates versioned siblings instead of overwriting existing reports.

Blocking `BUG` outcomes archive exactly one `severity: blocking` result payload, schedule a synthetic `BUG-<taskID>` bugfix task, and carry the live bug context on `project-state.yaml` task-pointer fields. Successful sessions may include `severity: non-blocking` result bugs; `HandleSuccess` archives each one under `.doug/intake/bugs/{epic}/` with `NB-BUG-<taskID>-<n>` IDs before advancing pointers. Archive failures for non-blocking bugs are warnings only: the otherwise successful task remains successful.

`doug plan` is the rediscovery path for deferred bugs. It reads unresolved bug reports from `.doug/intake/bugs/` (plus legacy `.doug/logs/bugs/` compatibility during the transition) and places them into the Doug-owned planning brief so the next planning cycle can turn them into new or updated `PLANNED` work intentionally. Planning intake lowercases and trims status/severity, skips malformed files with visible warnings instead of aborting, and filters terminal statuses `fixed`, `resolved`, `done`, and `closed` so completed reports do not reappear as unresolved work.

### Runtime Completion Handler

The runtime terminal completion path owns the `ACTIVE -> COMPLETED` transition. Its responsibilities are:

- finalize the active runtime epic
- archive the executed root `.doug/` runtime snapshot into `.doug/logs/epics/{epic}/`
- archive the executed runtime session history and related logs
- remove the live root `.doug/ACTIVE_TASK.md` once archival/finalization is complete
- mark the backlog epic `COMPLETED`
- preserve the original handed-off payload files without rewriting them in place

Completed work is retired history. If later follow-up is required, that work becomes a new epic with a new backlog package instead of reopening or editing the completed payload in place.

When review is enabled, Doug then runs a best-effort advisory post-epic review before any KB/changelog synthesis. The review reads finalized archives, archived sessions, recorded commit SHAs, and committed diffs, then writes `.doug/logs/epics/{epic}/epic-review.md` (or a versioned sibling). It is non-gating: warnings or agent failures do not reopen runtime task state or alter the completed backlog lifecycle. Operators can rerun it with `doug review <EPIC-ID>`.

When KB synthesis is enabled, Doug next runs a separate best-effort post-epic documentation and changelog-polish pass. That pass reads the archived runtime snapshot and session logs, writes its own `POST_EPIC_KB` session artifact, may commit KB updates, and may polish only the `[Unreleased]` section of `CHANGELOG.md` while preserving facts. It also does not reopen runtime task state or alter the completed backlog lifecycle.

If the completed epic did not originate from backlog planning, the runtime snapshot is still archived, but no backlog metadata update is attempted because no `.doug/plan/epics/<EPIC-ID>/metadata.yaml` exists for that runtime-only path.

## Runtime Authority Boundary

Lifecycle authority changes by phase:

- before promotion, backlog `metadata.yaml` is authoritative for epic lifecycle state
- during execution, root `.doug/project-state.yaml` and root `.doug/tasks.yaml` are authoritative for runtime progress
- on terminal completion, runtime propagates the final lifecycle result back into backlog metadata

This keeps backlog planning state and active runtime state separate while still allowing deterministic promotion between them.

Root `.doug/project-state.yaml` and `.doug/tasks.yaml` are authoritative files, not an external write API. Operators may author initial task content before runtime, but lifecycle mutations such as claiming active work, incrementing attempts, marking `DONE` or `BLOCKED`, advancing task pointers, stamping `completed_at`, and finalizing epics must flow through Doug-owned tools and handlers (`doug run`, mutating interactive MCP tools, and internal lifecycle helpers). Direct YAML edits for those transitions can violate coupled invariants and are unsupported.

For backend preparation, Doug also distinguishes between agent-facing surfaces and non-agent-facing control artifacts:

- Doug-owned control and lifecycle files such as root `.doug/tasks.yaml`, `.doug/project-state.yaml`, backlog metadata, and archive directories are non-agent-facing by default
- Doug-owned agent-facing files are exposed only when the run contract names them explicitly, such as `.doug/ACTIVE_TASK.md`, root `.doug/PRD.md`, or `.doug/plan/PLAN.md`; blocking bug context is rendered into the bugfix task brief from state
- repository-owned files remain project authority rather than Doug authority, even when they are loaded into the run context

Default writable surfaces are workflow-specific:

- runtime and scaffold runs expose the project workspace plus the canonical live Doug task brief (`ACTIVE_TASK.md`)
- planning runs expose only `.doug/ACTIVE_TASK.md` and `.doug/plan/PLAN.md`
- post-epic review runs expose only the relevant `.doug/logs/epics/{epic}/` review artifact and `.doug/ACTIVE_TASK.md` as writable surfaces; they may read project instructions, PRD, KB, changelog, runtime/session archives, and optional `.doug/plan/PLAN.md` as evidence
- post-epic KB/changelog runs expose only `docs/kb/`, `CHANGELOG.md`, and `.doug/ACTIVE_TASK.md` as writable surfaces; they may read archived runtime/session logs and optional `.doug/plan/PLAN.md` planning context

The Pi request contract mirrors this split so Doug can pass explicit path authority, context order, and writable-surface intent without inferring behavior from path strings alone.

## Manual Root-Level Path Remains Supported

The planning lifecycle is additive, not mandatory.

Users may continue to:

- edit root `.doug/PRD.md` directly
- author root `.doug/tasks.yaml` task content before runtime starts
- run doug against the root runtime workspace without creating backlog epics

That manual path remains a supported runtime contract. Planning simply provides an integrated route that produces the same root-level runtime artifacts. Once runtime is active, do not use root `.doug/tasks.yaml` or `.doug/project-state.yaml` as lifecycle control surfaces; use Doug-owned commands/tools instead.
