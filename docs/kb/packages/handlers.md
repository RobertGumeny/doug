---
title: internal/handlers — Outcome Handlers & LoopContext
updated: 2026-02-24
category: Packages
tags: [handlers, success, failure, bug, epic, loop-context, orchestration]
related_articles:
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/types.md
  - docs/kb/packages/state.md
  - docs/kb/packages/git.md
  - docs/kb/packages/metrics.md
  - docs/kb/packages/changelog.md
  - docs/kb/infrastructure/go.md
---

# internal/handlers — Outcome Handlers

## Overview

`internal/handlers` implements the four outcome handlers for the orchestration loop. Each handler receives a `*orchestrator.LoopContext` and performs the full response sequence for one agent outcome: SUCCESS, FAILURE, BUG, or EPIC_COMPLETE.

All four handlers share `protectedPaths`, a package-level var listing state files that must survive `git.RollbackChanges`:

```go
var protectedPaths = []string{"project-state.yaml", "tasks.yaml"}
```

---

## LoopContext

`LoopContext` is defined in `internal/orchestrator/context.go` and carries all per-iteration state. Every handler receives exactly one `*LoopContext` parameter.

```go
type LoopContext struct {
    // Per-iteration identity (snapshotted after IncrementAttempts)
    TaskID       string
    TaskType     types.TaskType
    Attempts     int
    CurrentEpic  types.EpicState

    // Agent output
    SessionResult *types.SessionResult

    // Config and infrastructure
    Config      *config.OrchestratorConfig
    BuildSystem build.BuildSystem
    ProjectRoot string
    TaskStartTime time.Time

    // Mutable state — mutated in memory and persisted by handlers
    State *types.ProjectState
    Tasks *types.Tasks

    // File system paths used by handlers
    StatePath     string  // project-state.yaml
    TasksPath     string  // tasks.yaml
    LogsDir       string  // logs/
    ChangelogPath string  // CHANGELOG.md
}
```

`LoopContext` is constructed fresh each iteration in `cmd/run.go` after `IncrementAttempts` and `ParseSessionResult`. Handler mutations to `ctx.State` and `ctx.Tasks` are visible to the next handler in the same iteration.

---

## HandleSuccess

```go
func HandleSuccess(ctx *orchestrator.LoopContext) (SuccessResult, error)
```

### SuccessResultKind

```go
type SuccessResultKind int

const (
    Continue     SuccessResultKind = iota  // normal forward progress
    Retry                                   // non-fatal; loop retries next iteration
    EpicComplete                            // KB synthesis done; caller runs HandleEpicComplete
)
```

### Sequence

1. **Install dependencies** — if `SessionResult.DependenciesAdded` is non-empty, call `BuildSystem.Install()`. On failure: rollback → return `Retry`.
2. **Build** — `BuildSystem.Build()`. On failure: rollback → return `Retry`.
3. **Test** — `BuildSystem.Test()`. On failure: rollback → return `Retry`.
4. **Record metrics** — `metrics.RecordTaskMetrics(ctx.State, taskID, "success", duration)`. Non-fatal.
5. **Changelog** — `changelog.UpdateChangelog(...)` if `ChangelogEntry != ""`. Non-fatal (logs warning on error).
6. **Mark task DONE** — `orchestrator.UpdateTaskStatus(...)` + `state.SaveTasks(...)`. Skipped for synthetic tasks (`IsSynthetic() == true`).
7. **Documentation task branch** — if `TaskType == TaskTypeDocumentation`: set `CurrentEpic.CompletedAt`, save state, commit (`"docs: " + taskID`), return `EpicComplete`.
8. **Advance or inject KB** — if `NeedsKBSynthesis()`: inject `KB_UPDATE` documentation task. Otherwise: `AdvanceToNextTask()`.
9. **Save state** — `state.SaveProjectState(...)`.
10. **Commit** — `git.Commit(commitMsg, ...)`. On failure: log warning, return `Retry` (non-fatal).
11. Return `Continue`.

### Commit message convention

| Task type | Prefix |
|-----------|--------|
| `feature` | `feat:` |
| `bugfix`  | `fix:`  |
| `documentation` | `docs:` |

### Key decisions

- **Rollback preserves `protectedPaths`**: After a build/test failure, rollback resets agent code changes but keeps `project-state.yaml` and `tasks.yaml` — the orchestrator's `IncrementAttempts` write survives.
- **Git commit failure is non-fatal**: Returns `Retry` without an error. The state/tasks writes already persisted to disk; the state machine recovers on the next iteration.
- **Documentation task sets `CompletedAt` before returning `EpicComplete`**: `HandleEpicComplete` receives a fully timestamped state.

---

## HandleFailure

```go
func HandleFailure(ctx *orchestrator.LoopContext) error
```

### Sequence

1. **Rollback** — `git.RollbackChanges(...)`. Non-fatal (log warning and continue).
2. **Record metrics** — `metrics.RecordTaskMetrics(..., "failure", ...)`. Non-fatal.
3. **Check retry count**:
   - `ctx.Attempts < cfg.MaxRetries` → log warning, return `nil` (loop retries).
   - `ctx.Attempts >= cfg.MaxRetries` → block the task:
     - Archive `logs/ACTIVE_FAILURE.md` to `logs/failures/{epic}/failure-{taskID}.md`. Non-fatal if file is absent.
     - Mark task `BLOCKED` in `tasks.yaml`. Skipped for synthetic tasks.
     - Set `active_task.type = manual_review` in state and save.
     - Return `fmt.Errorf("task %s blocked after %d attempts: requires manual review", ...)`.

### Archive path

```
logs/ACTIVE_FAILURE.md  →  logs/failures/{epic}/failure-{taskID}.md
```

Source is always the **flat** `logs/ACTIVE_FAILURE.md` path (CI-2 fix — previous Bash orchestrator used a subdirectory path inconsistently).

### Key decisions

- Rollback errors are non-fatal — execution always proceeds to the metrics step.
- Missing `ACTIVE_FAILURE.md` is non-fatal — archive is skipped with a warning; the task is still blocked.
- Synthetic tasks (`IsSynthetic() == true`) skip the `BLOCKED` write to `tasks.yaml` since synthetic tasks are never in `tasks.yaml`.

---

## HandleBug

```go
func HandleBug(ctx *orchestrator.LoopContext) error
```

### Sequence

1. **Nested bug check** — if `TaskType == TaskTypeBugfix`, return fatal error immediately (Tier 3). A bugfix task reporting BUG would create a death spiral.
2. **Rollback** — non-fatal.
3. **Record metrics** — `metrics.RecordTaskMetrics(..., "bug", ...)`. Non-fatal.
4. **Generate bug ID** — `"BUG-" + ctx.TaskID` (e.g., `BUG-EPIC-5-001`).
5. **Archive** — copy `logs/ACTIVE_BUG.md` to `logs/bugs/{epic}/bug-{taskID}.md`. Non-fatal if absent.
6. **Schedule bugfix** — set `active_task = { type: bugfix, id: BUG-{taskID} }`.
7. **Preserve interrupted task** — set `next_task = { type: resolveInterruptedType(), id: ctx.TaskID }`.
8. **Save state**.

### resolveInterruptedType

```go
// Synthetic tasks: return ctx.TaskType directly (they're never in tasks.yaml — CI-5 fix)
// User-defined tasks: look up by ID in ctx.Tasks.Epic.Tasks
// Fallback: ctx.TaskType with a warning log
```

**CI-5 fix**: Before this fix, the bug handler always searched `tasks.yaml` for the interrupted task type. Synthetic tasks (documentation) are not in `tasks.yaml`, so the lookup always failed, causing a deadlock where the orchestrator could never resume after a bug during KB synthesis.

### Archive path

```
logs/ACTIVE_BUG.md  →  logs/bugs/{epic}/bug-{taskID}.md
```

Source is always the **flat** `logs/ACTIVE_BUG.md` (CI-1 fix).

### Key decisions

- Nested bug check runs **before** rollback — prevents partial state mutation when the handler must abort.
- Returns `nil` for non-nested bugs — the bug handler is not a fatal exit; the loop continues with the new bugfix task.
- Missing `ACTIVE_BUG.md` is non-fatal — bug is still scheduled; the file may have been absent due to a prior iteration error.

---

## HandleEpicComplete

```go
func HandleEpicComplete(ctx *orchestrator.LoopContext) error
```

### Sequence

1. **Print summary** — `metrics.PrintEpicSummary(ctx.State)`.
2. **Commit finalization** — `git.Commit("chore: finalize {epicID}", ctx.ProjectRoot)`:
   - `git.ErrNothingToCommit` → non-fatal; log info and continue (all changes were already committed by prior handlers).
   - Any other error → return explicit error (Tier 3; CI-6 fix — never swallow commit failures silently).
3. **Print completion banner** — `log.Section("EPIC {epicID} COMPLETE")`.

### CI-6 fix

The Bash orchestrator silently swallowed epic commit failures. The Go port explicitly returns the error so `cmd/run.go` surfaces it as exit code 1.

---

## Run Loop Integration (cmd/run.go)

The full pre-loop and main loop sequence:

### Pre-loop

```
LoadConfig → apply CLI overrides → CheckDependencies
→ LoadProjectState + LoadTasks → BootstrapFromTasks
→ IsEpicAlreadyComplete (exit 0 if done)
→ NewBuildSystem → EnsureProjectReady
→ ValidateYAMLStructure → EnsureEpicBranch
→ InitializeTaskPointers
→ ValidateStateSync (skipped for synthetic active task)
→ SaveProjectState
```

### Main loop

```
for iteration < MaxIterations:
    IncrementAttempts → SaveProjectState (persist attempt count before agent)
    CreateSessionFile → WriteActiveTask
    RunAgent (non-zero exit is non-fatal)
    ParseSessionResult (parse failure → treat as FAILURE)

    switch outcome:
      SUCCESS   → HandleSuccess → [EpicComplete→HandleEpicComplete→exit 0 | Continue | Retry]
      FAILURE   → HandleFailure → [fatal error→exit 1 | nil→retry]
      BUG       → HandleBug → [fatal error→exit 1 | nil→continue]
      EPIC_COMPLETE → HandleEpicComplete → [error→exit 1 | nil→exit 0]

max iterations reached → exit 0
```

### Exit code policy

| Condition | Exit code |
|-----------|-----------|
| All tasks DONE | 0 |
| Max iterations reached | 0 |
| `HandleEpicComplete` returns nil | 0 |
| Nested bug detected | 1 |
| Task blocked (max retries) | 1 |
| `HandleEpicComplete` returns error | 1 |
| Fatal state/git error | 1 |

### CLI flags

All flags are applied only when explicitly set via `cmd.Flags().Changed("flag-name")`. This correctly distinguishes `--max-retries=0` from "not passed":

| Flag | Config field |
|------|-------------|
| `--agent` | `AgentCommand` |
| `--build-system` | `BuildSystem` |
| `--max-retries` | `MaxRetries` |
| `--max-iterations` | `MaxIterations` |
| `--kb-enabled` | `KBEnabled` |

### ValidateStateSync skipped for synthetic tasks

`ValidateStateSync` always returns `ValidationFatal` for synthetic active tasks (they are absent from `tasks.yaml` by design). The run loop guards this call with `IsSynthetic()`:

```go
if !projectState.ActiveTask.Type.IsSynthetic() {
    vResult, vErr := orchestrator.ValidateStateSync(...)
    ...
}
```

---

## Related Topics

- [internal/orchestrator](orchestrator.md) — BootstrapFromTasks, task pointers, ValidateStateSync, CheckDependencies, EnsureProjectReady
- [internal/types](types.md) — TaskType, Outcome constants, TaskPointer
- [internal/state](state.md) — SaveProjectState, SaveTasks (called by every handler)
- [internal/git](git.md) — RollbackChanges, Commit, ErrNothingToCommit
- [internal/metrics](metrics.md) — RecordTaskMetrics, PrintEpicSummary
- [internal/changelog](changelog.md) — UpdateChangelog
- [Go Infrastructure](../infrastructure/go.md) — three failure tiers, exec/atomic conventions
