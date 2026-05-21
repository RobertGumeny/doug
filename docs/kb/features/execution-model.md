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

Doug separates three concerns that are easy to conflate if you only read one surface:

- Doug-owned prompt text tells the agent what workflow to perform
- execution policy chooses backend routing, skill overrides, and restriction metadata
- Pi-owned runtime selection chooses the underlying provider, model, and tool configuration after Doug hands the run to Pi

The user-facing source of truth is split intentionally:

- built-in code constants own Doug's prompt text
- `.doug/doug.yaml` owns Doug's execution policy
- Pi owns downstream provider/model/tool selection once a Pi-backed `interaction_mode` (`interactive` or `rpc`) is active

## The Supported Model

### 1. Doug owns the workflow prompt

Doug builds the workflow prompt in code for each phase:

- runtime and scaffold use the canonical runtime prompt
- planning uses the planning prompt
- research uses the research prompt

These prompts are emitted by `config.BuildCommand(...)`, not read from `.doug/doug.yaml`. That keeps the Doug-managed run contract authoritative in the binary instead of in provider-specific CLI templates.

### 2. Source code owns backend routing by phase

Backend selection and Pi mode are controlled by Doug source, not task type and not `.doug/doug.yaml`.

- `planning` launches true terminal-interactive Pi.
- `runtime`, `scaffold`, `research`, and `post_epic_kb` launch Pi RPC one-shot execution.
- Unknown internal phases fail with a Doug error instead of falling back to another mode.

`policy.phases.*.interaction_mode` and `policy.tasks.*.interaction_mode` remain parsed only as a managed-config/migration surface; they cannot change the execution harness or Pi mode. `NewBackend` always returns a `PiAdapter` for production runtime dispatch.

### 3. Pi owns provider selection after the handoff

Once Doug routes a run through Pi:

- Doug does not specify provider, model, or temperature in the RPC payload
- Doug does not choose a provider CLI directly
- Pi launches `pi --mode rpc` and manages the downstream agent process lifecycle

This is the key ownership boundary: Doug chooses workflow semantics and execution policy; Pi chooses how that work is executed against the underlying agent stack.

### 4. `doug init` scaffolds Pi files and emits Pi policy by default

`doug init` scaffolds `.pi/extensions/handoff.ts` and `.pi/skills/**`. Provider-specific directories (`.claude/`, `.codex/`, `.gemini/`) are no longer installed.

It also writes explicit phase interaction defaults: `planning` uses `interaction_mode: interactive`, while `runtime`, `scaffold`, `research`, and `post_epic_kb` use `interaction_mode: rpc`. Pi is therefore the default supported execution path immediately after init, with planning routed through a visible interactive Pi session instead of an RPC one-shot.

Doug's runtime authority comes from:

- `.doug/ACTIVE_TASK.md`
- the Doug-owned workflow prompt built in code
- the resolved policy/backend contract

Provider-local files do not replace Doug-owned briefing, result parsing, or lifecycle artifacts.

## Remaining Compatibility Surfaces

The repository has moved to a Pi-first model, but a few compatibility surfaces remain intentionally available:

- `interaction_mode: subprocess` is no longer a supported transport and is rejected as stale configuration.
- `tool_policy` and `session_defaults` are already part of Doug's resolved execution contract, but the Pi adapter only maps the currently supported Pi payload fields.
- `doug plan` and `doug research` use the same Doug-owned prompt and policy resolution model as runtime tasks, but they still have workflow-specific interaction contracts rather than fully sharing the runtime retry/state-machine behavior.
- Root `.doug/PRD.md` plus `.doug/tasks.yaml` remains a supported manual runtime workspace even though `.doug/plan/` plus backlog promotion is the newer structured planning path.

These surfaces should be documented explicitly so the repository does not imply a cleaner cutover than the code currently implements.

## Repository Authoring Rules

EPIC-35 established the repository-facing rule for new docs, prompts, examples, and managed artifacts:

- describe source-owned Pi routing (`interactive` for planning; `rpc` for runtime/scaffold/research/post-epic KB) as the Doug interaction model
- do not describe `interaction_mode: subprocess` as compatibility or fallback behavior
- describe Doug-owned prompts as built-in command text rather than operator-edited provider launch templates
- keep managed init artifacts aligned with the supported Pi-first scaffold; do not reintroduce dormant `.claude/`, `.codex/`, or `.gemini/` examples or template baggage

When documentation needs to mention transitional behavior, name the exact surviving surface instead of implying Doug still chooses providers directly. The legacy `execution_mode` field was intentionally removed before wide release; stale configs must migrate to `interaction_mode` rather than relying on an alias.

## Pi Extension Surfaces

The only current Pi extension artifact scaffolded by Doug is `.pi/extensions/handoff.ts`.

Its role is narrow:

- it is a Pi-side helper for handoff workflows
- it is installed at init time as a convenience
- Doug does not read `.pi/extensions/*` during `doug run`

Treat `.pi/extensions/` as optional Pi-native integration space, not as a Doug runtime input surface.

## Follow-Up Notes

- Pi activation path: use the resolved phase defaults (`planning: interactive`; `runtime`, `scaffold`, `research`, and `post_epic_kb`: `rpc`) or set `interaction_mode` explicitly in policy.
- If future Pi integration introduces additional extension files or extension-owned runtime artifacts, document each surface explicitly. Current `.pi/` scaffolding does not imply broader authority.
- For the full Doug-to-Pi interaction contract — policy inputs, workflow interaction semantics, and compatibility boundaries — see [Doug-to-Pi Runtime Contract](pi-runtime-contract.md).

## Related Topics

- [internal/config](../packages/config.md) for the `policy` model and `.doug/doug.yaml`
- [cmd/init](../packages/init.md) for generated config and provider scaffolding
- [internal/templates](../packages/templates.md) for `.pi/extensions/handoff.ts`
- [internal/agent](../packages/agent.md) for backend selection and Pi adapter behavior
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) for the full post-cutover interaction contract
