---
title: internal/agent — ActiveTask, Invoke, Parse, Archive
updated: 2026-04-10
category: Packages
tags: [agent, active-task, invoke, parse, exec, frontmatter, yaml, archive]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/log.md
  - docs/kb/packages/templates.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
---

# internal/agent — ActiveTask, Invoke, Parse, Archive

## Overview

`internal/agent` is the boundary between the orchestrator and the agent process. It owns the full agent lifecycle for one iteration:

1. **Write** `ACTIVE_TASK.md` (task briefing + result block stub) → `activetask.go`
2. **Invoke** the agent command, stream output live → `invoke.go`
3. **Archive** `ACTIVE_TASK.md` to session log before any state change → `archive.go`
4. **Parse** the `## Agent Result` block from `ACTIVE_TASK.md`, validate the outcome → `parse.go`
5. **Clean up** the live root `ACTIVE_TASK.md` once outcome handling is complete → `archive.go`

No other package directly invokes the agent or reads session files.

> **EPIC-11 change**: The separate session file (`CreateSessionFile`) is gone. Agents write their result directly into `ACTIVE_TASK.md` under a `## Agent Result` heading. `ParseSessionResult` reads from `ACTIVE_TASK.md` and uses this heading as an anchor. The `SessionFilePath` field was removed from `ActiveTaskConfig`.

---

## activetask.go — WriteActiveTask, GetSkillForTaskType

### ActiveTaskConfig

```go
type ActiveTaskConfig struct {
    TaskID             string
    TaskType           types.TaskType
    DougDir            string   // ACTIVE_TASK.md → {DougDir}/ACTIVE_TASK.md
    Description        string   // task description from tasks.yaml
    AcceptanceCriteria []string // acceptance criteria from tasks.yaml
    Attempts           int      // current attempt number
    MaxRetries         int      // configured max retries
    BuildSystem        string   // e.g. "go", "npm", "pnpm"; controls briefing section
    TestFailureOutput  string   // non-empty: inject "Previous Test Failure Output" section
    ContextSections    []ActiveTaskSection // optional extra markdown sections appended before Agent Result
}

type ActiveTaskSection struct {
    Heading string
    Body    string
}
```

### WriteActiveTask

```go
func WriteActiveTask(config ActiveTaskConfig, l log.Logger) error
```

Writes `{DougDir}/ACTIVE_TASK.md`. **Always overwrites; never archives.**

> **EPIC-12**: `l log.Logger` added as second parameter. Warning messages (e.g., missing `ACTIVE_BUG.md`) are routed through the logger instead of the package-level `log.Warning`.

Content written:
1. Briefing header: Active Bug File path, Failure File path, and PRD File path
2. Task ID, type, attempt number, description, and acceptance criteria
3. Conditional `## Build System` section — when `BuildSystem` is a known key in `config.BuildSystems`
4. For bugfix tasks only: `## Bug Context` section from `{DougDir}/ACTIVE_BUG.md`
5. When `TestFailureOutput` is non-empty: `## Previous Test Failure Output` section with the raw test output, instructing the agent to fix the failures
6. Any `ContextSections` blocks appended as `## <Heading>` sections before the result stub
7. `## Agent Result` stub at the bottom — an empty YAML frontmatter block that the agent fills in with `outcome`, `changelog_entry`, and `dependencies_added`, followed by the implementation summary headings the agent writes into

If `ACTIVE_BUG.md` is missing for a bugfix task, a `log.Warning` is emitted and the section is omitted — not a fatal error. If `BuildSystem` is empty or not in the registry, the build system section is silently omitted.

`os.MkdirAll` is called on `DougDir` before writing.

### GetSkillForTaskType

```go
func GetSkillForTaskType(taskType, configPath string) (string, error)
```

Resolves the skill name for a task type using a two-tier fallback:

| Tier | Source | Used when |
|------|--------|-----------|
| 1 | `{configDir}/skills/{skillName}/SKILL.md` | Normal operation |
| 2 | hardcoded default names | SKILL.md file missing (logs warning) |

Skill name resolution (`resolveSkillName` private helper) also has two tiers:

| Tier | Source | Used when |
|------|--------|-----------|
| 1 | `skills-config.yaml` → `skill_mappings[taskType]` | Config present and type listed |
| 2 | `hardcodedSkillNames` map | Config absent or type not in config |

The resolved skills are generic task workflows. Repository-specific operating rules are expected to live in `AGENTS.md`, not in the task mapping itself.

**Hardcoded skill names**:

| Task type | Skill name |
|-----------|-----------|
| `feature` | `implement-feature` |
| `bugfix` | `implement-bugfix` |
| `documentation` | `implement-documentation` |
| `manual_review` | `manual-review` |
| `scaffold` | `scaffold` |

Returns an error for unknown task types not found in either source.

---

## invoke.go — RunAgent

```go
func RunAgent(
    ctx context.Context,
    agentCommand, projectRoot string,
    heartbeatInterval time.Duration,
    heartbeatFn func(elapsed time.Duration),
    output io.Writer,
) (time.Duration, error)
```

Invokes the agent. Blocks until the agent exits. Returns wall-clock duration.

> **EPIC-12**: `ctx context.Context` added as the first parameter. Cancelling the context kills the subprocess and returns `ctx.Err()`. The heartbeat goroutine also exits on `ctx.Done()`.

**Command parsing**: `splitShellArgs(agentCommand)` tokenises the command respecting single/double quotes and backslash escapes (POSIX-style). No `sh -c`, no shell wrapping. Empty/whitespace-only commands return a validation error before `exec` is reached.

**Context cancellation**: A dedicated goroutine calls `cmd.Process.Kill()` when `ctx.Done()` fires. After `cmd.Wait()` returns, if `ctx.Err() != nil` it is returned directly (not the exit code error).

**Output routing**: The `output` parameter controls where the agent's stdout and stderr go.

```go
cmd := exec.Command(parts[0], parts[1:]...)
cmd.Dir = projectRoot
if output != nil {
    cmd.Stdout = output   // capture to file / discard
    cmd.Stderr = output
} else {
    cmd.Stdout = os.Stdout   // fallback: stream live to terminal
    cmd.Stderr = os.Stderr
}
```

Pass `nil` to get the original pass-through behaviour. In `doug run`, the orchestrator always passes an open `*os.File` pointing to `.doug/logs/output/{epic}/output-{taskID}_attempt-{N}.log`. This prevents agents that unconditionally stream to the terminal (e.g. `codex exec`) from polluting the orchestrator display; output is still preserved on disk for post-run inspection.

**Exit code**: A non-zero exit code returns `fmt.Errorf("agent exited with code %d", exitErr.ExitCode())`. Callers can rely on the exit code appearing in the error message.

**Heartbeat support**: When `heartbeatInterval > 0` and `heartbeatFn != nil`, `RunAgent` emits elapsed-time callbacks on a ticker while the agent process is alive. Heartbeat goroutine exits on `ctx.Done()` or process completion.

> See [Exec Command Pattern](../patterns/pattern-exec-command.md) for the full streaming vs. buffering rationale.

---

## archive.go — ArchiveActiveTask

```go
func ArchiveActiveTask(dougDir, logsDir, epic, taskID string, attempt int) error
```

Copies `{dougDir}/ACTIVE_TASK.md` to `{logsDir}/sessions/{epic}/session-{taskID}_attempt-{attempt}.md` before any state change.

**Called as the first step in all four outcome handlers** (HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete). This preserves the full task briefing + agent result as a durable log entry even if the loop crashes mid-handler.

**Non-fatal**: `ArchiveActiveTask` errors are logged as warnings so a missing `ACTIVE_TASK.md` never blocks the loop. The handler continues regardless.

**Directory creation**: `os.MkdirAll` is called on the destination directory before copying.

```go
func CleanupActiveTask(dougDir string) error
```

Removes the live `{dougDir}/ACTIVE_TASK.md` after the outcome has been fully handled. Missing files are ignored.

Cleanup timing is intentional:

- `HandleFailure`, `HandleBug`, and non-epic-complete `HandleSuccess` returns remove the live file before returning to the main loop.
- `HandleSuccess` keeps the file in place when it returns `EpicComplete`, because `HandleEpicComplete` still needs the live briefing for the final runtime snapshot/archive step.
- `HandleEpicComplete` removes the live file after `plan.FinalizeEpicCompletion(...)` has archived the runtime snapshot, so `.doug/logs/archives/{epic}/ACTIVE_TASK.md` remains available when the file existed at finalization time.

---

## parse.go — ParseSessionResult

```go
func ParseSessionResult(filePath string) (*types.SessionResult, error)
```

Reads `ACTIVE_TASK.md` (path passed by caller), locates the `## Agent Result` heading, then extracts YAML frontmatter from the block that follows.

### Anchor-based extraction

```
## Agent Result

---
outcome: "SUCCESS"
changelog_entry: "..."
dependencies_added: []
---
```

`ParseSessionResult` scans for a line matching `## Agent Result` first, then looks for the `---` pair only within the lines that follow. This prevents false positives from any `---` horizontal-rule lines elsewhere in the briefing document.

**Backward compatibility**: If `## Agent Result` is not found, `searchFrom = 0` and the function behaves exactly as before (first `---` pair). This allows legacy session files and tests to continue working.

### Typed Errors

| Error | Type | Meaning |
|-------|------|---------|
| `os.ErrNotExist` | stdlib sentinel | File not found; use `errors.Is` |
| `ErrNoFrontmatter` | `errors.New` sentinel | No `---` delimiters or only one |
| `ErrMissingOutcome` | `errors.New` sentinel | Outcome field absent or empty |
| `*ErrInvalidOutcome` | struct with `Value string` | Outcome not in valid set |

### Valid Outcomes

```go
types.OutcomeSuccess        // "SUCCESS"
types.OutcomeBug            // "BUG"
types.OutcomeFailure        // "FAILURE"
types.OutcomeEpicComplete   // "EPIC_COMPLETE"
```

Both CRLF and LF are handled via pre-normalisation. Extra frontmatter fields are silently ignored.

---

## Key Decisions

**`## Agent Result` as anchor, not last `---` pair**: The heading is explicit, readable, and immune to horizontal-rule `---` lines appearing anywhere in the briefing body. Scanning for the last `---` pair was fragile and caused false positives in briefings with markdown section dividers.

**Backward-compatible fallback in `ParseSessionResult`**: If `## Agent Result` is not found, `searchFrom = 0` so the function behaves exactly as before. This handles legacy session files without a code branch.

**`ArchiveActiveTask` is non-fatal**: The archive is a best-effort audit trail. A missing ACTIVE_TASK.md (e.g., first iteration) must never block the handler — log warning, continue.

**`ArchiveActiveTask` placed in `internal/agent`**: I/O concerns (reading/writing agent-boundary files) belong to the agent package. Handlers import `agent` for this — not the reverse.

**`TestFailureOutput` in `ActiveTaskConfig`**: Injecting test failure context directly into the briefing gives the agent the exact failing test output it needs to fix the issue, without requiring a separate mechanism.

**`strings.Fields` for command splitting**: Handles multiple spaces and tabs; returns an empty slice on blank input.

**Sentinel errors for `ErrNoFrontmatter` and `ErrMissingOutcome`**: Expected failure modes with no diagnostic payload. `*ErrInvalidOutcome` is a struct because callers may need the bad value for error messages.

---

## Edge Cases & Gotchas

**`ACTIVE_TASK.md` is canonical for live agent output, not durable history**: Always `{DougDir}/ACTIVE_TASK.md` during an active iteration. The agent writes its result there; `ArchiveActiveTask` copies it to the session log; `ParseSessionResult` reads from it; handlers then remove the live file after outcome processing. Never re-introduce a separate session file path.

**Documentation tasks**: `TaskType` is preserved as `types.TaskTypeDocumentation` in the written briefing. No special-casing needed; only bugfix gets the extra Bug Context section.

**KB_UPDATE fallback**: If a documentation task leaves the default result stub untouched (`ErrMissingOutcome` or `ErrNoFrontmatter`), the orchestrator treats it as `EPIC_COMPLETE` instead of retrying and eventually blocking the synthetic task. This is a narrow safety valve for KB synthesis only; all other task types still treat parse failures as `FAILURE`.

**`ACTIVE_BUG.md` missing for bugfix**: Warning, not fatal. The task brief is still written without the bug context.

**`ParseSessionResult` does not validate `changelog_entry`**: Only `outcome` is validated. Empty `changelog_entry` is legal.

---

## Related Topics

- [internal/types](types.md) — `SessionResult`, `TaskType`, `Outcome` constants
- [internal/templates](templates.md) — `Runtime`, `Init` exports; template file contents
- [internal/log](log.md) — `log.Warning` used in graceful-degradation paths
- [Exec Command Pattern](../patterns/pattern-exec-command.md) — no `sh -c`, streaming output
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — when to use (state files) vs. when not to (session files)
- [Go Infrastructure](../infrastructure/go.md) — project structure and approved dependencies
