---
title: Transport Failure Recovery
updated: 2026-06-17
category: Features
tags: [pi, rpc, transport-failure, retries, infra-retries, attempt-start]
related_articles:
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/config.md
  - docs/kb/packages/types.md
---

# Transport Failure Recovery

## Overview

Doug separates Pi/provider transport failures from task workflow outcomes. A transport failure means Doug could not reliably reach an agent-owned `## Agent Result` in `.doug/ACTIVE_TASK.md`; it is not the same as an agent-reported `FAILURE`, `BUG`, `SUCCESS`, or `EPIC_COMPLETE`.

The recovery path is:

1. `internal/agent` classifies the Pi RPC launch as `RunStatusTransportFailure`.
2. `internal/orchestrator` handles that status before parsing `.doug/ACTIVE_TASK.md`.
3. Doug restores the task attempt counter, increments `active_task.infra_retries`, writes an infra-failure record, backs off, and retries until the infra cap is reached.
4. At the cap, Doug writes `.doug/ACTIVE_FAILURE.md` and halts without consuming a task attempt for the transport failures.

## Transport Classification

`RunResponse.Status` is transport metadata only. `RunStatusTransportFailure` is set when Pi RPC cannot complete cleanly before an authoritative workflow result is available, including:

- stdout closes before the startup response, prompt response, or `agent_end`
- scanner/read errors on Pi RPC stdout
- non-zero Pi exit plus known transport/provider patterns in stderr, such as websocket failures, broken pipes, connection resets/refusals, provider 5xx errors, or explicit `transport_failure` text

Context cancellation remains `RunStatusCancelled`, not a transport retry.

## Retry Semantics

Transport retries are counted separately from task attempts:

- `active_task.attempts` is incremented before launch for crash safety, then decremented when a transport failure is classified.
- `active_task.infra_retries` increments for each transport failure and is persisted in `project-state.yaml`.
- `max_infra_retries` controls the cap; the default is `3` and the value must be positive.
- Backoff is exponential from 1 second and capped at 30 seconds.
- When a later backend run is not a transport failure, Doug clears `infra_retries` before parsing the task result.

Doug must check `RunStatusTransportFailure` before `ParseSessionResult`; a broken Pi transport can leave `.doug/ACTIVE_TASK.md` without a valid result block.

## Durable Records

Every transport failure writes a Markdown record under the attempt forensic directory:

```text
.doug/logs/epics/{epic}/{taskID}/attempt-{taskAttempt}/infra-failure-{N}.md
.doug/logs/epics/{epic}/{taskID}/attempt-{taskAttempt}/infra-failure.md
```

`N` is the infra retry number, not the task attempt number. The record frontmatter includes:

- `task_id`
- `attempt`
- `failed_at`
- `class`
- `backend_status`
- `error`
- `exit_code`

Intermediate records use class `transport_failure`; the final cap-hit record uses `transport_failure_retry_cap`. Records do not include `output_log` because Doug no longer creates default raw output mirrors.

## Attempt-Start Marker

Before invoking the backend, Doug writes:

```text
.doug/logs/epics/{epic}/{taskID}/attempt-{N}/attempt-start.json
```

The marker contains:

```json
{
  "started_at": "2026-06-17T00:00:00Z",
  "attempt": 1,
  "task_id": "TASK-ID"
}
```

The marker is written atomically and shares the retained Pi session directory layout. Its presence with no corresponding completed `ACTIVE_TASK.md` result proves Doug started an attempt but the agent workflow did not complete.

## Operator Guidance

When diagnosing a suspected transport failure:

1. Check `.doug/logs/epics/{epic}/{taskID}/attempt-{N}/` for infra-failure records.
2. Check `attempt-start.json` in that directory to confirm launch start.
3. Inspect retained Pi-native transcripts in the same attempt directory when present.
4. If `.doug/ACTIVE_FAILURE.md` exists, the infra retry cap was reached and the run halted for operator intervention.

Do not treat transport retry records as task failures. The task attempt budget is intentionally preserved for agent-visible workflow attempts.

## Related Topics

- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md)
- [internal/agent](../packages/agent.md)
- [internal/orchestrator](../packages/orchestrator.md)
- [internal/config](../packages/config.md)
- [internal/types](../packages/types.md)
