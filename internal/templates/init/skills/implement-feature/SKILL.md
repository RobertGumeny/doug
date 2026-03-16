---
name: "implement-feature"
description: "Execute the full feature implementation workflow: clarify scope, plan the change, implement it, verify it, and report the result according to repository instructions."
---

# Feature Implementation Workflow

Read the repository instructions first, then use this workflow when the task requires a concrete code or product change.

## Phase 1: Clarify

1. Read the task request and repository guidance
2. Identify the expected user-visible or code-visible outcome
3. Confirm the acceptance criteria and any constraints before editing

## Phase 2: Research

1. Inspect the existing code paths, tests, and docs involved in the change
2. Determine which files should be modified
3. Resolve ambiguities from existing code and repository documentation before proceeding

## Phase 3: Plan

1. Choose the smallest coherent implementation that satisfies the requirement
2. Identify any tests or docs that need to change with the code
3. If a critical requirement remains undefined, stop and report the blocker instead of guessing

## Phase 4: Implement

1. Apply the planned code and test changes
2. Keep the change scoped to the task
3. If you discover an unrelated blocking bug, report it instead of hiding it with a workaround

## Phase 5: Verify

1. Run the relevant build, test, lint, format, or static checks
2. Fix any issues introduced by the change
3. Do not report success while known relevant failures remain

## Phase 6: Report

Report the outcome using the mechanism defined by the repository instructions. Keep the summary concrete: what changed, why, and how it was verified.
