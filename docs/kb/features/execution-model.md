---
title: Interaction Model And Pi Policy Ownership
updated: 2026-07-03
category: Features
tags: [execution, config, pi, policy]
related_articles:
  - docs/kb/packages/config.md
  - docs/kb/packages/init.md
  - docs/kb/packages/templates.md
  - docs/kb/packages/agent.md
  - docs/kb/features/pi-runtime-contract.md
---

# Interaction Model And Pi Policy Ownership

## Overview

Doug has a single supported execution contract:

- Doug orchestrates the workflow and writes the canonical brief
- Pi is the exclusive agent harness
- Pi mode is fixed by workflow phase

That means the current model is simple:

- `doug plan` launches true interactive Pi
- headless Implement (`doug run`), `doug scaffold`, `doug research`, post-epic review, and post-epic KB/changelog synthesis use Pi RPC one-shot execution
- interactive Implement uses Doug's MCP-first surface (`doug mcp`) so an already-active agent session can call Doug-owned lifecycle tools instead of editing lifecycle files
- `.doug/doug.yaml` stores ordinary project settings such as build system, retries, heartbeat, and KB enablement

## The Supported Model

### 1. Doug owns the workflow brief and prompt

Doug writes `.doug/ACTIVE_TASK.md` for each run and builds the initial workflow prompt in code through `config.BuildInitialPrompt(...)`.

The Doug binary defines the workflow contract.

### 2. Doug source owns Pi mode by phase

Doug always routes execution through Pi. The only phase distinction is how Pi is used:

- `planning` → true interactive Pi
- `runtime` → Pi RPC one-shot
- `scaffold` → Pi RPC one-shot
- `research` → Pi RPC one-shot
- `post_epic_review` → Pi RPC one-shot
- `post_epic_kb` → Pi RPC one-shot

`agent.PrepareExecution(...)` resolves the phase mode from built-in defaults, and `agent.NewBackend()` returns the Pi-backed production path.

### 3. Pi owns downstream execution

After Doug hands a run to Pi, Pi owns the downstream agent process lifecycle.

### 4. `doug init` scaffolds Pi-first repo artifacts

`doug init` scaffolds `.pi/extensions/handoff.ts` and the six namespaced built-in skills under `.agents/skills/doug-*/`. Claude receives the same canonical skills through the supported `.claude/skills` bridge (or ownership-recorded managed copies when its directory must be preserved). Pi's local-project trust requirement is unchanged.

The generated `.doug/doug.yaml` is intentionally small and focused on project/runtime settings.

## Authoring Rules

When updating docs, examples, or managed templates:

- state plainly that Pi is Doug's exclusive harness
- describe `doug plan` as true interactive Pi
- describe headless Implement (`doug run`), scaffold, research, post-epic review, and post-epic KB/changelog as Pi RPC one-shot flows
- describe interactive Implement as MCP-first (`doug mcp`) and keep lifecycle mutation routed through Doug-owned tools
- use headless/interactive terminology; do not revive PUSH/PULL naming
- keep examples and prose aligned with the Pi-only model
- treat `.pi/extensions/` as Pi-native integration space, not as a Doug runtime input surface

## Retired Concepts

The Pi-only simplification retired several older concepts:

- config-driven execution routing
- provider or phase-specific `*_agent_command` overrides
- subprocess backend selection or fallback narratives

If those concepts appear, treat them as historical or upgrade-only material rather than current behavior. The canonical cleanup path is [`doug upgrade`](upgrade.md), and the current runtime contract lives in the execution-model, Pi runtime contract, config, and agent KB articles.

## Related Topics

- [Interactive Implement MCP Surface](interactive-implement.md) for MCP tools, lifecycle authority, locking, and dispatcher/worker context hygiene
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) for the full Doug-to-Pi execution boundary
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md) for how planning and runtime phases use this contract
- [internal/config](../packages/config.md) for `.doug/doug.yaml` and source-owned interaction-mode defaults
- [cmd/init](../packages/init.md) for generated config and Pi scaffolding
- [internal/templates](../packages/templates.md) for `.pi/extensions/handoff.ts`
- [internal/agent](../packages/agent.md) for Pi adapter behavior and execution preparation
