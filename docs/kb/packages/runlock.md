---
title: internal/runlock — Shared Lifecycle Lock
updated: 2026-07-03
category: Packages
tags: [locking, lifecycle, run, mcp, concurrency]
related_articles:
  - docs/kb/features/interactive-implement.md
  - docs/kb/packages/mcp.md
  - docs/kb/packages/orchestrator.md
---

# internal/runlock — Shared Lifecycle Lock

## Overview

`internal/runlock` provides Doug's shared advisory lock for lifecycle drivers. It prevents concurrent mutating lifecycle operations from `doug run` and interactive MCP tools.

The lock file is `.doug/run.lock`. It is an OS advisory `flock`, not a YAML state file and not a durable lifecycle record.

## API

```go
const FileName = "run.lock"
var ErrHeld = errors.New("doug run lock is held")

func Path(dougDir string) string
func TryAcquire(dougDir, owner string) (*Lock, error)
func (l *Lock) Path() string
func (l *Lock) Close() error
```

`TryAcquire` creates `.doug/` if needed, opens `.doug/run.lock`, and attempts a non-blocking exclusive lock. If another process holds the lock, it returns `ErrHeld`. Callers should fail fast or return a lock-held response rather than waiting indefinitely.

On success, `TryAcquire` truncates the file and writes best-effort human-readable metadata:

```yaml
owner: <owner>
pid: <pid>
acquired_at: <RFC3339 UTC time>
```

`Close` unlocks and closes the file descriptor. Always defer it immediately after acquisition.

## Stale Lock Policy

A stale lock file does not permanently block Doug. Because locking is descriptor-based, the file can remain on disk after a crash; a later process can acquire it as soon as no process still holds the OS lock.

Do not implement separate timestamp-based stale-file deletion unless the lock semantics intentionally change.

## Current Callers

- `doug run` acquires the lock around headless runtime mutation.
- `internal/mcp.ToolHandler` acquires the lock for mutating tools: `get_next_task`, `report_task_complete`, and `report_task_blocked`.
- `get_status` does not acquire the lock because it is read-only and must not mutate lifecycle files.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [internal/mcp](mcp.md)
- [internal/orchestrator](orchestrator.md)
