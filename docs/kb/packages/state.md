---
title: internal/state — State File I/O
updated: 2026-03-14
category: Packages
tags: [state, yaml, atomic-write, error-handling, io, parse-error]
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
type ParseError struct {
    Path   string
    Err    error
    Fields []string // field-level messages from yaml.TypeError when available
    Hint   string   // formatting/recovery hint appended to parse errors
}
```

## Error Handling

Two distinct error kinds — use `errors.Is` / `errors.As` to distinguish them:

```go
tasks, err := state.LoadTasks("tasks.yaml")
if errors.Is(err, state.ErrNotFound) {
    // File missing — first-time setup or run `doug init`
}
var parseErr *state.ParseError
if errors.As(err, &parseErr) {
    // Malformed YAML — parseErr.Path has the filename
    // parseErr.Fields lists offending fields when available
    // parseErr.Hint has a formatting/recovery hint for the user
    log.Fatal("corrupt state file: %s: %v", parseErr.Path, parseErr.Err)
}
```

**`ErrNotFound`** — returned when `os.ErrNotExist` is true. Signal that state doesn't exist yet; not an error per se.

**`*ParseError`** — returned on YAML unmarshal failure. Contains:
- `Path` — the file path, for actionable error messages
- `Err` — the underlying YAML error
- `Fields` — populated from `*yaml.TypeError` when available; lists the parser's field-level type mismatch messages. Empty for non-type errors.
- `Hint` — a formatting/recovery hint appended to project-state.yaml and tasks.yaml parse errors.

`ParseError` implements `Unwrap()` so `errors.Is` can match the underlying cause. `Fields` and `Hint` are populated for both project-state.yaml and tasks.yaml parse errors.

### Improved tasks.yaml Error Messages

`LoadTasks` uses `newTasksParseError` to produce richer diagnostics:

```
parse error in tasks.yaml: yaml: line 5: cannot unmarshal !!str 'foo' into []string
  Fields: epic.tasks[0].steps
  Hint: Check that list fields use YAML sequence syntax (- item) and string fields are not sequences.
```

The fields list comes from a direct type assertion against `*yaml.TypeError` (which has no `Unwrap`). The hint is a package-level constant for each file type appended regardless of error kind.

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

**Same-directory `.tmp`**: The temp file is always `path + ".tmp"` (same directory as the target). `os.Rename` is only atomic on the same filesystem.

**`ErrNotFound` as a sentinel**: Using a package-level `errors.New` value lets callers use `errors.Is` without importing `os`.

**`*ParseError` as a named struct**: `errors.As` extracts it so callers can log the file path. `Unwrap()` implemented so the underlying YAML error is accessible.

**`Fields` via direct type assertion**: `yaml.TypeError` has no `Unwrap` method, so `errors.As` cannot reach it. A direct `err.(*yaml.TypeError)` assertion is used when constructing parse errors.

**`Hint` as a constant per file type**: Appended to project-state.yaml and tasks.yaml parse errors regardless of error kind. Keeps the implementation simple — no per-error-kind branching.

**No retry on parse error**: A corrupted state file is a Tier 3 condition. Return immediately; callers log and exit.

## Edge Cases & Gotchas

**Leftover `.tmp` files**: If the process is killed after writing `.tmp` but before `Rename`, a stale `.tmp` file remains. Not yet cleaned up automatically — flag if you encounter one.

**`SaveTasks` does not unset `UserDefined`**: The field has `yaml:"-"` so it is never written, but tasks retain their in-memory `UserDefined = true` after a save.

**`SaveProjectState` overwrites on every call**: No dirty-tracking. Load and save without mutating rewrites the file identically. Intentional — simplicity over optimization.

## Related Topics

- [Types](types.md) — structs and constants used by this package
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — pattern detail and rationale
