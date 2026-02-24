---
task_id: "EPIC-5-003"
outcome: "SUCCESS"
timestamp: "2026-02-24T23:55:00Z"
changelog_entry: "Added HandleBug handler with nested bug protection, bug ID generation, archive, and CI-5 synthetic task type fix"
duration_seconds: 480
estimated_tokens: 50000
files_modified:
  - internal/handlers/bug.go
  - internal/handlers/bug_test.go
tests_run: 33
tests_passed: 33
build_successful: true
---

## Implementation Summary

Created `internal/handlers/bug.go` implementing `HandleBug(ctx *LoopContext) error`. The handler processes BUG outcomes from the agent by scheduling a bugfix task and preserving the interrupted task as next_task.

## Files Changed

- `internal/handlers/bug.go` — HandleBug implementation with nested bug check, rollback, metrics recording, bug ID generation, ACTIVE_BUG.md archive, and state mutation
- `internal/handlers/bug_test.go` — 13 unit tests covering all acceptance criteria

## Key Decisions

- **Nested bug check before rollback**: A `TaskTypeBugfix` task reporting BUG returns a fatal error immediately (Tier 3), without attempting rollback, to prevent a death spiral.
- **Bug ID format**: `"BUG-" + ctx.TaskID` matches the Bash orchestrator convention (e.g., `BUG-EPIC-1-003`).
- **Archive path**: `logs/bugs/{epic}/bug-{taskID}.md` — reads from the flat `logs/ACTIVE_BUG.md` path (CI-1 fix). Archive is non-fatal; if ACTIVE_BUG.md is absent, a warning is logged and bug scheduling continues.
- **CI-5 fix (next_task.type)**: `resolveInterruptedType()` checks `IsSynthetic()` first and returns `ctx.TaskType` directly for synthetic tasks (documentation, etc.) since they are never in tasks.yaml. User-defined tasks are looked up by ID in the task list.
- **State persistence**: `state.SaveProjectState` is called after mutating active_task and next_task in memory, consistent with the failure handler pattern.

## Test Coverage

- ✅ Nested bug (bugfix task type) returns fatal error immediately
- ✅ Fatal error message contains task ID and "nested"
- ✅ Bug ID is prefixed with "BUG-" (e.g., BUG-EPIC-5-001)
- ✅ ActiveTask.Type is set to bugfix
- ✅ NextTask.ID is set to the interrupted task ID
- ✅ User-defined task: NextTask.Type looked up from tasks.yaml
- ✅ Synthetic task (documentation): NextTask.Type from ctx.TaskType (CI-5)
- ✅ Missing ACTIVE_BUG.md: bug still scheduled, returns nil
- ✅ Missing ACTIVE_BUG.md: archive directory not created
- ✅ ACTIVE_BUG.md present: archived to correct path
- ✅ CI-1 fix: reads from flat logs/ACTIVE_BUG.md (not subdirectory)
- ✅ Metrics recorded with "bug" outcome
- ✅ Feature task BUG outcome returns nil (non-fatal)
