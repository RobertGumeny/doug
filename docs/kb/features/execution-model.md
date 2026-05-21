---
title: Interaction Model And Pi Policy Ownership
updated: 2026-05-21
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
- Pi mode is source-owned by workflow phase, not operator-configured

That means the current model is simple:

- `doug plan` launches true interactive Pi
- `doug run`, `doug scaffold`, `doug research`, and post-epic KB synthesis use Pi RPC one-shot execution
- `.doug/doug.yaml` stores ordinary project settings such as build system, retries, heartbeat, and KB enablement — not agent-harness selection

## The Supported Model

### 1. Doug owns the workflow brief and prompt

Doug writes `.doug/ACTIVE_TASK.md` for each run and builds the initial workflow prompt in code through `config.BuildInitialPrompt(...)`.

Prompt ownership is intentionally not delegated to project config or provider-specific command templates. The Doug binary defines the workflow contract.

### 2. Doug source owns Pi mode by phase

Doug always routes execution through Pi. The only phase distinction is how Pi is used:

- `planning` → true interactive Pi
- `runtime` → Pi RPC one-shot
- `scaffold` → Pi RPC one-shot
- `research` → Pi RPC one-shot
- `post_epic_kb` → Pi RPC one-shot

`agent.PrepareExecution(...)` resolves the phase mode from source-owned defaults, and `agent.NewBackend()` returns the Pi-backed production path.

### 3. Pi owns downstream provider execution

After Doug hands a run to Pi, Pi owns the downstream agent process lifecycle. Doug does not pick a provider CLI directly and does not treat provider-specific files as runtime control surfaces.

### 4. `doug init` scaffolds Pi-first repo artifacts

`doug init` scaffolds `.pi/extensions/handoff.ts` and `.pi/skills/**`. It does not install provider-specific directories as current runtime surfaces.

The generated `.doug/doug.yaml` is intentionally small and does not define execution routing.

## Authoring Rules

When updating docs, examples, or managed templates:

- state plainly that Pi is Doug's exclusive harness
- describe `doug plan` as true interactive Pi
- describe runtime, scaffold, research, and post-epic KB as Pi RPC one-shot flows
- do not describe subprocess fallback, provider command templates, or config-driven backend selection as current behavior
- treat `.pi/extensions/` as Pi-native integration space, not as a Doug runtime input surface

## Historical Notes

Older Doug revisions exposed more execution-policy language in config and surrounding docs. That is historical context only, not part of the current operator contract. If a document needs to mention that transition, label it explicitly as upgrade or history material.

## Related Topics

- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) for the full Doug-to-Pi execution boundary
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md) for how planning and runtime phases use this contract
- [internal/config](../packages/config.md) for `.doug/doug.yaml` and source-owned interaction-mode defaults
- [cmd/init](../packages/init.md) for generated config and Pi scaffolding
- [internal/templates](../packages/templates.md) for `.pi/extensions/handoff.ts`
- [internal/agent](../packages/agent.md) for Pi adapter behavior and execution preparation
