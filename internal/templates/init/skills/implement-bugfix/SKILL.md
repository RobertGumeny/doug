---
name: implement-bugfix
description: Execute the full bugfix workflow including root cause analysis, fix implementation, regression testing, and reporting. Use when project-state.yaml indicates active_task.type is "bugfix". Agents are STATELESS - they read YAML/code, write code/session results, but NEVER touch Git or YAML updates.
---

# Bugfix Implementation Workflow

This skill guides you through the complete bug resolution process from diagnosis to session reporting.

## Agent Boundaries (Critical)

**You ARE allowed to:**

- ✅ Read `project-state.yaml`, `tasks.yaml`, `.doug/PRD.md`, and code
- ✅ Read the active bug report from the path provided in your briefing
- ✅ Write/modify source code and tests
- ✅ Run build, test, and lint commands
- ✅ Write session results to the path provided in your briefing
- ✅ Write new bug reports to the path provided in your briefing (if you find additional bugs)
- ✅ Write failure reports to the path provided in your briefing

**You are NOT allowed to:**

- ❌ Run ANY Git commands (checkout, commit, push, branch, etc.)
- ❌ Modify `project-state.yaml` or `tasks.yaml`
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive the active bug report (orchestrator does this)
- ❌ Delete or modify archived bug reports

**The orchestrator handles:** Git operations, YAML updates, CHANGELOG updates, file archiving.

## Phase 1: Research

1. Read `.doug/ACTIVE_TASK.md` to get task metadata and **the paths provided in your briefing**:
   - **Session File**, **Active Bug File**, **Failure File** paths
   - **Task ID**, **Task Type**, **Attempt** number

2. **Read the active bug report**: use the **Active Bug File** path from your briefing
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
4. **Termination Clause**: If root cause cannot be determined, write the failure report and exit with `outcome: FAILURE`

## Phase 3: Implement

1. Make minimal changes necessary to fix the bug — don't refactor unrelated code
2. Write regression tests that reproduce the original bug scenario
3. If you discover ADDITIONAL bugs while fixing this one: document in a comment, note in session result, do NOT write a new bug report

## Phase 4: Verify

1. **Build**: run the project build command and fix ALL errors
2. **Test**: ensure ALL tests pass, especially new regression tests
3. **Lint** (if available): fix all linter errors

## Phase 5: Report

### Session Result Path

Use the **Session File** path from your briefing.

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

Write the failure report to the **Failure File** path from your briefing, then write session result with `outcome: FAILURE`.

## Quick Reference

**Outcome Values:** `SUCCESS` | `FAILURE`

**File Locations:** see paths in your briefing header (`.doug/ACTIVE_TASK.md`)
