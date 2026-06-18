---
title: internal/types — Shared Structs & Constants
updated: 2026-06-17
category: Packages
tags: [types, structs, yaml, constants, session-result, project-status, paused]
related_articles:
  - docs/kb/packages/state.md
  - docs/kb/packages/config.md
  - docs/kb/packages/log.md
  - docs/kb/infrastructure/go.md
  - docs/kb/features/transport-failure-recovery.md
  - docs/kb/features/run-ux-provider-visibility.md
---

# internal/types — Shared Structs & Constants

## Overview

`internal/types` is the single source of truth for all structs, typed constants, and core task operations used by the doug orchestrator. Every other package imports from here; nothing imports back into types. YAML struct tags match the Bash orchestrator schema exactly (snake_case).

## Type Map

| Type | Mirrors | Notes |
|------|---------|-------|
| `ProjectState` | `project-state.yaml` (root) | Load/save via `internal/state` |
| `EpicState` | `current_epic` block | `CompletedAt` is `*string` for null round-trip |
| `TaskPointer` | `active_task` / `next_task` | `Attempts` and `InfraRetries` have `omitempty` — zero values are suppressed |
| `Metrics` | `metrics` block | — |
| `TaskMetric` | `metrics.tasks[]` entry | `CommitSHA`, `Attempts`, `TaskType`, `AgentDurationSeconds`, `ProviderWaitMs`, and `ProviderFailures` all `omitempty` |
| `ProviderFailure` | nested in `metrics.tasks[].provider_failures[]` | Pi/provider diagnostic with `type`, `message`, and `phase` |
| `Tasks` | `tasks.yaml` (root) | Load/save via `internal/state` |
| `EpicDefinition` | `epic` block in tasks.yaml | — |
| `Task` | `tasks[]` entry | `UserDefined bool` with `yaml:"-"` — not persisted |
| `SessionResult` | agent session front-matter | 4 fields; orchestrator manages all other metadata |

## Typed Constants

```go
// Task lifecycle
StatusTODO, StatusInProgress, StatusDone, StatusBlocked

// Agent-reported outcomes
OutcomeSuccess, OutcomeBug, OutcomeFailure, OutcomeEpicComplete

// Orchestrator-internal outcome (never written by agents)
OutcomeBuildFailure  // "BUILD_FAILURE" — returned by HandleSuccess on build/test verify failure

// Task classification (backlog task types)
TaskTypeFeature, TaskTypeBugfix, TaskTypeDocumentation
// Task classification (command-invoked; never in tasks.yaml)
TaskTypePlan      // used exclusively by the doug plan command
TaskTypeResearch  // used exclusively by the doug research command
// Task classification (runtime-only; never in tasks.yaml)
TaskTypeScaffold  // used exclusively by the doug scaffold command

// Project pause state
ProjectStatusPaused  // ProjectStatus("PAUSED") — set on project-state.yaml when build/test fails post-SUCCESS

// Changelog category (Keep a Changelog v1 set)
CategoryAdded, CategoryChanged, CategoryFixed, CategoryRemoved  // values: "added", "changed", "fixed", "removed"
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
    Attempts     int `yaml:"attempts,omitempty"`
    InfraRetries int `yaml:"infra_retries,omitempty"`

    // Test failure retry state (persisted so they survive process restarts)
    ConsecutiveTestFailures int    `yaml:"consecutive_test_failures,omitempty"`
    TestFailureOutput       string `yaml:"test_failure_output,omitempty"`
}
```

`InfraRetries` tracks transport-level agent launch failures separately from task attempts. Doug increments it when the backend returns `RunStatusTransportFailure`, restores the task attempt counter, and resets it after transport recovers.

`ConsecutiveTestFailures` and `TestFailureOutput` track the test-failure-retry cycle across iterations:

- On first test failure after a successful build: `ConsecutiveTestFailures = 1`, output stored, loop returns `Retry` (attempt counter increments normally).
- On second consecutive test failure: `pauseProject` is called — project paused, attempt decremented.
- When tests pass: both fields reset to zero/empty.

These fields are stored in `TaskPointer` (not a separate struct) so they survive process restarts between iterations without a separate state file.

## SessionResult

```go
type SessionResult struct {
    Outcome           Outcome           `yaml:"outcome"`
    ChangelogCategory ChangelogCategory `yaml:"changelog_category"`
    ChangelogEntry    string            `yaml:"changelog_entry"`
    DependenciesAdded []string          `yaml:"dependencies_added"`
}
```

**Four fields.** The orchestrator manages all other session metadata (timestamps, test counts, file lists).

`ChangelogCategory` is optional. When set by the agent, it must be one of `added`, `changed`, `fixed`, or `removed` (case-insensitive; `ParseSessionResult` normalizes to lowercase and clears invalid values). When absent or cleared, `HandleSuccess` falls back to `taskTypeToCategory(ctx.TaskType)`. Do not add fields to `SessionResult` without a corresponding update to `ParseSessionResult`.

## UserDefined vs Synthetic Distinction

```go
// On Task (from tasks.yaml): set by the state loader, never persisted
UserDefined bool `yaml:"-"`

// On TaskType (for TaskPointer contexts where no Task struct exists)
func (t TaskType) IsSynthetic() bool {
    return t == TaskTypeScaffold  // only scaffold is runtime-only
}
```

- **UserDefined = true** → task came from `tasks.yaml`; it will appear in commit messages and status tracking
- **Synthetic / runtime-only** → `scaffold` only; used exclusively by `doug scaffold`, never written to `tasks.yaml`
- **Backlog task types**: `feature`, `bugfix`, and `documentation` can appear in `tasks.yaml` and PLAN.md handoff data. Handler-injected bugfix tasks (`BUG-xxx` IDs) share the `bugfix` type with user-authored tasks — the distinction is at the task-ID level, not the type level.
- **Command-invoked task types**: `plan` and `research` are used exclusively by `doug plan` and `doug research`. They are not runtime-only in the same sense as `scaffold` (they do not appear in the run loop), but they are also not user-authorable in `tasks.yaml`. `IsSynthetic()` returns `false` for both.

`LoadTasks` (in `internal/state`) sets `UserDefined = true` on every task it reads. You never set this field manually.

## Key Decisions

**`ProjectStatus` as a named string type**: Keeps the PAUSED constant type-safe without adding a new integer enum. `omitempty` on the field ensures backward compatibility with existing state files.

**`ConsecutiveTestFailures` + `TestFailureOutput` on `TaskPointer`**: Co-locating with the task pointer (rather than a top-level field) makes it clear these belong to the active task's retry lifecycle. `omitempty` keeps the YAML clean for tasks that never hit test failures.

**`OutcomeBuildFailure` is orchestrator-internal**: Agents never report `"BUILD_FAILURE"`. It is returned by `HandleSuccess` when build or test verification fails, and dispatched by `cmd/run.go` to exit cleanly (exit 0) while leaving the project in PAUSED state.

**`CompletedAt *string`**: `EpicState.CompletedAt` is a pointer so YAML round-trips correctly for `null`. A value type would unmarshal `null` as an empty string, breaking equality checks.

**`Attempts` / `InfraRetries` `omitempty`**: `TaskPointer.Attempts` and `TaskPointer.InfraRetries` use `omitempty` so zero values are omitted from YAML. Transport retry state stays separate from task-failure attempts.

**`yaml:"-"` on UserDefined**: The field must never reach YAML. Tasks are loaded from `tasks.yaml` (where the field doesn't exist) and written back (where it must not appear). The loader sets it in memory only.

**ProviderFailure is persisted diagnostics, not outcome state**: `ProviderFailure{Type, Message, Phase}` records Pi/provider transport diagnostics for metrics and later analysis. It does not change workflow outcome parsing.

**No `interface{}` or `map[string]any`**: All YAML shapes are fully typed. If the YAML schema changes, the Go structs are the authority.

## Edge Cases & Gotchas

**`TaskMetric.Outcome` is `string`, not `Outcome`**: The metrics block stores outcome as a plain string copied from the session result. This matches the Bash orchestrator schema and avoids a circular dependency. Always pass `string(types.OutcomeSuccess)` etc. — never bare lowercase strings like `"success"`.

**`TaskMetric` extended fields (all `omitempty`)**: `CommitSHA string` (40-char SHA backfilled after git commit), `Attempts int` (iteration count), `TaskType string` (for example `feature`, `bugfix`, `documentation`, `scaffold`), `AgentDurationSeconds int` (wall-clock seconds the agent process ran), `ProviderWaitMs int64` (elapsed milliseconds until the first non-startup Pi event), and `ProviderFailures []ProviderFailure` (provider/transport diagnostics observed during the run). Legacy entries without these fields serialize cleanly due to `omitempty`.

**Nil `CompletedAt`**: When constructing a new `EpicState`, leave `CompletedAt` nil. Only the epic completion handler sets it. Do not set it to a pointer to an empty string.

**Zero-value `TaskPointer`**: `next_task` is often a zero-value struct (`type: ""`, `id: ""`). Callers must check `pointer.ID == ""` to detect an absent next task — there is no sentinel value or pointer.

## Related Topics

- [LoopContext & Task Ops](types-loop-context.md) — per-iteration execution context and in-memory task operations
- [State I/O](state.md) — how types are loaded and saved
- [Go Infrastructure](../infrastructure/go.md) — YAML dependency and conventions
- [Transport Failure Recovery](../features/transport-failure-recovery.md) — semantics of `TaskPointer.InfraRetries`
- [Run UX + Provider Stall Visibility](../features/run-ux-provider-visibility.md) — how provider wait/failure data is produced
