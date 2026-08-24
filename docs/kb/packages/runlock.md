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

`internal/runlock` provides Doug's shared lock for lifecycle drivers. It prevents concurrent mutating lifecycle operations from `doug run` and interactive MCP tools.

The lock file is `.doug/run.lock`. It is an OS file lock, not a YAML state file and not a durable lifecycle record.

## Platform Implementations

`runlock.go` holds all portable logic and imports no syscall package. The two locking primitives, `lockFile` and `unlockFile`, are supplied per platform:

| File | Build tag | Primitive |
| --- | --- | --- |
| `runlock_unix.go` | `!windows` | `syscall.Flock` with `LOCK_EX\|LOCK_NB` |
| `runlock_windows.go` | `windows` | `windows.LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK\|LOCKFILE_FAIL_IMMEDIATELY` |

Two differences matter and are easy to get wrong:

**Non-blocking is opt-in on Windows.** `LOCKFILE_FAIL_IMMEDIATELY` is the analogue of `LOCK_NB`. Without it `LockFileEx` blocks until the holder releases, which would hang a lifecycle driver instead of returning `ErrHeld`.

**Windows locks are mandatory, not advisory.** A locked byte range on Windows is unreadable by every other handle — including `os.ReadFile` and git's indexer, which fails `git add -A` with `unable to index file '.doug/run.lock'`. Because `ReadMetadata` exists to read the file *while another process holds the lock*, the Windows implementation locks a single byte at offset 2^62, far past any real content. Locking beyond end-of-file is legal on Windows, so mutual exclusion holds while the metadata at the start of the file stays readable.

Any change to the locked range must keep it clear of the metadata bytes, and every doug process must agree on the same range.

## API

```go
const FileName = "run.lock"
var ErrHeld = errors.New("doug run lock is held")

func Path(dougDir string) string
func ReadMetadata(dougDir string) (Metadata, bool)
func HeldDetails(dougDir string) string
func TryAcquire(dougDir, owner string) (*Lock, error)
func (l *Lock) Path() string
func (l *Lock) Close() error
```

`TryAcquire` creates `.doug/` if needed, opens `.doug/run.lock`, and attempts a non-blocking exclusive lock. If another process holds the lock, it returns `ErrHeld`. Callers should fail fast or return a lock-held response rather than waiting indefinitely. Lock-held messages should include `owner`, `pid`, and `acquired_at` from `ReadMetadata`/`HeldDetails` when those fields are available.

On success, `TryAcquire` truncates the file and writes best-effort human-readable metadata:

```yaml
owner: <owner>
pid: <pid>
acquired_at: <RFC3339 UTC time>
```

`Close` unlocks and closes the file descriptor. Always defer it immediately after acquisition.

## Stale Lock Policy

A stale lock file does not permanently block Doug. Because locking is held by the OS against an open handle rather than by file content, the file can remain on disk after a crash; a later process can acquire it as soon as no process still holds the OS lock.

Do not implement separate timestamp-based stale-file deletion unless the lock semantics intentionally change.

## Current Callers

- `doug run` acquires the lock around headless runtime mutation.
- `internal/mcp.ToolHandler` acquires the lock for mutating tools: `get_next_task`, `reconcile_lifecycle`, `report_task_complete`, and `report_task_blocked`.
- `get_status` and `diagnose_lifecycle` do not acquire the lock because they are read-only and must not mutate lifecycle files.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [internal/mcp](mcp.md)
- [internal/orchestrator](orchestrator.md)
