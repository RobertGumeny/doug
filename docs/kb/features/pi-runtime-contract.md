---
title: Doug-to-Pi Runtime Contract
updated: 2026-05-14
category: Features
tags: [pi, rpc, execution, contract, policy, backend]
related_articles:
  - docs/kb/features/execution-model.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
---

# Doug-to-Pi Runtime Contract

## Overview

After the `execution_mode: rpc` cutover, Pi is the required Doug-to-agent execution boundary. Every agent invocation in a Pi-configured project routes through Pi. Doug does not launch an agent subprocess directly in this mode.

This article defines:

- Pi's mandatory role in the post-cutover execution architecture
- The policy inputs Doug sends to Pi per invocation
- The workflow interaction semantics (startup through completion)
- The explicit compatibility boundaries between Doug and Pi

## Pi's Mandatory Role

In the Pi-era runtime model Pi is not one selectable peer backend among several. It is the required execution layer between Doug and the underlying agent process. When `execution_mode: rpc` is resolved from `.doug/doug.yaml`, Doug's `NewBackend` factory returns a `PiAdapter`. Doug never launches an agent subprocess directly in this mode.

**Doug owns:**

- Workflow orchestration: task ordering, retry logic, epic lifecycle, and exit codes
- Task briefing: authoring and writing `ACTIVE_TASK.md` before each invocation
- Policy resolution: skill name, execution mode, restrictions, and artifact surfaces — all resolved before the backend is invoked
- Outcome authority: `## Agent Result` in `ACTIVE_TASK.md` is the only valid source of workflow outcomes (`SUCCESS`, `FAILURE`, `BUG`, `EPIC_COMPLETE`)
- Session archiving and log management under `.doug/logs/`

**Pi owns:**

- Model and provider selection: Doug does not specify model, temperature, or provider in the Pi RPC request
- Tool permission enforcement: when `Restrictions.Write.Mode` is `allow_list`, Pi enforces the write boundary natively
- Agent process lifecycle: Pi spawns and manages the agent subprocess (e.g. Claude Code, Codex)
- Session state retention under the Doug-scoped session directory
- Agent execution output: Pi streams agent stdout/stderr to Doug's managed output log

## Policy Inputs Doug Sends to Pi

Doug resolves all policy before constructing a `RunRequest`. The `PiAdapter` translates these Doug-native inputs into a private Pi RPC request. No call site outside `internal/agent` may depend on the Pi wire format.

### Task Context

| Field | Description |
|-------|-------------|
| `Task.ID` | Doug task identifier (e.g. `EPIC-33-001`) |
| `Task.Type` | Task type (`feature`, `bugfix`, `documentation`, etc.) |
| `Task.Attempt` | Current retry attempt from Doug's retry loop |
| `Task.MaxRetries` | Maximum retries configured in `.doug/doug.yaml` |
| `Task.EpicID` | Parent epic identifier; used to scope the Pi session directory |
| `Task.EpicName` | Human-readable epic name |

### Briefing and Context

| Field | Description |
|-------|-------------|
| `Brief.Path` | Absolute path to `ACTIVE_TASK.md` (authority: `doug`) |
| `Brief.Format` | Always `markdown` in the current contract |
| `ContextLoadOrder` | Ordered list of stable context artifacts for prompt-cache-friendly loading |

Context load order for runtime tasks:

1. `AGENTS.md` — project instructions, authority: `project`, not required
2. `PRD.md` — product context, authority: `doug`, not required
3. `ACTIVE_TASK.md` — canonical brief, authority: `doug`, required

### Routing and Policy

| Field | Description |
|-------|-------------|
| `Routing.Workflow` | Resolved workflow name (e.g. `runtime`) |
| `Routing.SkillName` | Resolved skill name (e.g. `implement-feature`) |
| `Routing.ExecutionMode` | Always `rpc` in the Pi path |
| `Policy.SessionPolicy` | Resolved session routing profile (mapped to Pi payload) |
| `Policy.ToolPolicy` | Resolved tool-access policy identifier (reserved; not yet mapped to Pi payload) |
| `Policy.SessionDefaults` | Resolved session defaults identifier (reserved; not yet mapped to Pi payload) |

`ToolPolicy` and `SessionDefaults` are present in `PolicyInputs` and in the Doug contract but are not yet translated into the private Pi RPC request. Add them in the Pi adapter when Pi supports those fields.

### Artifact Surfaces

Doug sends explicit read and write surfaces to Pi. Each surface carries:

- `Path` — file system path
- `Purpose` — classified artifact type (`canonical_brief`, `project_workspace`, `bug_handoff`, etc.)
- `Authority` — which system owns the artifact (`project`, `doug`, or `pi`)
- `AgentFacing` — whether the agent should see this surface

Doug-owned artifacts (`authority: doug`) are non-agent-facing by default unless the run contract explicitly exposes them. Pi uses the `AgentFacing` flag to decide which surfaces to present to the agent.

### Restrictions

| Field | Description |
|-------|-------------|
| `Restrictions.Read.Mode` | `inherit` or `allow_list` |
| `Restrictions.Read.Paths` | Explicit allow-list paths when mode is `allow_list` |
| `Restrictions.Write.Mode` | `inherit` or `allow_list` |
| `Restrictions.Write.Paths` | Explicit allow-list paths when mode is `allow_list` |

When `Write.Mode` is `allow_list`, Pi enforces the write boundary natively. For non-Pi (`DefaultBackend`) runs, Doug injects a `## Write Scope Constraints` section into `ACTIVE_TASK.md` as a briefing-level fallback so the policy restriction is still explicit to the agent.

## Workflow Interaction Semantics

One Pi RPC interaction per Doug task iteration:

1. **Doug writes `ACTIVE_TASK.md`** — the briefing artifact is the canonical task contract for the invocation.
2. **Doug calls `PiAdapter.Run`** — the adapter translates `RunRequest` into a private `piLaunchSpec` and delegates to `piCLILauncher`.
3. **Doug launches `pi --mode rpc --session-dir <dir>`** — session directory is scoped to `.doug/logs/pi-sessions/{epicID}/{taskID}/attempt-{n}`.
4. **Doug sends `get_state`** — Doug retrieves the Pi session ID before sending a prompt.
5. **Doug sends `prompt`** — the resolved command string is the prompt message payload; restriction metadata is included when configured.
6. **Pi runs the agent** — Pi spawns the underlying agent process, manages its lifecycle, and exposes the project workspace and Doug briefing artifacts per the artifact surfaces.
7. **Pi signals `agent_end`** — Doug awaits this event to know the agent has completed its turn.
8. **Doug reads `ACTIVE_TASK.md`** — the `## Agent Result` block is the authoritative outcome. Pi's `RunResponse` is runtime transport metadata only and does not carry Doug workflow outcomes.
9. **Doug archives and cleans up** — `ACTIVE_TASK.md` is copied to `.doug/logs/sessions/{epicID}/` before any state change; the live file is removed after outcome handling.

## Compatibility Boundaries

### What Doug Must Not Do

- Interpret Pi's intermediate RPC messages (`response`, `agent_end`, etc.) as workflow outcomes.
- Embed model, provider, or permission configuration in the Pi RPC payload; those are Pi's domain.
- Read `.pi/` files during `doug run`; Pi extension files are setup conveniences, not Doug runtime inputs.

### What Pi Must Not Do

- Override Doug-owned artifact authority markers.
- Write to `.doug/` paths not listed in the run's `Artifacts.Write` surfaces.
- Return workflow outcomes via `RunResponse`; outcomes belong in `ACTIVE_TASK.md`.

### Stable Interface Points

- `internal/agent.RunRequest` and `RunResponse` are the stable Doug-native contract; Pi adapter internals may change without touching any call site.
- `internal/agent.RunContract` and its workflow-specific constructors (`RuntimeContract`, `PlanningContract`, etc.) are the integration point for adding new workflow phases or artifact surfaces.
- `ArtifactAuthorityPi` is reserved for future Pi-owned artifacts so later adapter work can add Pi surfaces without overloading Doug or project ownership semantics.

## Related Topics

- [Execution Model And Provider Presets](execution-model.md) — `doug.yaml` config surfaces, preset rewrite vs. backend selection, and Pi activation
- [internal/agent](../packages/agent.md) — `Backend` interface, `PiAdapter`, `RunRequest`, `RunContract`, and the full agent lifecycle
- [internal/config](../packages/config.md) — `PolicyConfig`, `ResolvedExecution`, and policy resolution
