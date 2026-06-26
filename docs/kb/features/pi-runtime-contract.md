---
title: Doug-to-Pi Runtime Contract
updated: 2026-06-18
category: Features
tags: [pi, rpc, execution, contract, policy, backend]
related_articles:
  - docs/kb/features/execution-model.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
  - docs/kb/features/transport-failure-recovery.md
---

# Doug-to-Pi Runtime Contract

## Overview

Pi is Doug's required execution boundary.

Phase routing is fixed in source:

- `doug plan` uses true interactive Pi
- runtime, scaffold, research, post-epic review, and post-epic KB use Pi RPC one-shot execution

## Ownership Boundary

**Doug owns:**

- orchestration, retries, lifecycle, and exit behavior
- writing `.doug/ACTIVE_TASK.md`
- resolving the workflow skill, phase, prompt, and artifact surfaces
- reading `## Agent Result` as the only authoritative workflow outcome
- session archival under `.doug/logs/`

**Pi owns:**

- launching and supervising the downstream agent process
- session management for the Pi run
- enforcing write restrictions where supported
- provider/model/tool implementation details behind the Pi boundary

## What Doug Sends To Pi

Doug assembles a `RunRequest` and the Pi adapter translates it into Pi's private launch/RPC format. The Doug-native request includes:

- task context (`Task.ID`, `Task.Type`, attempt, epic scope)
- canonical brief path (`.doug/ACTIVE_TASK.md`)
- ordered context artifacts; for post-epic review this includes review evidence and the target `.doug/logs/reviews/{epic}/` artifact, and for post-epic KB this includes optional `.doug/plan/PLAN.md` so documentation/changelog synthesis can use planning rationale, scope decisions, and non-goals
- artifact read/write surfaces
- routing data (`workflow`, `skill`, `interaction_mode`)
- read/write restrictions
- the built-in Doug prompt text

The important operator-facing rule is that these inputs are resolved by Doug source code before the Pi run starts.

## Runtime Flow

For runtime, scaffold, research, post-epic review, and post-epic KB phases, one Doug task iteration maps to one supervised Pi RPC run:

1. Doug writes `.doug/ACTIVE_TASK.md`; every generated brief includes a concise `Doug Lifecycle` context section describing `planning → handoff → runtime tasks → post_epic_review → post_epic_kb`
2. Doug launches Pi in RPC mode under the Doug-scoped session directory
3. Doug sends the Doug-owned prompt and artifact contract
4. Pi runs the agent and manages the downstream lifecycle
5. Doug reads `## Agent Result` from `.doug/ACTIVE_TASK.md`
6. Doug archives logs and updates workflow state

Before invoking Pi, Doug also writes an `attempt-start.json` marker in the retained Pi session directory. If Pi transport fails before an `Agent Result` is available, Doug classifies the backend status as `transport_failure`, records the infra failure, and retries through the infra-retry path rather than treating it as an agent workflow failure.

For planning, Doug launches a visible interactive Pi session instead of the RPC one-shot path.

## Boundaries

### Doug must not

- treat Pi transport events as workflow outcomes
- treat `.pi/` files as live Doug runtime inputs during `doug run`
- blur the Doug/Pi ownership boundary

### Pi must not

- override Doug-owned artifact authority
- write outside the exposed writable surfaces
- return workflow outcomes as transport metadata instead of through `ACTIVE_TASK.md`

## Historical Note

Older documentation discussed more execution-policy and compatibility detail. That material should be treated as historical transition context, not as the current public contract.

Legacy routing fields, provider command templates, and subprocess-backend narratives are no longer part of Doug's supported interface. When those terms still appear, they should be confined to changelog/history material or explicit upgrade-retirement guidance.

## Related Topics

- [Interaction Model And Pi Policy Ownership](execution-model.md) — Doug-owned prompts, Pi-only routing, and phase-based Pi activation
- [internal/agent](../packages/agent.md) — `Backend` interface, `PiAdapter`, `RunRequest`, and the full agent lifecycle
- [Transport Failure Recovery](transport-failure-recovery.md) — how Doug separates Pi/provider transport errors from agent workflow outcomes
- [internal/config](../packages/config.md) — source-owned interaction-mode defaults and config loading
- [doug upgrade](upgrade.md) — retirement and cleanup path for legacy execution config/artifacts
