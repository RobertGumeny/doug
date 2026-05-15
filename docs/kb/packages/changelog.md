---
title: internal/changelog — Idempotent CHANGELOG Update
updated: 2026-05-15
category: Packages
tags: [changelog, idempotent, file-manipulation, pure-go, atomic-write]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
---

# internal/changelog — Idempotent CHANGELOG Update

## Overview

`internal/changelog` provides a single exported function, `UpdateChangelog`, that inserts a bullet entry into the correct subsection of the `## [Unreleased]` block in a `CHANGELOG.md` file. It is idempotent, uses pure Go string manipulation (no `exec.Command`, no `sed`/`awk`), and is non-fatal on recoverable errors.

## API

```go
func UpdateChangelog(path, entry string, category types.ChangelogCategory) error
```

`category` must be one of the four `types.ChangelogCategory` constants. An empty or unrecognized value returns an error; the caller should fall back to `taskTypeToCategory` before calling (already done in `HandleSuccess`).

## Behavior

| Condition | Result |
|-----------|--------|
| `## [Unreleased]` absent from file | Returns error |
| Bullet `"- {entry}"` already in `## [Unreleased]` | Returns nil; file unchanged (idempotent) |
| Target subsection not found within `## [Unreleased]` | Returns error |
| Unknown `category` | Returns error |
| File not found | Returns error (wrapped `os.ReadFile` error) |
| Success | Inserts bullet immediately after section header line |

All errors are non-fatal from the caller's perspective — callers should log and continue.

## ChangelogCategory → Section Mapping

```
types.CategoryAdded   → ### Added
types.CategoryChanged → ### Changed
types.CategoryFixed   → ### Fixed
types.CategoryRemoved → ### Removed
```

Any other value returns an error.

### Deriving category from task type

`HandleSuccess` resolves the category via `result.ChangelogCategory` (agent-supplied) with a fallback to `taskTypeToCategory(ctx.TaskType)`:

```
TaskTypeFeature       → CategoryAdded
TaskTypeBugfix        → CategoryFixed
TaskTypeDocumentation → CategoryChanged
```

Unknown task types pass their raw string as a `ChangelogCategory`, which `UpdateChangelog` rejects (non-fatal warning).

## ## [Unreleased] Block Scoping

All operations are scoped to the `## [Unreleased]` block only. The block is bounded from the `## [Unreleased]` header to the next `\n## ` section (or end of file).

- **Idempotency check**: `strings.Contains(unreleasedBlock, "- "+entry)` — only the unreleased block is searched. A bullet in a released version section (e.g., `## [1.0.0]`) does not prevent insertion into `## [Unreleased]`.
- **Subsection search**: `strings.Index(unreleasedBlock, header)` — only the unreleased block is searched. A `### Fixed` header in a released section is ignored; if the header is absent from `## [Unreleased]`, an error is returned.

This prevents false-positive idempotency (skipping insertion because the bullet exists in a released section) and wrong-section insertion (writing into a released section when the target header is missing from unreleased).

## Insertion Order

New entries are inserted **immediately after the section header line**, so newer entries appear first within the section:

```markdown
### Added
- Newest entry    ← inserted here
- Older entry
```

## Non-Fatal Error Pattern

`UpdateChangelog` errors are warnings, not failures. Callers should log them and continue:

```go
if err := changelog.UpdateChangelog(changelogPath, entry, category); err != nil {
    log.Warning("changelog update skipped: %v", err)
    // do not return err — this is non-fatal
}
```

## Key Decisions

**Pure Go string manipulation**: No `exec.Command`. The file is read fully into memory, manipulated as a string, and written back via `state.AtomicWrite`. Acceptable because CHANGELOG files are small.

**Atomic write**: `UpdateChangelog` uses `state.AtomicWrite` (write-to-`.tmp`-then-rename pattern). Consistent with how the orchestrator writes all state files; safe against mid-write process kills.

**Scoped block extraction**: The unreleased block is extracted as a substring from `unreleasedIdx` to the next `\n## ` or EOF. All subsequent operations work against this substring before converting back to absolute file offsets for insertion.

**Header at end of file**: If the section header has no trailing newline, the bullet is appended with `content + "\n" + bullet + "\n"`.

## Related

- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — `state.AtomicWrite` used by `UpdateChangelog` for safe writes
