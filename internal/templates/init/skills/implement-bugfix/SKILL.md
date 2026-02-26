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
- ✅ Run build, test, and lint commands
- ✅ Write session results to `logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md`
- ✅ Write new bug reports to `logs/ACTIVE_BUG.md` (if you find additional bugs)
- ✅ Write failure reports to `logs/ACTIVE_FAILURE.md`

**You are NOT allowed to:**

- ❌ Run ANY Git commands (checkout, commit, push, branch, etc.)
- ❌ Modify `project-state.yaml` or `tasks.yaml`
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive `logs/ACTIVE_BUG.md` (orchestrator does this)
- ❌ Delete or modify archived bug reports

**The orchestrator handles:** Git operations, YAML updates, CHANGELOG updates, file archiving.

## Phase 1: Research

1. Read `logs/ACTIVE_TASK.md` to get:
   - **Task ID**, **Task Type**, and **Session File** path
   - **Attempt** number (current attempt / max retries)

2. **Read the active bug report**: `logs/ACTIVE_BUG.md`
   - Understand what's broken
   - Note the location in the codebase
   - Review steps to reproduce

3. **Pre-Flight Check**: Verify the bug hasn't already been resolved
   - If the bug no longer exists, write session result with `outcome: SUCCESS` and exit

4. Examine the code around the bug location to understand the system

## Phase 2: Plan

1. **Root Cause Analysis**: Identify WHY the bug exists
2. **Propose a Fix**: Design a minimal solution that addresses the root cause
3. **Define Regression Tests**: Plan tests that would have caught this bug
4. **Termination Clause**: If root cause cannot be determined, write `logs/ACTIVE_FAILURE.md` and exit with `outcome: FAILURE`

## Phase 3: Implement

1. Make minimal changes necessary to fix the bug — don't refactor unrelated code
2. Write regression tests that reproduce the original bug scenario
3. If you discover ADDITIONAL bugs while fixing this one: document in a comment, note in session result, do NOT write a new `ACTIVE_BUG.md`

## Phase 4: Verify

1. **Build**: run the project build command and fix ALL errors
2. **Test**: ensure ALL tests pass, especially new regression tests
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
changelog_entry: "Fixed [brief description of what was broken]"
dependencies_added: []
---

## Bugfix Summary
## Root Cause
## Solution
## Regression Tests Added
```

### On Failure (After 5 Attempts)

Write `logs/ACTIVE_FAILURE.md`, then write session result with `outcome: FAILURE`.

## Quick Reference

**Outcome Values:** `SUCCESS` | `FAILURE`

**File Locations:**
- Active bug report (input): `logs/ACTIVE_BUG.md`
- Session result (output): `logs/sessions/{epic}/session-{bug_id}_attempt-{attempts}.md`
- Active failure (if needed): `logs/ACTIVE_FAILURE.md`
