# doug — Architecture Overview

This document describes the current Go implementation of `doug` as of March 2026.
It is intended as a high-signal reference for contributors and agent planning.

---

## What doug is

`doug` is a CLI orchestrator that runs coding tasks through an AI agent loop.
It owns state progression, retry policy, bug/failure escalation, build/test validation, changelog updates, and Git actions.

`doug` is a single Go binary with three subcommands:
- `doug init`
- `doug run`
- `doug switch`

---

## Runtime filesystem model

The orchestrator is centered around a `.doug/` working directory in the target project.

- `.doug/doug.yaml`: orchestrator configuration
- `.doug/project-state.yaml`: mutable runtime state
- `.doug/tasks.yaml`: user-authored task list
- `.doug/ACTIVE_TASK.md`: per-iteration task briefing for the agent
- `.doug/ACTIVE_BUG.md`: agent-authored bug report (when `outcome: BUG`)
- `.doug/ACTIVE_FAILURE.md`: agent-authored failure report (when `outcome: FAILURE`)
- `.doug/logs/sessions/{epic}/session-{taskID}_attempt-{N}.md`: session result per attempt
- `.doug/logs/bugs/{epic}/bug-{taskID}.md`: archived bug reports
- `.doug/logs/failures/{epic}/failure-{taskID}.md`: archived failure reports

Project-root files also participate:
- `CHANGELOG.md` (best-effort updates)
- `PRD.md`
- `docs/kb/` (KB synthesis output)
- `.agents/skills-config.yaml` (task-type -> skill-name mapping)

---

## Task and outcome model

### Task types

- `feature`: user-defined in `.doug/tasks.yaml`
- `bugfix`: synthetic (orchestrator injected after `BUG`)
- `documentation`: synthetic (KB synthesis `KB_UPDATE` task)
- `manual_review`: synthetic (set after max retries)

`TaskType.IsSynthetic()` is the canonical branch point for logic that must skip `.doug/tasks.yaml` mutations.

### Task statuses

- `TODO`
- `IN_PROGRESS`
- `DONE`
- `BLOCKED`

### Agent outcomes

- `SUCCESS`
- `FAILURE`
- `BUG`
- `EPIC_COMPLETE`

The session parser (`internal/agent/parse.go`) enforces that `outcome` is present and one of these four values.

---

## Configuration and precedence

`internal/config.LoadConfig` reads `.doug/doug.yaml` with partial-file semantics:
missing fields fall back to defaults.

Defaults:
- `agent_command: claude`
- `skills_dir: .agents/skills`
- `build_system: go`
- `max_retries: 5`
- `max_iterations: 20`
- `kb_enabled: true`

In `cmd/run.go`, CLI flags override config fields only when explicitly provided:
- `--agent`
- `--build-system`
- `--max-retries`
- `--max-iterations`
- `--kb-enabled`

`doug switch [agent]` rewrites `.doug/doug.yaml` fields:
- `agent_command`
- `skills_dir`

Supported switch targets are currently `claude`, `codex`, and `gemini`.

---

## `doug run` lifecycle

### Pre-loop

`cmd/run.go` executes this sequence once:

1. Resolve paths from current working directory.
2. Load config from `.doug/doug.yaml` and apply CLI overrides.
3. `CheckDependencies` (agent binary, `git`, and `go` or `npm`).
4. Load `.doug/project-state.yaml` and `.doug/tasks.yaml`.
5. `BootstrapFromTasks` (first-run initialization only).
6. Early exit if `IsEpicAlreadyComplete`.
7. Construct build system (`go` or `npm`).
8. `EnsureProjectReady` pre-flight build/test (skips when uninitialized).
9. `ValidateYAMLStructure`.
10. `EnsureEpicBranch`.
11. `InitializeTaskPointers`.
12. `ValidateStateSync` (skipped for synthetic active task).
13. Persist project state.

### Main loop

For each iteration (bounded by `max_iterations`):

1. `IncrementAttempts` on active task.
2. Persist state before invoking agent.
3. `CreateSessionFile` in `.doug/logs/sessions/...`.
4. `WriteActiveTask` to `.doug/ACTIVE_TASK.md`.
5. Resolve skill name for task type.
6. Invoke agent command.
7. Parse session frontmatter from session file.
8. Dispatch to outcome handler.

Non-zero agent process exit is non-fatal. Session file content is authoritative.

If parse fails, orchestration treats the iteration as `FAILURE`.

---

## Outcome handlers

### `HandleSuccess`

Main behavior:
1. Install dependencies when `dependencies_added` is non-empty.
2. Build verification.
3. Test verification.
4. Record metrics.
5. Best-effort `CHANGELOG.md` update.
6. Mark user-defined task `DONE` in `.doug/tasks.yaml`.
7. For `documentation`: set `current_epic.completed_at`, save state, commit `docs: {taskID}`, return epic-complete signal.
8. Otherwise inject `KB_UPDATE` when needed, else advance task pointer.
9. Save state.
10. Commit (`feat:` / `fix:` / `docs:` prefix by task type).

Build/test/install/commit failures are handled as retry paths (with rollback).

### `HandleFailure`

1. Rollback (warning if rollback itself fails).
2. Record failure metrics.
3. If attempts < `max_retries`: retry next iteration.
4. If attempts >= `max_retries`:
   - archive `.doug/ACTIVE_FAILURE.md` if present
   - mark user-defined task `BLOCKED`
   - set `active_task` to `manual_review`
   - persist state
   - return fatal error

### `HandleBug`

1. Nested bug guard: bugfix task reporting `BUG` is fatal.
2. Rollback.
3. Record bug metrics.
4. Archive `.doug/ACTIVE_BUG.md` if present.
5. Set active task to synthetic bugfix: `BUG-{taskID}`.
6. Set `next_task` to interrupted task.
7. Persist state.

### `HandleEpicComplete`

1. Print metrics summary.
2. Final commit: `chore: finalize {epicID}`.
   - `ErrNothingToCommit` is non-fatal.
   - other commit errors are fatal.
3. Print completion banner.

---

## Skill resolution model

Runtime skill selection is task-type driven:

1. Read `.agents/skills-config.yaml` (`skill_mappings`).
2. Fallback map in code:
   - `feature -> implement-feature`
   - `bugfix -> implement-bugfix`
   - `documentation -> implement-documentation`
   - `manual_review -> manual-review`

`cmd/run.go` then substitutes `{{skill_name}}` in `agent_command` before invoking the agent.

`ACTIVE_TASK.md` contains task metadata and paths. It does not embed SKILL.md content.

---

## `doug init` scaffolding

`doug init` creates baseline project artifacts:

- `.doug/doug.yaml`
- `.doug/project-state.yaml`
- `.doug/tasks.yaml`
- `PRD.md`
- `AGENTS.md`
- `.agents/skills-config.yaml`
- `.agents/skills/implement-feature/SKILL.md`
- `.agents/skills/implement-bugfix/SKILL.md`
- `.agents/skills/implement-documentation/SKILL.md`
- `.doug/logs/SESSION_RESULTS_TEMPLATE.md`
- `.doug/logs/BUG_REPORT_TEMPLATE.md`
- `.doug/logs/FAILURE_REPORT_TEMPLATE.md`
- `.gemini/settings.json`
- `docs/kb/` directory

`CLAUDE.md` template is intentionally skipped by current init routing.

---

## Package map

- `cmd/`
  - `run.go`: orchestration loop wiring
  - `init.go`: scaffolding and template copy
  - `switch.go`: agent profile switching in `.doug/doug.yaml`
  - `agents.go`: built-in agent registry
- `internal/types`: shared types and constants
- `internal/state`: YAML load/save + atomic writes
- `internal/config`: config defaults and loading
- `internal/orchestrator`: bootstrap, pointer math, validation, startup checks, loop context
- `internal/agent`: session file I/O, active task briefing, skill lookup, agent execution, result parsing
- `internal/handlers`: outcome handlers
- `internal/git`: branch, rollback, commit operations
- `internal/build`: Go/NPM build system adapters
- `internal/changelog`: scoped `## [Unreleased]` updates
- `internal/metrics`: per-task and epic summary metrics
- `internal/templates`: embedded runtime/init templates
- `internal/log`: structured console logging helpers

---

## Invariants and trust boundaries

- State writes are atomic (`.tmp` + rename).
- Synthetic tasks are never persisted to `.doug/tasks.yaml`.
- Attempt counters are persisted before agent invocation.
- Agent exit code does not determine outcome; session frontmatter does.
- Rollback protects `.doug/project-state.yaml` and `.doug/tasks.yaml`.
- Changelog updates are best-effort and non-fatal.
- Handler/state errors that can corrupt flow are fatal (exit code 1).

---

## Known architecture seams

These are current seams worth keeping explicit when changing behavior:

- `skills_dir` in config is not currently the source of truth for runtime skill mapping lookup; `cmd/run.go` reads `.agents/skills-config.yaml` directly.
- Agent profiles in `cmd/agents.go` still point at agent-specific `skills_dir` values (`.claude/skills`, `.codex/skills`, `.gemini/skills`) while `doug init` scaffolds shared skills under `.agents/skills`.

Any future refactor should decide whether `skills_dir` remains informational or becomes a first-class runtime lookup path.
