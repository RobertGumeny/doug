# doug — Architecture Overview

This document is a dense reference for LLM planning and brainstorming sessions. It describes what doug *is*, how it works, what each package does, and the invariants that must be preserved when changing anything.

---

## What doug is

**doug** is a CLI orchestrator that drives AI coding agents (Claude Code, Aider, etc.) through a structured task loop. It manages a queue of tasks in `tasks.yaml`, invokes an agent for each one, reads the agent's result, and decides what to do next — commit, retry, inject a bugfix, or stop. It owns all Git operations so agents don't have to.

Doug is a Go binary with two subcommands: `doug init` (scaffold a new project) and `doug run` (run the orchestration loop).

---

## The two state files

Everything in doug's state machine lives in two YAML files at the project root:

**`project-state.yaml`** — mutable runtime state. The orchestrator reads and writes this every iteration. Key fields:
- `current_epic` — epic ID/name/branch and timestamps
- `active_task` — the task being worked on right now (`id`, `type`, `attempts`)
- `next_task` — the task queued after the active one
- `metrics` — per-task telemetry (outcome, duration, timestamp)
- `kb_enabled` — whether a KB synthesis documentation pass runs after all feature tasks

**`tasks.yaml`** — the user-editable task list. The orchestrator reads this and writes task statuses (`TODO → IN_PROGRESS → DONE / BLOCKED`). Agents must never touch it.

Both files are written atomically (`.tmp` → `os.Rename`). Agents are blocked from reading them (via `.claude/settings.json` deny list) so they always work from fresh context in `ACTIVE_TASK.md`.

---

## Key files agents interact with

**`logs/ACTIVE_TASK.md`** — written by doug before each agent invocation. Contains task ID, type, session file path, attempt number, description, acceptance criteria, and the full skill instructions for the task type. Always overwritten in place; never archived. This is the agent's primary briefing.

**`logs/sessions/{epic}/session-{taskID}_attempt-{N}.md`** — written by the agent as its result. Contains exactly three YAML frontmatter fields: `outcome`, `changelog_entry`, `dependencies_added`. Doug parses this after the agent exits.

**`logs/ACTIVE_BUG.md`** — written by the agent when it discovers a blocking bug unrelated to its task. Doug reads this, archives it to `logs/bugs/{epic}/bug-{taskID}.md`, and injects a `bugfix` task as the next active task.

**`logs/ACTIVE_FAILURE.md`** — written by the agent when it cannot complete a task and escalates. Doug archives it to `logs/failures/{epic}/failure-{taskID}.md` and marks the task BLOCKED after `max_retries`.

---

## doug.yaml — orchestrator configuration

```yaml
agent_command: claude       # command to invoke the agent
build_system: go            # "go" or "npm"
max_retries: 5              # FAILURE outcomes before marking a task BLOCKED
max_iterations: 20          # loop iterations before exit 0
kb_enabled: true            # inject a KB synthesis task after all feature tasks
```

Missing `doug.yaml` → defaults are used (never an error). Partial files overlay only fields present. CLI flags (`--agent`, `--build-system`, etc.) always override config.

---

## Task types

| Type | Origin | In tasks.yaml? |
|------|--------|----------------|
| `feature` | User-defined | Yes |
| `bugfix` | Orchestrator-injected on BUG outcome | No (synthetic) |
| `documentation` | Orchestrator-injected when `kb_enabled` and all tasks done | No (synthetic) |
| `manual_review` | Orchestrator sets this when a task is BLOCKED | No |

**Synthetic tasks** (`bugfix`, `documentation`) live only in `project-state.yaml.active_task`. They are never written to `tasks.yaml`. The method `TaskType.IsSynthetic()` is the canonical check. Any code that touches `tasks.yaml` must skip synthetic tasks.

---

## Task statuses

| Status | Meaning |
|--------|---------|
| `TODO` | Not started |
| `IN_PROGRESS` | Agent is working / orchestrator was interrupted mid-task |
| `DONE` | Completed successfully |
| `BLOCKED` | Failed `max_retries` times; requires human intervention |

---

## Agent outcomes

The agent writes one of these four values to the `outcome` field in its session file:

| Outcome | What doug does next |
|---------|---------------------|
| `SUCCESS` | Install deps → build → test → commit → advance to next task |
| `FAILURE` | Rollback → retry; block task after `max_retries` |
| `BUG` | Rollback → archive `ACTIVE_BUG.md` → inject bugfix task; resume interrupted task after bugfix |
| `EPIC_COMPLETE` | Print summary → finalize commit → exit 0 |

---

## The full run loop

### Pre-loop (runs once)

```
LoadConfig (doug.yaml) → apply CLI flag overrides
CheckDependencies (agent binary, git, go/npm must be on PATH)
LoadProjectState + LoadTasks
BootstrapFromTasks (first-run only: populate current_epic, active_task, next_task)
IsEpicAlreadyComplete → exit 0 if all tasks DONE and KB synthesis done/disabled
NewBuildSystem → EnsureProjectReady (preflight build + test; skipped if go.sum absent)
ValidateYAMLStructure (structural sanity check)
EnsureEpicBranch (create/checkout feature/{epicID} branch)
InitializeTaskPointers (set active_task = first IN_PROGRESS or TODO; next_task = next TODO)
ValidateStateSync (skipped for synthetic active task — they're not in tasks.yaml)
SaveProjectState
```

### Main loop (per iteration, up to max_iterations)

```
IsEpicAlreadyComplete → exit 0
ValidateYAMLStructure → ValidateStateSync → NeedsKBSynthesis
IncrementAttempts → SaveProjectState   ← persisted BEFORE agent runs

CreateSessionFile (logs/sessions/{epic}/session-{taskID}_attempt-{N}.md)
WriteActiveTask (logs/ACTIVE_TASK.md with task brief + skill instructions)
RunAgent (streams stdout/stderr live; non-zero exit is non-fatal)
ParseSessionResult (parse YAML frontmatter from session file; parse failure → treat as FAILURE)

switch outcome:
  SUCCESS        → HandleSuccess  → Continue | Retry | EpicComplete
  FAILURE        → HandleFailure  → nil (retry) | error (blocked, exit 1)
  BUG            → HandleBug      → nil (continue with bugfix task) | error (nested bug, exit 1)
  EPIC_COMPLETE  → HandleEpicComplete → nil (exit 0) | error (exit 1)

max_iterations reached → exit 0
```

---

## Outcome handler details

### HandleSuccess
1. `BuildSystem.Install()` if `dependencies_added` is non-empty → on failure: rollback → Retry
2. `BuildSystem.Build()` → on failure: rollback → Retry
3. `BuildSystem.Test()` → on failure: rollback → Retry
4. Record metrics (non-fatal)
5. Update CHANGELOG.md (non-fatal)
6. Mark task DONE in tasks.yaml (skip for synthetic tasks)
7. If documentation task: set `CompletedAt`, commit with `"docs: {taskID}"`, return EpicComplete
8. If `NeedsKBSynthesis()`: inject KB_UPDATE documentation task; else `AdvanceToNextTask()`
9. SaveProjectState
10. `git commit` with `feat:`/`fix:` prefix → on failure: Retry (non-fatal, state already saved)

**Rollback preserves `project-state.yaml` and `tasks.yaml`** — the attempt count write survives.

### HandleFailure
1. Rollback (non-fatal if git fails)
2. Record metrics
3. If `attempts < max_retries`: log warning, return nil (loop retries next iteration)
4. If `attempts >= max_retries`: archive ACTIVE_FAILURE.md, mark task BLOCKED, set active_task.type = manual_review, return error → exit 1

### HandleBug
1. **Nested bug guard**: if current task is already a bugfix → fatal error (prevents death spiral)
2. Rollback
3. Record metrics
4. Archive `ACTIVE_BUG.md` → `logs/bugs/{epic}/bug-{taskID}.md`
5. Set `active_task = { type: bugfix, id: "BUG-{taskID}" }`
6. Set `next_task = { type: <resolved from tasks.yaml or ctx>, id: ctx.TaskID }` (resume interrupted task after bugfix)
7. SaveProjectState

### HandleEpicComplete
1. PrintEpicSummary
2. `git commit "chore: finalize {epicID}"` — `ErrNothingToCommit` is non-fatal, all other errors fatal
3. Print completion banner, exit 0

---

## Package map

```
main.go                   → cmd.Execute() (one line)
cmd/run.go                → wires pre-loop and main loop; all logic in internal/
cmd/init.go               → doug init: scaffolds project files from embedded templates

internal/types/           → all shared structs and typed constants; single source of truth
internal/state/           → LoadProjectState, SaveProjectState, LoadTasks, SaveTasks; atomic writes
internal/config/          → OrchestratorConfig, LoadConfig (partial-file pattern), DetectBuildSystem
internal/log/             → Info, Success, Warning, Error, Fatal, Section; ANSI colors
internal/build/           → BuildSystem interface; GoBuildSystem (go build/test); NpmBuildSystem (npm)
internal/git/             → EnsureEpicBranch, RollbackChanges (in-memory backup), Commit
internal/orchestrator/    → BootstrapFromTasks, task pointer management, tiered validation, LoopContext
internal/metrics/         → RecordTaskMetrics, UpdateMetricTotals, PrintEpicSummary; non-fatal
internal/changelog/       → UpdateChangelog — idempotent pure-Go CHANGELOG.md insert; non-fatal
internal/agent/           → CreateSessionFile, WriteActiveTask, GetSkillForTaskType, RunAgent, ParseSessionResult
internal/templates/       → //go:embed runtime/ (session_result.md) and init/ (scaffolding files)
internal/handlers/        → HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete
```

### Key package rules
- `internal/types` imports nothing from this project. All other packages import from it.
- `cmd/` wires things together only. Logic belongs in `internal/`.
- `internal/handlers` imports `internal/orchestrator` (for `LoopContext`). `LoopContext` is defined in `internal/orchestrator/context.go` to avoid a circular import.
- No go-git. All git calls use `exec.Command("git", ...)` with an explicit args slice.
- No `sh -c`. Ever.

---

## Trust boundary

| Doug owns | Agents own |
|-----------|-----------|
| All Git operations | Source code and tests |
| Updating project-state.yaml and tasks.yaml | Running build/test/lint |
| Updating CHANGELOG.md | Writing the session result file |
| Archiving session/bug/failure files in logs/ | Writing logs/ACTIVE_BUG.md |
| Branch creation, commit, rollback | Writing logs/ACTIVE_FAILURE.md |

Agents cannot break the state machine because:
- `.claude/settings.json` blocks reads of state files (agents act on ACTIVE_TASK.md only)
- `.claude/settings.json` blocks all git write operations

---

## LoopContext — the per-iteration struct

```go
type LoopContext struct {
    // Snapshotted after IncrementAttempts
    TaskID, TaskType, Attempts, CurrentEpic

    // Agent output (after ParseSessionResult)
    SessionResult *types.SessionResult

    // Infrastructure
    Config, BuildSystem, ProjectRoot, TaskStartTime

    // Mutable state — changes persist across handlers within one iteration
    State *types.ProjectState
    Tasks *types.Tasks

    // File system paths
    StatePath, TasksPath, LogsDir, ChangelogPath
}
```

Constructed fresh each iteration in `cmd/run.go` after `IncrementAttempts`. Handler mutations to `State` and `Tasks` are visible to subsequent handlers in the same iteration.

---

## Skill resolution

When writing ACTIVE_TASK.md, doug looks up skill instructions for the task type via a two-tier fallback:

1. `.claude/skills/{skillName}/SKILL.md` (customizable per project)
2. Hardcoded content compiled into the binary

Skill name is resolved via:
1. `.claude/skills-config.yaml` → `skill_mappings[taskType]`
2. Hardcoded map: `feature → implement-feature`, `bugfix → implement-bugfix`, `documentation → implement-documentation`

---

## KB synthesis

When `kb_enabled: true` and all user-defined tasks are DONE, doug injects a synthetic `KB_UPDATE` documentation task. This runs the `implement-documentation` skill, which synthesizes session logs into `docs/kb/`. After the documentation task succeeds, `IsEpicAlreadyComplete` returns true on the next loop iteration and the run exits 0.

---

## Validation tiers

Doug uses a three-tier model for handling unexpected conditions:

| Tier | Behavior | Example |
|------|----------|---------|
| 1 (silent) | Self-correct with no log output | Attempt counter off-by-one |
| 2 (warning) | Log warning, auto-correct, continue | `ValidateStateSync` redirects active task with one unambiguous candidate |
| 3 (fatal) | Log error, exit 1 | Nested bug, ambiguous state sync, git failures that touch shared state |

Before any self-correction: ask "could this same condition re-trigger next iteration?" If yes → Tier 3.

`ValidateStateSync` is skipped entirely when the active task is synthetic (bugfix/documentation) because synthetic tasks are intentionally absent from `tasks.yaml`, so "not found" is always expected — not a signal of corruption.

---

## Git commit message conventions

| Task type | Commit prefix |
|-----------|---------------|
| `feature` | `feat: {taskID}` |
| `bugfix` | `fix: {taskID}` |
| `documentation` | `docs: {taskID}` |
| epic finalization | `chore: finalize {epicID}` |

---

## File path conventions

- Session files: `logs/sessions/{epic}/session-{taskID}_attempt-{N}.md`
- Bug archives: `logs/bugs/{epic}/bug-{taskID}.md`
- Failure archives: `logs/failures/{epic}/failure-{taskID}.md`
- Active task briefing: `logs/ACTIVE_TASK.md` (flat, always overwritten)
- Active bug: `logs/ACTIVE_BUG.md` (flat, always overwritten by agent)
- Active failure: `logs/ACTIVE_FAILURE.md` (flat, always overwritten by agent)

Source of archived files is always the **flat** `logs/ACTIVE_*.md` path — never a subdirectory variant.

---

## Exit codes

| Condition | Exit code |
|-----------|-----------|
| All tasks DONE | 0 |
| Max iterations reached | 0 |
| Epic complete (HandleEpicComplete returns nil) | 0 |
| Task blocked after max retries | 1 |
| Nested bug detected | 1 |
| HandleEpicComplete returns error | 1 |
| Fatal state/git/validation error | 1 |

---

## What `doug init` scaffolds

Running `doug init` in a new project creates:
- `doug.yaml` — orchestrator config
- `tasks.yaml` — empty task list
- `project-state.yaml` — empty state
- `PRD.md` — product requirements template
- `CLAUDE.md` — agent onboarding guide (the file you're reading now)
- `.claude/settings.json` — deny list blocking agents from touching state files and git
- `.claude/skills-config.yaml` — skill name mappings
- `.claude/skills/implement-feature/SKILL.md`
- `.claude/skills/implement-bugfix/SKILL.md`
- `.claude/skills/implement-documentation/SKILL.md`
- `logs/SESSION_RESULTS_TEMPLATE.md`
- `logs/BUG_REPORT_TEMPLATE.md`
- `logs/FAILURE_REPORT_TEMPLATE.md`

`doug init` is idempotent with `--force`; without it, it exits if any output file already exists.
