---
title: doug research — Read-Only Codebase Analysis
updated: 2026-06-27
category: Features
tags: [research, command, read-only, analysis, contract, restriction, planning-intake]
related_articles:
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
  - docs/kb/packages/plan.md
  - docs/kb/features/planning-lifecycle.md
---

# doug research — Read-Only Codebase Analysis

## Overview

`doug research` writes a Doug-owned brief for read-only codebase analysis and executes one Pi RPC research run. It writes the research report to `.doug/intake/research/` and does not modify product code, docs, or task files.

Top-level markdown reports in `.doug/intake/research/` are also surfaced later by `doug plan` as recent-research planning candidates. This creates a lightweight research-to-planning intake path without giving research runs permission to modify planning artifacts directly.

## Usage

```bash
doug research [topic...]
doug research --topic "authentication flow"
doug research --topic "logging coverage" --scope codebase
```

| Flag | Values | Description |
|------|--------|-------------|
| `--topic` | any string | Explicit research topic; also accepts positional args |
| `--scope` | `feature`, `file`, `codebase` | Optional scope hint injected into the brief |

If both `--topic` and positional args are provided, they must be identical. Conflicting values return an error before the agent is launched.

## Write Contract

`ResearchContract` restricts the agent's write surface to two paths:

- `.doug/ACTIVE_TASK.md` — canonical brief (agent writes `## Agent Result`)
- `.doug/intake/research/` — research report output directory

The write restriction mode is `AllowList`, so Pi can enforce the boundary natively. The agent is not permitted to write to the project root, `docs/`, or any other path.

Report files should be named by the agent using the pattern `report_[scope]-[timestamp].md` under `.doug/intake/research/`. Do not create `RESEARCH_REPORT.md` in the project root.

## Planning Intake

`doug plan` loads simple research context from top-level markdown files directly under `.doug/intake/research/`. Each included report is rendered in `PLAN.md` as a planning candidate with a report ID derived from the filename and a relative source path. `README.md` and subdirectories are ignored. Legacy `.doug/logs/research/` reports are still read during the transition for backward compatibility.

This intake is intentionally minimal. Doug does not yet provide frontmatter filtering, status/disposition filtering, candidate capping, or automatic archival to `.doug/intake/research/history/`. Reports continue appearing as planning candidates until an operator or future workflow moves, renames, or removes them from the top-level research directory.

## Execution Path

1. `runResearch` resolves project root and run context from flags/args.
2. `agent.PrepareExecution(RunPhaseResearch, "research", "RESEARCH")` resolves the built-in skill, prompt text, and Pi interaction mode.
3. `agent.WriteActiveTask` writes `.doug/ACTIVE_TASK.md` with task brief, acceptance criteria, and a `## Research Output` context section.
4. `agent.ResearchContract` assembles the Doug-native artifact contract.
5. `researchRunAgent.Run` dispatches via the `Backend` interface (injectable for tests via `var researchRunAgent agent.Backend`).

The production path is still Pi RPC one-shot execution. It uses the shared sanitized heartbeat/status indicator and writes a standard end-of-turn summary, but does not mirror raw agent output to a log file.

## Config

Research prompt text comes from Doug's built-in research prompt via `config.BuildInitialPrompt(...)`. Research runs use Pi RPC one-shot execution.

## Key Decisions

**Write-only to `.doug/intake/research/`**: Research is read-only from the project's perspective. Restricting writes to a known output directory prevents research runs from accidentally modifying product code or planning artifacts. Planning intake reads those reports later; the research command itself does not write `PLAN.md`.

**No retry loop**: Research is a one-shot agent invocation. There is no orchestration retry/state-machine loop — the command exits after the Pi run returns.

**`researchRunAgent` package var**: `cmd/research.go` exposes `var researchRunAgent agent.Backend` so tests can inject a stub. When no stub is injected, production code resolves the Pi-backed backend via `agent.NewBackend(...)`.

**`researchTaskID = "RESEARCH"`**: Fixed constant (no epic or numeric suffix) because research is not part of the task-sequence state machine.

## Related Topics

- [internal/agent](../packages/agent.md) — `ResearchContract`, `RunPhaseResearch`, `Backend` interface, call sites
- [cmd/plan](../packages/plan.md) — `IntakeSections` and simple research intake into `PLAN.md`
- [internal/config](../packages/config.md) — built-in prompts, interaction-mode defaults, and config loading
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md) — research is outside the epic/task state machine
