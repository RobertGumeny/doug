---
title: internal/agent — Pi Backend, ActiveTask, Parse, Archive
updated: 2026-06-17
category: Packages
tags: [agent, backend, active-task, pi, rpc, frontmatter, yaml, archive, execution-prep]
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
---

# internal/agent — Pi Backend, ActiveTask, Parse, Archive

## Overview

`internal/agent` is Doug's boundary to Pi. Pi is the exclusive production agent harness.

The package owns these pieces of the lifecycle:

1. write `.doug/ACTIVE_TASK.md` with the canonical task brief and `## Agent Result` stub
2. resolve execution preparation (`PrepareExecution`) — skill name, interaction mode, and initial prompt from built-in defaults
3. dispatch supervised runs through the `Backend` interface, whose production implementation is `PiAdapter`
4. launch true terminal-interactive Pi sessions through `PiInteractiveLauncher` for planning
5. archive and clean up active task files
6. parse the authoritative `## Agent Result` block from `ACTIVE_TASK.md`
7. write backend runtime metadata sidecars
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

`RunResponse` is transport metadata only: status, duration, exit code, Pi session IDs, and restriction violations. It never carries Doug workflow outcomes. `SUCCESS`, `FAILURE`, `BUG`, and `EPIC_COMPLETE` remain authoritative only in `ACTIVE_TASK.md`.

`RunStatusTransportFailure` identifies Pi/provider transport breakage before a trustworthy workflow outcome is available. The Pi launcher sets it when RPC stdout closes before startup/prompt completion/`agent_end`, when stdout scanning fails, or when Pi exits non-zero with known transport/provider error patterns. The orchestrator handles this status before parsing `ACTIVE_TASK.md`. See [Transport Failure Recovery](../features/transport-failure-recovery.md).

## PiAdapter

```go
type PiAdapter struct{}
func NewPiAdapter() PiAdapter
func (a PiAdapter) Run(ctx context.Context, req RunRequest) (RunResponse, error)
```

`PiAdapter` translates `RunRequest` into Doug's private Pi RPC launch spec and starts:

```text
pi --mode rpc --session-dir <.doug/logs/pi-sessions/...>
```

The adapter sends `get_state`, sends the prompt payload, waits for completion events, mirrors Pi RPC output when requested, records observed Pi session IDs, and fires lifecycle hooks on cancellation/deadline expiry. Pi owns the downstream provider/model/tool lifecycle.

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
- source-owned phase interaction mode (`planning` → `interactive`; runtime/scaffold/research/post-epic KB → `rpc`)
- Doug-owned workflow prompt from `config.BuildInitialPrompt(...)`

The result is an `ExecutionPrep` with `SkillName`, `InitialPrompt`, and `InteractionMode`. Unknown task types or phases fail with a clear Doug error.

## ActiveTask and Results

`WriteActiveTask` writes the live `.doug/ACTIVE_TASK.md` brief and bottom result stub. `ParseSessionResult` reads the `## Agent Result` frontmatter block and validates outcome values. `ArchiveActiveTask` copies the live task file to `.doug/logs/sessions/{epic}/` before state changes; `CleanupActiveTask` removes the live file after handling.

## Attempt-Start Markers

`WriteAttemptStart` writes `.doug/logs/pi-sessions/{epic}/{taskID}/attempt-{N}/attempt-start.json` before the backend invocation. The JSON contains `started_at`, integer `attempt`, and `task_id`, and is written atomically through a temporary file and rename.

The marker shares the retained Pi session layout so operators can distinguish “Doug started an attempt” from “the agent completed and wrote a parseable `ACTIVE_TASK.md` result.”

## Run Metadata

`WriteRunMetadata` writes `<output log>.meta.json` containing backend-visible runtime facts. The sidecar is observability only and never replaces `ACTIVE_TASK.md` as the outcome authority.

## Related Topics

- [Doug-to-Pi Runtime Contract](../features/pi-runtime-contract.md)
- [Transport Failure Recovery](../features/transport-failure-recovery.md)
- [Interaction Model And Pi Policy Ownership](../features/execution-model.md)
- [internal/config](config.md)
- [Exec Command Pattern](../patterns/pattern-exec-command.md)
- [Drain Subprocess Pipes Before Wait](../patterns/pattern-pipe-drain.md)
