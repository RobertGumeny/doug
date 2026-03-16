---
name: "implement-bugfix"
description: "Execute the full bugfix workflow: understand the failure, identify root cause, implement the fix, verify it, and report the result according to repository instructions."
---

# Bugfix Implementation Workflow

Read the repository instructions first, then use this workflow when the task is to diagnose and fix an existing defect.

## Phase 1: Understand the Bug

1. Read the bug report, task request, and repository guidance
2. Identify the expected behavior and the observed failure
3. Gather any reproduction steps, failing tests, or error output

## Phase 2: Diagnose

1. Reproduce or validate the defect
2. Inspect the relevant code paths and surrounding tests
3. Identify the root cause before changing code

## Phase 3: Plan the Fix

1. Design the smallest fix that addresses the root cause
2. Identify the regression coverage needed to prove the fix
3. If the bug cannot be explained from the available evidence, stop and report the blocker clearly

## Phase 4: Implement

1. Apply the fix with minimal unrelated movement
2. Add or update regression tests when behavior changes
3. If you uncover a separate out-of-scope bug, report it instead of folding it into the same patch

## Phase 5: Verify

1. Run the most relevant reproduction and regression checks first
2. Run broader build, test, lint, format, or static checks as appropriate
3. Do not report success unless the bug is fixed and the relevant verification passes

## Phase 6: Report

Report the root cause, the fix, and the verification performed using the mechanism defined by the repository instructions.
