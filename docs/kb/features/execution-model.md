---
title: Execution Model And Provider Presets
updated: 2026-05-15
category: Features
tags: [execution, config, pi, presets]
related_articles:
  - docs/kb/packages/config.md
  - docs/kb/packages/init.md
  - docs/kb/packages/templates.md
  - docs/kb/packages/agent.md
  - docs/kb/features/pi-runtime-contract.md
---

# Execution Model And Provider Presets

## Overview

Doug separates three concerns that are easy to conflate if you only read one surface:

- provider presets choose which prompt template Doug emits for each workflow phase
- execution policy chooses backend routing, skill overrides, and restriction metadata
- provider-local scaffolding gives the selected agent ecosystem convenient project files without changing Doug's runtime authority

The canonical config file for all three is `.doug/doug.yaml`, but each concern lives in a different part of that file and has a different owner.

## The Supported Model

### 1. Provider presets are the four mode-specific command fields

Doug's preset layer is the set of four top-level command fields in `.doug/doug.yaml`:

- `run_agent_command`
- `plan_agent_command`
- `scaffold_agent_command`
- `research_agent_command`

These fields define the prompt or CLI template Doug resolves for each workflow phase. They are a convenience surface for configuring agent command sets, not a second execution-policy system. Edit these fields directly in `.doug/doug.yaml` to change providers.

### 2. Execution policy owns backend routing

Backend selection is controlled by the resolved `policy` contract.

- `policy.phases.*.execution_mode`
- `policy.tasks.*.execution_mode`

When the resolved `execution_mode` is `rpc`, Doug's `NewBackend` factory returns a `PiAdapter`. Pi is the required execution boundary in this mode — Doug does not launch agent subprocesses directly. For non-Pi projects, or where no execution mode is configured, Doug uses `DefaultBackend` (subprocess).

Pi activation requires both Pi-flavored preset commands and `execution_mode: rpc` in the resolved policy.

### 3. Pi command templates are prompt payloads, not CLI invocations

Pi is different from Claude, Codex, and Gemini at the preset layer. Its command templates are prompt-only strings. Doug's Pi adapter supplies the `pi --mode rpc` launch itself and sends the resolved template as the RPC message payload.

That is why Pi activation requires both:

- Pi-flavored preset commands
- `execution_mode: rpc` in the resolved policy

### 4. `doug init` scaffolds Pi files; runtime authority stays with Doug

`doug init` scaffolds `.pi/extensions/handoff.ts` and `.pi/skills/**`. Provider-specific directories (`.claude/`, `.codex/`, `.gemini/`) are no longer installed — Pi is the supported execution model.

Doug's runtime authority comes from:

- `.doug/ACTIVE_TASK.md`
- the resolved mode-specific command template
- the resolved policy/backend contract

Provider-local files do not replace Doug-owned briefing, result parsing, or lifecycle artifacts.

## Pi Extension Surfaces

The only current Pi extension artifact scaffolded by Doug is `.pi/extensions/handoff.ts`.

Its role is narrow:

- it is a Pi-side helper for handoff workflows
- it is installed at init time as a convenience
- Doug does not read `.pi/extensions/*` during `doug run`

Treat `.pi/extensions/` as optional Pi-native integration space, not as a Doug runtime input surface.

## Follow-Up Notes

- Pi activation path: set Pi-flavored preset commands in `.doug/doug.yaml` and add `execution_mode: rpc` to the resolved policy.
- If future Pi integration introduces additional extension files or extension-owned runtime artifacts, document each surface explicitly. Current `.pi/` scaffolding does not imply broader authority.
- For the full Doug-to-Pi interaction contract — policy inputs, workflow interaction semantics, and compatibility boundaries — see [Doug-to-Pi Runtime Contract](pi-runtime-contract.md).

## Related Topics

- [internal/config](../packages/config.md) for the `policy` model and `.doug/doug.yaml`
- [cmd/init](../packages/init.md) for generated config and provider scaffolding
- [internal/templates](../packages/templates.md) for `.pi/extensions/handoff.ts`
- [internal/agent](../packages/agent.md) for backend selection and Pi adapter behavior
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) for the full post-cutover interaction contract
