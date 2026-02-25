---
name: implement-feature
description: Execute the full feature implementation workflow including research, planning, coding, testing, and reporting. Use when project-state.yaml indicates active_task.type is "feature" or when implementing a new feature from tasks.yaml. Agents are STATELESS - they read YAML/code, write code/session results, but NEVER touch Git or YAML updates.
---

# Feature Implementation Workflow

This skill guides you through the complete feature implementation process from research to session reporting.

## Agent Boundaries (Critical)

**You ARE allowed to:**

- ✅ Read `project-state.yaml`, `tasks.yaml`, `PRD.md`, and code
- ✅ Write/modify source code and tests
- ✅ Run build, test, and lint commands
- ✅ Write session results to `logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md`
- ✅ Write bug reports to `logs/ACTIVE_BUG.md`
- ✅ Write failure reports to `logs/ACTIVE_FAILURE.md`

**You are NOT allowed to:**

- ❌ Run ANY Git commands (checkout, commit, push, branch, etc.)
- ❌ Modify `project-state.yaml` or `tasks.yaml`
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive files in `/logs/`

**The orchestrator handles:** Git operations, YAML updates, CHANGELOG updates, file archiving.

## Phase 1: Research

1. Read `project-state.yaml` to get:
   - `current_epic`: The epic you're working on
   - `task_id` (from `.active_task.id`): Your task ID
   - `attempts` (from `.active_task.attempts`): Current attempt number (for session filename)

2. Read `tasks.yaml` to get:
   - Task description
   - Acceptance criteria
   - Task status

3. **Pre-Flight Check**: Verify the task is not already marked `DONE` in `tasks.yaml`
   - If already `DONE`, write session result with `outcome: EPIC_COMPLETE` and exit
   - Check if there are any remaining `TODO` tasks
   - If no `TODO` tasks remain in the epic, write session result with `outcome: EPIC_COMPLETE` and exit

4. Read `PRD.md` for product context and requirements

5. Survey existing codebase to understand structure

## Phase 2: Plan

1. Propose exactly which files you will create or modify

2. **Ambiguity Check**: If any requirement is unclear, search `PRD.md` thoroughly
   - Check for related features or patterns
   - Look for architectural decisions
   - Review any constraints or guidelines

3. **Termination Clause**: If the requirement remains undefined after checking PRD:
   - DO NOT guess or make assumptions
   - Write `logs/ACTIVE_FAILURE.md` using the failure report template
   - Write session result with `outcome: FAILURE`
   - Exit immediately

4. Ensure your plan complies with all architectural rules in `CLAUDE.md`

## Phase 3: Implement

1. Execute code implementation according to your proposed plan

2. Write unit tests for all new core functionality
   - Test happy paths
   - Test edge cases
   - Test error handling

3. **Integrity Check - No Workarounds Rule**:
   - If you discover a blocking bug in existing code (not part of your task):
     - **STOP immediately** - do not attempt to fix it or work around it
     - Write `logs/ACTIVE_BUG.md` using the bug report template
     - Write session result with `outcome: BUG`, noting the bug location
     - Exit immediately
   - The orchestrator will schedule a bugfix task next

## Phase 4: Verify

Run verification steps in order. Fix any issues before proceeding.

1. **Build**: run the project build command and fix ALL errors
2. **Test**: run the test suite and ensure ALL tests pass
3. **Lint** (if available): fix all linter errors

## Phase 5: Report

### Session Result Path

```
logs/sessions/{current_epic}/session-{task_id}_attempt-{attempts}.md
```

### On Success

```yaml
---
outcome: "SUCCESS"
changelog_entry: "Brief user-facing description of what changed"
dependencies_added: []
---

## Implementation Summary
## Files Changed
## Key Decisions
## Test Coverage
```

### On Bug Discovery

Write `logs/ACTIVE_BUG.md`, then write session result with `outcome: BUG`.

### On Failure (After 5 Attempts)

Write `logs/ACTIVE_FAILURE.md`, then write session result with `outcome: FAILURE`.

## Quick Reference

**Outcome Values:** `SUCCESS` | `BUG` | `FAILURE` | `EPIC_COMPLETE`

**File Locations:**
- Session result: `logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md`
- Active bug: `logs/ACTIVE_BUG.md`
- Active failure: `logs/ACTIVE_FAILURE.md`
