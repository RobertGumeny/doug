---
title: internal/state — State File I/O
updated: 2026-02-24
category: Packages
tags: [state, yaml, atomic-write, error-handling, io]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
  - docs/kb/infrastructure/go.md
---

# internal/state — State File I/O

## Overview

`internal/state` provides the four load/save functions for the two orchestrator state files. All writes are atomic (write to `.tmp`, then `os.Rename`). Load functions return typed errors that callers can distinguish with `errors.Is` and `errors.As`.

## API

```go
// project-state.yaml
func LoadProjectState(path string) (*types.ProjectState, error)
func SaveProjectState(path string, state *types.ProjectState) error

// tasks.yaml
func LoadTasks(path string) (*types.Tasks, error)
func SaveTasks(path string, tasks *types.Tasks) error

// Sentinel errors
var ErrNotFound = errors.New("state file not found")
type ParseError struct { Path string; Err error }
```

## Error Handling

Two distinct error kinds — use `errors.Is` / `errors.As` to distinguish them:

```go
tasks, err := state.LoadTasks("tasks.yaml")
if errors.Is(err, state.ErrNotFound) {
    // File missing — first-time setup
}
var parseErr *state.ParseError
if errors.As(err, &parseErr) {
    // Malformed YAML — parseErr.Path has the filename
    log.Fatal("corrupt state file: %s: %v", parseErr.Path, parseErr.Err)
}
```

**`ErrNotFound`** — returned when `os.ErrNotExist` is true. Use this to detect first-run or missing state, not as an error condition.

**`*ParseError`** — returned on YAML unmarshal failure. Contains the file path so error messages are actionable. Implements `Unwrap()` so `errors.Is` can match the underlying cause if needed.

## UserDefined Flag

`LoadTasks` sets `UserDefined = true` on every `Task` it reads:

```go
for i := range tasks.Epic.Tasks {
    tasks.Epic.Tasks[i].UserDefined = true
}
```

This establishes the UserDefined vs Synthetic distinction at the type level. **Never set `UserDefined` manually** — rely on `LoadTasks` to set it.

## Atomic Write Implementation

```go
func atomicWrite(path string, data []byte) error {
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("write temp file %s: %w", tmp, err)
    }
    if err := os.Rename(tmp, path); err != nil {
        _ = os.Remove(tmp) // best-effort cleanup
        return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
    }
    return nil
}
```

`os.Remove(tmp)` on rename failure is best-effort — the error is intentionally discarded so the rename error is what the caller sees.

## Usage Pattern

```go
// Load once
state, err := state.LoadProjectState("project-state.yaml")
if err != nil { ... }

// Mutate in memory — all changes before the save
state.ActiveTask.Attempts++
state.ActiveTask.Type = types.TaskTypeBugfix

// Save once, atomically
if err := state.SaveProjectState("project-state.yaml", state); err != nil { ... }
```

Never call Save more than once to accumulate changes. Load → mutate → save is a single operation.

## Key Decisions

**Same-directory `.tmp`**: The temp file is always `path + ".tmp"` (same directory as the target). `os.Rename` is only atomic on the same filesystem. Putting temp files in `os.TempDir()` risks crossing filesystem boundaries.

**`ErrNotFound` as a sentinel**: Using a package-level `errors.New` value lets callers use `errors.Is` without importing `os`. It is not an error — it's a signal that the state file doesn't exist yet.

**`*ParseError` as a named struct**: `errors.As` extracts it so callers can log the file path. `Unwrap()` is implemented so the underlying YAML error is accessible via `errors.Is` if needed.

**No retry on parse error**: A corrupted state file is a Tier 3 condition (ambiguous, manual intervention required). `LoadProjectState` and `LoadTasks` return the error immediately; callers are expected to log and exit.

## Edge Cases & Gotchas

**Leftover `.tmp` files**: If the process is killed after writing `.tmp` but before `Rename`, a stale `.tmp` file remains. The orchestrator startup should detect and clean these. Not yet implemented — flag if you encounter one.

**`SaveTasks` does not unset `UserDefined`**: The field has `yaml:"-"` so it is never written, but tasks retain their in-memory `UserDefined = true` after a save. This is fine — it matches reality.

**`SaveProjectState` overwrites on every call**: There is no dirty-tracking. If you load and save without mutating, the file is rewritten identically. This is intentional — simplicity over optimization.

## Related Topics

- [Types](types.md) — structs and constants used by this package
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — pattern detail and rationale
