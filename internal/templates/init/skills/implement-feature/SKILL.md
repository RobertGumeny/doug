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
- ✅ Write session results to the path provided in your briefing
- ✅ Write bug reports to the path provided in your briefing
- ✅ Write failure reports to the path provided in your briefing

**You are NOT allowed to:**

- ❌ Run ANY Git commands (checkout, commit, push, branch, etc.)
- ❌ Modify `project-state.yaml` or `tasks.yaml`
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive files in `.doug/logs/`

**The orchestrator handles:** Git operations, YAML updates, CHANGELOG updates, file archiving.

## Phase 1: Research

1. Read `.doug/ACTIVE_TASK.md` to get task metadata and **the paths provided in your briefing**:
   - **Session File**, **Active Bug File**, **Failure File** paths
   - **Task ID**, **Task Type**, **Attempt** number
   - **Description** and **Acceptance Criteria** for this task

2. **Pre-Flight Check**: Verify the task is not already marked `DONE` in `tasks.yaml`
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
   - Write the failure report to the path from your briefing
   - Write session result with `outcome: FAILURE`
   - Exit immediately

## Phase 3: Implement

1. Execute code implementation according to your proposed plan

2. Write unit tests for all new core functionality
   - Test happy paths
   - Test edge cases
   - Test error handling

3. **Integrity Check - No Workarounds Rule**:
   - If you discover a blocking bug in existing code (not part of your task):
     - **STOP immediately** - do not attempt to fix it or work around it
     - Write the bug report to the path from your briefing
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

Use the **Session File** path from your briefing.

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

Write the bug report to the **Active Bug File** path from your briefing, then write session result with `outcome: BUG`.

### On Failure (After 5 Attempts)

Write the failure report to the **Failure File** path from your briefing, then write session result with `outcome: FAILURE`.

## Quick Reference

**Outcome Values:** `SUCCESS` | `BUG` | `FAILURE` | `EPIC_COMPLETE`

**File Locations:** see paths in your briefing header (`.doug/ACTIVE_TASK.md`)
