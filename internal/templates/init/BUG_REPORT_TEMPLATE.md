---
bug_id: "bug-{task_id}"
discovered_by_task: "{task_id}"
timestamp: "{timestamp}"
severity: "high"
status: "open"
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

## Severity Guide

The `severity` field uses the archive loader vocabulary:

- `critical` — data loss, security issue, or complete system failure
- `high` — significant feature breakage; most bugs discovered during tasks
- `medium` — degraded behaviour with a workaround available
- `low` — cosmetic or minor deviation from spec

Archive reports (this file) always use `severity: critical | high | medium | low`.

**Session result routing is separate.** Agents working from `ACTIVE_TASK.md` report bugs
through that file's structured `bugs:` result field using `severity: blocking` or
`severity: non-blocking`. A `severity: blocking` session bug triggers a synthetic
bugfix task and must interrupt the current run; `severity: non-blocking` bugs are
archived without interrupting execution. Planning-discovered blockers should be
represented as explicit planned work (tasks in the backlog), not as ad-hoc
`severity: blocking` archive reports.

## Status Vocabulary

The `status` field must be one of:

- `open` — bug confirmed, not yet investigated
- `investigating` — root cause analysis in progress
- `fixed` — fix has been applied and verified
- `wont_fix` — acknowledged but will not be addressed

## Archive Destination

Non-blocking bugs are durable archive content. They are written to
`.doug/logs/bugs/{epic}/` and kept permanently as a project record even when
the current task continues without interruption.

Agents working from `ACTIVE_TASK.md` do not write a separate active bug
handoff file. All bug reporting flows through the structured `bugs:` list in
the `## Agent Result` frontmatter of `ACTIVE_TASK.md`.

## Proposed Fix (Optional)

Agent's suggestion for how to fix, if any
