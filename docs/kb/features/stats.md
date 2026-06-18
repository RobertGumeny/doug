---
title: doug stats — Local Run Statistics
updated: 2026-06-17
category: Features
tags: [stats, command, observability, pi, logs]
related_articles:
  - docs/kb/packages/stats.md
  - docs/kb/packages/agent.md
  - docs/kb/features/run-ux-provider-visibility.md
  - docs/kb/features/research.md
  - docs/kb/features/scaffold.md
---

# doug stats — Local Run Statistics

## Overview

`doug stats` is Doug's local read surface for per-run execution statistics. It reads only Doug-owned JSON records under `.doug/logs/stats/`; the command does not call Pi or depend on retained Pi session state at read time.

The stats feature is write-time integrated with Pi-backed runtime, planning, research, and scaffold runs. Pi's `get_session_stats` RPC supplies token and cost data when a run finishes, and Doug persists that data with the run's phase and observability metrics.

## Usage

```bash
doug stats
doug stats EPIC-46
```

- With no argument, the command scans every stats bucket under `.doug/logs/stats/`.
- With an epic ID argument, it reads only `.doug/logs/stats/{epic_id}/`.
- Missing stats directories produce an empty-result message rather than an error.

The output table includes:

| Column | Meaning |
|--------|---------|
| `EPIC` | Stats bucket directory, usually an epic ID |
| `PHASE` | `runtime`, `planning`, `research`, or `scaffold` |
| `TASK` | Doug task ID (`PLAN`, `RESEARCH`, `SCAFFOLD`, or runtime task ID) |
| `RUNS` | Number of persisted records aggregated into the row |
| `COST` | Sum of recorded USD cost |
| `INPUT`, `OUTPUT`, `CACHE` | Sum of token counts; cache combines cache read and write tokens |
| `DURATION` | Sum of run durations |
| `FIRST_RESPONSE` | Average first-response latency for rows that recorded a positive value |

The final `TOTAL` row sums run count, cost, token counts, and duration across the displayed rows. Its first-response value is the average of row averages that have a positive first-response measurement.

## Persistence Contract

Stats records live in a dedicated Doug-owned tree:

```text
.doug/logs/stats/{bucket}/stats-{task_id}_attempt-{N}.json
```

Runtime and scaffold runs use the current epic ID as the bucket. Planning can use the optional `doug plan --epic` hint as the bucket; if no epic hint is provided, the shared helper falls back to the phase name. Research uses the `research` bucket. Non-runtime phase buckets allow `doug stats` to include one-shot command runs even when they are outside the runtime task state machine.

Each record is written at session end using `internal/stats.WriteRunStats`, which performs an atomic write via the state package. Write failures are logged as warnings and do not replace the authoritative workflow result in `.doug/ACTIVE_TASK.md`.

## Data Sources

Stats records are built from `agent.RunResponse` by `internal/stats.FromRunResponse`:

- token counts and cost come from Pi `get_session_stats`, requested by the Pi adapter at write time
- `first_response_ms`, `tool_call_count`, and `provider_failure_count` are copied from Doug's `RunResponse` observability fields
- the stats reader never re-derives values from transcripts, output logs, `.meta.json` sidecars, or live Pi RPC

Records without `phase` are treated as `runtime` during summary loading for compatibility with early stats artifacts.

## Related Topics

- [internal/stats](../packages/stats.md) — schema, writer, loader, aggregation behavior
- [internal/agent](../packages/agent.md) — `RunResponse` and Pi `get_session_stats` source fields
- [Run UX + Provider Stall Visibility](run-ux-provider-visibility.md) — first-response/tool/provider observability
