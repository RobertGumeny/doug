---
title: Execution Model And Pi Policy Ownership
updated: 2026-05-15
category: Features
tags: [execution, config, pi, policy]
related_articles:
  - docs/kb/packages/config.md
  - docs/kb/packages/init.md
  - docs/kb/packages/templates.md
  - docs/kb/packages/agent.md
  - docs/kb/features/pi-runtime-contract.md
---

# Execution Model And Pi Policy Ownership

## Overview

Doug separates three concerns that are easy to conflate if you only read one surface:

- Doug-owned prompt text tells the agent what workflow to perform
- execution policy chooses backend routing, skill overrides, and restriction metadata
- Pi-owned runtime selection chooses the underlying provider, model, and tool configuration after Doug hands the run to Pi

The user-facing source of truth is split intentionally:

- built-in code constants own Doug's prompt text
- `.doug/doug.yaml` owns Doug's execution policy
- Pi owns downstream provider/model/tool selection once `execution_mode: rpc` is active

## The Supported Model

### 1. Doug owns the workflow prompt

Doug builds the workflow prompt in code for each phase:

- runtime and scaffold use the canonical runtime prompt
- planning uses the planning prompt
- research uses the research prompt

These prompts are emitted by `config.BuildCommand(...)`, not read from `.doug/doug.yaml`. That keeps the Doug-managed run contract authoritative in the binary instead of in provider-specific CLI templates.

### 2. Execution policy owns backend routing

Backend selection is controlled by the resolved `policy` contract.

- `policy.phases.*.execution_mode`
- `policy.tasks.*.execution_mode`

When the resolved `execution_mode` is `rpc`, Doug's `NewBackend` factory returns a `PiAdapter`. Pi is the required execution boundary in this mode: Doug writes `.doug/ACTIVE_TASK.md`, resolves the run contract, and sends that prompt-plus-policy payload to Pi. Doug does not launch an underlying provider subprocess directly in this mode.

For non-Pi projects, or where no execution mode is configured, Doug uses `DefaultBackend` (subprocess).

### 3. Pi owns provider selection after the handoff

Once Doug routes a run through Pi:

- Doug does not specify provider, model, or temperature in the RPC payload
- Doug does not choose a provider CLI directly
- Pi launches `pi --mode rpc` and manages the downstream agent process lifecycle

This is the key ownership boundary: Doug chooses workflow semantics and execution policy; Pi chooses how that work is executed against the underlying agent stack.

### 4. `doug init` scaffolds Pi files and emits Pi policy by default

`doug init` scaffolds `.pi/extensions/handoff.ts` and `.pi/skills/**`. Provider-specific directories (`.claude/`, `.codex/`, `.gemini/`) are no longer installed.

It also writes `policy.phases.*.execution_mode: rpc` for every Doug workflow phase, so Pi is the default supported execution path immediately after init.

Doug's runtime authority comes from:

- `.doug/ACTIVE_TASK.md`
- the Doug-owned workflow prompt built in code
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

- Pi activation path: set `execution_mode: rpc` in the resolved policy. `doug init` already does this for every phase.
- If future Pi integration introduces additional extension files or extension-owned runtime artifacts, document each surface explicitly. Current `.pi/` scaffolding does not imply broader authority.
- For the full Doug-to-Pi interaction contract — policy inputs, workflow interaction semantics, and compatibility boundaries — see [Doug-to-Pi Runtime Contract](pi-runtime-contract.md).

## Related Topics

- [internal/config](../packages/config.md) for the `policy` model and `.doug/doug.yaml`
- [cmd/init](../packages/init.md) for generated config and provider scaffolding
- [internal/templates](../packages/templates.md) for `.pi/extensions/handoff.ts`
- [internal/agent](../packages/agent.md) for backend selection and Pi adapter behavior
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) for the full post-cutover interaction contract
