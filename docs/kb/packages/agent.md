---
title: internal/agent — Session, ActiveTask, Invoke, Parse
updated: 2026-02-24
category: Packages
tags: [agent, session, active-task, invoke, parse, exec, frontmatter, yaml]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/log.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
---

# internal/agent — Session, ActiveTask, Invoke, Parse

## Overview

`internal/agent` is the boundary between the orchestrator and the agent process. It owns the full agent lifecycle for one iteration:

1. **Create** the session file (pre-filled template) → `session.go`
2. **Write** `ACTIVE_TASK.md` (task briefing + skill instructions) → `activetask.go`
3. **Invoke** the agent command, stream output live → `invoke.go`
4. **Parse** the session file the agent wrote, validate the outcome → `parse.go`

No other package directly invokes the agent or reads session files.

---

## session.go — CreateSessionFile

```go
func CreateSessionFile(logsDir, epic, taskID string, attempt int) (string, error)
```

Creates the pre-filled session file the agent will write its results into.

**Path pattern**: `{logsDir}/sessions/{epic}/session-{taskID}_attempt-{attempt}.md`

**Template**: Embedded from `internal/templates/session_result.md` via `//go:embed`. The `task_id: ""` placeholder is replaced with the actual task ID using `strings.ReplaceAll` + `fmt.Sprintf("task_id: %q", taskID)`.

**Write method**: `os.WriteFile` directly (no atomic rename). Session files are created fresh before the agent runs — they are never updated in place, so partial-write corruption is not a concern here.

**Directory creation**: `os.MkdirAll` is called before writing so the parent directory is always created.

Returns the path to the created file (passed to `WriteActiveTask` and then to the agent).

### internal/templates package

`internal/templates/templates.go` exports a single `string` variable:

```go
//go:embed session_result.md
var SessionResult string
```

This is the only export. The `agent` package imports it; no other package does. Go embed does not allow `..` paths, so the template lives in its own package rather than adjacent to `session.go`.

---

## activetask.go — WriteActiveTask, GetSkillForTaskType

### ActiveTaskConfig

```go
type ActiveTaskConfig struct {
    TaskID           string
    TaskType         types.TaskType
    SessionFilePath  string
    LogsDir          string          // ACTIVE_TASK.md → {LogsDir}/ACTIVE_TASK.md
    SkillsConfigPath string          // e.g. ".claude/skills-config.yaml"
}
```

### WriteActiveTask

```go
func WriteActiveTask(config ActiveTaskConfig) error
```

Writes `{LogsDir}/ACTIVE_TASK.md`. **Always overwrites; never archives.**

Content written:
1. Task ID, type, and session file path header
2. Skill instructions (via `GetSkillForTaskType`)
3. For bugfix tasks only: `## Bug Context` section from `{LogsDir}/ACTIVE_BUG.md`

If `ACTIVE_BUG.md` is missing for a bugfix task, a `log.Warning` is emitted and the section is omitted — this is not a fatal error.

`os.MkdirAll` is called on `LogsDir` before writing (consistent with `CreateSessionFile`).

### GetSkillForTaskType

```go
func GetSkillForTaskType(taskType, configPath string) (string, error)
```

Resolves skill instructions for a task type using a two-tier fallback:

| Tier | Source | Used when |
|------|--------|-----------|
| 1 | `{configDir}/skills/{skillName}/SKILL.md` | Normal operation |
| 2 | `hardcodedSkillContent` map | SKILL.md file missing (logs warning) |

Skill name resolution (`resolveSkillName` private helper) also has two tiers:

| Tier | Source | Used when |
|------|--------|-----------|
| 1 | `skills-config.yaml` → `skill_mappings[taskType]` | Config present and type listed |
| 2 | `hardcodedSkillNames` map | Config absent or type not in config |

**Hardcoded skill names** (mirror the Bash `get_skill_for_task_type` fallback exactly):

| Task type | Skill name |
|-----------|-----------|
| `feature` | `implement-feature` |
| `bugfix` | `implement-bugfix` |
| `documentation` | `implement-documentation` |
| `manual_review` | `manual-review` |

Returns an error for unknown task types not found in either source.

---

## invoke.go — RunAgent

```go
func RunAgent(agentCommand, projectRoot string) (time.Duration, error)
```

Invokes the agent. Blocks until the agent exits. Returns wall-clock duration.

**Command parsing**: `strings.Fields(agentCommand)` splits on any whitespace. No `sh -c`, no shell wrapping. Empty/whitespace-only commands return a validation error before `exec` is reached.

```go
parts := strings.Fields(trimmed)
cmd := exec.Command(parts[0], parts[1:]...)
cmd.Dir = projectRoot
cmd.Stdout = os.Stdout   // stream live — never buffer
cmd.Stderr = os.Stderr   // stream live — never buffer
```

**Duration measurement**: Wall-clock time from immediately before `cmd.Start()` to `cmd.Wait()` completion. Includes all agent I/O.

**Exit code**: A non-zero exit code returns `fmt.Errorf("agent exited with code %d", exitErr.ExitCode())`. Callers can rely on the exit code appearing in the error message.

> See [Exec Command Pattern](../patterns/pattern-exec-command.md) for the full streaming vs. buffering rationale.

---

## parse.go — ParseSessionResult

```go
func ParseSessionResult(filePath string) (*types.SessionResult, error)
```

Reads the session file, extracts YAML frontmatter, and validates the outcome.

### Typed Errors

| Error | Type | Meaning |
|-------|------|---------|
| `os.ErrNotExist` | stdlib sentinel | File not found; use `errors.Is` |
| `ErrNoFrontmatter` | `errors.New` sentinel | No `---` delimiters or only one |
| `ErrMissingOutcome` | `errors.New` sentinel | Outcome field absent or empty |
| `*ErrInvalidOutcome` | struct with `Value string` | Outcome not in valid set |

### Frontmatter Extraction

Pure Go string scanning — no `awk`, no `yq`, no regex:

```go
// Normalise line endings first
content := strings.ReplaceAll(string(data), "\r\n", "\n")

// Find first --- (start), then second --- (end)
// strings.TrimSpace(line) == "---" tolerates trailing whitespace
// Frontmatter is lines[start+1 : end]
```

Both CRLF and LF are handled via pre-normalisation.

### Valid Outcomes

```go
types.OutcomeSuccess        // "SUCCESS"
types.OutcomeBug            // "BUG"
types.OutcomeFailure        // "FAILURE"
types.OutcomeEpicComplete   // "EPIC_COMPLETE"
```

Extra fields in the frontmatter beyond the three `SessionResult` fields are silently ignored (`yaml.Unmarshal` default behaviour).

---

## Key Decisions

**`os.WriteFile` for session files, not atomic rename**: Session files are created fresh before the agent runs and not updated in-place. Atomic rename is reserved for state files (`project-state.yaml`, `tasks.yaml`) where partial writes are a real corruption risk.

**`strings.Fields` for command splitting**: Handles multiple spaces and tabs; returns an empty slice on blank input. `strings.Split(s, " ")` is incorrect here — it produces empty strings on multiple consecutive spaces.

**`resolveSkillName` as a private helper**: Separates config-reading from file-reading, making both fallback tiers independently testable.

**Sentinel errors for `ErrNoFrontmatter` and `ErrMissingOutcome`**: These are expected failure modes with no diagnostic payload. `*ErrInvalidOutcome` is a struct type because callers may need the bad value for error messages.

**CRLF normalisation before line scanning**: Agents running on Windows produce CRLF. Normalising once at the top of `ParseSessionResult` means all downstream logic is LF-only.

---

## Edge Cases & Gotchas

**`ACTIVE_TASK.md` path is canonical**: Always `{LogsDir}/ACTIVE_TASK.md`. Never in a subdirectory, never archived. The previous Bash orchestrator had path mismatch bugs (CI-1, CI-2) from inconsistent path construction — the Go port uses a single write path.

**Documentation tasks in `WriteActiveTask`**: `TaskType` is preserved as `types.TaskTypeDocumentation` (`"documentation"`) in the written file. No special-casing is needed; only bugfix gets the extra Bug Context section.

**`ACTIVE_BUG.md` missing for bugfix**: This is a warning, not a fatal error. The orchestrator may have failed to write the bug file in a prior iteration. The task brief is still written without the bug context.

**`ParseSessionResult` does not validate `changelog_entry`**: The session parser only validates `outcome`. Empty `changelog_entry` is legal — the changelog handler writes a no-op entry when it's empty.

---

## Related Topics

- [internal/types](types.md) — `SessionResult`, `TaskType`, `Outcome` constants
- [internal/log](log.md) — `log.Warning` used in graceful-degradation paths
- [Exec Command Pattern](../patterns/pattern-exec-command.md) — no `sh -c`, streaming output
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — when to use (state files) vs. when not to (session files)
- [Go Infrastructure](../infrastructure/go.md) — project structure and approved dependencies
