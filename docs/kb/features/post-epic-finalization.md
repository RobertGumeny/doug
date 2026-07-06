---
title: Post-Epic Review, KB Synthesis, And Changelog Polish
updated: 2026-07-04
category: Features
tags: [post-epic, review, kb, changelog, finalization]
related_articles:
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/config.md
  - docs/kb/packages/git.md
  - docs/kb/features/planning-lifecycle.md
---

# Post-Epic Review, KB Synthesis, And Changelog Polish

## Overview

After an epic reaches terminal completion, Doug keeps runtime finalization separate from post-epic analysis and documentation upkeep:

1. `HandleEpicComplete` finalizes and archives the completed runtime state.
2. The advisory post-epic review runs when `review_enabled` is true.
3. The post-epic KB/changelog pass runs when `kb_enabled` is true.

Both post-epic phases are best-effort and non-gating. Their Pi-backed turns use the shared sanitized heartbeat/status indicator and print the standard end-of-turn summary. Their failures produce warnings for the operator, but they must not reopen runtime state, change the completed backlog lifecycle, or block the completed epic.

## Shared Finalization Ordering

All epic-finalization paths route through the same finalization helper:

- resumed finalization for an already-completed runtime state
- terminal `SUCCESS` on the final user task
- explicit `EPIC_COMPLETE` from an agent result

The ordering is always:

```text
HandleEpicComplete → runPostEpicReview → runPostEpicKB
```

Review runs before KB/changelog synthesis so the advisory pass inspects completed evidence before repo-facing docs are synthesized. Review errors are warning-only and do not prevent the KB/changelog pass from running.

## Advisory Review Phase

The automatic review phase uses synthetic task ID `POST_EPIC_REVIEW` and routes through Pi RPC with the documentation workflow. It creates a versioned markdown artifact under:

```text
.doug/logs/epics/{epic}/epic-review.md
.doug/logs/epics/{epic}/epic-review-v2.md
...
```

The review artifact is pre-created from a stable skeleton before the agent runs. The agent fills the skeleton in place and may write only that review artifact plus `.doug/ACTIVE_TASK.md`.

The structured review input covers these v1 dimensions:

- acceptance-criteria faithfulness
- likely regressions
- implementation coherence
- release-readiness

For each user-defined task, Doug assembles the task description, acceptance criteria, archived outcome, changelog entry, recorded commit SHA, and committed diff. Missing metrics, missing session data, missing SHAs, or unavailable diffs become warnings in the review input rather than hard failures.

## `doug review <EPIC-ID>`

`doug review <EPIC-ID>` reruns the same advisory review against completed archives without running runtime tasks, finalization, KB synthesis, or changelog polish.

The explicit command ignores `review_enabled`; that flag only controls the automatic post-run review. The command requires:

- `.doug/logs/epics/{epic}/project-state.yaml`
- `.doug/logs/epics/{epic}/tasks.yaml`
- at least one archived session under `.doug/logs/epics/{epic}/{taskID}/attempt-N/session.md`
- a completed archived state whose epic ID matches the requested ID

On success it prints the written review artifact path.

## KB And Changelog Phase

The KB/changelog phase uses synthetic task ID `POST_EPIC_KB` and routes through Pi RPC with the documentation workflow. It points the agent at:

- `docs/kb/README.md` as the KB entrypoint
- `.doug/logs/epics/{epic}/` for the finalized runtime snapshot
- `.doug/logs/epics/{epic}/{taskID}/attempt-N/session.md` for archived task results
- optional `.doug/plan/PLAN.md` for planning rationale, scope decisions, and non-goals
- a freshness signal listing changed files and inferred Go package directories from recorded commit SHAs for the just-completed epic

The brief tells the documentation agent to use that freshness signal to re-verify matching package articles under `docs/kb/packages/` and feature articles under `docs/kb/features/` before deciding what to update. The changed-file list is the complete advisory signal; inferred Go package directories are narrowed to non-test `.go` file changes, so `_test.go`-only changes still require judgment from the file list.

Allowed repository-facing outputs are intentionally narrow:

- KB output under `docs/kb/**`
- changelog polish in `CHANGELOG.md` only
- the live `.doug/ACTIVE_TASK.md` result stub

When editing the changelog, the agent may edit only `[Unreleased]`, must preserve every factual entry, must invent nothing, and must not touch released sections.

## Output Classification, Completion, And Commits

After the KB/changelog agent returns, Doug classifies pending paths into:

- KB paths: `docs/kb/**`
- changelog paths: `CHANGELOG.md`
- unrelated dirty paths

Unrelated dirty paths fail validation before completion is inferred or commits are created.

Synthetic post-epic completion is resilient, but scoped:

- **Review**: Doug pre-creates the review artifact skeleton. The pass is considered complete when that artifact differs from the skeleton; parsing the active task result is only a fallback when the artifact is still pristine.
- **KB/changelog**: Doug honors a parseable result first. `SUCCESS` and `EPIC_COMPLETE` are accepted; explicit `FAILURE` or `BUG` still fail the pass. If result frontmatter is missing or the outcome is empty, Doug may derive success only when validated in-scope `docs/kb/**` or `CHANGELOG.md` changes exist. In that case Doug logs a warning naming the in-scope paths and continues.

Missing/empty outcome with no in-scope output remains a failure. Unrelated dirty paths remain a failure even if in-scope files also changed.

In-scope outputs are committed with path-scoped commits so unrelated work is not swept into post-epic commits:

- `docs: synthesize KB for {epicID}` for changed KB files
- `docs: polish changelog for {epicID}` for changed changelog files

When both categories changed, Doug creates separate scoped commits.

## Operator Model

- Use `review_enabled: false` to skip only automatic review.
- Use `kb_enabled: false` to skip only post-epic KB/changelog synthesis.
- Use `doug review <EPIC-ID>` to rerun review for completed archives even when automatic review is disabled.
- Treat review findings as advisory evidence; follow-up work should be planned as a new epic rather than reopening completed runtime state.

## Related

- [internal/orchestrator](../packages/orchestrator.md) — finalization helper, review runner, KB runner, output classification
- [internal/agent](../packages/agent.md) — post-epic contracts and Pi routing
- [internal/config](../packages/config.md) — `review_enabled` and `kb_enabled`
- [internal/git](../packages/git.md) — committed-diff evidence and path-scoped commits
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md) — lifecycle ownership and archive model
