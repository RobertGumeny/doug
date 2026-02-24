---
task_id: "EPIC-5-002"
outcome: "SUCCESS"
timestamp: "2026-02-24T23:50:00Z"
changelog_entry: "Added HandleFailure handler with retry logic, failure archiving, and task blocking"
duration_seconds: 720
estimated_tokens: 30000
files_modified:
  - internal/handlers/failure.go
  - internal/handlers/failure_test.go
tests_run: 9
tests_passed: 9
build_successful: true
---

## Implementation Summary

Implemented `HandleFailure` in `internal/handlers/failure.go`. The handler follows the specified sequence: rollback uncommitted changes (non-fatal on rollback error) → record task metrics → check attempt count against `config.MaxRetries`. Below the limit it logs a warning and returns nil so the main loop retries. At or above the limit it archives the failure report from `logs/ACTIVE_FAILURE.md`, marks the task BLOCKED in `tasks.yaml`, sets `active_task` to `manual_review` in project state, persists both files, and returns a fatal error containing the task ID and retry count.

## Files Changed

- `internal/handlers/failure.go` — Implemented `HandleFailure` and the private `archiveFailureReport` helper
- `internal/handlers/failure_test.go` — 9 unit tests covering all acceptance criteria branches

## Key Decisions

- **Rollback errors are non-fatal**: logged as a warning and execution continues, consistent with the task spec ("rollback → record metrics" implies we always proceed to the metrics step)
- **Missing `ACTIVE_FAILURE.md` is non-fatal**: `archiveFailureReport` returns a descriptive error; `HandleFailure` logs it as a warning and continues — satisfying the "archiving is skipped with a warning log" acceptance criterion
- **`protectedPaths` is reused** from `success.go` — already defined as a package-level var in the `handlers` package, no duplication needed
- **Synthetic task guard**: `IsSynthetic()` is checked before attempting to write `BLOCKED` status to `tasks.yaml`, since synthetic tasks (bugfix, documentation) never appear there

## Test Coverage

- ✅ Below max_retries returns nil
- ✅ At max_retries returns non-nil error
- ✅ Error message contains task ID and retry count
- ✅ Missing ACTIVE_FAILURE.md → archive skipped (non-fatal), fatal error still returned
- ✅ Archive written to correct path `logs/failures/{epic}/failure-{taskID}.md`
- ✅ Task marked BLOCKED in tasks.yaml at max_retries
- ✅ `active_task` set to `manual_review` in state at max_retries
- ✅ Metrics recorded with outcome "failure"
- ✅ Synthetic task (bugfix) skips BLOCKED marking for user-defined tasks
