---
title: cmd/plan — Planning Workbook Subcommand
updated: 2026-05-15
category: Packages
tags: [plan, planning, workbook, interactive, cobra, intent, handoff]
related_articles:
  - docs/kb/features/planning-lifecycle.md
  - docs/kb/packages/interactive.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# cmd/plan — Planning Workbook Subcommand

## Overview

`cmd/plan.go` implements `doug plan [planning-intent...]`. The command owns the planning-session setup path before the planning agent is launched:

- resolve the planning intent for the current run
- refresh `.doug/plan/PLAN.md` with Doug-owned run context
- rewrite root `.doug/ACTIVE_TASK.md` as the canonical brief for the planning run
- emit the Doug planning prompt and resolved policy through Pi with the `plan` skill and the planning contract

The command does not perform handoff itself. `PLAN.md` remains the editable workbook, while backlog packages and `manifest.yaml` stay downstream artifacts owned by `doug handoff`.

## Run Context Resolution

`resolvePlanRunContext(cmd, args)` normalizes three user-facing inputs into a deterministic `planRunContext`:

```go
type planRunContext struct {
    Intent string
    Mode   string
    Epic   string
}
```

### Planning intent precedence

Intent resolution is strict:

1. Positional text and `--intent` may both be provided only when they normalize to the same trimmed string.
2. If they disagree, the command errors: `planning intent provided twice with different values; use either positional intent or --intent`.
3. If explicit intent is still empty and the session is interactive, the command opens the shared single-line prompt surface through `interactive.Prompter.Text(...)`.
4. If explicit intent is empty and the session is non-interactive, the command fails before any plan files or agent state are created.

This preserves explicit CLI intent as the highest-precedence source and prevents stale workbook prose from silently becoming the run objective.

### Mode validation

`--mode` is normalized to lowercase and must be one of:

- `discovery`
- `roadmapping`
- `definition`
- `feature`
- `refactor`
- `bugfix`
- `greenfield`

Any other value fails validation before the planning run starts.

### Interactive capture

When interactive capture is needed, `promptPlanningIntent(...)` uses the shared single-line text prompt:

- prompt: `Planning intent required. Describe what this doug plan session should accomplish.`
- submit: Enter
- blank submission: hard error

Blank interactive input is treated the same as missing input. The command returns `planning intent is required; provide positional text, --intent, or enter it in the interactive prompt`.

## Planning Setup Flow

`runPlan(...)` resolves the working directory, builds `planRunContext`, then delegates to `planProjectContext(...)`.

`planProjectContext(...)` performs the planning setup in this order:

1. Load `.doug/doug.yaml` through `config.LoadConfig`.
2. Load unresolved archived bug context from `.doug/logs/bugs/` through `plan.LoadArchivedBugContext`.
3. Create or refresh `.doug/plan/PLAN.md` through `plan.EnsurePlanDocument(...)`.
4. Prepare provider execution policy and command resolution through `agent.PrepareExecution(...)`.
5. Rewrite root `.doug/ACTIVE_TASK.md` through `agent.WriteActiveTask(...)`.
6. Dispatch the Doug planning interaction through `agent.Backend.Run(...)` with `agent.PlanningContract(...)`.

The terminal output is intentionally small: the command prints either `Created .doug/plan/PLAN.md` or `Using existing .doug/plan/PLAN.md` before the planning agent runs.

## Brief And Workbook Contract

`doug plan` uses two Doug-managed artifacts with different authority:

- `.doug/ACTIVE_TASK.md` is the canonical brief for the current planning run
- `.doug/plan/PLAN.md` is the editable planning workbook and the persisted planning-intent surface

`planProjectContext(...)` writes a dedicated `Planning Workbook` context section into `ACTIVE_TASK.md` that tells the agent to:

- treat `ACTIVE_TASK.md` as the canonical brief
- update `PLAN.md` directly
- when clarification is needed, check the codebase and KB first; ask the user only when the repository cannot answer the question
- when material ambiguity remains after lookup, ask one high-leverage question at a time rather than presenting a list
- before finalizing handoff-ready epics and tasks, produce an explicit alignment summary and get confirmation
- promote execution-relevant guidance discovered during planning into the epic PRD or task contracts rather than leaving it only in workbook narrative
- treat handoff outputs as downstream artifacts rather than competing briefs

At the same time, `plan.EnsurePlanDocument(...)` refreshes the Doug-owned brief block at the top of `PLAN.md` so the workbook carries current run context before agent launch. That brief includes:

- resolved planning intent
- planning mode
- target epic hint
- latest handoff context when present
- unresolved archived bug intake when present

If the existing workbook narrative conflicts with the current run context, the planning session is expected to reconcile the workbook instead of following stale prose.

## Archived Bug Intake

`plan.LoadArchivedBugContext(...)` turns unresolved bug reports under `.doug/logs/bugs/{epic}/` into planning-time intake bullets. Each bullet includes:

- bug ID
- source epic
- archived bug status and severity
- summary text from the bug report when present
- source epic lifecycle status when backlog metadata exists
- planning guidance based on lifecycle state

The lifecycle guidance is intentional:

- `PLANNED`: update the existing planned package if scope still fits, or create new planned follow-up work
- `ACTIVE`: plan follow-up as new work; do not reopen the active package
- `COMPLETED`: plan follow-up as new work; do not reopen the completed historical package
- missing backlog metadata: still turn it into explicit new or updated planned work

This keeps bug rediscovery tied to the durable archive instead of a second manual intake file.

## Agent Contract

The planning command uses `agent.PlanningContract(projectRoot, dougDir, planPath)` and then applies policy scope restrictions from `PrepareExecution(...)`.

Important runtime characteristics:

- workflow: `plan`
- task id: `PLAN`
- task type: `plan`
- output log writer: `nil`, so the planning session stays interactive in the terminal
- heartbeat: none; planning intentionally suppresses runtime heartbeat logging

The prompt text comes from Doug's built-in planning prompt (`config.BuildCommand(...)` for the planning phase), and backend routing comes from the resolved policy. In the default post-init path, that means Pi receives the planning prompt plus policy and chooses the downstream provider/model configuration itself.

## Boundaries

`cmd/plan` is only responsible for planning-session setup and launch. It does not:

- generate backlog packages under `.doug/plan/epics/`
- generate `.doug/plan/manifest.yaml`
- promote a planned epic into runtime
- treat older `PLAN.md` prose as authoritative when fresh CLI intent was provided

Those boundaries matter because the command is designed to preserve one canonical planning brief (`ACTIVE_TASK.md`) and one editable planning workbook (`PLAN.md`) without introducing a third competing planning surface.
