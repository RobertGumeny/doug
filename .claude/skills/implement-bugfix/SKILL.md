---
name: implement-bugfix
description: Execute the full bugfix workflow including root cause analysis, fix implementation, regression testing, and reporting. Use when project-state.yaml indicates active_task.type is "bugfix". Agents are STATELESS - they read YAML/code, write code/session results, but NEVER touch Git or YAML updates.
---

# Bugfix Implementation Workflow

This skill guides you through the complete bug resolution process from diagnosis to session reporting.

## Agent Boundaries (Critical)

**You ARE allowed to:**

- ✅ Read `project-state.yaml`, `tasks.yaml`, `PRD.md`, `logs/ACTIVE_BUG.md`, and code
- ✅ Write/modify source code and tests
- ✅ Run `npm run build`, `npm run test`, `npm run lint`
- ✅ Write session results to `logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md`
- ✅ Write new bug reports to `logs/ACTIVE_BUG.md` (if you find additional bugs)
- ✅ Write failure reports to `logs/ACTIVE_FAILURE.md`

**You are NOT allowed to:**

- ❌ Run ANY Git commands (checkout, commit, push, branch, etc.)
- ❌ Modify `project-state.yaml` or `tasks.yaml`
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive `logs/ACTIVE_BUG.md` (orchestrator does this)
- ❌ Delete or modify archived bug reports
- ❌ Run `npm run dev` or any dev server

**The orchestrator handles:** Git operations, YAML updates, CHANGELOG updates, file archiving.

## Phase 1: Research

1. Read `project-state.yaml` to get:
   - `current_epic.id`: The epic you're working on
   - `active_task.id`: Your bug ID (e.g., "BUG-TASK-001")
   - `active_task.attempts`: Current attempt number (for session filename)

2. **Read the active bug report**: `logs/ACTIVE_BUG.md`
   - Understand what's broken
   - Note the location in the codebase
   - Review steps to reproduce
   - Check the original task that discovered this bug

3. **Pre-Flight Check**: Verify the bug hasn't already been resolved
   - Try to reproduce the bug using steps in the report
   - If the bug no longer exists (e.g., fixed in another commit):
     - Write session result with `outcome: SUCCESS` noting bug already resolved
     - Exit immediately
   - The orchestrator will archive the bug report and resume the original task

4. Reference `PRD.md` and `tasks.yaml` for context on the impacted task/epic

5. Examine the code around the bug location to understand the system

## Phase 2: Plan

1. **Root Cause Analysis**:
   - Identify WHY the bug exists
   - Trace the code path that triggers it
   - Understand the intended behavior

2. **Propose a Fix**:
   - Design a solution that addresses the root cause
   - Ensure your fix won't break other functionality
   - Consider edge cases and side effects

3. **Define Regression Tests**:
   - Plan tests that would have caught this bug
   - Ensure tests will prevent the bug from returning
   - Cover both the specific bug scenario and related cases

4. **Ambiguity Check**: If the bug report is unclear or incomplete:
   - Search `PRD.md` for context on the affected feature
   - Review related code and tests
   - If you still cannot determine the correct behavior:
     - Write `logs/ACTIVE_FAILURE.md` documenting the ambiguity
     - Write session result with `outcome: FAILURE`
     - Exit immediately

5. **Termination Clause**: If root cause cannot be determined after thorough investigation:
   - Write `logs/ACTIVE_FAILURE.md` explaining what you've analyzed
   - Write session result with `outcome: FAILURE`
   - Exit immediately

## Phase 3: Implement

1. Execute the fix according to your plan:
   - Make minimal changes necessary to fix the bug
   - Don't refactor unrelated code
   - Keep changes focused and reviewable

2. Write or update regression tests:
   - Add test that reproduces the original bug scenario
   - Verify the test fails without your fix
   - Verify the test passes with your fix
   - Add related edge case tests

3. **Integrity Check - No Workarounds Rule**:
   - If you discover ADDITIONAL bugs while fixing this one:
     - **STOP** - do not attempt to fix the new bug
     - Complete the current bugfix first
     - Document the new bug in a comment in the code
     - Note it in your session result for human awareness
     - Do NOT write a new ACTIVE_BUG.md (only one active bug at a time)

## Phase 4: Verify

Run verification steps in order. Fix any issues before proceeding.

1. **Build Verification**:

   ```bash
   npm run build
   ```

   - Fix ALL errors and warnings
   - Do not proceed until build is clean

2. **Test Verification**:

   ```bash
   npm run test
   ```

   - Ensure ALL tests pass, especially your new regression tests
   - Verify no existing tests broke
   - Confirm the specific bug scenario is now handled correctly

3. **Linting** (if available):

   ```bash
   npm run lint
   ```

   - Fix all linter errors
   - Address any warnings

4. **Manual Verification**:
   - Review the specific functionality described in the bug report
   - Verify the bug is actually fixed
   - Check that related functionality still works

**Critical**: Do NOT run `npm run dev` or any development server.

## Phase 5: Report

### Session Result Path Construction

Your session result MUST be written to:

```
logs/sessions/{current_epic}/session-{task_id}_attempt-{attempts}.md
```

Where:

- `current_epic` from `project_state.yaml.current_epic.id`
- `task_id` from `project_state.yaml.active_task.id` (this will be the bug ID, e.g., "BUG-TASK-001")
- `attempts` from `project_state.yaml.active_task.attempts` (use current value + 1)

**Example**: If current_epic is "EPIC-1", task_id is "BUG-TASK-003", and attempts is 0:

```
logs/sessions/EPIC-1/session-BUG-TASK-003_attempt-1.md
```

### On Success

Write session result with frontmatter:

```yaml
---
task_id: "BUG-TASK-001"
outcome: "SUCCESS"
timestamp: "2025-02-05T10:30:00Z"
changelog_entry: "Fixed null pointer exception in token validation"
duration_seconds: 120
estimated_tokens: 30000
bug_fixed: true
original_task: "TASK-001"
files_modified:
  - src/services/authService.ts
  - src/services/authService.test.ts
tests_run: 15
tests_passed: 15
build_successful: true
---

## Bugfix Summary

Fixed null pointer exception in `validateToken` method by adding null check.

## Root Cause

[Brief explanation of what caused the bug]

## Solution

[Brief explanation of how you fixed it]

## Files Changed

- `src/services/authService.ts` - Added null check in validateToken method
- `src/services/authService.test.ts` - Added regression tests for null/undefined tokens

## Regression Tests Added

- ✅ Test validateToken with null token
- ✅ Test validateToken with undefined token
- ✅ Test validateToken with empty string
- ✅ Existing tests still pass

## Verification

- Build passes
- All tests pass (15/15)
- Manually verified that middleware can now handle missing tokens gracefully
```

**CHANGELOG Entry Guidelines for Bugfixes:**

The `changelog_entry` field should be:

- A single line (no newlines)
- User-facing description of what was fixed
- Written in past tense
- Start with "Fixed" when possible
- Focused on the impact, not the implementation

**Metrics Tracking (Required):**

The orchestrator tracks task duration and token usage. Include these fields:

- `duration_seconds`: Estimated time spent on this bugfix (in seconds)
  - Include diagnosis, fixing, testing, and verification time
  - Bugfixes are often faster than features
  - Example: 120 (2 minutes), 300 (5 minutes), 900 (15 minutes)

- `estimated_tokens`: Rough estimate of tokens consumed
  - Count all characters in files you read (bug report, affected code, related files)
  - Count all characters in files you wrote/modified
  - Divide total by 4 for rough token count
  - Multiply by 1.5x for bugfixes (less overhead than features)
  - Example calculation:
    - Read 8,000 chars + wrote 2,000 chars = 10,000 chars total
    - 10,000 / 4 = 2,500 base tokens
    - 2,500 × 1.5 = 3,750 estimated tokens (for bugfix work)
  - Round to nearest 1000 or 5000 for simplicity
  - If you can't estimate accurately, provide your best guess

**Then exit**. The orchestrator will:

- Verify build/tests again
- Update CHANGELOG.md with your entry
- Commit your changes with "fix:" prefix
- Archive `logs/ACTIVE_BUG.md` to `logs/bugs/{epic}/bug-{original_task_id}.md`
- Set active_task back to the original task that was interrupted
- Continue the epic

### On Failure (After 5 Attempts)

If you've attempted this bugfix 5 times and still cannot complete it, write `logs/ACTIVE_FAILURE.md` and exit with `outcome: FAILURE`.

The orchestrator will:

- Archive failure report
- Mark bug as BLOCKED
- Set active_task to manual_review
- Stop execution for human intervention

## Quick Reference

**Outcome Values:**

- `SUCCESS` - Bug fixed, all tests passing
- `FAILURE` - Failed after 5 attempts, wrote ACTIVE_FAILURE.md

**File Locations:**

- Active bug report (input): `logs/ACTIVE_BUG.md`
- Session result (output): `logs/sessions/{epic}/session-{bug_id}_attempt-{attempts}.md`
- Active failure (if needed): `logs/ACTIVE_FAILURE.md`

**Required Session Result Fields (SUCCESS):**

- `outcome`: SUCCESS
- `timestamp`: ISO 8601 format
- `changelog_entry`: (REQUIRED) User-facing description of the fix
- `duration_seconds`: (REQUIRED) Time spent on task in seconds
- `estimated_tokens`: (REQUIRED) Rough token count estimate
- `bug_fixed`: true
- `original_task`: The task ID that discovered this bug
- `files_modified`: Array of changed files
- `tests_run`: Number of tests executed
- `tests_passed`: Number of tests that passed
- `build_successful`: Boolean

**Remember:**

- You read ACTIVE_BUG.md, you don't move or delete it
- You fix the bug and write session results with changelog entry
- Orchestrator archives the bug report after success
- Orchestrator updates CHANGELOG.md with your entry
- Orchestrator resumes the original interrupted task
