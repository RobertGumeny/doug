---
title: doug research — Read-Only Codebase Analysis
updated: 2026-05-04
category: Features
tags: [research, command, read-only, analysis, contract, restriction]
related_articles:
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
  - docs/kb/features/planning-lifecycle.md
---

# doug research — Read-Only Codebase Analysis

## Overview

`doug research` writes a Doug-owned brief for read-only codebase analysis and emits the research interaction through the same `Backend` interface as all other Doug commands. It writes the research report to `.doug/logs/research/` and does not modify product code, docs, or task files.

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
- `.doug/logs/research/` — research report output directory

The write restriction mode is `AllowList`, so future Pi-backed backends can enforce the boundary natively. The agent is not permitted to write to the project root, `docs/`, or any other path. A `## Write Scope Constraints` section is injected into `ACTIVE_TASK.md` when `policy.tasks.research.write_scopes` is set in `doug.yaml`.

Report files should be named by the agent using the pattern `report_[scope]-[timestamp].md` under `.doug/logs/research/`. Do not create `RESEARCH_REPORT.md` in the project root.

## Execution Path

1. `runResearch` resolves project root and run context from flags/args.
2. `agent.PrepareExecution(RunPhaseResearch, "research", "RESEARCH", cfg.Policy)` resolves skill, built-in prompt text, and policy.
3. `agent.WriteActiveTask` writes `.doug/ACTIVE_TASK.md` with task brief, acceptance criteria, and a `## Research Output` context section.
4. `agent.ResearchContract` assembles the Doug-native artifact contract.
5. `agent.ApplyPolicyScopeRestrictions` merges any policy write scopes and read additions.
6. `researchRunAgent.Run` dispatches via the `Backend` interface (injectable for tests via `var researchRunAgent agent.Backend`).

`Output` is set to `nil`, so the agent runs interactively in the terminal (same as `doug plan`). There is no heartbeat and no output log file.

## Config

Research prompt text comes from Doug's built-in research prompt via `config.BuildCommand(...)`. Backend routing comes from the resolved `policy` in `.doug/doug.yaml`; after `doug init`, that policy defaults to `interaction_mode: rpc` for research runs, so Doug sends the prompt and restrictions to Pi and Pi chooses the downstream provider/model configuration.

```yaml
# .doug/doug.yaml
policy:
  phases:
    research:
      interaction_mode: rpc
```

## Key Decisions

**Write-only to `.doug/logs/research/`**: Research is read-only from the project's perspective. Restricting writes to a known output directory prevents research runs from accidentally modifying product code or planning artifacts. The `AllowList` mode makes this intention explicit to future backends.

**No retry loop**: Research is a one-shot agent invocation. There is no `## Agent Result` outcome parsing or state machine — the command exits after the agent returns. Errors from the backend propagate directly to the user.

**`researchRunAgent` package var**: `cmd/research.go` exposes `var researchRunAgent agent.Backend` so tests can inject a stub without modifying production routing. When no stub is injected, production code resolves the backend from policy via `agent.NewBackend(...)`.

**`researchTaskID = "RESEARCH"`**: Fixed constant (no epic or numeric suffix) because research is not part of the task-sequence state machine.

## Related Topics

- [internal/agent](../packages/agent.md) — `ResearchContract`, `RunPhaseResearch`, `Backend` interface, call sites
- [internal/config](../packages/config.md) — execution policy, built-in prompts, and config loading
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md) — research is outside the epic/task state machine
