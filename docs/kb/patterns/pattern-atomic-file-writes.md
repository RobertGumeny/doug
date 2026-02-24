---
title: Atomic File Writes
updated: 2026-02-23
category: Patterns
tags: [files, state, safety, yaml]
related_articles:
  - docs/kb/infrastructure/go.md
---

# Atomic File Writes

## Overview

All state file writes in doug use a write-to-temp-then-rename pattern. This guarantees that a file is never left in a partially-written state if the process is killed mid-write — the destination file is either the old version or the new version, never something in between.

This applies to: `project-state.yaml`, `tasks.yaml`, session files, `CHANGELOG.md`.

## Implementation

```go
func writeFileAtomic(path string, data []byte) error {
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0644); err != nil {
        return fmt.Errorf("writing temp file %s: %w", tmp, err)
    }
    if err := os.Rename(tmp, path); err != nil {
        return fmt.Errorf("renaming %s to %s: %w", tmp, path, err)
    }
    return nil
}
```

`os.Rename` is atomic on all platforms doug targets (Linux, macOS, Windows) when the source and destination are on the same filesystem — which they always are here since the `.tmp` file sits next to the target.

## Usage Pattern

The canonical flow for any state mutation:

```go
// 1. Load once
state, err := state.LoadProjectState(path)
if err != nil { ... }

// 2. Mutate in memory — as many changes as needed
state.ActiveTask.Attempts++
state.ActiveTask.Type = types.TaskTypeBugfix

// 3. Write once, atomically
if err := state.SaveProjectState(path, state); err != nil { ... }
```

Never call `Save` multiple times in a sequence to accumulate changes. Load → mutate → save is one operation.

## Key Decisions

**Why not `os.WriteFile` directly?** If the process is killed after a partial write, the file is corrupted. YAML parsers will fail on next startup with a cryptic error. `os.Rename` is the standard solution — the kernel guarantees the rename is atomic.

**Why `.tmp` suffix on the same directory?** `os.Rename` is only atomic when source and destination are on the same filesystem. Placing the temp file in a different directory (e.g. `os.TempDir()`) risks crossing filesystem boundaries on some configurations. Same directory, same filesystem, always safe.

**Why not a lock file?** Doug assumes single-process execution (documented in `PRD.md`). Locking adds complexity with no benefit in v1.

## Edge Cases & Gotchas

**Leftover `.tmp` files**: If the process crashes after writing the `.tmp` but before the rename, a `.tmp` file will be left on disk. On next startup, the orchestrator should treat any `*.tmp` state files as stale and delete them. This is not yet implemented — flag if you encounter it.

**Windows rename semantics**: On Windows, `os.Rename` will fail if the destination file is open by another process. Since doug is single-process and never holds state files open across iterations, this should not occur in practice.

## Related Topics

See [Go Infrastructure & Best Practices](../infrastructure/go.md) for the broader file write conventions.
