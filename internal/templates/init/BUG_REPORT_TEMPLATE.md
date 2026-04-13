---
bug_id: "bug-{task_id}"
discovered_by_task: "{task_id}"
timestamp: "{timestamp}"
severity: "blocking" | "non-blocking"
status: "open" | "in_progress" | "fixed"
---

# Bug Report: {bug_id}

## Summary

Brief one-line description of the bug

## Location

File(s) and line number(s) where bug exists

## Description

What is broken and the conditions where it appears

## Expected Behavior

What should happen

## Actual Behavior

What currently happens

## Steps to Reproduce

1. Step one
2. Step two

## Impact

How this affects the current task, related features, and whether it is blocking or can be deferred

## Routing

- Archive destination: `.doug/logs/bugs/{epic}/`
- Use `.doug/ACTIVE_BUG.md` only when `severity: "blocking"` and Doug must hand live context into a follow-up bugfix task
- Non-blocking bugs still belong in the archive even when the current task continues

## Proposed Fix (Optional)

Agent's suggestion for how to fix, if any
