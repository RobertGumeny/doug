---
name: "manual-review"
description: "Handle a blocked-task manual review checkpoint by inspecting the current state, summarizing why automation stopped, and recording the best next action according to repository instructions."
---

# Manual Review Workflow

Read the repository instructions first, then use this workflow when the orchestrator routes a blocked task into a manual review checkpoint for human inspection.

## Phase 1: Gather Context

1. Read the active task brief, failure context, and repository guidance
2. Identify what the orchestrator was trying to achieve and what caused progress to stop
3. Inspect the relevant code, logs, or prior attempts needed to understand the blockage

## Phase 2: Assess The Blocker

1. Determine whether the task is blocked by missing requirements, conflicting constraints, repeated failed attempts, or an external dependency
2. Separate confirmed facts from your inferences
3. If the blocker is actually a fixable in-scope defect and you can resolve it safely, do so and verify the result before reporting success

## Phase 3: Decide The Next Action

1. Choose the smallest correct next step: unblock with a targeted fix, document a decision needed from a human, or record why the task should remain blocked
2. Prefer concrete next actions over generic summaries
3. Do not invent missing requirements or pretend the blocker is resolved when it is not

## Phase 4: Verify

1. Run any focused checks needed to support your conclusion
2. Confirm that your reported next step matches the current repository state
3. Do not report success unless the blocking issue has actually been resolved

## Phase 5: Report

1. Report the blocker, any work you completed, and the recommended next step using the mechanism defined by the repository instructions or task brief, if one exists. If no specific reporting mechanism is defined, report the result in your current session.
