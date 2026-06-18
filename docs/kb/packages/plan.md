---
title: cmd/plan — Planning Workbook Subcommand
updated: 2026-06-18
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
- auto-detect greenfield planning mode for near-empty repositories when `--mode` is omitted
- refresh `.doug/plan/PLAN.md` with Doug-owned run context
- rewrite root `.doug/ACTIVE_TASK.md` as the canonical brief for the planning run
- launch Pi in true terminal-interactive mode with a bootstrap prompt to read `.doug/ACTIVE_TASK.md`

The command does not perform handoff itself. `PLAN.md` remains the editable workbook, while backlog packages and `manifest.yaml` stay downstream artifacts owned by `doug handoff`.

## Run Context Resolution

`resolvePlanRunContext(cmd, args)` normalizes three user-facing inputs into a deterministic `planRunContext`:

```go
type planRunContext struct {
    Intent           string
    Mode             string
    Epic             string
    ModeAutoDetected bool
}
```

### Planning intent precedence

Intent resolution is strict:

1. Positional text and `--intent` may both be provided only when they normalize to the same trimmed string.
2. If they disagree, the command errors: `planning intent provided twice with different values; use either positional intent or --intent`.
3. If explicit intent is still empty and the session is interactive, the command opens the shared wrapped multiline composer through `interactive.Prompter.Compose(...)`.
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

Any other value fails validation before the planning run starts. An explicitly supplied `--mode` always wins over auto-detection, including non-greenfield modes.

### Greenfield auto-detection

When `--mode` is omitted, `runPlan(...)` may set the planning mode to `greenfield` before `PLAN.md` is created or refreshed. The heuristic is intentionally conservative and requires all of these conditions:

- `config.DetectBuildSystem(projectRoot)` returns empty, meaning no recognized build marker is present
- git history is shallow: no readable commits, no Git repository, or `git rev-list --count --max-count=2 HEAD` reports at most one commit
- the repository contains at most three non-directory files outside `.doug/` and `.git/`

When this path applies, `planRunContext.Mode` becomes `greenfield`, `ModeAutoDetected` is set, and the command logs `auto-detected greenfield planning mode for near-empty repository` before the planning session starts. This changes only the planning-intent hint; the handoff `project.mode` still comes from `## Handoff Data`.

### Interactive capture

When interactive capture is needed, `promptPlanningIntent(...)` uses the shared wrapped multiline composer:

- prompt: `Planning intent required. Describe what this doug plan session should accomplish.`
- submit: Enter
- insert newline: Shift+Enter
- blank submission: hard error

Blank interactive input is treated the same as missing input. The command returns `planning intent is required; provide positional text, --intent, or enter it in the interactive prompt`.

## Planning Setup Flow

`runPlan(...)` resolves the working directory, builds `planRunContext`, then delegates to `planProjectContext(...)`.

`planProjectContext(...)` performs the planning setup in this order:

1. Load unresolved archived bug context from `.doug/logs/bugs/` through `plan.LoadArchivedBugContext`.
2. Create or refresh `.doug/plan/PLAN.md` through `plan.EnsurePlanDocument(...)`.
3. Validate the planning phase contract through `agent.PrepareExecution(...)`.
4. Rewrite root `.doug/ACTIVE_TASK.md` through `agent.WriteActiveTask(...)`.
5. Launch visible terminal-interactive Pi through `agent.PiInteractiveLauncher.Run(...)`.

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
- planning mode, including auto-detected `greenfield` when the near-empty repository heuristic applies
- target epic hint
- a greenfield-only directive requiring the `manifest` block in `## Handoff Data`
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

## Pi Launch Contract

After writing `ACTIVE_TASK.md`, `doug plan` creates a planning `agent.TaskContext` (`PLAN`, type `plan`, attempt `1`) and launches Pi through `agent.PiInteractiveLauncher`.

Important runtime characteristics:

- command shape: `pi --session-dir <dir> [prompt]`
- session directory: `agent.PiInteractiveSessionDir(projectRoot, agent.RunPhasePlanning, taskCtx)`
- working directory: project root
- stdio: attached directly to the current terminal
- bootstrap prompt: tells Pi to read `.doug/ACTIVE_TASK.md` and follow it for the planning session

`doug plan` no longer routes planning through `agent.Backend.Run(...)`, `PiAdapter`, or `PlanningContract(...)`. Doug still resolves the built-in planning prompt before launch, but true terminal-interactive Pi owns the visible planning conversation. When the user exits Pi, control returns to the shell and the user can run `doug handoff` when the workbook is ready.

## Generic Skill Boundary

The `plan` skill works in two modes: generic (outside a Doug workspace) and Doug-managed (launched by `doug plan`).

In **generic mode** the skill applies without additional constraints. The working artifact, handoff data format, and confirmation mechanics depend on the repository's own conventions. There is no prescribed file path, YAML schema, or downstream tool that parses the output.

In **Doug-managed mode** the following additional requirements apply:

- `.doug/ACTIVE_TASK.md` is the canonical brief for the current planning run. The skill must treat it as authoritative over older workbook prose or the inline launch prompt.
- `.doug/plan/PLAN.md` is the sole editable planning workbook. Alternate planning files must not be created.
- Greenfield planning sessions must produce a `manifest` block in `## Handoff Data`; the initial workbook seed uses `project.mode: "greenfield"` for greenfield mode instead of the brownfield default.
- The `## Handoff Data` section of `PLAN.md` must contain a fenced YAML block that `doug handoff` can parse deterministically. The schema is fixed and unknown fields are rejected.
- **Handoff readiness is a confirmed state, not a parseable state.** A plan whose `## Handoff Data` section contains valid YAML is not handoff-ready. The plan advances from draft to handoff-ready only when the user explicitly confirms the alignment summary. Parseable YAML is a necessary condition; explicit user confirmation is the sufficient one.
- `doug handoff` owns all deterministic derivative outputs (backlog epic packages, `manifest.yaml`). These are downstream artifacts generated from `PLAN.md`, not competing planning briefs.

The generic mode applies whenever the skill is used without a Doug workspace or without being launched through `doug plan`. Doug-specific behavior is additive; it does not replace the core planning contract.

## Boundaries

`cmd/plan` is only responsible for planning-session setup and launch. It does not:

- generate backlog packages under `.doug/plan/epics/`
- generate `.doug/plan/manifest.yaml`
- promote a planned epic into runtime
- treat older `PLAN.md` prose as authoritative when fresh CLI intent was provided

Those boundaries matter because the command is designed to preserve one canonical planning brief (`ACTIVE_TASK.md`) and one editable planning workbook (`PLAN.md`) without introducing a third competing planning surface.
