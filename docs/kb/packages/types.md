---
title: internal/types — Shared Structs & Constants
updated: 2026-03-24
category: Packages
tags: [types, structs, yaml, constants, session-result, project-status, paused, loop-context, task-ops]
related_articles:
  - docs/kb/packages/state.md
  - docs/kb/packages/config.md
  - docs/kb/packages/log.md
  - docs/kb/infrastructure/go.md
---

# internal/types — Shared Structs & Constants

## Overview

`internal/types` is the single source of truth for all structs, typed constants, and core task operations used by the doug orchestrator. Every other package imports from here; nothing imports back into types. YAML struct tags match the Bash orchestrator schema exactly (snake_case).

> **EPIC-12**: `LoopContext` moved from `internal/orchestrator/context.go` to `internal/types/loop_context.go`. `UpdateTaskStatus`, `NeedsKBSynthesis`, and `AdvanceToNextTask` moved to `internal/types/task_ops.go` (orchestrator forwarding wrappers remain for compatibility).

## Type Map

| Type | Mirrors | Notes |
|------|---------|-------|
| `ProjectState` | `project-state.yaml` (root) | Load/save via `internal/state` |
| `EpicState` | `current_epic` block | `CompletedAt` is `*string` for null round-trip |
| `TaskPointer` | `active_task` / `next_task` | `Attempts` has `omitempty` — suppressed on `next_task` |
| `Metrics` | `metrics` block | — |
| `TaskMetric` | `metrics.tasks[]` entry | `CommitSHA`, `Attempts`, `TaskType`, `AgentDurationSeconds` all `omitempty` |
| `Tasks` | `tasks.yaml` (root) | Load/save via `internal/state` |
| `EpicDefinition` | `epic` block in tasks.yaml | — |
| `Task` | `tasks[]` entry | `UserDefined bool` with `yaml:"-"` — not persisted |
| `SessionResult` | agent session front-matter | Exactly 3 fields |

## Typed Constants

```go
// Task lifecycle
StatusTODO, StatusInProgress, StatusDone, StatusBlocked

// Agent-reported outcomes
OutcomeSuccess, OutcomeBug, OutcomeFailure, OutcomeEpicComplete

// Orchestrator-internal outcome (never written by agents)
OutcomeBuildFailure  // "BUILD_FAILURE" — returned by HandleSuccess on build/test verify failure

// Task classification
TaskTypeFeature, TaskTypeBugfix, TaskTypeDocumentation, TaskTypeManualReview

// Project pause state
ProjectStatusPaused  // ProjectStatus("PAUSED") — set on project-state.yaml when build/test fails post-SUCCESS
```

Use the typed constants everywhere — never bare strings like `"SUCCESS"` or `"bugfix"`.

## ProjectState and ProjectStatus

```go
type ProjectStatus string
const ProjectStatusPaused ProjectStatus = "PAUSED"

type ProjectState struct {
    CurrentEpic EpicState   `yaml:"current_epic"`
    ActiveTask  TaskPointer `yaml:"active_task"`
    NextTask    TaskPointer `yaml:"next_task"`
    Metrics     Metrics     `yaml:"metrics"`
    Status      ProjectStatus `yaml:"status,omitempty"`  // empty = running; "PAUSED" = paused
}
```

`Status` uses `omitempty` so existing `project-state.yaml` files remain valid — active (non-paused) projects have no `status` field at all.

### PAUSED state lifecycle

| Event | Status value |
|-------|-------------|
| Normal operation | `""` (field absent) |
| Build or test verification fails after agent SUCCESS (1st test fail is exception — see below) | `"PAUSED"` |
| `doug run` on a paused project and build/tests pass | `""` (cleared by `HandleResume`) |
| `doug run` on a paused project and build/tests still fail | `"PAUSED"` (re-set) |

## TaskPointer — Test Failure Fields

```go
type TaskPointer struct {
    Type     TaskType `yaml:"type"`
    ID       string   `yaml:"id"`
    Attempts int      `yaml:"attempts,omitempty"`

    // Test failure retry state (persisted so they survive process restarts)
    ConsecutiveTestFailures int    `yaml:"consecutive_test_failures,omitempty"`
    TestFailureOutput       string `yaml:"test_failure_output,omitempty"`
}
```

`ConsecutiveTestFailures` and `TestFailureOutput` track the test-failure-retry cycle across iterations:

- On first test failure after a successful build: `ConsecutiveTestFailures = 1`, output stored, loop returns `Retry` (attempt counter increments normally).
- On second consecutive test failure: `pauseProject` is called — project paused, attempt decremented.
- When tests pass: both fields reset to zero/empty.

These fields are stored in `TaskPointer` (not a separate struct) so they survive process restarts between iterations without a separate state file.

## SessionResult

```go
type SessionResult struct {
    Outcome           Outcome  `yaml:"outcome"`
    ChangelogEntry    string   `yaml:"changelog_entry"`
    DependenciesAdded []string `yaml:"dependencies_added"`
}
```

**Exactly three fields.** The orchestrator manages all other session metadata (timestamps, test counts, file lists). Do not add fields here.

## UserDefined vs Synthetic Distinction

```go
// On Task (from tasks.yaml): set by the state loader, never persisted
UserDefined bool `yaml:"-"`

// On TaskType (for TaskPointer contexts where no Task struct exists)
func (t TaskType) IsSynthetic() bool {
    return t == TaskTypeBugfix || t == TaskTypeDocumentation || t == TaskTypeScaffold
}
```

- **UserDefined = true** → task came from `tasks.yaml`; it will appear in commit messages and status tracking
- **Synthetic** → orchestrator-injected (`bugfix`, `documentation`, `scaffold`); lives only in `project-state.yaml.active_task`; never written to `tasks.yaml`

`LoadTasks` (in `internal/state`) sets `UserDefined = true` on every task it reads. You never set this field manually.

## LoopContext (loop_context.go)

`LoopContext` carries all per-iteration state for the orchestration main loop. It is constructed once per iteration in `Orchestrator.Run` and passed to every handler.

```go
type LoopContext struct {
    // Per-iteration identity (snapshotted after IncrementAttempts)
    TaskID      string
    TaskType    TaskType
    Attempts    int
    CurrentEpic EpicState

    // Orchestrator configuration + infrastructure
    Config      *config.OrchestratorConfig
    BuildSystem build.BuildSystem
    ProjectRoot string
    TaskStartTime time.Time

    // Mutable shared state — mutated in memory and persisted by handlers
    State *ProjectState
    Tasks *Tasks

    // File system paths used by handlers
    StatePath     string // .doug/project-state.yaml
    TasksPath     string // .doug/tasks.yaml
    DougDir       string // .doug/
    LogsDir       string // .doug/logs/
    ChangelogPath string // CHANGELOG.md

    // Logger is the structured output writer for this iteration.
    Logger log.Logger
}
```

`SessionResult` and `AgentDurationSeconds` were removed from `LoopContext` in EPIC-12 — they are now passed as explicit parameters to `HandleSuccess`. Do not re-add them to the struct.

`orchestrator.LoopContext` is a type alias for `types.LoopContext`; both names refer to the same type.

## task_ops.go — Task Operations

These functions operate on `*ProjectState` and `*Tasks` in memory. Callers persist via `state.SaveProjectState`/`state.SaveTasks`.

```go
func UpdateTaskStatus(tasks *Tasks, id string, status Status) error
func NeedsKBSynthesis(state *ProjectState, tasks *Tasks, kbEnabled bool) bool
func AdvanceToNextTask(state *ProjectState, tasks *Tasks) bool
```

These were previously in `internal/orchestrator`. The `orchestrator` package still exports forwarding wrappers for existing callers. **Prefer calling `types.*` directly in new code.**

See `docs/kb/packages/orchestrator.md` for the full behavioral spec of each function.

## Key Decisions

**`ProjectStatus` as a named string type**: Keeps the PAUSED constant type-safe without adding a new integer enum. `omitempty` on the field ensures backward compatibility with existing state files.

**`ConsecutiveTestFailures` + `TestFailureOutput` on `TaskPointer`**: Co-locating with the task pointer (rather than a top-level field) makes it clear these belong to the active task's retry lifecycle. `omitempty` keeps the YAML clean for tasks that never hit test failures.

**`OutcomeBuildFailure` is orchestrator-internal**: Agents never report `"BUILD_FAILURE"`. It is returned by `HandleSuccess` when build or test verification fails, and dispatched by `cmd/run.go` to exit cleanly (exit 0) while leaving the project in PAUSED state.

**`CompletedAt *string`**: `EpicState.CompletedAt` is a pointer so YAML round-trips correctly for `null`. A value type would unmarshal `null` as an empty string, breaking equality checks.

**`Attempts omitempty`**: `TaskPointer.Attempts` uses `omitempty` so `next_task` serialization omits the field entirely, matching the Bash orchestrator schema where `next_task` has no `attempts` field.

**`yaml:"-"` on UserDefined**: The field must never reach YAML. Tasks are loaded from `tasks.yaml` (where the field doesn't exist) and written back (where it must not appear). The loader sets it in memory only.

**No `interface{}` or `map[string]any`**: All YAML shapes are fully typed. If the YAML schema changes, the Go structs are the authority.

## Edge Cases & Gotchas

**`TaskMetric.Outcome` is `string`, not `Outcome`**: The metrics block stores outcome as a plain string copied from the session result. This matches the Bash orchestrator schema and avoids a circular dependency. Always pass `string(types.OutcomeSuccess)` etc. — never bare lowercase strings like `"success"`.

**`TaskMetric` extended fields (all `omitempty`)**: `CommitSHA string` (40-char SHA backfilled after git commit), `Attempts int` (iteration count), `TaskType string` (for example `feature`, `bugfix`, `documentation`, `scaffold`), `AgentDurationSeconds int` (wall-clock seconds the agent process ran). Legacy entries without these fields serialize cleanly due to `omitempty`.

**Nil `CompletedAt`**: When constructing a new `EpicState`, leave `CompletedAt` nil. Only the epic completion handler sets it. Do not set it to a pointer to an empty string.

**Zero-value `TaskPointer`**: `next_task` is often a zero-value struct (`type: ""`, `id: ""`). Callers must check `pointer.ID == ""` to detect an absent next task — there is no sentinel value or pointer.

## Related Topics

- [State I/O](state.md) — how types are loaded and saved
- [Go Infrastructure](../infrastructure/go.md) — YAML dependency and conventions
