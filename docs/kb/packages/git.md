---
title: internal/git — Git Operations
updated: 2026-02-24
category: Packages
tags: [git, branch, rollback, commit, exec]
related_articles:
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/infrastructure/go.md
---

# internal/git — Git Operations

## Purpose

`internal/git` wraps the three git operations the orchestrator performs: branch setup, rollback on failure, and commit after success. All functions accept `projectRoot string` — no package-level globals.

## Key Facts

- All git commands use `exec.Command("git", ...)` with an explicit args slice — no `sh -c`
- `RollbackChanges` uses **in-memory backups** (not temp files on disk) for protected paths
- `ErrNothingToCommit` is a sentinel — callers use `errors.Is` and treat it as non-fatal
- `branchExists` uses `git branch --list` (empty output = branch absent) to avoid parsing exit codes

## API

```go
// Branch management
func EnsureEpicBranch(branchName, projectRoot string) error

// Rollback on FAILURE outcome
func RollbackChanges(projectRoot string, protectedPaths []string) error

// Commit after SUCCESS outcome
func Commit(message, projectRoot string) error

// Sentinel
var ErrNothingToCommit = errors.New("nothing to commit")
```

## EnsureEpicBranch

Three-state logic — idempotent:

| Condition | Action |
|-----------|--------|
| Already on `branchName` | no-op |
| Branch exists locally | `git checkout branchName` |
| Branch does not exist | `git checkout -b branchName` |

Used at orchestrator startup to put the working tree on the epic branch before any task runs.

## RollbackChanges

Called when an agent reports a FAILURE outcome. Resets all agent changes while preserving state files.

```
1. Read protectedPaths into []fileBackup (skip missing files silently)
2. git reset --hard HEAD   — reverts tracked changes
3. Write backed-up files back to disk (MkdirAll for missing parent dirs)
4. git clean -fd --exclude=logs/ --exclude=docs/kb/ --exclude=.env --exclude=*.backup
```

**In-memory backups**: protected file contents are stored in a `[]fileBackup` slice, not written to `os.TempDir()`. This avoids temp-dir cleanup concerns and cross-filesystem rename issues.

Typical `protectedPaths` value (from orchestrator):

```go
[]string{"project-state.yaml", "tasks.yaml"}
```

## Commit

```go
err := git.Commit("feat: EPIC-2-001", projectRoot)
if errors.Is(err, git.ErrNothingToCommit) {
    // Non-fatal: agent made no changes
}
// All other errors are fatal (Tier 3)
```

Steps: `git add -A` → `git commit -m message`. Detects "nothing to commit" in output and wraps `ErrNothingToCommit`.

## Common Pitfalls

- **`ErrNothingToCommit` is non-fatal** — the orchestrator logs a warning and continues. Do not treat it as a Tier 3 error.
- **`RollbackChanges` silently skips missing protected files** — if `project-state.yaml` does not exist at rollback time, it is not restored (which is correct — it didn't exist before the agent ran either).
- **Windows CRLF in tests** — tests comparing file content after a git reset must normalize `\r\n` → `\n` when `core.autocrlf=true` is set. Production code needs no change.
- **`git clean -fd` removes untracked files** — agents that create files outside `logs/`, `docs/kb/`, `.env`, or `*.backup` will lose those files on rollback. This is intentional.

## Related

- [Exec Command Pattern](../patterns/pattern-exec-command.md) — subprocess invocation rules
- [Go Infrastructure](../infrastructure/go.md) — no go-git policy, exec.Command rule
