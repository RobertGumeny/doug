---
title: internal/agent — Pi Backend, ActiveTask, Parse, Archive
updated: 2026-07-04
category: Packages
tags: [agent, backend, active-task, pi, rpc, frontmatter, yaml, archive, execution-prep, lifecycle, post-epic-review, post-epic-kb]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/log.md
  - docs/kb/packages/config.md
  - docs/kb/packages/templates.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/features/transport-failure-recovery.md
  - docs/kb/features/run-ux-provider-visibility.md
---

# internal/agent — Pi Backend, ActiveTask, Parse, Archive

## Overview

`internal/agent` is Doug's boundary to Pi. Pi is the exclusive production agent harness.

The package owns these pieces of the lifecycle:

1. write `.doug/ACTIVE_TASK.md` with the canonical task brief, universal lifecycle context, and `## Agent Result` stub
2. resolve execution preparation (`PrepareExecution`) — skill name, interaction mode, and initial prompt from built-in defaults
3. dispatch supervised runs through the `Backend` interface, whose production implementation is `PiAdapter`
4. launch true terminal-interactive Pi sessions through `PiInteractiveLauncher` for planning
5. archive and clean up active task files
6. parse the authoritative `## Agent Result` block from `ACTIVE_TASK.md`, including the optional structured `bugs:` list
6a. own the shared bug archive writer (`WriteBugArchive`, `UpdateBugArchiveResolved`) for durable `.doug/intake/bugs/` records
7. capture runtime observability from Pi JSONL (first response, tool calls, provider failures, heartbeat activity)
8. write pre-launch attempt-start markers in retained Pi session directories

## Pi Invocation APIs

### Backend and NewBackend

```go
type Backend interface {
    Run(ctx context.Context, req RunRequest) (RunResponse, error)
}

func NewBackend() Backend
```

`NewBackend` always returns `NewPiAdapter()` in production.

The `Backend` interface exists as a Doug seam for testing and orchestration reuse.

## RunRequest and RunResponse

`RunRequest` carries Doug-native inputs: phase, task context, canonical brief, ordered context, artifact surfaces, routing, policy, restrictions, lifecycle hooks, the Doug-owned workflow prompt, project root, heartbeat settings, and optional output writer.

`RunResponse` is transport metadata only: status, duration, exit code, Pi session IDs, restriction violations, runtime observability (`FirstResponseMs`, `ToolCallCount`, `ProviderFailures`, `ProviderFailureDetails`), and Pi `get_session_stats` token/cost data. It never carries Doug workflow outcomes. `SUCCESS`, `FAILURE`, `BUG`, and `EPIC_COMPLETE` remain authoritative only in `ACTIVE_TASK.md`.

`RunStatusTransportFailure` identifies Pi/provider transport breakage before a trustworthy workflow outcome is available. The Pi launcher sets it when RPC stdout closes before startup/prompt completion/`agent_end`, when stdout scanning fails, or when Pi exits non-zero with known transport/provider error patterns. The orchestrator handles this status before parsing `ACTIVE_TASK.md`. See [Transport Failure Recovery](../features/transport-failure-recovery.md).

## PiAdapter

```go
type PiAdapter struct{}
func NewPiAdapter() PiAdapter
func (a PiAdapter) Run(ctx context.Context, req RunRequest) (RunResponse, error)
```

`PiAdapter` translates `RunRequest` into Doug's private Pi RPC launch spec and starts:

```text
pi --mode rpc --session-dir <.doug/logs/epics/.../attempt-N>
```

The adapter sends `get_state`, sends the prompt payload, waits for completion events, requests `get_session_stats`, mirrors Pi RPC output when requested, records observed Pi session IDs, captures JSONL observability, and fires lifecycle hooks on cancellation/deadline expiry. Pi owns the downstream provider/model/tool lifecycle.

Runtime observability is collected while reading Pi JSONL:

- the first non-startup event records `FirstResponseMs` and fires `FirstResponseFn` once
- tool-call events increment `ToolCallCount`
- provider/transport diagnostics are extracted as `types.ProviderFailure` values and counted in `ProviderFailures`
- a mutex-guarded activity tracker keeps the latest sanitized heartbeat label: `tool first-path-or-command`, `generating...`, or `(no activity)`

The activity label intentionally logs only a tool name plus one safe path/file/command-like string, with whitespace normalization and aggressive truncation. It must not log file contents or multi-argument payloads.

The stdout reader (`readPiJSONL`) must drain the pipe to EOF on every exit path before `cmd.Wait()` runs — an early `return` while Pi still has buffered output deadlocks the launcher. See [Drain Subprocess Pipes Before Wait](../patterns/pattern-pipe-drain.md).

## PiInteractiveLauncher

`PiInteractiveLauncher` starts a normal visible Pi CLI session:

```text
pi --session-dir <dir> [prompt]
```

It is used for true terminal-interactive planning flows. It is separate from `PiAdapter`, which supervises JSON-RPC runs.

## Execution Preparation

`PrepareExecution(phase, taskType, taskID)` resolves:

- built-in default skill from `DefaultSkillName(taskType)`
- source-owned phase interaction mode (`planning` → `interactive`; runtime/scaffold/research/post-epic review/post-epic KB → `rpc`)
- Doug-owned workflow prompt from `config.BuildInitialPrompt(...)`

The result is an `ExecutionPrep` with `SkillName`, `InitialPrompt`, and `InteractionMode`. Unknown task types or phases fail with a clear Doug error.

## ActiveTask and Results

`WriteActiveTask` writes the live `.doug/ACTIVE_TASK.md` brief and bottom result stub. It prepends a concise `Doug Lifecycle` context section through the same `ContextSections` rendering path used by caller-supplied sections. That section tells every phase the canonical sequence is `planning → handoff → runtime tasks → post_epic_review → post_epic_kb`, and that the automatic review pass writes advisory artifacts under `.doug/logs/epics/` before the post-epic KB/changelog pass synthesizes `docs/kb/` and polishes `CHANGELOG.md` from archives and session logs.

For bugfix tasks, the `## Bug Context` section is rendered directly from the `BugID`, `BugSeverity`, `BugSourceTask`, `BugBody`, and `BugArchivePath` fields in `ActiveTaskConfig`. These are populated from the same-named fields on `TaskPointer` (persisted in `project-state.yaml`). No separate `ACTIVE_BUG.md` file is read; the payload is self-contained on the active task state and survives crash/restart. The brief points to the durable archive path as a reference but does not depend on reading it.

`ParseSessionResult` reads the `## Agent Result` frontmatter block and validates outcome values. It also parses the optional structured `bugs:` list into `[]types.SessionBug`, lowercase-normalizes each entry's severity, and rejects unknown severities with `ErrInvalidSessionBugSeverity` (carrying the offending index and value). Result files that omit `bugs:` parse unchanged. The parser does not route bugs; routing is owned by the handlers. `ArchiveActiveTask` copies the live task file to `.doug/logs/epics/{epic}/{taskID}/attempt-{N}/session.md` before state changes; `CleanupActiveTask` removes the live file after handling.

## Bug Archive Writer And Structured Bug Parsing

`internal/agent` owns the single shared writer for durable bug intake archives under `.doug/intake/bugs/{epic}/`. Handlers never hand-author bug files; they pass a `types.BugPayload` to this writer. The writer still accepts `logsDir` for compatibility, then infers sibling `.doug/intake/bugs/` as the canonical destination.

### WriteBugArchive

```go
func WriteBugArchive(logsDir, epicID string, payload types.BugPayload) (string, error)
```

`WriteBugArchive` writes under `.doug/intake/bugs/{epic}/` and stamps required frontmatter (`bug_id`, `discovered_by_task`, `timestamp`, `severity`, `status`), timestamping with the current RFC3339 time when `Timestamp` is empty. It validates `Severity` and `Status` against the closed writer vocabularies in [internal/types](types.md#bug-result-and-archive-types), returning `*ErrUnknownBugSeverity` or `*ErrUnknownBugStatus` before writing anything. It returns the absolute archive path. Repeated writes for the same task never overwrite: the writer allocates a versioned sibling (`bug-{taskID}.md`, `bug-{taskID}-v2.md`, `bug-{taskID}-v3.md`, …) via `nextBugArchivePath`. Writes are atomic (temp file + rename). `payload.Body` is appended verbatim after the frontmatter block. Planning intake later excludes terminal statuses `fixed`, `resolved`, `done`, and `closed`.

### UpdateBugArchiveResolved

```go
func UpdateBugArchiveResolved(archivePath, resolvedBy string) error
```

Used by `HandleSuccess` when a synthetic `BUG-<taskID>` bugfix task completes. It rewrites the matching archive's `status` to `fixed` and stamps resolver metadata (resolver task ID, resolved timestamp) while preserving the original report body and all required frontmatter fields. Callers treat its errors as non-fatal warnings so a missing, unreadable, or malformed archive never blocks a successful bugfix or runtime resume.

## Attempt-Start Markers

`WriteAttemptStart` writes `.doug/logs/epics/{epic}/{taskID}/attempt-{N}/attempt-start.json` before the backend invocation. The JSON contains `started_at`, integer `attempt`, and `task_id`, and is written atomically through a temporary file and rename.

The marker shares the retained Pi session layout so operators can distinguish “Doug started an attempt” from “the agent completed and wrote a parseable `ACTIVE_TASK.md` result.”

## Post-Epic Review Contract

`PostEpicReviewContract(projectRoot, dougDir, epicID)` exposes the advisory review pass as a narrow read/write contract:

- read context: project instructions, root PRD when present, `docs/kb/`, optional `.doug/plan/PLAN.md`, `CHANGELOG.md`, runtime archive, session archive, and canonical `ACTIVE_TASK.md`
- write surfaces: the review artifact under `.doug/logs/epics/{epic}/` and `.doug/ACTIVE_TASK.md` only
- restrictions: inherit read access for review evidence and enforce an allow-list for the review artifact plus the live brief

The review contract is deliberately advisory and non-gating. The agent fills a pre-created skeleton review artifact under `.doug/logs/epics/{epic}/` and must not mutate project code, KB files, runtime archives, or changelog content.

## Post-Epic KB Contract

`PostEpicKBContract(projectRoot, dougDir, epicID)` exposes the post-epic documentation pass as a narrow read/write contract:

- read context: project instructions, root PRD when present, canonical `ACTIVE_TASK.md`, optional `.doug/plan/PLAN.md`, `docs/kb/`, `CHANGELOG.md`, runtime archive, and session archive
- write surfaces: `docs/kb/`, `CHANGELOG.md`, and `.doug/ACTIVE_TASK.md` only
- restrictions: inherit read access for those context paths and enforce an allow-list for those write surfaces

`PLAN.md` is optional because manually-authored root runtime epics may not have used `doug plan`, but when present it gives the KB/changelog agent planning rationale, scope decisions, and non-goals. The canonical post-epic KB brief also includes a freshness signal assembled from recorded task commit SHAs: changed files, inferred Go package directories, and warnings for missing/unreadable commit evidence. The agent is told to re-verify matching `docs/kb/packages/` and `docs/kb/features/` articles before editing. Inferred Go package directories come from non-test `.go` changes; `_test.go`-only changes remain visible in the changed-file list but do not add a package-directory inference. The KB contract may polish only `CHANGELOG.md`'s `[Unreleased]` section, preserving every factual entry and inventing nothing; released sections are out of scope. The agent should still fill `.doug/ACTIVE_TASK.md` with a normal result, but the orchestrator may derive synthetic success from validated in-scope KB/changelog changes if provider transport prevents the final result outcome from being written.

## Run Metadata

Doug no longer writes default raw output mirrors or `<output log>.meta.json` sidecars. `.doug/logs/output/` is absent by default and should only appear for opt-in/debug capture. Normalized stats records (`stats.json` under `.doug/logs/epics/.../attempt-N/`) are the persisted runtime metadata source; retained Pi-native transcripts remain in the same attempt directory when Pi writes them.

## Related Topics

- [Doug-to-Pi Runtime Contract](../features/pi-runtime-contract.md)
- [Transport Failure Recovery](../features/transport-failure-recovery.md)
- [Run UX + Provider Stall Visibility](../features/run-ux-provider-visibility.md)
- [Interaction Model And Pi Policy Ownership](../features/execution-model.md)
- [internal/config](config.md)
- [Exec Command Pattern](../patterns/pattern-exec-command.md)
- [Drain Subprocess Pipes Before Wait](../patterns/pattern-pipe-drain.md)
