---
title: internal/changelog — Idempotent CHANGELOG Update
updated: 2026-02-24
category: Packages
tags: [changelog, idempotent, file-manipulation, pure-go]
related_articles:
  - docs/kb/packages/types.md
---

# internal/changelog — Idempotent CHANGELOG Update

## Overview

`internal/changelog` provides a single exported function, `UpdateChangelog`, that inserts a bullet entry into the correct section of a `CHANGELOG.md` file. It is idempotent, uses pure Go string manipulation (no `exec.Command`, no `sed`/`awk`), and is non-fatal on recoverable errors.

## API

```go
func UpdateChangelog(path, entry, taskType string) error
```

## Behavior

| Condition | Result |
|-----------|--------|
| Bullet `"- {entry}"` already in file | Returns nil; file unchanged (idempotent) |
| Section header not found | Returns non-fatal error; caller should warn |
| Unknown `taskType` | Returns non-fatal error |
| File not found | Returns non-fatal error (wrapped `os.ReadFile` error) |
| Success | Inserts bullet immediately after section header line |

## Task Type → Section Mapping

```
"feature"       → ### Added
"bugfix"        → ### Fixed
"documentation" → ### Changed
```

Any other `taskType` returns an error.

## Insertion Order

New entries are inserted **immediately after the section header line**, so newer entries appear first within the section:

```markdown
### Added
- Newest entry    ← inserted here
- Older entry
```

## Deduplication

Deduplication uses `strings.Contains(content, "- "+entry)`. If the exact bullet string appears anywhere in the file, the file is left unchanged. This is sufficient for the orchestrator's use case (unique changelog descriptions per task).

## Non-Fatal Error Pattern

`UpdateChangelog` errors are warnings, not failures. Callers should log them and continue:

```go
if err := changelog.UpdateChangelog(changelogPath, entry, taskType); err != nil {
    log.Warning("changelog update skipped: %v", err)
    // do not return err — this is non-fatal
}
```

## Key Decisions

**Pure Go string manipulation**: No `exec.Command`, no temp files, no `os.Rename`. The file is read fully into memory, manipulated as a string, and written back with `os.WriteFile`. Acceptable because CHANGELOG files are small.

**Note — not atomic**: `UpdateChangelog` uses `os.WriteFile` directly (not the write-to-`.tmp`-then-rename pattern). A process kill mid-write could corrupt the changelog. This is acceptable for CHANGELOG (non-critical, human-readable) but do not use this approach for `project-state.yaml` or `tasks.yaml`.

**Header at end of file**: If the section header has no trailing newline, the bullet is appended with `content + "\n" + bullet + "\n"`.

## Related

- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — use this pattern for state files; changelog intentionally skips it
