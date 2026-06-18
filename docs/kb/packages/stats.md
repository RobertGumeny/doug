---
title: internal/stats — Per-run Stats Records
updated: 2026-06-17
category: Packages
tags: [stats, observability, pi, logs]
related_articles:
  - docs/kb/features/stats.md
  - docs/kb/packages/agent.md
  - docs/kb/features/run-ux-provider-visibility.md
---

# internal/stats — Per-run Stats Records

`internal/stats` owns Doug's normalized per-run stats artifact, the atomic writer for `.doug/logs/stats/`, and the summary loader used by `doug stats`.

## RunStats

`RunStats` is persisted as JSON and includes:

- `phase` (`runtime`, `planning`, `research`, or `scaffold`)
- `task_id`
- `attempt`
- `session_id`
- `input_tokens`, `output_tokens`, `cache_tokens`
- `cost_usd`
- `first_response_ms`
- `tool_call_count`
- `provider_failure_count`
- `duration_ms`
- `completed_at`

Token and cost fields come from Pi's `get_session_stats` RPC at write time.
Observability fields (`first_response_ms`, `tool_call_count`, and
`provider_failure_count`) are copied from `agent.RunResponse` and are not
re-derived from transcripts or stats files. `cache_tokens` is the sum of Pi cache-read and cache-write token counts.

`FromRunResponse(phase, taskID, attempt, completedAt, resp)` is the canonical constructor. If `resp.SessionStats` has a session ID and `resp.SessionID` is empty, the record uses the session stats ID.

## Persistence

`doug run`, `doug plan`, `doug research`, and `doug scaffold` write stats after their Pi-backed session ends. Runtime records are grouped by epic; non-runtime records use their target epic when one exists and otherwise fall back to a phase-named bucket (`planning`, `research`, or `scaffold`).

Stats are written under:

```text
.doug/logs/stats/{bucket}/stats-{task_id}_attempt-{N}.json
```

`WriteRunStats(logsDir, epicID, record)` creates the bucket directory and writes JSON atomically. Empty runtime bucket input falls back to `runtime`; callers for non-runtime commands should pass a target epic or phase bucket.

This tree is Doug-owned and intentionally separate from `.doug/logs/output/*.meta.json` sidecars. Stats write errors are non-fatal at call sites: Doug logs a warning and continues to rely on `.doug/ACTIVE_TASK.md` for workflow outcome authority.

## Summary Loading

`LoadSummary(logsDir, epicID)` powers `doug stats`. It reads JSON records from `.doug/logs/stats/`, optionally restricted to a single bucket, and returns per-task rows plus totals.

Aggregation keys include bucket, phase, and task ID. For each row, input/output/cache tokens, cost, duration, and run count are summed. First-response latency is averaged across records with a positive value. Summary rows are sorted by bucket, then phase, then task ID.

Records missing `phase` load as `runtime` for compatibility with early stats files.

## Related Topics

- [doug stats](../features/stats.md) — CLI behavior and operator-facing output
- [internal/agent](agent.md) — `RunResponse`, Pi session stats, and provider observability

