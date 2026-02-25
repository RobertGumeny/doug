# Research Report: Agent Information Hiding — ACTIVE_TASK.md Enrichment & YAML Access Removal

**Generated**: 2026-02-25
**Scope Type**: Feature/Module
**Related Epic**: Post-EPIC-6 (proposed EPIC-7)
**Related Tasks**: None (forward-looking implementation plan)

---

## Overview

Agents currently read `tasks.yaml` and `project-state.yaml` directly to discover their task description, acceptance criteria, attempt number, and epic ID — information the orchestrator already holds in memory. This report maps the exact gap between what `ACTIVE_TASK.md` currently provides and what agents need, defines the concrete changes to `ActiveTaskConfig`, `WriteActiveTask`, `cmd/run.go`, the skill files, and `settings.json` to close it, and documents the special-case handling for bugfix and documentation tasks.

---

## File Manifest

| File | Purpose |
| --- | --- |
| `internal/agent/activetask.go` | `ActiveTaskConfig` struct, `WriteActiveTask` function — primary change target |
| `internal/agent/activetask_test.go` | Existing tests; new tests needed for added fields |
| `internal/orchestrator/context.go` | `LoopContext` — carries all per-iteration data available at `WriteActiveTask` call |
| `cmd/run.go` | Call site for `WriteActiveTask`; passes `taskID`, `taskType`, `attempts`, `tasks`, `projectState`, `cfg` |
| `internal/types/types.go` | `Task` struct — `Description string`, `AcceptanceCriteria []string` confirmed present |
| `internal/templates/init/skills/implement-feature/SKILL.md` | Reads YAML files in Phase 1; needs updating |
| `internal/templates/init/skills/implement-bugfix/SKILL.md` | Reads `project-state.yaml` in Phase 1; needs updating |
| `internal/templates/init/skills/implement-documentation/SKILL.md` | Already forbidden from reading YAML — model implementation |
| `.claude/settings.json` | Permission allow/deny rules; needs new deny entries |

---

## Current State: What ACTIVE_TASK.md Contains

`WriteActiveTask` (`activetask.go:113`) builds this document:

```
# Active Task

**Task ID**: {TaskID}
**Task Type**: {TaskType}
**Session File**: {SessionFilePath}

---

{skillContent}

[bugfix tasks only:]
---

## Bug Context

{content of logs/ACTIVE_BUG.md}
```

`ActiveTaskConfig` has exactly five fields:

```go
type ActiveTaskConfig struct {
    TaskID           string
    TaskType         types.TaskType
    SessionFilePath  string
    LogsDir          string
    SkillsConfigPath string
}
```

**Absent**: `Description`, `AcceptanceCriteria`, `Attempts`, `MaxRetries`. Agents must fetch these themselves.

---

## What Agents Currently Read from YAML Files

### Feature skill (`implement-feature/SKILL.md`) — Phase 1 Research

| Field read | Source file | Purpose |
|------------|-------------|---------|
| `current_epic.id` | `project-state.yaml` | Construct session file path |
| `active_task.id` | `project-state.yaml` | Confirm task ID |
| `active_task.attempts` | `project-state.yaml` | Construct session file path |
| `description` | `tasks.yaml` | Understand what to build |
| `acceptance_criteria` | `tasks.yaml` | Know what "done" looks like |
| `status` | `tasks.yaml` | Pre-flight DONE check |

### Bugfix skill (`implement-bugfix/SKILL.md`) — Phase 1 Research

| Field read | Source file | Purpose |
|------------|-------------|---------|
| `current_epic.id` | `project-state.yaml` | Construct session file path |
| `active_task.id` | `project-state.yaml` | Confirm bug task ID |
| `active_task.attempts` | `project-state.yaml` | Construct session file path |
| `logs/ACTIVE_BUG.md` | (already injected) | Bug description — **already in ACTIVE_TASK.md** |

Bugfix tasks are **synthetic** — they never appear in `tasks.yaml`. Bugfix agents do not read `tasks.yaml` at all today. The only YAML dependency is `project-state.yaml` for identity/path fields.

### Documentation skill (`implement-documentation/SKILL.md`) — Phase 1

The documentation skill already has this in its "NOT allowed" list:

> ❌ Read `project-state.yaml` or `tasks.yaml` (not needed — session logs have all context)

**The documentation skill is already fully compliant.** It is the model to replicate for feature and bugfix skills. No changes needed.

---

## The Key Insight: Session File Path Is Already Resolved

All three skills read `current_epic.id` and `active_task.attempts` primarily to **construct the session file path**:

```
logs/sessions/{epic}/session-{task_id}_attempt-{attempts}.md
```

But the orchestrator already resolves this path and injects it as:

```
**Session File**: logs/sessions/EPIC-6/session-EPIC-6-003_attempt-2.md
```

Agents do not need to construct the path. They just write to the path given to them. This makes removing `project-state.yaml` access completely safe — agents lose nothing they need.

---

## Data Available at the `WriteActiveTask` Call Site

In `cmd/run.go`, `WriteActiveTask` is called at `run.go:205`. At that point the following are all in scope:

```go
taskID     string                  // e.g. "EPIC-6-003"
taskType   types.TaskType          // e.g. "feature"
attempts   int                     // e.g. 2  (already incremented)
sessionPath string                 // full resolved path
projectState *types.ProjectState   // CurrentEpic.ID, CurrentEpic.Name
cfg        *config.OrchestratorConfig  // MaxRetries, MaxIterations
tasks      *types.Tasks            // Epic.Tasks[*].Description, .AcceptanceCriteria
logsDir    string
skillsConfigPath string
```

**`tasks.Epic.Tasks` is already loaded.** Finding the current task's description and acceptance criteria requires only a linear search by ID:

```go
func findTask(tasks *types.Tasks, taskID string) *types.Task {
    for i := range tasks.Epic.Tasks {
        if tasks.Epic.Tasks[i].ID == taskID {
            return &tasks.Epic.Tasks[i]
        }
    }
    return nil // synthetic task (bugfix, documentation) — not in tasks.yaml
}
```

When `findTask` returns `nil` (synthetic tasks), `Description` and `AcceptanceCriteria` are empty — the sections are simply omitted from ACTIVE_TASK.md. No special-casing needed in `WriteActiveTask`.

---

## Implementation Plan

### 1. `internal/agent/activetask.go` — Extend `ActiveTaskConfig`

Add four fields:

```go
type ActiveTaskConfig struct {
    TaskID           string
    TaskType         types.TaskType
    SessionFilePath  string
    LogsDir          string
    SkillsConfigPath string

    // New fields — populated from tasks.Tasks and cfg at the call site.
    // Empty for synthetic tasks (bugfix, documentation).
    Description        string   // Task.Description
    AcceptanceCriteria []string // Task.AcceptanceCriteria

    // New fields — populated from LoopContext at the call site.
    Attempts   int // active_task.attempts (already incremented)
    MaxRetries int // cfg.MaxRetries
}
```

### 2. `internal/agent/activetask.go` — Update `WriteActiveTask`

Update the header section and add conditional body sections:

```go
// Header — always present
sb.WriteString("# Active Task\n\n")
sb.WriteString(fmt.Sprintf("**Task ID**: %s\n", config.TaskID))
sb.WriteString(fmt.Sprintf("**Task Type**: %s\n", string(config.TaskType)))
sb.WriteString(fmt.Sprintf("**Attempt**: %d of %d\n", config.Attempts, config.MaxRetries))
sb.WriteString(fmt.Sprintf("**Session File**: %s\n", config.SessionFilePath))

// Task context — omitted for synthetic tasks (Description == "")
if config.Description != "" {
    sb.WriteString("\n## Your Task\n\n")
    sb.WriteString(config.Description + "\n")

    if len(config.AcceptanceCriteria) > 0 {
        sb.WriteString("\n## Acceptance Criteria\n\n")
        for _, ac := range config.AcceptanceCriteria {
            sb.WriteString(fmt.Sprintf("- %s\n", ac))
        }
    }
}

sb.WriteString("\n---\n\n")
sb.WriteString(skillContent)

// Bug context — bugfix tasks only (unchanged)
if config.TaskType == types.TaskTypeBugfix { ... }
```

### 3. `cmd/run.go` — Populate New Fields at Call Site

```go
// Find the task definition (nil for synthetic tasks).
var taskDesc string
var taskAC   []string
for i := range tasks.Epic.Tasks {
    if tasks.Epic.Tasks[i].ID == taskID {
        taskDesc = tasks.Epic.Tasks[i].Description
        taskAC   = tasks.Epic.Tasks[i].AcceptanceCriteria
        break
    }
}

if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
    TaskID:             taskID,
    TaskType:           taskType,
    SessionFilePath:    sessionPath,
    LogsDir:            logsDir,
    SkillsConfigPath:   skillsConfigPath,
    Description:        taskDesc,
    AcceptanceCriteria: taskAC,
    Attempts:           attempts,
    MaxRetries:         cfg.MaxRetries,
}); err != nil {
    return fmt.Errorf("write active task: %w", err)
}
```

### 4. `.claude/settings.json` — Add Deny Rules

Add to the existing `deny` array:

```json
"Read(tasks.yaml)",
"Read(project-state.yaml)"
```

These work with Claude Code's glob-style permission matching. Agents attempting to read either file will be blocked at the permission layer — a mechanical enforcement rather than an instructional one.

### 5. Skill Files — Remove YAML Reading Instructions

**`implement-feature/SKILL.md` — Phase 1 Research (new version):**

Replace steps 1–3 (reading YAML files) with:

```markdown
## Phase 1: Understand Your Task

Your task briefing is in `logs/ACTIVE_TASK.md`. It contains:
- Your task ID, type, and attempt number
- The session file path to write your result to
- Your task description and acceptance criteria

Start from there, then survey the codebase to understand the context.
```

Remove the pre-flight DONE check entirely. The orchestrator guarantees agents are only invoked for tasks that are not yet DONE — the check is redundant.

**`implement-bugfix/SKILL.md` — Phase 1 Research (new version):**

Replace step 1 (reading `project-state.yaml`) with:

```markdown
## Phase 1: Understand Your Task

Your task briefing is in `logs/ACTIVE_TASK.md`. It contains:
- Your bug ID, type, and attempt number
- The session file path to write your result to
- The full bug report in the "Bug Context" section
```

Step 2 (read ACTIVE_BUG.md) becomes optional — the content is already injected into ACTIVE_TASK.md. Agents can still read the raw file for additional context if needed, but it's no longer required.

---

## New ACTIVE_TASK.md Shape (Feature Task Example)

```markdown
# Active Task

**Task ID**: EPIC-7-002
**Task Type**: feature
**Attempt**: 1 of 5
**Session File**: logs/sessions/EPIC-7/session-EPIC-7-002_attempt-1.md

## Your Task

Implement the `--windows` flag for `doug init` that writes a `.claude/settings.json`
with a PreToolUse hook disabling the Bash tool.

## Acceptance Criteria

- Running `doug init --windows` creates `.claude/settings.json` in the target directory
- The settings file contains a PreToolUse hook matching "Bash" with exit 2
- Existing settings.json is not overwritten without `--force`
- `go build ./...` passes; `go test ./...` passes

---

[skill content]
```

---

## New ACTIVE_TASK.md Shape (Bugfix Task Example)

```markdown
# Active Task

**Task ID**: BUG-EPIC-7-002
**Task Type**: bugfix
**Attempt**: 1 of 5
**Session File**: logs/sessions/EPIC-7/session-BUG-EPIC-7-002_attempt-1.md

---

[bugfix skill content]

---

## Bug Context

[content of logs/ACTIVE_BUG.md]
```

Description and AC sections absent (synthetic task). Bug context already injected. No change to existing bugfix flow.

---

## New ACTIVE_TASK.md Shape (Documentation Task Example)

```markdown
# Active Task

**Task ID**: KB_UPDATE
**Task Type**: documentation
**Attempt**: 1 of 5
**Session File**: logs/sessions/EPIC-7/session-KB_UPDATE_attempt-1.md

---

[documentation skill content]
```

Description and AC sections absent (synthetic task). No other changes — the documentation skill is already compliant.

---

## Test Impact

The existing `activetask_test.go` tests call `WriteActiveTask` without the new fields. Since Go zero-values `string` as `""` and `[]string` as `nil`, all existing tests remain valid — the new sections simply don't appear when fields are empty. No existing test breaks.

New tests to add:

| Test | Verifies |
|------|----------|
| `feature task with description renders Your Task section` | Description appears in output |
| `feature task with AC renders Acceptance Criteria section` | AC bullet list appears |
| `feature task empty description omits both sections` | No "## Your Task" when Description == "" |
| `Attempt and MaxRetries appear in header` | `**Attempt**: 2 of 5` format |
| `bugfix task has no task context sections` | No "## Your Task" or "## Acceptance Criteria" |

---

## Data Flow (After Change)

```
cmd/run.go (loop body)
    │
    ├─ taskID, taskType, attempts — local vars
    ├─ cfg.MaxRetries             — from config
    ├─ tasks.Epic.Tasks           — already loaded; find by ID
    │       └─ Description, AcceptanceCriteria
    │
    ▼
agent.WriteActiveTask(ActiveTaskConfig{...all fields...})
    │
    ▼
logs/ACTIVE_TASK.md
    ├─ Header: TaskID, TaskType, Attempt N of M, SessionFile
    ├─ Body: ## Your Task + ## Acceptance Criteria  (feature/bugfix user tasks)
    ├─ Skill content
    └─ ## Bug Context  (bugfix only)

Agent reads ONLY:
    ├─ logs/ACTIVE_TASK.md         ← complete briefing
    ├─ Source code files            ← for implementation context
    ├─ PRD.md                       ← for product context
    └─ docs/kb/                     ← for patterns and lessons

Agent never reads:
    ├─ tasks.yaml                   ← blocked by settings.json deny rule
    └─ project-state.yaml           ← blocked by settings.json deny rule
```

---

## Dependencies

### Internal
- `internal/agent/activetask.go` — primary change
- `internal/agent/activetask_test.go` — new tests
- `cmd/run.go` — call site update
- `internal/types/types.go` — no changes; `Task.Description` and `Task.AcceptanceCriteria` already exist

### External
- None

---

## Patterns Observed

- **Documentation skill as reference implementation**: `implement-documentation/SKILL.md` already forbids reading YAML files and derives all context from session logs. The feature and bugfix skills need to catch up to this pattern.
- **Zero-value safety**: Adding optional fields to `ActiveTaskConfig` with zero-value omission (`if config.Description != ""`) means the change is fully backward-compatible — existing callers, tests, and the bugfix/documentation task paths all work without modification.
- **Session file path already injected**: The orchestrator already resolves and injects the full session path. Agents never needed to construct it from `current_epic.id` + `attempts` — they were doing redundant work.

---

## Anti-Patterns & Tech Debt (Resolved by This Change)

- **Agents as YAML readers**: Feature and bugfix agents currently read orchestrator state files directly. This creates the "too aware" failure modes (premature EPIC_COMPLETE, state manipulation attempts, retry gaming). Removing access eliminates the failure mode at the source rather than trying to prevent it by instruction.
- **Redundant pre-flight DONE check**: The feature skill instructs agents to check if their task is already `DONE` in `tasks.yaml`. The orchestrator calls `IsEpicAlreadyComplete` before the loop and only invokes agents for tasks that are not done. The agent check is dead logic.
- **Instructional enforcement of boundaries**: Currently, "don't read tasks.yaml" is enforced only by the skill file text. A `settings.json` deny rule makes it mechanical — Claude Code blocks the read before it happens.

---

## PRD Alignment

The PRD defines the agent contract as: a command to invoke, a file to read before invocation (`ACTIVE_TASK.md`), and a file to write after invocation (the session file). Tasks.yaml and project-state.yaml are not part of the agent contract — they are orchestrator-internal state. This change aligns the implementation with the stated contract: agents interact with exactly two files (`ACTIVE_TASK.md` in, session file out) plus the source code they are working on.

---

## Raw Notes

- The `Attempts` field addition also opens the door to **retry context injection**: on attempt 2+, the orchestrator knows the previous session file path (`attempt - 1`) and could inject a "Previous Attempt" section summarising what was tried. This is a natural follow-on to this change and would give retry agents exactly the context they need without letting them read arbitrary state files.
- The `MaxRetries` field in the header gives agents behavioral guidance: an agent on attempt 4 of 5 should be more conservative than one on attempt 1 of 5. This is a free behavioral nudge.
- Denying `Read(tasks.yaml)` and `Read(project-state.yaml)` in `settings.json` also prevents agents from accidentally writing to these files (since you can't write what you haven't read in most workflows). The existing `Write(*.yaml)` deny rule already blocks writes, but the read deny adds a second layer.
- The `internal/templates/init/settings.json` template (proposed in the Windows research report) would ship these deny rules to all new projects automatically — combining both improvements into a single deliverable.
