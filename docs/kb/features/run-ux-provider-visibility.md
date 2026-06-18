---
title: Run UX + Provider Stall Visibility
updated: 2026-06-17
category: Features
tags: [run, ux, heartbeat, provider-stall, metrics, pi, observability]
related_articles:
  - docs/kb/packages/agent.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/config.md
  - docs/kb/packages/metrics.md
  - docs/kb/packages/types.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/features/transport-failure-recovery.md
---

# Run UX + Provider Stall Visibility

## Overview

Doug makes long `doug run` attempts legible without changing the workflow outcome authority. Runtime observability comes from Pi JSONL transport events and is surfaced in terminal logs, run metadata sidecars, and task metrics. Agent outcomes still come only from the `## Agent Result` block in `.doug/ACTIVE_TASK.md`.

## Terminal UX During an Attempt

Each runtime attempt now exposes four operator-facing signals:

1. **Attempt header with description** — the section header is:
   ```text
   [EPIC-X-NNN] attempt N/M — <task description>
   ```
   Descriptions come from the already-loaded `tasks.yaml` entry and are truncated to 80 runes including `...`.
2. **Live heartbeat activity** — each heartbeat logs:
   ```text
   [TASK-ID] +<elapsed> — <activity>
   ```
   The activity label is a sanitized Pi event summary.
3. **First response callout** — when the first non-startup Pi JSONL event arrives:
   ```text
   ► first response (+<elapsed>)
   ```
4. **End-of-turn summary** — after the agent exits and before outcome logging:
   ```text
   agent finished in Xm Xs — first response +Xs, N tool calls, N provider failures
   ```

The summary is emitted for all valid parsed outcomes: `SUCCESS`, `FAILURE`, `BUG`, and `EPIC_COMPLETE`.

## First Response And Stall Warning

`internal/agent.PiAdapter` records the first non-startup Pi JSONL event with a `sync.Once`-style primitive and calls `RunRequest.FirstResponseFn(elapsed)` once. `RunResponse.FirstResponseMs` stores the elapsed milliseconds; zero means no non-startup event was observed.

`internal/orchestrator` uses this callback to print the first-response callout. Its heartbeat callback also emits a one-shot warning if no first response has arrived after `first_response_threshold` seconds:

```text
⚠ no provider response yet (+<elapsed>)
```

The threshold defaults to `90` seconds and is configured in `.doug/doug.yaml` with `first_response_threshold`. Set it to `0` to disable the warning. Heartbeat logging itself is controlled by `agent_heartbeat_seconds` (`0` disables heartbeat callbacks).

## Activity Label Rules

Pi JSONL reading updates a mutex-guarded activity tracker. Heartbeats read the latest sanitized label; this keeps activity formatting independent from first-response detection.

Label rules:

- Tool events show only tool name plus the first safe path/file/command-like string argument.
- Tool name and argument are whitespace-normalized and truncated to roughly 40 runes each.
- Multi-argument inputs, file contents, and other secret-bearing payloads are not logged.
- Text/content events set the label to `generating...`.
- Before any observed event, the label is `(no activity)`.

## Persisted Observability

`RunResponse` carries runtime-only observability fields:

- `FirstResponseMs int64`
- `ToolCallCount int`
- `ProviderFailures int`
- `ProviderFailureDetails []types.ProviderFailure`

`WriteRunMetadata` copies these fields into `<output log>.meta.json` next to the raw Pi output log.

The orchestrator copies provider observability into `LoopContext` before dispatching success/failure/bug handlers. Handlers pass it to `metrics.RecordTaskMetrics`, which persists:

- `provider_wait_ms`
- `provider_failures[]` with `type`, `message`, and `phase`

This data is diagnostic only. It must not be used as a replacement for the agent-owned workflow result.

## Implementation Surfaces

- `internal/agent/backend.go` defines callback and response fields.
- `internal/agent/pi_adapter.go` reads Pi JSONL, detects first response, counts tool calls, extracts provider failures, and tracks heartbeat activity.
- `internal/orchestrator/run.go` formats the attempt header, heartbeat lines, first-response callout, stall warning, and end-of-turn summary.
- `internal/config/config.go` defines `first_response_threshold` and heartbeat defaults/validation.
- `internal/types/types.go` defines `ProviderFailure` and metrics fields.
- `internal/metrics/metrics.go` records provider wait/failure diagnostics.

## Related Topics

- [internal/agent](../packages/agent.md) — Pi adapter request/response contract
- [internal/orchestrator](../packages/orchestrator.md) — run loop call order and logging points
- [internal/config](../packages/config.md) — heartbeat and first-response threshold settings
- [internal/metrics](../packages/metrics.md) — persisted task metrics
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) — outcome authority boundary
