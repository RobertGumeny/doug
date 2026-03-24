---
title: internal/orchestrator — Core Orchestration Logic
updated: 2026-03-16
category: Packages
tags: [orchestrator, bootstrap, task-pointers, validation, state-management, loop-context, startup, paths, context]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/state.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/log.md
  - docs/kb/infrastructure/go.md
---

# internal/orchestrator — Core Orchestration Logic

## Overview

`internal/orchestrator` contains the `Orchestrator` struct and full orchestration lifecycle (`run.go`), plus supporting files for bootstrapping, task pointer management, startup checks, and state validation. All mutation functions operate in-memory — callers are responsible for calling `SaveProjectState`/`SaveTasks` after mutating state.

> **EPIC-12**: The orchestration loop was extracted from `cmd/run.go` into `Orchestrator.Run`. `LoopContext` moved to `internal/types`. `UpdateTaskStatus`, `NeedsKBSynthesis`, `AdvanceToNextTask` moved to `internal/types/task_ops.go` (orchestrator stills exports forwarding wrappers). A new `Paths` struct consolidates all `.doug/` path derivation.

## orchestrator.go — Orchestrator Struct

```go
type Orchestrator struct {
    cfg         *config.OrchestratorConfig
    paths       Paths
    logger      log.Logger
    buildSystem build.BuildSystem
}

func New(cfg *config.OrchestratorConfig, paths Paths) (*Orchestrator, error)
```

`New` constructs the orchestrator: resolves the `BuildSystem` from `cfg.BuildSystem` and `paths.ProjectRoot`, creates a `log.New()` stderr logger. Returns an error if the build system identifier is unrecognized.

Called from `cmd/run.go`:

```go
func runOrchestrate(cmd *cobra.Command, args []string) error {
    cfg, paths, err := loadConfig(cmd)
    if err != nil { return err }
    orch, err := orchestrator.New(cfg, paths)
    if err != nil { return err }
    return orch.Run(cmd.Context())
}
```

## paths.go — Paths

```go
type Paths struct {
    ProjectRoot      string // absolute path to the project root
    DougDir          string // <root>/.doug
    ConfigPath       string // <root>/.doug/doug.yaml
    StatePath        string // <root>/.doug/project-state.yaml
    TasksPath        string // <root>/.doug/tasks.yaml
    LogsDir          string // <root>/.doug/logs
    ChangelogPath    string // <root>/CHANGELOG.md
    SkillsConfigPath string // <root>/.doug/skills-config.yaml
}

func NewPaths(projectRoot string) Paths
```

`NewPaths` derives all paths from the project root with a single call. Used in `cmd/config.go:loadConfig` and passed to `orchestrator.New`. Do not construct `Paths` manually — always use `NewPaths`.

## context.go — LoopContext alias

```go
// LoopContext is a type alias for types.LoopContext.
type LoopContext = types.LoopContext
```

`orchestrator.LoopContext` is preserved as an alias so existing callers compile without change. The canonical definition lives in `internal/types/loop_context.go`. See [internal/types](types.md) for the full field reference.

```go
// AgentResult captures agent output for explicit handler dispatch.
type AgentResult struct {
    SessionResult   *types.SessionResult
    DurationSeconds int
}
```

`AgentResult` is defined here but not currently used as a struct in dispatch — `Orchestrator.Run` passes `result` and `durationSeconds` directly as parameters to handler calls.

## bootstrap.go

### API

```go
func PrepareForEpicRollover(state *types.ProjectState, tasks *types.Tasks) (bool, error)
func BootstrapFromTasks(state *types.ProjectState, tasks *types.Tasks)
func NeedsKBSynthesis(state *types.ProjectState, tasks *types.Tasks, kbEnabled bool) bool
func IsEpicAlreadyComplete(state *types.ProjectState, tasks *types.Tasks, kbEnabled bool) bool
```

### PrepareForEpicRollover

Detects when `tasks.yaml` has switched to a new epic ID and conditionally resets runtime state for seamless next-epic bootstrap.

- If `state.current_epic.id == ""` or IDs match: no-op.
- If IDs differ and `current_epic.completed_at` is empty: returns a fatal guardrail error.
- If IDs differ and previous epic is completed: resets `current_epic`, `active_task`, `next_task`, and per-epic metrics, then caller runs `BootstrapFromTasks`.

### BootstrapFromTasks

No-op when `state.CurrentEpic.ID != ""`. On first run, populates `current_epic` (id, name, branch name as `"feature/" + epic.ID`, RFC3339 started_at), `active_task` (first task), and `next_task` (second task or zero value).

**Guard**: The `CurrentEpic.ID != ""` check is the bootstrapped sentinel. Do not change this condition — it's how the orchestrator distinguishes first-run from restart.

### NeedsKBSynthesis

Forwarding wrapper for `types.NeedsKBSynthesis`. Returns `true` only when all of these hold:
1. `kbEnabled == true` (parameter, sourced from `cfg.KBEnabled`)
2. `state.ActiveTask.Type != TaskTypeDocumentation` (KB not already running)
3. No task has `Status == TODO` or `Status == IN_PROGRESS`

Used by the orchestrator loop to decide whether to inject a synthetic KB_UPDATE task.

### IsEpicAlreadyComplete

Returns `true` when all user-defined tasks are `DONE` **and** either:
- `kbEnabled == false` (no KB synthesis required), **or**
- `state.ActiveTask.Type == TaskTypeDocumentation` (KB synthesis ran in a previous iteration and completed)

Called once in the pre-loop startup sequence, before `EnsureProjectReady` and before task-pointer reinitialization. When KB synthesis has already run, the persisted state typically still points at the documentation task, so a fresh `doug run` exits early here.

## taskpointers.go

### API

```go
func InitializeTaskPointers(state *types.ProjectState, tasks *types.Tasks, kbEnabled bool)
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

If no user tasks remain and `kbEnabled == true` (parameter), injects a synthetic `KB_UPDATE` documentation task.

### AdvanceToNextTask

Forwarding wrapper for `types.AdvanceToNextTask`. Returns `false` immediately (no state mutation) if `NextTask.ID == ""`. On success:
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

Forwarding wrapper for `types.UpdateTaskStatus`. Returns a descriptive error for unknown IDs — no silent no-ops. Always check the return value.

## validation.go

### API

```go
type ValidationKind int  // ValidationOK | ValidationAutoCorrected | ValidationFatal
type ValidationResult struct { Kind ValidationKind; Description string }

func ValidateYAMLStructure(state *types.ProjectState, tasks *types.Tasks) error
func ValidateStateSync(state *types.ProjectState, tasks *types.Tasks) (ValidationResult, error)
func ValidateTaskTypes(tasks *types.Tasks) error
```

### ValidateTaskTypes

Ensures no task in `tasks.yaml` uses a synthetic type (`bugfix` or `documentation`). These types are orchestrator-injected at runtime and must never appear in user-authored task lists — `HandleSuccess` skips marking synthetic tasks DONE, causing stuck loops.

Returns an error for the first offending task, suggesting `feature` as a replacement type. Called after `ValidateYAMLStructure` in the pre-loop sequence.

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

## startup.go

### CheckDependencies

```go
func CheckDependencies(cfg *config.OrchestratorConfig) error
```

Verifies that all required binaries are on `PATH` before the loop starts:
- `cfg.AgentCommand` (e.g., `"claude"`)
- `"git"` (always required)
- `"go"` (default build system) or `"npm"` (when `cfg.BuildSystem == "npm"`)

Returns a single error listing all missing binaries; nil if all are present. Called once in the pre-loop sequence of `internal/orchestrator/run.go`.

### EnsureProjectReady

```go
func EnsureProjectReady(buildSys build.BuildSystem, buildSystemName string, l log.Logger) error
```

Runs a pre-flight `Build()` then `Test()` to verify the project is in a clean state before the orchestration loop begins. Accepts the build system name string (e.g. `cfg.BuildSystem`) rather than the full config, to minimize the API surface.

- If `buildSys.IsInitialized()` returns `false` (e.g., `go.sum` absent for Go projects): emits a visible warning and returns `nil`. Handles fresh checkouts.
- Any build or test failure returns an error already containing the last 50 lines of output (embedded by the `BuildSystem` implementations). Treat as fatal.

Called once in the pre-loop sequence, **after** `CheckDependencies` and **before** `ValidateYAMLStructure`. The caller passes `o.cfg.BuildSystem` (the string field, not the whole config struct).

## Call Order in Orchestrator.Run

```
pre-loop (Orchestrator.Run):
  CheckDependencies → return error on missing binary
  LoadProjectState + LoadTasks
  Detect PAUSED → resumeFromPause=true
  PrepareForEpicRollover
  BootstrapFromTasks
  IsEpicAlreadyComplete → return nil if done
  EnsureProjectReady (skipped on resume) → return error on build/test failure
  ValidateYAMLStructure + ValidateTaskTypes → return error on structural/type error
  EnsureEpicBranch
  InitializeTaskPointers
  ValidateStateSync (skipped for synthetic active task)
  SaveProjectState

main loop (per iteration):
  ctx.Done() check → return ctx.Err() on cancellation
  if resumeFromPause:
    Section("RESUME — task {id}")
    HandleResume → [BuildFailure→return nil | Continue | EpicComplete→HandleEpicComplete→return nil | Retry]
    resumeFromPause = false; continue
  IncrementAttempts → SaveProjectState (persist before agent)
  Section("[{taskID}] attempt {n}/{maxRetries} ({taskType})")
  WriteActiveTask (injects TestFailureOutput if non-empty)
  bugfix guard: require .doug/ACTIVE_BUG.md for bugfix tasks
  resolve {{skill_name}} + {{task_id}} in agent_command
  RunAgent(ctx, ...) → outputLog at .doug/logs/output/{epic}/output-{taskID}_attempt-{n}.log
    heartbeat: Info("[{taskID}] +{elapsed}")
  ParseSessionResult (failure → treat as FAILURE)
  Info("outcome: {outcome}" or "outcome: {outcome} — {changelogEntry}")
  → handler dispatch (HandleSuccess / HandleFailure / HandleBug / HandleEpicComplete)

max iterations reached → return nil
```

## Related

- [types.md](./types.md) — LoopContext, task_ops (UpdateTaskStatus, NeedsKBSynthesis, AdvanceToNextTask), structs, constants
- [state.md](./state.md) — SaveProjectState, SaveTasks (callers must persist after mutations)
- [handlers.md](./handlers.md) — outcome handlers; HandleResume; run loop integration
- [log.md](./log.md) — Logger interface; New() / Discard() constructors
- [agent.md](./agent.md) — RunAgent (now takes context.Context); WriteActiveTask; ParseSessionResult
- [go.md](../infrastructure/go.md) — three failure tiers and exec/atomic conventions
