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
- ✅ Run `npm run build`, `npm run test`, `npm run lint`
- ✅ Write session results to `logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md`
- ✅ Write bug reports to `logs/ACTIVE_BUG.md`
- ✅ Write failure reports to `logs/ACTIVE_FAILURE.md`

**You are NOT allowed to:**

- ❌ Run ANY Git commands (checkout, commit, push, branch, etc.)
- ❌ Modify `project-state.yaml` or `tasks.yaml`
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive files in `/logs/`
- ❌ Run `npm run dev` or any dev server

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

4. Ensure your plan complies with all architectural rules in `CLAUDE.md`:
   - Follow privacy guidelines
   - Use service layers for database access
   - Separate business logic into hooks
   - Keep components presentational

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
     - Set frontmatter: `outcome: BUG`, `discovered_by_task: {your_task_id}`
     - Write session result with `outcome: BUG`, noting the bug location
     - Exit immediately
   - The orchestrator will schedule a bugfix task next

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

   - Ensure ALL tests pass
   - Pay special attention to new tests you wrote

3. **Linting** (if available):

   ```bash
   npm run lint
   ```

   - Fix all linter errors
   - Address any warnings

**Critical**: Do NOT run `npm run dev` or any development server.

## Phase 5: Report

### Session Result Path Construction

Your session result MUST be written to:

```
logs/sessions/{current_epic}/session-{task_id}_attempt-{attempts}.md
```

Where:

- `current_epic` from `project_state.yaml.current_epic.id`
- `task_id` from `project_state.yaml.active_task.id`
- `attempts` from `project_state.yaml.active_task.attempts` (use current value + 1)

**Example**: If current_epic is "EPIC-1", task_id is "TASK-003", and attempts is 1:

```
logs/sessions/EPIC-1/session-TASK-003_attempt-2.md
```

### On Success

Write session result with frontmatter:

```yaml
---
task_id: "TASK-001"
outcome: "SUCCESS"
timestamp: "2025-02-05T10:30:00Z"
changelog_entry: "Added JWT token generation with configurable expiration"
duration_seconds: 180
estimated_tokens: 45000
files_modified:
  - src/auth/token.ts
  - src/auth/token.test.ts
tests_run: 12
tests_passed: 12
build_successful: true
---

## Implementation Summary

[Brief description of what you implemented]

## Files Changed

- `src/auth/token.ts` - Implemented JWT token generation
- `src/auth/token.test.ts` - Added unit tests for token generation

## Key Decisions

- Used `jsonwebtoken` library for token signing
- Set default expiration to 24 hours
- Included user ID and role in token payload

## Test Coverage

- ✅ Token generation with valid input
- ✅ Token includes correct claims
- ✅ Token expiration is set properly
- ✅ Error handling for missing user data
```

**CHANGELOG Entry Guidelines:**

The `changelog_entry` field should be:

- A single line (no newlines)
- User-facing description of what changed
- Written in past tense
- Focused on the "what" not the "how"
- Example good entries:
  - "Added JWT token generation with configurable expiration"
  - "Fixed null pointer exception in token validation"
  - "Improved error messages for authentication failures"
- Example bad entries:
  - "Updated src/auth/token.ts" (too technical)
  - "Completed TASK-001" (not descriptive)
  - "Made some changes to auth" (too vague)

**Metrics Tracking (Required):**

The orchestrator tracks task duration and token usage. Include these fields:

- `duration_seconds`: Estimated time spent on this task (in seconds)
  - Include research, planning, coding, and verification time
  - Be honest about time spent - this helps improve future estimates
  - Example: 180 (3 minutes), 600 (10 minutes), 1800 (30 minutes)

- `estimated_tokens`: Rough estimate of tokens consumed
  - Count all characters in files you read (codebase exploration, PRD, etc.)
  - Count all characters in files you wrote/modified
  - Divide total by 4 for rough token count
  - Multiply by 2x for features (overhead for planning, multiple passes)
  - Example calculation:
    - Read 10,000 chars + wrote 5,000 chars = 15,000 chars total
    - 15,000 / 4 = 3,750 base tokens
    - 3,750 × 2 = 7,500 estimated tokens (for feature work)
  - Round to nearest 1000 or 5000 for simplicity
  - If you can't estimate accurately, provide your best guess (better than nothing)

**Then exit**. The orchestrator will:

- Verify build/tests again
- Update CHANGELOG.md with your entry
- Commit your changes
- Update task status to DONE
- Update next_task to the next TODO task

### On Bug Discovery

1. Write `logs/ACTIVE_BUG.md`:

````yaml
---
bug_id: "BUG-TASK-001"
discovered_by_task: "TASK-001"
timestamp: "2025-02-05T10:30:00Z"
severity: "blocking"
status: "open"
outcome: "BUG"
---

## Bug Report: BUG-TASK-001

### Summary

User authentication service throws null pointer exception

### Location

`src/services/authService.ts`, line 45

### Description

The `validateToken` method doesn't check if token is null before calling `verify()`. This blocks implementation of middleware that needs to handle missing tokens gracefully.

### Expected Behavior

Should return false or throw a specific error when token is null

### Actual Behavior

Crashes with null pointer exception

### Steps to Reproduce

1. Call `validateToken(null)`
2. Exception is thrown

### Impact

Cannot implement TASK-001 (token validation middleware) until this is fixed.

### Proposed Fix

Add null check before calling verify:
```typescript
if (!token) {
  return false;
}
````

````

2. Write session result with `outcome: BUG`:

```yaml
---
task_id: "TASK-001"
outcome: "BUG"
timestamp: "2025-02-05T10:30:00Z"
bug_location: "src/services/authService.ts:45"
files_modified: []
tests_run: 0
tests_passed: 0
build_successful: false
---

## Bug Discovered

Encountered blocking bug in existing code that prevents completion of this task.

See `logs/ACTIVE_BUG.md` for full details.

## Context

Was implementing token validation middleware when discovered that the underlying `validateToken` service method doesn't handle null tokens.

Cannot proceed without fixing the underlying service.
````

**Then exit**. The orchestrator will:

- Rollback any uncommitted changes
- Set next_task to bugfix type
- Invoke the bugfix skill on next iteration

### On Failure (After 5 Attempts)

If you've attempted this task 5 times and still cannot complete it:

1. Write `logs/ACTIVE_FAILURE.md` using the failure report template:

```yaml
---
failure_id: "FAILURE-TASK-001"
task_id: "TASK-001"
timestamp: "2025-02-05T10:30:00Z"
failure_type: "Build Failure"
outcome: "FAILURE"
---

# FAILURE REPORT: TASK-001

## Context

- **Task**: TASK-001 - Implement JWT token generation
- **Epic**: EPIC-1
- **Date**: 2025-02-05
- **Failure Type**: Build Failure

## Assessment

Unable to resolve TypeScript compilation errors related to type inference in the jsonwebtoken library after 5 attempts.

## Attempts Made

### Attempt 1
- **Action**: Initial implementation using jsonwebtoken
- **Result**: TypeScript error - cannot infer type of payload

### Attempt 2
- **Action**: Added explicit type annotations
- **Result**: Still failing - @types/jsonwebtoken version mismatch

[... document all 5 attempts ...]

## Relevant Error Messages

```

src/auth/token.ts:15:23 - error TS2345: Argument of type 'UserPayload'
is not assignable to parameter of type 'string | object | Buffer'.

```

## Files Involved

- src/auth/token.ts
- src/types/auth.ts

## Recommendations

1. Check if @types/jsonwebtoken version matches jsonwebtoken version
2. Consider using alternative JWT library with better TypeScript support
3. Review if UserPayload type definition is correct

## Questions for Human Review

1. Is the current version of jsonwebtoken compatible with our TypeScript setup?
2. Should we upgrade/downgrade @types/jsonwebtoken?
3. Is there a preferred JWT library for this project?
```

2. Write session result with `outcome: FAILURE`:

```yaml
---
task_id: "TASK-001"
outcome: "FAILURE"
timestamp: "2025-02-05T10:30:00Z"
files_modified:
  - src/auth/token.ts (incomplete)
tests_run: 0
tests_passed: 0
build_successful: false
---

## Failure Summary

After 5 attempts, unable to resolve TypeScript compilation errors with jsonwebtoken library.

See `logs/ACTIVE_FAILURE.md` for detailed analysis and recommendations.

## What Was Attempted

- Multiple type annotation strategies
- Different import patterns
- Type casting approaches

All attempts failed with similar TypeScript errors.

## Recommendation

Requires human review of dependency versions and TypeScript configuration.
```

**Then exit**. The orchestrator will:

- Archive failure report
- Mark task as BLOCKED
- Set next_task to manual_review
- Stop execution for human intervention

## Epic Completion

If during Pre-Flight Check you discover all tasks are DONE:

Write session result with `outcome: EPIC_COMPLETE`:

```yaml
---
task_id: "TASK-001"
outcome: "EPIC_COMPLETE"
timestamp: "2025-02-05T10:30:00Z"
files_modified: []
tests_run: 0
tests_passed: 0
build_successful: true
---
## Epic Complete

All tasks in the current epic are marked DONE.

No further work required.
```

**Then exit**. The orchestrator will mark the epic as complete.

## Quick Reference

**Outcome Values:**

- `SUCCESS` - Task completed, all tests passing
- `BUG` - Found blocking bug, wrote ACTIVE_BUG.md
- `FAILURE` - Failed after 5 attempts, wrote ACTIVE_FAILURE.md
- `EPIC_COMPLETE` - All tasks done

**File Locations:**

- Session result: `logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md`
- Active bug: `logs/ACTIVE_BUG.md`
- Active failure: `logs/ACTIVE_FAILURE.md`

**Required Session Result Fields:**

- `outcome`: SUCCESS | BUG | FAILURE | EPIC_COMPLETE
- `timestamp`: ISO 8601 format
- `changelog_entry`: (REQUIRED for SUCCESS only) User-facing change description
- `duration_seconds`: (REQUIRED) Time spent on task in seconds
- `estimated_tokens`: (REQUIRED) Rough token count estimate
- `files_modified`: Array of changed files
- `tests_run`: Number of tests executed
- `tests_passed`: Number of tests that passed
- `build_successful`: Boolean
