---
title: internal/status — TTY-Gated Live Status Indicator
updated: 2026-07-05
category: Packages
tags: [status, tui, tty, heartbeat, terminal, pi]
related_articles:
  - docs/kb/features/run-ux-provider-visibility.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/log.md
---

# internal/status — TTY-Gated Live Status Indicator

## Purpose

`internal/status` owns Doug's shared live status primitive for long Pi-backed waits. It gives real terminals one managed in-flight status line while preserving ordinary line-oriented heartbeat logs for non-TTY output, CI, and log files.

## Key Facts

- `New(status.Options)` returns an `*Indicator` configured with a task ID, display delay, writer, TTY flag, optional logger, interrupt hint, and fallback waiting text.
- `Heartbeat(elapsed, activity)` is the single progress callback used by runtime, post-epic, scaffold, and research Pi-backed calls.
- `FormatAgentEndSummary(duration, firstResponseMs, toolCallCount, providerFailures)` renders the shared completion line for Pi-backed turns.
- On TTY output, heartbeats before `Delay` are ignored; later heartbeats render one carriage-return status line to the configured writer.
- On non-TTY output, each heartbeat becomes a durable logger line: `[TASK-ID] +<elapsed> — <activity>`.
- `Finish()` clears a visible TTY status line before normal Doug logs resume. It is best-effort terminal output and must not affect workflow control flow.
- A nil writer disables TTY rendering; a nil logger disables non-TTY heartbeat logs.
- `SanitizeActivity(activity, fallback)` strips ANSI control sequences, normalizes control whitespace, removes non-printable controls, falls back when empty, and bounds labels to one line.

## Runtime Contract

Callers should create the indicator before invoking a Pi-backed backend run, route `RunRequest.HeartbeatFn` to `Indicator.Heartbeat`, and call `Finish()` after the backend returns before writing subsequent terminal logs.

Current callers include:

- Runtime task execution in `internal/orchestrator/run.go`.
- Automatic post-epic review in `internal/orchestrator/post_epic_review.go`.
- Automatic post-epic KB/changelog synthesis in `internal/orchestrator/post_epic_kb.go`.
- Manifest-driven scaffold runs in `cmd/scaffold.go`.
- Read-only research runs in `cmd/research.go`.

The indicator and end-summary formatter are output decorators only. They do not replace Pi outcome parsing, first-response tracking, provider metrics, retry handling, or post-epic finalization rules.

## Common Pitfalls

- **Do not emit separate per-heartbeat log lines on TTY while the indicator is active** — the indicator owns the live line after its delay.
- **Do not pass raw provider text directly to terminal output elsewhere** — use the heartbeat activity supplied by the agent layer or `SanitizeActivity` before rendering.
- **Do not make status rendering fatal** — writer failures are intentionally ignored, matching Doug's best-effort terminal-output policy.
- **Keep activities short and safe** — labels are for operator liveness, not transcript logging or raw tool/provider payloads.

## Related

- [Run UX + Provider Stall Visibility](../features/run-ux-provider-visibility.md) — cross-cutting runtime UX contract
- [internal/agent](agent.md) — Pi backend callbacks and activity labels
- [internal/orchestrator](orchestrator.md) — runtime and post-epic call sites
- [internal/log](log.md) — non-TTY heartbeat fallback logging
