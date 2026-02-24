---
title: internal/types — Shared Structs & Constants
updated: 2026-02-24
category: Packages
tags: [types, structs, yaml, constants, session-result]
related_articles:
  - docs/kb/packages/state.md
  - docs/kb/packages/config.md
  - docs/kb/infrastructure/go.md
---

# internal/types — Shared Structs & Constants

## Overview

`internal/types` is the single source of truth for all structs and typed constants used by the doug orchestrator. Every other package imports from here; nothing imports back into types. YAML struct tags match the Bash orchestrator schema exactly (snake_case).

## Type Map

| Type | Mirrors | Notes |
|------|---------|-------|
| `ProjectState` | `project-state.yaml` (root) | Load/save via `internal/state` |
| `EpicState` | `current_epic` block | `CompletedAt` is `*string` for null round-trip |
| `TaskPointer` | `active_task` / `next_task` | `Attempts` has `omitempty` — suppressed on `next_task` |
| `Metrics` | `metrics` block | — |
| `TaskMetric` | `metrics.tasks[]` entry | — |
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

// Task classification
TaskTypeFeature, TaskTypeBugfix, TaskTypeDocumentation, TaskTypeManualReview
```

Use the typed constants everywhere — never bare strings like `"SUCCESS"` or `"bugfix"`.

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
    return t == TaskTypeBugfix || t == TaskTypeDocumentation
}
```

- **UserDefined = true** → task came from `tasks.yaml`; it will appear in commit messages and status tracking
- **Synthetic** → orchestrator-injected (`bugfix`, `documentation`); lives only in `project-state.yaml.active_task`; never written to `tasks.yaml`

`LoadTasks` (in `internal/state`) sets `UserDefined = true` on every task it reads. You never set this field manually.

## Key Decisions

**`CompletedAt *string`**: `EpicState.CompletedAt` is a pointer so YAML round-trips correctly for `null`. A value type would unmarshal `null` as an empty string, breaking equality checks.

**`Attempts omitempty`**: `TaskPointer.Attempts` uses `omitempty` so `next_task` serialization omits the field entirely, matching the Bash orchestrator schema where `next_task` has no `attempts` field.

**`yaml:"-"` on UserDefined**: The field must never reach YAML. Tasks are loaded from `tasks.yaml` (where the field doesn't exist) and written back (where it must not appear). The loader sets it in memory only.

**No `interface{}` or `map[string]any`**: All YAML shapes are fully typed. If the YAML schema changes, the Go structs are the authority.

## Edge Cases & Gotchas

**`TaskMetric.Outcome` is `string`, not `Outcome`**: The metrics block stores outcome as a plain string copied from the session result. This matches the Bash orchestrator schema and avoids a circular dependency. Do not change this to `Outcome`.

**Nil `CompletedAt`**: When constructing a new `EpicState`, leave `CompletedAt` nil. Only the epic completion handler sets it. Do not set it to a pointer to an empty string.

**Zero-value `TaskPointer`**: `next_task` is often a zero-value struct (`type: ""`, `id: ""`). Callers must check `pointer.ID == ""` to detect an absent next task — there is no sentinel value or pointer.

## Related Topics

- [State I/O](state.md) — how types are loaded and saved
- [Go Infrastructure](../infrastructure/go.md) — YAML dependency and conventions
