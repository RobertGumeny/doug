---
title: cmd/revert — Revert Epic Progress to a Prior Task
updated: 2026-04-14
category: Features
tags: [revert, git, reset, sha, cli]
related_articles:
  - docs/kb/packages/git.md
  - docs/kb/packages/state.md
  - docs/kb/packages/types.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# cmd/revert — Revert Epic Progress to a Prior Task

## Overview

`doug revert <task_id>` rewinds git history to the commit boundary of a completed task, then rewrites local Doug state under `.doug/` to match that boundary. This keeps `.doug/` local-only even when it is gitignored. The testable core is `doRevert(projectRoot, taskID string, force bool) error`.

## Validation Sequence

Before any destructive action, `doug revert` validates:

1. `.doug/` exists.
2. Load `project-state.yaml` and `tasks.yaml`.
3. Task ID exists in `tasks.yaml`.
4. Task status is `DONE` — only completed tasks have a commit SHA to revert to.
5. Look up the commit SHA: check `TaskMetric.CommitSHA` first; fall back to `git.LookupCommitByGrep` with a warning if `CommitSHA` is absent.
6. Confirm the SHA exists in the local repository (`SHAExists`).
7. No uncommitted changes (`HasUncommittedChanges`) — unless `--force` is passed.
8. Warn if the current branch differs from `current_epic.branch_name`.
9. Interactive confirmation prompt — requires typing `yes` exactly (bypassed by `--force`).

Any validation failure returns an error before `git reset --hard` is executed.

## Execution Phase

After validation passes:

1. Compute everything needed from the current in-memory Doug state **before** reset.
2. Rewrite `tasks.yaml` in memory so tasks through the target are `DONE` and every later task becomes `TODO`.
3. Trim `project-state.yaml` metrics after the target, recompute totals, clear transient paused/build-failure state, and rebuild `active_task` / `next_task` so `doug run` resumes at the next task after the revert point.
4. `git.ResetHard(sha, projectRoot)` — rewinds tracked repository contents to the target commit.
5. Write the rewritten local `.doug/tasks.yaml` and `.doug/project-state.yaml` back to disk.
6. Delete attempt-scoped session logs for all after-point task IDs under `.doug/logs/epics/{epic}/{taskID}/attempt-N/session.md`.
7. Print short-SHA (7-char) success message.
8. Print next-steps guidance (`doug run` to continue).
9. If `HasRemoteTrackingBranch` returns true, warn that a force-push is required and suggest `--force-with-lease`.

The confirmation prompt and next-steps text use the shared `cmd` best-effort output helper. A failed terminal write does not change revert behavior. See [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md).

## Usage Example

```bash
# Revert to the state just after EPIC-5-003 completed
doug revert EPIC-5-003

# Skip dirty-tree check and confirmation prompt
doug revert EPIC-5-003 --force
```

## Key Decisions

- **Compute before reset, rewrite after reset**: `git reset --hard` only restores tracked files. Revert therefore computes task/metric/pointer changes from the current local `.doug/` state first, then writes the reconciled `.doug/` files back after reset.
- **SHA fallback to grep**: If `TaskMetric.CommitSHA` is empty, `LookupCommitByGrep` searches for the commit message matching the task. This is a warning, not a hard error, so revert can still proceed.
- **Session log cleanup is task-scoped**: Only logs for user-defined tasks after the revert point are deleted. Logs at or before the target task are preserved.
- **`--force-with-lease` suggestion**: Safer than `--force` because it protects against overwriting remote commits you haven't seen.

## Edge Cases & Gotchas

- **Task not DONE**: Reverting to an in-progress or TODO task is rejected — there is no commit SHA to target.
- **Missing `CommitSHA`**: Tasks without a recorded `CommitSHA` use the grep fallback. Ensure commit messages follow the `feat: {taskID}` convention or the grep will not match.
- **Dirty working tree without `--force`**: Validation fails early. Use `--force` only when you understand what will be discarded.
- **Gitignored `.doug/` is supported**: `doug init` can leave `.doug/` untracked; revert no longer depends on git restoring Doug state.

## Related Topics

- [internal/git](../packages/git.md) — `ResetHard`, `LookupCommitByGrep`, `SHAExists`, `HasUncommittedChanges`, `HasRemoteTrackingBranch`
- [internal/state](../packages/state.md) — `LoadProjectState`, `LoadTasks`
- [internal/types](../packages/types.md) — `TaskMetric.CommitSHA`
