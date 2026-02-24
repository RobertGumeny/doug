---
task_id: "EPIC-3-002"
outcome: "SUCCESS"
timestamp: "2026-02-24T23:15:00Z"
changelog_entry: "Added task pointer management functions for the Go orchestrator (InitializeTaskPointers, AdvanceToNextTask, FindNextActiveTask, IncrementAttempts, UpdateTaskStatus)"
duration_seconds: 420
estimated_tokens: 35000
files_modified:
  - internal/orchestrator/taskpointers.go
  - internal/orchestrator/taskpointers_test.go
tests_run: 31
tests_passed: 31
build_successful: true
---

## Implementation Summary

Created `internal/orchestrator/taskpointers.go` with five functions that manage the active/next task pointer state for the Go orchestrator. All functions operate in-memory; callers are responsible for persisting state via SaveState/SaveTasks.

## Files Changed

- `internal/orchestrator/taskpointers.go` — Five exported functions: InitializeTaskPointers, AdvanceToNextTask, FindNextActiveTask, IncrementAttempts, UpdateTaskStatus
- `internal/orchestrator/taskpointers_test.go` — 17 unit tests covering all functions and edge cases

## Key Decisions

- **InitializeTaskPointers**: Scans tasks in two passes (IN_PROGRESS first, then TODO) to handle orchestrator-restart recovery. Injects a synthetic KB_UPDATE documentation task when all user tasks are DONE and kb_enabled is true.
- **AdvanceToNextTask**: Returns false immediately when NextTask.ID is empty (nothing to advance to), otherwise always returns true after promotion. Resets Attempts to 0 on promotion. Uses positional search (find first TODO after the newly active task's position) so it works correctly regardless of whether UpdateTaskStatus has been called yet.
- **FindNextActiveTask**: Pure scan function (IN_PROGRESS preferred, then TODO). Intentionally separate from AdvanceToNextTask's internal next-finding logic since the two use different algorithms (global first-match vs. positional after-current).
- **UpdateTaskStatus**: Returns a descriptive error for unknown IDs; no silent no-ops.

## Test Coverage

- ✅ InitializeTaskPointers prefers IN_PROGRESS over TODO
- ✅ InitializeTaskPointers falls back to first TODO when no IN_PROGRESS
- ✅ InitializeTaskPointers sets first task as active (all TODO)
- ✅ InitializeTaskPointers with last task active (no next)
- ✅ InitializeTaskPointers injects KB_UPDATE when all tasks DONE and kb_enabled=true
- ✅ InitializeTaskPointers does NOT inject KB_UPDATE when kb_enabled=false
- ✅ AdvanceToNextTask advances through 3 tasks (T1→T2→T3, correct next pointers)
- ✅ AdvanceToNextTask returns false when next_task is empty (last task)
- ✅ AdvanceToNextTask resets Attempts to 0 on promotion
- ✅ FindNextActiveTask prefers IN_PROGRESS
- ✅ FindNextActiveTask returns first TODO when no IN_PROGRESS
- ✅ FindNextActiveTask returns empty when no candidates
- ✅ IncrementAttempts increments from non-zero value
- ✅ IncrementAttempts increments from zero
- ✅ UpdateTaskStatus updates correct task, leaves others unchanged
- ✅ UpdateTaskStatus returns error for unknown ID
- ✅ UpdateTaskStatus can set any valid status (DONE tested)
