---
title: internal/git — Git Operations
updated: 2026-04-20
category: Packages
tags: [git, branch, rollback, commit, exec, revert, sha, protected-paths]
related_articles:
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/infrastructure/go.md
  - docs/kb/features/revert.md
---

# internal/git — Git Operations

## Purpose

`internal/git` wraps all git operations the orchestrator and CLI commands perform: branch setup, rollback on failure, commit after success, SHA tracking, and history rewind. All functions accept `projectRoot string` — no package-level globals.

## Key Facts

- All git commands use `exec.Command("git", ...)` with an explicit args slice — no `sh -c`
- `RollbackChanges` uses **in-memory backups** (not temp files on disk) for protected paths
- `ErrNothingToCommit` is a sentinel — callers use `errors.Is` and treat it as non-fatal
- `ErrGuardedPath` is a sentinel — commit callers receive it when pending changes include guarded generated directories such as `node_modules/` or `dist/`
- `branchExists` uses `git branch --list` (empty output = branch absent) to avoid parsing exit codes
- `SHAExists` and `IsFileTracked` detect non-zero exit codes via `*exec.ExitError` — non-zero is a valid "not found" result, not an error
- `ResetHard` rewinds tracked repository contents only; `doug revert` is responsible for rewriting local `.doug/` state after the reset

## API

```go
// Branch management
func EnsureEpicBranch(branchName, projectRoot string) error
func CurrentBranch(projectRoot string) (string, error)
func HasUncommittedChanges(projectRoot string) (bool, error)
func HasRemoteTrackingBranch(branchName, projectRoot string) (bool, error)

// Rollback on FAILURE outcome
func RollbackChanges(projectRoot string, protectedPaths []string) error

// Commit after SUCCESS outcome
func Commit(message, projectRoot string) error

// SHA introspection
func CurrentSHA(projectRoot string) (string, error)
func LookupCommitByGrep(pattern, projectRoot string) (string, error)
func SHAExists(sha, projectRoot string) (bool, error)
func IsFileTracked(file, projectRoot string) (bool, error)

// History rewind (used by doug revert)
func ResetHard(sha, projectRoot string) error

// Sentinel
var ErrNothingToCommit = errors.New("nothing to commit")
var ErrGuardedPath = errors.New("guarded generated directory would be committed")

// Single source of truth for orchestrator state files to preserve across rollback.
// Handlers must use this var, not define their own literals.
var DefaultProtectedPaths = []string{
    ".doug/project-state.yaml",
    ".doug/tasks.yaml",
}
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

Handlers pass `git.DefaultProtectedPaths` (defined in this package) — the single source of truth for protected path literals. Do not duplicate this list in callers.

## Commit

```go
err := git.Commit("feat: EPIC-2-001", projectRoot)
if errors.Is(err, git.ErrNothingToCommit) {
    // Non-fatal: agent made no changes
}
// All other errors are fatal (Tier 3)
```

Steps: `git add -A` → `git commit -m message`. Detects "nothing to commit" in output and wraps `ErrNothingToCommit`.

Before staging, `Commit` runs a deterministic repository-hygiene guard against pending `git status` paths. If the changes include a common generated dependency/build directory from the guarded set (`node_modules/`, `dist/`, `build/`, `coverage/`, `.next/`, `.nuxt/`, `.svelte-kit/`), the commit is refused with `ErrGuardedPath` and an actionable message telling the caller to fix `.gitignore` or untrack the directory first. Correctly ignored directories do not trigger the guard because ignored paths are absent from `git status`.

## CommitPaths

```go
err := git.CommitPaths("docs: synthesize KB for EPIC-2", projectRoot, []string{"docs/kb/article.md"})
```

Path-scoped commit: stages and commits **only** the listed paths via `git add -- <paths>` → `git commit -m message -- <paths>`. Unlike `Commit`, it never uses a broad `git add -A`, so unrelated dirty working-tree files are never swept into the commit and are left dirty afterward. Returns `ErrNothingToCommit` when `paths` is empty or none of the listed paths have changes. Used by the post-epic KB/changelog pass to commit changed `docs/kb/` files and `CHANGELOG.md` with separate scoped commits.

## SHA Helpers

### CurrentSHA

Returns the full 40-character HEAD SHA via `git rev-parse HEAD`. Used by `HandleSuccess` to backfill `TaskMetric.CommitSHA` after each commit.

### LookupCommitByGrep

```go
sha, err := git.LookupCommitByGrep("feat: EPIC-5-003", projectRoot)
```

Runs `git log --grep=<pattern> -1 --format=%H`. Returns the most recent matching SHA, or empty string if no match (not an error). Used by `doug revert` as a fallback when `CommitSHA` is absent from the metrics entry.

### SHAExists / IsFileTracked

Both use `*exec.ExitError` detection — a non-zero exit is "not found" (returns `false, nil`), while `exec` setup errors are propagated as real errors. Do not use `err != nil` alone to check for absence.

## History Rewind

### ResetHard

```go
err := git.ResetHard(sha, projectRoot)
```

Executes `git reset --hard <sha>`. Intentionally separate from `RollbackChanges` (which always targets HEAD and preserves protected paths). Error message is prefixed `"ResetHard:"` for consistency. Used exclusively by `doug revert`, which now reconciles `.doug/tasks.yaml`, `.doug/project-state.yaml`, and session logs after the reset so `.doug/` can remain gitignored/local-only.

## Branch Introspection

### CurrentBranch

Thin public wrapper over the internal `currentBranch` function. Returns the short branch name (`git rev-parse --abbrev-ref HEAD`).

### HasUncommittedChanges

Uses `git status --porcelain` — catches staged, unstaged, and untracked files. Returns `true` if any output is present.

### HasRemoteTrackingBranch

Uses `git rev-parse --abbrev-ref <branch>@{upstream}`. Non-zero exit = no upstream (`false, nil`). Other exec errors propagate. Used by `doug revert` to warn about required force-push.

## Common Pitfalls

- **`ErrNothingToCommit` is non-fatal** — the orchestrator logs a warning and continues. Do not treat it as a Tier 3 error.
- **`ErrGuardedPath` is intentional** — it means the repository would commit a generated directory because ignore hygiene is missing or incomplete. Fix `.gitignore` or untrack the directory; do not work around it by weakening the guard.
- **`RollbackChanges` silently skips missing protected files** — if `project-state.yaml` does not exist at rollback time, it is not restored (which is correct — it didn't exist before the agent ran either).
- **Windows CRLF in tests** — tests comparing file content after a git reset must normalize `\r\n` → `\n` when `core.autocrlf=true` is set. Production code needs no change.
- **`git clean -fd` removes untracked files** — agents that create files outside `logs/`, `docs/kb/`, `.env`, or `*.backup` will lose those files on rollback. This is intentional.
- **`ResetHard` vs `RollbackChanges`**: `ResetHard` is for deliberate history rewind to a specific SHA (revert command). `RollbackChanges` is for discarding agent changes during the run loop. Never swap them.
- **`ResetHard` does not restore ignored local state**: if follow-up behavior depends on `.doug/` contents, callers must capture what they need before reset and rewrite local state afterward.

## Related

- [Exec Command Pattern](../patterns/pattern-exec-command.md) — subprocess invocation rules
- [Go Infrastructure](../infrastructure/go.md) — no go-git policy, exec.Command rule
- [doug revert](../features/revert.md) — user-facing command that uses ResetHard and the SHA helpers
