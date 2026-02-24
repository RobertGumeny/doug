---
task_id: "EPIC-2-004"
outcome: "SUCCESS"
timestamp: "2026-02-24T21:00:00Z"
changelog_entry: "Added git package with EnsureEpicBranch, RollbackChanges, and Commit operations"
duration_seconds: 240
estimated_tokens: 30000
files_modified:
  - internal/git/git.go
  - internal/git/git_test.go
tests_run: 11
tests_passed: 11
build_successful: true
---

## Implementation Summary

Created `internal/git/git.go` with three public functions and a sentinel error for the orchestrator's git operations. All functions accept `projectRoot string` as a parameter (no package-level globals) and use `exec.Command` with explicit args slices throughout.

## Files Changed

- `internal/git/git.go` — Implemented `EnsureEpicBranch`, `RollbackChanges`, `Commit`, and `ErrNothingToCommit`
- `internal/git/git_test.go` — 11 unit tests covering all three functions and their edge cases

## Key Decisions

- **RollbackChanges uses in-memory backups** rather than writing to `os.MkdirTemp`. Reading protected files into a `[]fileBackup` slice is simpler and avoids temp-dir cleanup concerns while satisfying the "temp-copy-then-restore" pattern.
- **`ErrNothingToCommit` is a sentinel `var`** (not a custom type), consistent with Go stdlib conventions. Callers use `errors.Is` to detect it and treat it as non-fatal.
- **Windows CRLF in tests**: one test comparing a git-reset file normalises `\r\n` → `\n` to handle `core.autocrlf=true` on Windows without changing production code.
- `branchExists` uses `git branch --list <name>` — an empty output means the branch does not exist; non-empty means it does. This avoids parsing exit codes of `git show-ref`.

## Test Coverage

- ✅ `EnsureEpicBranch` — already on branch (no-op)
- ✅ `EnsureEpicBranch` — existing branch → checkout
- ✅ `EnsureEpicBranch` — new branch → create with checkout -b
- ✅ `RollbackChanges` — protected file content preserved across reset
- ✅ `RollbackChanges` — unprotected tracked file reverted by reset
- ✅ `RollbackChanges` — untracked file removed by git clean
- ✅ `RollbackChanges` — missing protected file silently skipped
- ✅ `RollbackChanges` — multiple protected files all preserved
- ✅ `Commit` — nothing to commit returns `ErrNothingToCommit`
- ✅ `Commit` — with changes creates a commit
- ✅ `Commit` — stages all untracked files via `git add -A`
