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
`severity: non-blocking`. Blocking means the current task's acceptance criteria
cannot be verified or the would-be committed change would be wrong/unsafe;
otherwise capture the finding as non-blocking and continue. A `severity: blocking`
session bug triggers a synthetic bugfix task and must interrupt the current run;
`severity: non-blocking` bugs are archived without interrupting execution.
Planning-discovered blockers should be represented as explicit planned work
(tasks in the backlog), not as ad-hoc `severity: blocking` archive reports.

## Required Frontmatter Schema

Every reported-bug file under `.doug/intake/bugs/{epic}/` must start with YAML
frontmatter containing these required fields:

- `bug_id` — stable bug identifier, usually `bug-{task_id}` or `NB-BUG-{taskID}-{n}`
- `discovered_by_task` — task ID that discovered the issue
- `timestamp` — RFC3339 discovery timestamp
- `severity` — one of `critical`, `high`, `medium`, `low`
- `status` — one of the archive statuses below

Doug may add resolver metadata (`resolved_by`, `resolved_at`) when a synthetic
bugfix task completes.

## Status Vocabulary

Archive writers use these statuses:

- `open` — bug confirmed, not yet investigated
- `investigating` — root cause analysis in progress
- `fixed` — fix has been applied and verified
- `wont_fix` — acknowledged but will not be addressed

Planning intake treats `fixed`, `resolved`, `done`, and `closed` as terminal
statuses and excludes those reports from new planning briefs.

## Archive Destination

Non-blocking bugs are durable intake content. They are written to
`.doug/intake/bugs/{epic}/` and kept permanently as a project record even when
the current task continues without interruption.

Agents working from `ACTIVE_TASK.md` do not write a separate active bug
handoff file. All bug reporting flows through the structured `bugs:` list in
the `## Agent Result` frontmatter of `ACTIVE_TASK.md`.

## Proposed Fix (Optional)

Agent's suggestion for how to fix, if any
