---
title: internal/orchestrator — Core Orchestration Logic
updated: 2026-02-24
category: Packages
tags: [orchestrator, bootstrap, task-pointers, validation, state-management, loop-context, startup]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/state.md
  - docs/kb/packages/handlers.md
  - docs/kb/infrastructure/go.md
---

# internal/orchestrator — Core Orchestration Logic

## Overview

`internal/orchestrator` contains the three files that govern the orchestrator main loop: startup bootstrapping, task pointer management, and state/task consistency validation. All functions operate in-memory — callers are responsible for calling `SaveProjectState`/`SaveTasks` after mutating state.

## bootstrap.go

### API

```go
func BootstrapFromTasks(state *types.ProjectState, tasks *types.Tasks)
func NeedsKBSynthesis(state *types.ProjectState, tasks *types.Tasks) bool
func IsEpicAlreadyComplete(state *types.ProjectState, tasks *types.Tasks) bool
```

### BootstrapFromTasks

No-op when `state.CurrentEpic.ID != ""`. On first run, populates `current_epic` (id, name, branch name as `"feature/" + epic.ID`, RFC3339 started_at), `active_task` (first task), and `next_task` (second task or zero value).

**Guard**: The `CurrentEpic.ID != ""` check is the bootstrapped sentinel. Do not change this condition — it's how the orchestrator distinguishes first-run from restart.

### NeedsKBSynthesis

Returns `true` only when all of these hold:
1. `state.KBEnabled == true`
2. `state.ActiveTask.Type != TaskTypeDocumentation` (KB not already running)
3. No task has `Status == TODO` or `Status == IN_PROGRESS`

Used by the orchestrator loop to decide whether to inject a synthetic KB_UPDATE task.

### IsEpicAlreadyComplete

Returns `true` when all user-defined tasks are `DONE` **and** either:
- `state.KBEnabled == false` (no KB synthesis required), **or**
- `state.ActiveTask.Type == TaskTypeDocumentation` (KB synthesis ran in a previous iteration and completed)

Called at the **top** of each orchestrator loop iteration (before running an agent). When KB synthesis runs, `AdvanceToNextTask` returns `false` and active stays as documentation. The *next* top-of-loop check then returns `true`.

## taskpointers.go

### API

```go
func InitializeTaskPointers(state *types.ProjectState, tasks *types.Tasks)
func AdvanceToNextTask(state *types.ProjectState, tasks *types.Tasks) bool
func FindNextActiveTask(tasks *types.Tasks) (id string, taskType types.TaskType)
func IncrementAttempts(state *types.ProjectState)
func UpdateTaskStatus(tasks *types.Tasks, id string, status types.Status) error
```

### InitializeTaskPointers

Selection order for `active_task`:
1. First `IN_PROGRESS` task (handles orchestrator-restart recovery)
2. First `TODO` task (normal forward progress)

`next_task` is set to the first `TODO` that appears **after** the selected active task in the list (positional search, not global first-match).

If no user tasks remain and `kb_enabled == true`, injects a synthetic `KB_UPDATE` documentation task.

### AdvanceToNextTask

Returns `false` immediately (no state mutation) if `NextTask.ID == ""`. On success:
- Promotes `NextTask → ActiveTask`, resets `Attempts` to `0`
- Finds new `NextTask`: first `TODO` appearing after the newly active task (positional)
- Returns `true`

**Caller must call `IncrementAttempts` at the start of the next iteration** — `AdvanceToNextTask` resets to zero intentionally.

### FindNextActiveTask vs InitializeTaskPointers next-finding

These use *different* algorithms:
- `FindNextActiveTask`: global first-match (IN_PROGRESS preferred, then TODO) — for scanning the whole list
- `AdvanceToNextTask`/`InitializeTaskPointers`: positional (first TODO *after* the current active) — for sequential advance

Do not conflate them.

### UpdateTaskStatus

Returns a descriptive error for unknown IDs — no silent no-ops. Always check the return value.

## validation.go

### API

```go
type ValidationKind int  // ValidationOK | ValidationAutoCorrected | ValidationFatal
type ValidationResult struct { Kind ValidationKind; Description string }

func ValidateYAMLStructure(state *types.ProjectState, tasks *types.Tasks) error
func ValidateStateSync(state *types.ProjectState, tasks *types.Tasks) (ValidationResult, error)
```

### ValidateYAMLStructure

Structural sanity check run before any orchestration logic. Returns an error if:
- `state.CurrentEpic.ID` is empty
- `state.ActiveTask.Type` is empty
- `state.ActiveTask.ID` is empty
- Any task has an unrecognized `Status` value (must be `TODO`, `IN_PROGRESS`, `DONE`, or `BLOCKED`)

### ValidateStateSync — Tiered Recovery

Checks if `state.ActiveTask.ID` refers to a real task in `tasks.yaml`:

| Condition | Tier | Outcome |
|-----------|------|---------|
| ID found | — | `ValidationOK`, no mutation |
| ID not found, synthetic active type (`bugfix`/`documentation`) | 3 | `ValidationFatal` + error — manual intervention required |
| ID not found, exactly 1 TODO/IN_PROGRESS candidate | 2 | `ValidationAutoCorrected`, state redirected, `Attempts` preserved |
| ID not found, 0 or 2+ candidates | 3 | `ValidationFatal` + error |

**Key**: `AutoCorrected` is not an error — the function returns `(result, nil)`. The caller should log `result.Description` as a warning and continue.

**`Attempts` is preserved** on auto-correction: the attempt count is still relevant after a redirect.

**Synthetic type mismatch is always Fatal**: synthetic tasks are intentionally absent from `tasks.yaml`. Any not-found for a synthetic active task is inherently ambiguous.

### ValidationKind is an int enum

`ValidationKind` uses `int` (not `string`) to keep comparisons zero-allocation. Use `result.Description` for human-readable output; compare `result.Kind` directly.

## Call Order in the Orchestrator Loop

```
top of loop:
  IsEpicAlreadyComplete → exit if true
  ValidateYAMLStructure → fatal exit on error
  ValidateStateSync     → fatal exit on ValidationFatal; log warning on AutoCorrected
  NeedsKBSynthesis      → inject KB_UPDATE if true
  IncrementAttempts
  ... run agent ...
  UpdateTaskStatus
  AdvanceToNextTask
  SaveProjectState / SaveTasks
```

## context.go — LoopContext

`LoopContext` carries all per-iteration state for the orchestration loop. Defined here so `internal/handlers` (which imports `internal/orchestrator`) can reference it without a circular dependency.

See [internal/handlers](handlers.md) for the full field list and usage. The key rule: `LoopContext` is constructed in `cmd/run.go` after `IncrementAttempts`, snapshotting `TaskID`, `TaskType`, and `Attempts`, then passed to handler functions. Mutations to `ctx.State` and `ctx.Tasks` persist in memory across handlers within one iteration.

## startup.go

### CheckDependencies

```go
func CheckDependencies(cfg *config.OrchestratorConfig) error
```

Verifies that all required binaries are on `PATH` before the loop starts:
- `cfg.AgentCommand` (e.g., `"claude"`)
- `"git"` (always required)
- `"go"` (default build system) or `"npm"` (when `cfg.BuildSystem == "npm"`)

Returns a single error listing all missing binaries; nil if all are present. Called once in the pre-loop sequence of `cmd/run.go`.

### EnsureProjectReady

```go
func EnsureProjectReady(buildSys build.BuildSystem, cfg *config.OrchestratorConfig) error
```

Runs a pre-flight `Build()` then `Test()` to verify the project is in a clean state before the orchestration loop begins.

- If `buildSys.IsInitialized()` returns `false` (e.g., `go.sum` absent for Go projects): emits a visible warning and returns `nil`. Handles fresh checkouts.
- Any build or test failure returns an error already containing the last 50 lines of output (embedded by the `BuildSystem` implementations). Treat as fatal.

Called once in the pre-loop sequence, **after** `CheckDependencies` and **before** `ValidateYAMLStructure`.

## Updated Call Order in the Orchestrator Loop

```
pre-loop:
  LoadConfig → apply CLI overrides
  CheckDependencies → fatal on missing binary
  LoadProjectState + LoadTasks
  BootstrapFromTasks
  IsEpicAlreadyComplete → exit 0 if done
  NewBuildSystem → EnsureProjectReady → fatal on build/test failure
  ValidateYAMLStructure → fatal on structural error
  EnsureEpicBranch
  InitializeTaskPointers
  ValidateStateSync (skipped for synthetic active task)
  SaveProjectState

main loop (per iteration):
  IncrementAttempts → SaveProjectState (persist before agent)
  CreateSessionFile → WriteActiveTask → RunAgent → ParseSessionResult
  → handler dispatch (HandleSuccess / HandleFailure / HandleBug / HandleEpicComplete)
```

## Related

- [types.md](./types.md) — structs and typed constants used throughout
- [state.md](./state.md) — SaveProjectState, SaveTasks (callers must persist after mutations)
- [handlers.md](./handlers.md) — outcome handlers; LoopContext field reference
- [go.md](../infrastructure/go.md) — three failure tiers and exec/atomic conventions
