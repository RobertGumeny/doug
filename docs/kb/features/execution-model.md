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
- Source-owned execution routing determines the Pi interaction mode by phase
- Pi-owned runtime selection chooses the underlying provider, model, and tool configuration after Doug hands the run to Pi

The user-facing source of truth is intentional:

- built-in code constants own Doug's prompt text and execution routing
- `.doug/doug.yaml` owns boring project configuration (build system, retry limits, lint settings)
- Pi owns downstream provider/model/tool selection once a Pi-backed `interaction_mode` (`interactive` or `rpc`) is active

## The Supported Model

### 1. Doug owns the workflow prompt

Doug builds the workflow prompt in code for each phase:

- runtime and scaffold use the canonical runtime prompt
- planning uses the planning prompt
- research uses the research prompt

These prompts are emitted by `config.BuildInitialPrompt(...)`, not read from `.doug/doug.yaml`. That keeps the Doug-managed run contract authoritative in the binary instead of in provider-specific CLI templates.

### 2. Source code owns backend routing by phase

Backend selection and Pi mode are controlled by Doug source, not task type and not `.doug/doug.yaml`.

- `planning` launches true terminal-interactive Pi.
- `runtime`, `scaffold`, `research`, and `post_epic_kb` launch Pi RPC one-shot execution.
- Unknown internal phases fail with a Doug error instead of falling back to another mode.

`NewBackend()` always returns a `PiAdapter` for production runtime dispatch. Interaction mode is determined by `config.DefaultInteractionModeForPhase(phase)` in `agent.PrepareExecution`.

### 3. Pi owns provider selection after the handoff

Once Doug routes a run through Pi:

- Doug does not specify provider, model, or temperature in the RPC payload
- Doug does not choose a provider CLI directly
- Pi launches `pi --mode rpc` and manages the downstream agent process lifecycle

This is the key ownership boundary: Doug chooses workflow semantics and execution policy; Pi chooses how that work is executed against the underlying agent stack.

### 4. `doug init` scaffolds Pi files

`doug init` scaffolds `.pi/extensions/handoff.ts` and `.pi/skills/**`. Provider-specific directories (`.claude/`, `.codex/`, `.gemini/`) are no longer installed.

Init generates a minimal `.doug/doug.yaml` with project settings (build system, retry limits, KB toggle) but no `policy:` block. Execution routing is source-owned and does not need to be written to config.

Doug's runtime authority comes from:

- `.doug/ACTIVE_TASK.md`
- the Doug-owned workflow prompt built in code
- the resolved policy/backend contract

Provider-local files do not replace Doug-owned briefing, result parsing, or lifecycle artifacts.

## Remaining Compatibility Surfaces

The repository has moved to a Pi-first model, but a few compatibility surfaces remain intentionally available:

- `interaction_mode: subprocess` is no longer a supported transport. It is rejected if found in source code but silently ignored if present in old `policy:` YAML (since the policy block itself is now ignored).
- `tool_policy` and `session_defaults` fields in `RunRequest.Policy` remain available as empty-string defaults for future Pi adapter extension but are not set from config.
- `doug plan` and `doug research` use workflow-specific interaction contracts rather than fully sharing the runtime retry/state-machine behavior.
- Root `.doug/PRD.md` plus `.doug/tasks.yaml` remains a supported manual runtime workspace even though `.doug/plan/` plus backlog promotion is the newer structured planning path.

These surfaces should be documented explicitly so the repository does not imply a cleaner cutover than the code currently implements.

## Repository Authoring Rules

EPIC-35 and EPIC-41 established the repository-facing rule for new docs, prompts, examples, and managed artifacts:

- describe source-owned Pi routing (`interactive` for planning; `rpc` for runtime/scaffold/research/post-epic KB) as the Doug interaction model
- do not describe `interaction_mode: subprocess` as compatibility or fallback behavior
- describe Doug-owned prompts as built-in command text rather than operator-edited provider launch templates
- keep managed init artifacts aligned with the supported Pi-first scaffold; do not reintroduce dormant `.claude/`, `.codex/`, or `.gemini/` examples or template baggage
- do not add `policy:` blocks to generated config; describe execution routing as source-owned, not operator-configured

When documentation needs to mention transitional behavior, name the exact surviving surface instead of implying Doug still chooses providers directly.

## Pi Extension Surfaces

The only current Pi extension artifact scaffolded by Doug is `.pi/extensions/handoff.ts`.

Its role is narrow:

- it is a Pi-side helper for handoff workflows
- it is installed at init time as a convenience
- Doug does not read `.pi/extensions/*` during `doug run`

Treat `.pi/extensions/` as optional Pi-native integration space, not as a Doug runtime input surface.

## Follow-Up Notes

- Pi activation path: resolved phase defaults are source-owned (`planning: interactive`; `runtime`, `scaffold`, `research`, and `post_epic_kb`: `rpc`). These cannot be overridden from `.doug/doug.yaml`.
- If future Pi integration introduces additional extension files or extension-owned runtime artifacts, document each surface explicitly. Current `.pi/` scaffolding does not imply broader authority.
- For the full Doug-to-Pi interaction contract — policy inputs, workflow interaction semantics, and compatibility boundaries — see [Doug-to-Pi Runtime Contract](pi-runtime-contract.md).

## Related Topics

- [internal/config](../packages/config.md) for the `policy` model and `.doug/doug.yaml`
- [cmd/init](../packages/init.md) for generated config and provider scaffolding
- [internal/templates](../packages/templates.md) for `.pi/extensions/handoff.ts`
- [internal/agent](../packages/agent.md) for backend selection and Pi adapter behavior
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) for the full post-cutover interaction contract
