---
title: internal/metrics — Task Metric Recording & Epic Summary
updated: 2026-02-24
category: Packages
tags: [metrics, telemetry, summary, epic, duration]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/state.md
---

# internal/metrics — Task Metric Recording & Epic Summary

## Overview

`internal/metrics` records per-task telemetry into `state.Metrics` and prints a formatted epic summary at completion. All functions operate in-memory; callers must `SaveProjectState` to persist. Metric errors are **non-fatal by design** — log as a warning and continue.

## API

```go
func RecordTaskMetrics(state *types.ProjectState, taskID string, outcome string, durationSeconds int)
func UpdateMetricTotals(state *types.ProjectState)
func PrintEpicSummary(state *types.ProjectState)
```

## RecordTaskMetrics

Appends a `types.TaskMetric` (with RFC3339 UTC timestamp) to `state.Metrics.Tasks`, then calls `UpdateMetricTotals` to refresh totals.

```go
// Note: outcome is a plain string — matches types.TaskMetric.Outcome (string, not Outcome type)
metrics.RecordTaskMetrics(state, task.ID, string(result.Outcome), durationSeconds)
```

**No error return**: struct append cannot fail. The "non-fatal" guidance applies to the caller — if anything downstream errors, log a warning rather than failing the whole task.

## UpdateMetricTotals

Recalculates `TotalTasksCompleted` and `TotalDurationSeconds` from scratch by iterating the full `Tasks` slice. Overwrites any stale previously stored totals. Safe to call multiple times (idempotent).

```go
// Called automatically by RecordTaskMetrics, but can be called standalone
// if you need to repair stale totals after state manipulation.
metrics.UpdateMetricTotals(state)
```

## PrintEpicSummary

Prints a box-draw table to stdout. Safe to call with zero tasks (no divide-by-zero). Average time is integer division (`totalSec / total`).

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EPIC SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Total Tasks:           5
  Total Time:            12m 30s
  Average Time:          150s per task
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Duration format: `0s` / `45s` / `3m 15s` / `1h 2m 30s`. Zero/negative seconds renders as `"0s"` (never empty string).

## Key Decisions

**`TaskMetric.Outcome` is `string`, not `types.Outcome`**: matches the YAML schema and avoids circular dependency. Pass `string(result.Outcome)` at the call site; do not change this field's type.

**Recalculate-from-scratch totals**: `UpdateMetricTotals` never increments — it sums the full slice every time. This keeps it idempotent and correct if called after state repair.

**`RecordTaskMetrics` has no error return**: The caller is expected to handle errors at its own level (e.g., save failures), not from metric recording.

## Related

- [types.md](./types.md) — `TaskMetric` struct and `Metrics` aggregate
- [state.md](./state.md) — `SaveProjectState` to persist after recording
