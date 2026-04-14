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

`doug revert <task_id>` rewinds the git history and session log state to the point just after a specific task completed. It runs a ten-step fail-fast validation sequence before executing `git reset --hard <sha>`. The testable core is `doRevert(projectRoot, taskID string, force bool) error`.

## Validation Sequence

Before any destructive action, `doug revert` validates:

1. Load `tasks.yaml` — confirm the task list is readable.
2. Load `project-state.yaml` — confirm metrics are accessible.
3. Task ID exists in `tasks.yaml`.
4. Task status is `DONE` — only completed tasks have a commit SHA to revert to.
5. `project-state.yaml` is tracked by git (`IsFileTracked`) — confirms a git repo exists.
6. Current branch matches the epic branch from `tasks.yaml`.
7. No uncommitted changes (`HasUncommittedChanges`) — unless `--force` is passed.
8. Look up the commit SHA: check `TaskMetric.CommitSHA` first; fall back to `git.LookupCommitByGrep` with a warning if `CommitSHA` is absent.
9. Confirm the SHA exists in the local repository (`SHAExists`).
10. Interactive confirmation prompt — requires typing `yes` exactly (bypassed by `--force`).

Any validation failure returns an error before `git reset --hard` is executed.

## Execution Phase

After validation passes:

1. Collect task IDs **after** the revert point (in-memory, before reset overwrites `tasks.yaml` on disk).
2. `git.ResetHard(sha, projectRoot)` — rewinds to the target commit.
3. Delete session logs for all after-point task IDs: globs `session-{id}_attempt-*.md` under `.doug/logs/sessions/{epic}/`.
4. Print short-SHA (7-char) success message.
5. Print next-steps guidance (`doug run` to continue).
6. If `HasRemoteTrackingBranch` returns true, warn that a force-push is required and suggest `--force-with-lease`.

The confirmation prompt and next-steps text use the shared `cmd` best-effort output helper. A failed terminal write does not change revert behavior. See [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md).

## Usage Example

```bash
# Revert to the state just after EPIC-5-003 completed
doug revert EPIC-5-003

# Skip dirty-tree check and confirmation prompt
doug revert EPIC-5-003 --force
```

## Key Decisions

- **In-memory task list before reset**: Task IDs after the revert point are collected from the loaded `tasks.Tasks.Epic.Tasks` slice before `git.ResetHard` runs, so the list is correct even after reset overwrites `tasks.yaml`.
- **SHA fallback to grep**: If `TaskMetric.CommitSHA` is empty, `LookupCommitByGrep` searches for the commit message matching the task. This is a warning, not a hard error, so revert can still proceed.
- **Session log cleanup is task-scoped**: Only logs for user-defined tasks after the revert point are deleted. The command does not add synthetic post-epic or documentation task IDs on its own.
- **`--force-with-lease` suggestion**: Safer than `--force` because it protects against overwriting remote commits you haven't seen.

## Edge Cases & Gotchas

- **Task not DONE**: Reverting to an in-progress or TODO task is rejected — there is no commit SHA to target.
- **Missing `CommitSHA`**: Tasks without a recorded `CommitSHA` use the grep fallback. Ensure commit messages follow the `feat: {taskID}` convention or the grep will not match.
- **Dirty working tree without `--force`**: Validation fails early. Use `--force` only when you understand what will be discarded.

## Related Topics

- [internal/git](../packages/git.md) — `ResetHard`, `LookupCommitByGrep`, `SHAExists`, `IsFileTracked`, `HasUncommittedChanges`, `HasRemoteTrackingBranch`
- [internal/state](../packages/state.md) — `LoadProjectState`, `LoadTasks`
- [internal/types](../packages/types.md) — `TaskMetric.CommitSHA`
