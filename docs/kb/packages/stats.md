---
title: internal/stats — Per-run Stats Records
updated: 2026-06-17
category: Packages
tags: [stats, observability, pi, logs]
related_articles:
  - docs/kb/packages/agent.md
  - docs/kb/features/run-ux-provider-visibility.md
---

# internal/stats — Per-run Stats Records

`internal/stats` owns Doug's normalized per-run stats artifact.

## RunStats

`RunStats` is persisted as JSON and includes:

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
Runtime observability fields (`first_response_ms`, `tool_call_count`, and
`provider_failure_count`) are copied from `agent.RunResponse` and are not
re-derived from transcripts or stats files.

## Persistence

Runtime `doug run` writes stats after the Pi-backed turn ends, under:

```text
.doug/logs/stats/{epic_id}/stats-{task_id}_attempt-{N}.json
```

This tree is Doug-owned and intentionally separate from `.doug/logs/output/*.meta.json`
sidecars.
