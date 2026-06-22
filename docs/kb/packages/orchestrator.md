---
title: internal/orchestrator — Core Orchestration Logic
updated: 2026-06-17
category: Packages
tags: [orchestrator, bootstrap, task-pointers, validation, state-management, loop-context, startup, paths, module-root, context, backend, seam, execution-prep]
related_articles:
  - docs/kb/features/module-root.md
  - docs/kb/packages/types.md
  - docs/kb/packages/types-loop-context.md
  - docs/kb/packages/state.md
  - docs/kb/packages/handlers.md
  - docs/kb/packages/log.md
  - docs/kb/packages/agent.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/features/transport-failure-recovery.md
  - docs/kb/features/run-ux-provider-visibility.md
  - docs/kb/infrastructure/go.md
---

# internal/orchestrator — Core Orchestration Logic

## Overview

`internal/orchestrator` contains the `Orchestrator` struct and full orchestration lifecycle (`run.go`), plus supporting files for bootstrapping, task pointer management, startup checks, and state validation. All mutation functions operate in-memory — callers are responsible for calling `SaveProjectState`/`SaveTasks` after mutating state.

## orchestrator.go — Orchestrator Struct

```go
type Orchestrator struct {
    cfg         *config.OrchestratorConfig
    paths       Paths
    logger      log.Logger
    buildSystem build.BuildSystem
    backend     agent.Backend  // execution seam; all agent launches go through this
}

func New(cfg *config.OrchestratorConfig, paths Paths) (*Orchestrator, error)
```

`New` constructs the orchestrator: creates a `log.New()` stderr logger, resolves the build root as `filepath.Join(paths.ProjectRoot, cfg.ModuleRoot)`, emits the non-fatal missing-`go.mod` module-root warning when applicable, and passes that resolved root to `build.NewBuildSystem(cfg.BuildSystem, modulePath)`. `backend` is left nil; the production backend is selected at invocation time via `execBackend`. Returns an error if the build system identifier is unrecognized.

The private `execBackend()` helper selects the backend for each agent invocation. When `o.backend` is set (test injection) it is returned unchanged; otherwise `agent.NewBackend()` returns the production Pi backend:

```go
func (o *Orchestrator) execBackend() agent.Backend {
    if o.backend != nil {
        return o.backend
    }
    return agent.NewBackend()
}
```

`agent.NewBackend` always returns `PiAdapter` for production dispatch. Source-owned phase routing controls Pi mode: planning is terminal-interactive Pi, while runtime/scaffold/research/post-epic KB are Pi RPC one-shot runs. Unknown phases are rejected during execution preparation/adapter dispatch instead of falling back to another backend. See [internal/agent](agent.md) for the full selection contract.

`module_root` changes only the build-system root. It does not change any `Paths` values or relocate `.doug/` runtime state.

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
    ManifestPath     string // <root>/.doug/plan/manifest.yaml
    LogsDir          string // <root>/.doug/logs
    ChangelogPath    string // <root>/CHANGELOG.md
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
func IsEpicAlreadyComplete(state *types.ProjectState, tasks *types.Tasks) bool
```

### PrepareForEpicRollover

Detects when `tasks.yaml` has switched to a new epic ID and conditionally resets runtime state for seamless next-epic bootstrap.

- If `state.current_epic.id == ""` or IDs match: no-op.
- If IDs differ and `current_epic.completed_at` is empty: returns a fatal guardrail error.
- If IDs differ and previous epic is completed: resets `current_epic`, `active_task`, `next_task`, and per-epic metrics, then caller runs `BootstrapFromTasks`.

### BootstrapFromTasks

No-op when `state.CurrentEpic.ID != ""`. On first run, populates `current_epic` (id, name, branch name as `"feature/" + epic.ID`, RFC3339 started_at), `active_task` (first task), and `next_task` (second task or zero value).

**Guard**: The `CurrentEpic.ID != ""` check is the bootstrapped sentinel. Do not change this condition — it's how the orchestrator distinguishes first-run from restart.

### IsEpicAlreadyComplete

Returns `true` only when all user-defined tasks are `DONE`, `current_epic.completed_at` is populated, and both runtime task pointers are empty. That means the epic has already been finalized, not merely that execution reached the last user task.

Called once in the pre-loop startup sequence, before `EnsureProjectReady` and before task-pointer reinitialization. This lets a fresh `doug run` exit cleanly when the prior run already finalized the epic and cleared runtime task pointers.

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

**Guard**: If `active_task.id` is set but not present in `tasks.yaml` (e.g., a handler-injected `BUG-xxx` bugfix task), the function returns immediately without re-initializing pointers. This prevents clobbering an in-progress handler-injected task.

Selection order for `active_task` (when not guarded):
1. First `IN_PROGRESS` task (handles orchestrator-restart recovery)
2. First `TODO` task (normal forward progress)

`next_task` is set to the first `TODO` that appears **after** the selected active task in the list (positional search, not global first-match).

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
func NormalizeLegacyManualReviewState(state *types.ProjectState, tasks *types.Tasks) (bool, error)
func ValidateActiveTaskIsRunnable(state *types.ProjectState, tasks *types.Tasks) error
func ValidateStateSync(state *types.ProjectState, tasks *types.Tasks) (ValidationResult, error)
func ValidateTaskTypes(tasks *types.Tasks) error
```

### ValidateTaskTypes

Ensures no task in `tasks.yaml` uses a forbidden task type. Runtime-only `scaffold` and removed legacy `manual_review` are rejected. Other supported task types remain valid for normal Doug backlog execution.

Returns an error for the first offending task. Called after `ValidateYAMLStructure` in the pre-loop sequence.

### NormalizeLegacyManualReviewState / ValidateActiveTaskIsRunnable

`NormalizeLegacyManualReviewState` provides backward compatibility for legacy `project-state.yaml` files that still use `active_task.type = manual_review`. It rewrites them to the current model by marking the originating backlog task `BLOCKED` and restoring `active_task` to that real backlog task. Failed synthetic bugfix states fold blockage back onto `next_task` when it points at the interrupted backlog task.

`ValidateActiveTaskIsRunnable` halts `doug run` cleanly when the active backlog task is already `BLOCKED`, preventing retries or auto-advance until a human resolves or unblocks the task.

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
| ID not found, runtime-only type (`scaffold`) | 3 | `ValidationFatal` + error — manual intervention required |
| ID not found, exactly 1 TODO/IN_PROGRESS candidate | 2 | `ValidationAutoCorrected`, state redirected, `Attempts` preserved |
| ID not found, 0 or 2+ candidates | 3 | `ValidationFatal` + error |

**Note**: callers must skip `ValidateStateSync` for active tasks not in `tasks.yaml` (e.g., handler-injected `BUG-xxx` bugfix tasks). The scaffold type check is a safety net for corrupt state; it should not be reached in normal operation. The run loop now performs an explicit ID-in-backlog check to decide whether to call `ValidateStateSync` at all.

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
- `"pi"` always — Doug routes all agent execution through Pi; Pi is the exclusive execution boundary regardless of task type or workflow phase
- `"git"` (always required)
- `"go"` (default build system), `"npm"` (when `cfg.BuildSystem == "npm"`), or `"pnpm"` (when `cfg.BuildSystem == "pnpm"`)

Returns a single error listing all missing binaries; nil if all are present. Called once in the pre-loop sequence of `internal/orchestrator/run.go`.

### EnsureProjectReady

```go
func EnsureProjectReady(buildSys build.BuildSystem, buildSystemName string, l log.Logger) error
```

Runs a pre-flight `Build()` then `Test()` to verify the project is in a clean state before the orchestration loop begins. Accepts the build system name string (e.g. `cfg.BuildSystem`) rather than the full config, to minimize the API surface.

- If `buildSys.IsInitialized()` returns `false` (for example, `go.mod` absent for Go projects or `node_modules/` absent for Node projects): emits a visible warning and returns `nil`. Handles fresh checkouts and uninitialized dependencies.
- Any build or test failure returns an error already containing the last 50 lines of output (embedded by the `BuildSystem` implementations). Treat as fatal.

Called once in the pre-loop sequence, **after** `CheckDependencies` and **before** `ValidateYAMLStructure`. The caller passes `o.cfg.BuildSystem` (the string field, not the whole config struct).

## post_epic_kb.go

### `runPostEpicKB`

```go
func (o *Orchestrator) runPostEpicKB(ctx context.Context, state *types.ProjectState) error
```

Runs best-effort KB synthesis after epic finalization. It writes a synthetic documentation briefing with task ID `POST_EPIC_KB` that points the agent at `.doug/logs/archives/{epic}/`, `.doug/logs/sessions/{epic}/`, and optional `.doug/plan/PLAN.md` planning context.

Key properties:

- skips entirely when `cfg.KBEnabled == false`
- never mutates runtime task pointers or reopens finalized runtime state
- resolves the built-in documentation skill and source-owned Pi RPC routing via `agent.PrepareExecution(RunPhasePostEpicKB, "documentation", ...)`
- explicitly tells the agent to use the documentation workflow, start from `docs/kb/README.md`, read `PLAN.md` when planning rationale/scope/non-goals are relevant, and keep KB output inside `docs/kb/`
- writes raw output to `.doug/logs/output/{epic}/output-post_epic_kb.log`
- archives the result as `session-POST_EPIC_KB_attempt-1.md`
- rejects pending KB synthesis changes outside `docs/kb/` before commit
- accepts `SUCCESS` or `EPIC_COMPLETE`; also tolerates a missing outcome (`agent.ErrMissingOutcome`, typically a provider transport issue) as a best-effort soft success **only when** in-scope `docs/kb/` files actually changed — a missing outcome with no `docs/kb/` changes, or any other parse error, is still fatal
- commits KB changes via `git.CommitPaths` scoped to the changed `docs/kb/` paths only (never a broad `git add -A`), as `docs: synthesize KB for {epicID}`, and treats `git.ErrNothingToCommit` as informational

The main run loop treats post-epic KB failures as warning-only after finalization. The epic remains completed either way.

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
  ValidateStateSync (skipped when active task ID is not in tasks.yaml)
  SaveProjectState
  if all user tasks DONE and completed_at already set:
    HandleEpicComplete
    runPostEpicKB (warning-only on failure)
    return nil

main loop (per iteration):
  ctx.Done() check → return ctx.Err() on cancellation
  if resumeFromPause:
    Section("RESUME — task {id}")
    HandleResume → [BuildFailure→return nil | Continue | EpicComplete→HandleEpicComplete→return nil | Retry]
    resumeFromPause = false; continue
  IncrementAttempts → SaveProjectState (persist before agent)
  load task description from the already-loaded tasks.yaml entry
  Section("[{taskID}] attempt {n}/{maxRetries} — {description}") with description truncated to 80 characters including ellipsis when longer
  WriteActiveTask (injects TestFailureOutput if non-empty)
  bugfix guard: require blocking bug payload on active_task for synthetic bugfix tasks
  PrepareExecution(RunPhaseRuntime, taskType, taskID) → ExecutionPrep{SkillName, InitialPrompt, InteractionMode}
  WriteAttemptStart → .doug/logs/pi-sessions/{epic}/{taskID}/attempt-{n}/attempt-start.json
  execBackend().Run(ctx, RunRequest{Routing.SkillName=prep.SkillName, Routing.InteractionMode=prep.InteractionMode, InitialPrompt=prep.InitialPrompt}) → outputLog at .doug/logs/output/{epic}/output-{taskID}_attempt-{n}.log
    heartbeat: Info("[{taskID}] +{elapsed} — {activity}"); if first_response_threshold elapses first, Warning("⚠ no provider response yet (+{elapsed})") once
    first response callback: Info("► first response (+{elapsed})")
  if RunStatusTransportFailure:
    restore task Attempts, increment InfraRetries, write .doug/logs/failures/{epic}/infra-failure-{taskID}-attempt-{infraRetries}.md, save state
    if below max_infra_retries: bounded exponential backoff, continue
    if at cap: write .doug/ACTIVE_FAILURE.md and halt before parsing ACTIVE_TASK.md
  ParseSessionResult (failure → archive session, restore attempt count, return explicit contract/parse error)
  Info("agent finished in {minutes}m {seconds}s — first response +{seconds}s, {toolCalls} tool calls, {providerFailures} provider failures")
  Info("outcome: {outcome}" or "outcome: {outcome} — {changelogEntry}")
  → handler dispatch (HandleSuccess / HandleFailure / HandleBug / HandleEpicComplete)
  EpicComplete from SUCCESS or explicit EPIC_COMPLETE:
    HandleEpicComplete
    runPostEpicKB (warning-only on failure)
    return nil

max iterations reached → return nil
```

## Related

- [Build-System Module Root](../features/module-root.md) — subdirectory build root, Go sentinel, and warning contract
- [types.md](./types.md) — LoopContext, task_ops (`UpdateTaskStatus`, `AdvanceToNextTask`, `AreAllUserTasksComplete`), structs, constants
- [state.md](./state.md) — SaveProjectState, SaveTasks (callers must persist after mutations)
- [handlers.md](./handlers.md) — outcome handlers; HandleResume; run loop integration
- [log.md](./log.md) — Logger interface; New() / Discard() constructors
- [agent.md](./agent.md) — Backend interface, PiAdapter, PiInteractiveLauncher, WriteActiveTask, ParseSessionResult
- [Transport Failure Recovery](../features/transport-failure-recovery.md) — transport classification, infra retries, durable records, and attempt-start markers
- [Run UX + Provider Stall Visibility](../features/run-ux-provider-visibility.md) — attempt header, heartbeat, first-response, stall warning, summary, and metrics UX
- [go.md](../infrastructure/go.md) — three failure tiers and exec/atomic conventions
