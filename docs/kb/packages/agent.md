---
title: internal/agent — Backend, ActiveTask, Invoke, Parse, Archive
updated: 2026-04-30
category: Packages
tags: [agent, backend, active-task, invoke, parse, exec, frontmatter, yaml, archive, seam, execution-prep, policy]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/log.md
  - docs/kb/packages/templates.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
---

# internal/agent — Backend, ActiveTask, Invoke, Parse, Archive

## Overview

`internal/agent` is the boundary between the orchestrator and the agent process. It owns the full agent lifecycle for one iteration:

1. **Dispatch** through `Backend.Run` — the single execution seam for all call sites → `backend.go`
2. **Write** `ACTIVE_TASK.md` (task briefing + result block stub) → `activetask.go`
3. **Invoke** the agent command via `RunAgent`, stream output live → `invoke.go`
4. **Archive** `ACTIVE_TASK.md` to session log before any state change in runtime handlers → `archive.go`
5. **Parse** the `## Agent Result` block from `ACTIVE_TASK.md`, validate the outcome → `parse.go`
6. **Clean up** the live root `ACTIVE_TASK.md` once outcome handling is complete → `archive.go`

All agent execution routes through the `Backend` interface — no call site invokes `RunAgent` directly.

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

Content written:
1. Briefing header: Active Bug File path, Failure File path, and PRD File path
2. Task ID, type, attempt number, description, and acceptance criteria
3. Conditional `## Build System` section — when `BuildSystem` is a known key in `config.BuildSystems`
4. For bugfix tasks only: `## Bug Context` section from `{DougDir}/ACTIVE_BUG.md`, which is the live blocking-bug handoff file
5. When `TestFailureOutput` is non-empty: `## Previous Test Failure Output` section with the raw test output, instructing the agent to fix the failures
6. Any `ContextSections` blocks appended as `## <Heading>` sections before the result stub
7. `## Agent Result` stub at the bottom — an empty YAML frontmatter block that the agent fills in with `outcome`, `changelog_entry`, and `dependencies_added`, followed by the implementation summary headings the agent writes into

`ACTIVE_BUG.md` is reserved for blocking runtime interruptions only. Durable bug history belongs under `.doug/logs/bugs/{epic}/`, not in the live bugfix briefing file. If `ACTIVE_BUG.md` is missing for a bugfix task, the current runtime guard in `internal/orchestrator/run.go` fails before dispatch, so `WriteActiveTask` normally only sees bugfix tasks that already have guaranteed blocking context. If `BuildSystem` is empty or not in the registry, the build system section is silently omitted.

`os.MkdirAll` is called on `DougDir` before writing.

### GetSkillForTaskType

```go
func GetSkillForTaskType(taskType, configPath string) (string, error)
```

**Deprecated**: Called by `PrepareExecution` as the skills-config legacy tier. During final rollout this function will be replaced by `DefaultSkillName` as the direct fallback in `PrepareExecution`; the `skills-config.yaml` file-reading tier will be removed. See [config.md — Legacy Policy-Resolution Paths](config.md) for the full removal checklist.

Resolves the skill name for a task type using a two-tier fallback:

| Tier | Source | Used when |
|------|--------|-----------|
| 1 | `skills-config.yaml` → `skill_mappings[taskType]` | Config present and type listed |
| 2 | `hardcodedSkillNames` map (`DefaultSkillName`) | Config absent or type not listed |

Returns an error for unknown task types not found in either source. The resolved skill name is then passed to `policy.ResolveSkill` as the fallback; `policy.tasks[taskType].skill` in `doug.yaml` always wins.

---

## execution_prep.go — PrepareExecution, ExecutionPrep, DefaultSkillName

### ExecutionPrep

```go
type ExecutionPrep struct {
    SkillName       string
    ResolvedCommand string
    Exec            config.ResolvedExecution
}
```

`ExecutionPrep` is the fully resolved execution contract for one agent invocation. All policy inputs are determined before `RunRequest` is assembled so the backend never needs to invent policy.

### PrepareExecution

```go
func PrepareExecution(phase, taskType, taskID, commandTemplate, skillsConfigPath string, policy config.PolicyConfig) (ExecutionPrep, error)
```

Produces an `ExecutionPrep` in one call:

1. Calls `GetSkillForTaskType(taskType, skillsConfigPath)` to obtain the skills-config / hardcoded fallback skill name.
2. Calls `policy.ResolveSkill(taskType, fallback)` — if `policy.tasks[taskType].skill` is set in `doug.yaml` it wins; otherwise the fallback from step 1 is used.
3. Calls `policy.ResolveExecution(phase, taskType)` to produce a `config.ResolvedExecution` with all seven policy fields resolved in one pass.
4. Substitutes `{{skill_name}}` and `{{task_id}}` in `commandTemplate` to produce `ResolvedCommand`.

All four call sites (runtime loop, `runPostEpicKB`, `cmd/plan.go`, `cmd/scaffold.go`) call `PrepareExecution` before constructing `RunRequest`. `Routing.SkillName`, `Routing.ExecutionMode`, `Policy.*`, `Restrictions.*.Paths`, and `Command` are all populated from the returned `ExecutionPrep`.

**Deprecated parameter**: `skillsConfigPath` is the legacy path to `skills-config.yaml`. During final rollout this parameter will be removed and `GetSkillForTaskType` replaced with `DefaultSkillName` as the direct fallback, so the resolution chain becomes `policy.tasks[type].skill` → hardcoded defaults only.

### DefaultSkillName

```go
func DefaultSkillName(taskType string) (string, bool)
```

Returns the built-in hardcoded skill name for a task type. Returns `("", false)` for unknown types. This is the final fallback that `PrepareExecution` will use directly once the `skills-config.yaml` legacy tier is removed.

| Task type | Default skill |
|-----------|--------------|
| `feature` | `implement-feature` |
| `bugfix` | `implement-bugfix` |
| `documentation` | `implement-documentation` |
| `manual_review` | `manual-review` |
| `scaffold` | `scaffold` |
| `plan` | `plan` |

---

## backend.go — Backend, RunRequest, RunResponse, DefaultBackend

The `Backend` interface is the single execution seam through which all agent invocations flow. Call sites never call `RunAgent` directly.

### Interface

```go
type Backend interface {
    Run(ctx context.Context, req RunRequest) (RunResponse, error)
}
```

### Request and Response Types

```go
type RunRequest struct {
    Phase            RunPhase
    Task             TaskContext
    Brief            CanonicalBrief
    ContextLoadOrder []ContextInput
    Artifacts        ArtifactSurfaces
    Routing          RoutingInputs
    Policy           PolicyInputs
    Restrictions     RestrictionHooks
    Lifecycle        LifecycleHooks

    Command           string
    ProjectRoot       string
    HeartbeatInterval time.Duration
    HeartbeatFn       func(elapsed time.Duration)
    Output            io.Writer
}

type RunResponse struct {
    Status                RunStatus
    Duration              time.Duration
    ExitCode              *int
    SessionID             string
    AvailableSessionIDs   []string
    RestrictionViolations []RestrictionViolation
}
```

The request now has two layers:

- **Doug-native contract**: `Phase`, `Task`, `Brief`, ordered `ContextLoadOrder`, explicit `Artifacts`, `Routing`, `Policy`, and `Restrictions`
- **Current subprocess transport**: `Command`, `ProjectRoot`, heartbeat knobs, and `Output`

This keeps call sites speaking in Doug terms while `DefaultBackend` remains a transparent shell-process adapter.

`RunResponse` is **runtime-only backend metadata**. It may report transport facts such as backend status, elapsed time, exit code, session identifier, or restriction violations, but it never carries Doug workflow outcomes. `SUCCESS`, `FAILURE`, `BUG`, and `EPIC_COMPLETE` remain authoritative only in `ACTIVE_TASK.md`, parsed later by `ParseSessionResult`.

### Artifact Authority And Surfaces

```go
type ArtifactAuthority string
const (
    ArtifactAuthorityProject ArtifactAuthority = "project"
    ArtifactAuthorityDoug    ArtifactAuthority = "doug"
    ArtifactAuthorityPi      ArtifactAuthority = "pi"
)

type ArtifactSurface struct {
    Path        string
    Purpose     ArtifactPurpose
    Authority   ArtifactAuthority
    AgentFacing bool
}

type ArtifactSurfaces struct {
    Read  []ArtifactSurface
    Write []ArtifactSurface
}
```

`ArtifactAuthorityDoug` marks Doug-owned runtime and planning artifacts such as `ACTIVE_TASK.md`, root `.doug/PRD.md`, archives, and lifecycle handoff files. `ArtifactAuthorityProject` marks repository-owned surfaces such as `AGENTS.md`, source files, and `docs/kb/`. `ArtifactAuthorityPi` is reserved for future Pi-owned artifacts so later adapter work can add Pi surfaces without overloading Doug or project ownership semantics.

`Artifacts.Read` is the backend-facing read-path hook list for the run. `Artifacts.Write` is the intended writable-surface list for the run. Doug-owned control and lifecycle artifacts are non-agent-facing by default unless a run contract explicitly exposes them. This gives backend preparation code one place to inspect default path authority and write boundaries before any provider-specific policy translation exists.

### Doug-native request fields

```go
type RunPhase string
const (
    RunPhaseRuntime    RunPhase = "runtime"
    RunPhasePlanning   RunPhase = "planning"
    RunPhaseScaffold   RunPhase = "scaffold"
    RunPhasePostEpicKB RunPhase = "post_epic_kb"
)

type TaskContext struct {
    ID         string
    Type       string
    Attempt    int
    MaxRetries int
    EpicID     string
    EpicName   string
}

type CanonicalBrief struct {
    Path      string
    Format    BriefFormat   // currently "markdown"
    Authority ArtifactAuthority
}

type ContextInput struct {
    Kind      ContextInputKind
    Path      string
    Required  bool
    Authority ArtifactAuthority
}

type RoutingInputs struct {
    Workflow  string
    SkillName string
}

type PolicyInputs struct {
    SessionPolicy string
}

type RestrictionHooks struct {
    Read  RestrictionHook
    Write RestrictionHook
}

type LifecycleHooks struct {
    Timeout      func(elapsed time.Duration)
    Cancellation func(elapsed time.Duration, cause error)
}

type RunStatus string
const (
    RunStatusCompleted RunStatus = "completed"
    RunStatusRejected  RunStatus = "rejected"
    RunStatusCancelled RunStatus = "cancelled"
)

type RestrictionViolation struct {
    Kind   string
    Path   string
    Detail string
}
```

`ContextLoadOrder` is the hook point for prompt-cache-friendly context sequencing. Current call sites order stable project instructions and optional PRD context before the canonical brief; planning additionally loads `PLAN.md` as a required working artifact after the canonical brief. Each entry also carries explicit artifact authority so backend prep code can distinguish project-owned context from Doug-owned context without re-deriving it from paths. `Restrictions` remains the provider-policy hook point; current production behavior still comes from repository/runtime conventions, not backend enforcement. `Lifecycle` is the interruption-observability hook point: callers may provide lightweight timeout/cancellation callbacks without changing transport control flow or workflow outcome authority.

## contract.go — Shared Workflow Contracts

### API

```go
func RuntimeContract(projectRoot, dougDir string) RunContract
func ScaffoldContract(projectRoot, dougDir, manifestPath string) RunContract
func PlanningContract(projectRoot, dougDir, planPath string) RunContract
func PostEpicKBContract(projectRoot, dougDir, epicID string) RunContract
```

These helpers centralize the Doug-native contract assembly that used to be duplicated across call sites.

- `RuntimeContract` exposes the project workspace plus live Doug handoff files (`ACTIVE_TASK.md`, `ACTIVE_BUG.md`, `ACTIVE_FAILURE.md`) as writable surfaces, while keeping broader Doug lifecycle files out of the default artifact lists.
- `ScaffoldContract` preserves the runtime writable surface but also names `.doug/plan/manifest.yaml` as a required Doug-owned working artifact in the ordered context/read contract.
- `PlanningContract` exposes the project workspace as a read surface while keeping only `.doug/ACTIVE_TASK.md` and `.doug/plan/PLAN.md` writable.
- `PostEpicKBContract` exposes only `docs/kb/` and `.doug/ACTIVE_TASK.md` as writable surfaces, while listing the archived runtime snapshot and archived session logs as Doug-owned read-only inputs.

This is the intended integration point for later Pi-backed request preparation: the contract already spells out artifact authority, context order, read-path hook points, and default writable surfaces in one shared package.

`Output == nil` is the interactive-terminal convention. `HeartbeatFn == nil` and `HeartbeatInterval == 0` suppress heartbeat ticking. Both are valid combinations.

### DefaultBackend

```go
type DefaultBackend struct{}

func (DefaultBackend) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
    d, err := RunAgent(ctx, req.Command, req.ProjectRoot, req.HeartbeatInterval, req.HeartbeatFn, req.Output)
    // Response contains transport/runtime facts only; workflow outcome still
    // comes from ParseSessionResult(ACTIVE_TASK.md).
    return RunResponse{...}, err
}
```

`DefaultBackend` is a transparent wrapper over `RunAgent`. It still ignores most Doug-native fields and uses only the subprocess transport fields plus optional lifecycle hooks, preserving existing behavior while establishing the richer contract that later backends can translate. It populates runtime-only facts:

- `Status = "completed"` for launched subprocesses, even when the subprocess exits non-zero
- `Status = "cancelled"` when `ctx` is cancelled
- `Status = "rejected"` when the request is rejected before launch, such as an empty command
- `ExitCode` when a subprocess exit code exists; `nil` when no subprocess was launched
- `SessionID = ""` and no restriction violations in the current shell-backed implementation
- `Lifecycle.Timeout` on deadline expiry and `Lifecycle.Cancellation` on any cancellation path when those callbacks are supplied

### PiAdapter

```go
type PiAdapter struct{}

func NewPiAdapter() PiAdapter
func (a PiAdapter) Run(ctx context.Context, req RunRequest) (RunResponse, error)
```

`PiAdapter` is the Doug-owned Phase 1 Pi RPC backend. It preserves the public `Backend` seam and translates `RunRequest` into a private Pi launch spec inside `internal/agent/pi_adapter.go`; command handlers and orchestrator code continue to depend only on Doug-native request and response types.

Current Phase 1 behavior:

- launches `pi --mode rpc --session-dir <dir>` with `cmd.Dir = req.ProjectRoot`
- computes the retained Pi session directory as `.doug/logs/pi-sessions/{epicID}/{taskID}/attempt-{n}`
- sends a startup `get_state` request to capture the initial Pi session ID
- sends a single `prompt` request when `req.Command` is non-empty, then waits for `agent_end`
- mirrors Pi RPC stdout JSONL lines into the Doug-managed output log when `req.Output` is non-nil
- scans RPC envelopes for `sessionId` keys and returns them as ordered, deduplicated `AvailableSessionIDs`
- reports cancellation via `ctx.Err()` when shutdown races with stream closure, so interrupted runs do not degrade into transport EOF errors

The Pi RPC request shape is intentionally private. The adapter currently maps these Doug-native inputs into the Pi payload:

- `Phase` as the session component and workflow label
- `Task`, `Brief`, and ordered `ContextLoadOrder`
- read/write `Artifacts` including authority and agent-facing flags
- `Routing`, `Policy`, and `Restrictions`
- optional `Lifecycle` hooks, fired on cancellation and deadline expiry without changing Doug outcome authority

`PiAdapter` continues the same workflow-semantics rule as `DefaultBackend`: `RunResponse` contains only runtime facts. Final outcomes still come from `ParseSessionResult(ACTIVE_TASK.md)`.

### Call Sites

All four call sites that launch agent subprocesses route through `Backend.Run`:

| Call site | File | Heartbeat | Output |
|-----------|------|-----------|--------|
| Orchestrator main loop | `internal/orchestrator/run.go` | yes | file log |
| `runPostEpicKB` | `internal/orchestrator/post_epic_kb.go` | yes | file log |
| `scaffoldProjectContext` | `cmd/scaffold.go` | yes | file log |
| `planProjectContext` | `cmd/plan.go` | no | nil (interactive); canonical brief is `ACTIVE_TASK.md`, working artifact is `PLAN.md` |

`cmd/scaffold.go` and `cmd/plan.go` expose package-level `Backend` variables (`scaffoldRunAgent`, `planRunAgent`) initialized to `DefaultBackend{}` so tests can inject stubs without modifying production code.

The `Orchestrator` struct holds a `backend agent.Backend` field set to `DefaultBackend{}` in `New`. A private `execBackend()` method returns `o.backend` with a `DefaultBackend{}` fallback for test-constructed orchestrators that do not set the field.

### Test Injection Pattern

All four call sites are testable by replacing the injected backend with a function-adaptor stub:

```go
type backendFunc func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error)
func (f backendFunc) Run(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
    return f(ctx, req)
}
```

This adapter appears (by convention) in tests for each call site. No test touches `DefaultBackend` or `RunAgent` directly.

## run_metadata.go — WriteRunMetadata

```go
func RunMetadataPath(outputLogPath string) string
func WriteRunMetadata(outputLogPath string, resp RunResponse, runErr error) error
```

`WriteRunMetadata` persists backend-visible runtime facts as `<output log>.meta.json` next to the normal Doug output log. This sidecar is observability only; it never changes workflow semantics or replaces the authoritative result in `ACTIVE_TASK.md`.

Persisted fields:

- `status`
- `duration_ms`
- `exit_code`
- `session_id`
- `available_session_ids`
- `restriction_violations`
- `error`

Current call sites write metadata for runtime task execution, scaffold runs, and post-epic KB synthesis after `Backend.Run` returns. This is the durable record for Pi-visible session correlation and other transport facts that should survive after the live process exits.

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

**`Backend` interface as the single execution seam**: All agent launches go through `Backend.Run`. This keeps workflow ownership in `internal/orchestrator` and `cmd/` while enabling test injection at every call site — with zero changes to production routing logic.

**`DefaultBackend` is a transparent wrapper**: It delegates directly to `RunAgent` with no added logic. This means the seam costs nothing at runtime and the full execution behavior remains in `invoke.go` where it belongs.

**Package-level `Backend` vars in `cmd/scaffold.go` and `cmd/plan.go`**: Command packages can't receive constructor injection, so `scaffoldRunAgent` and `planRunAgent` are exported-width package variables. Tests replace them before calling the function under test.

**`execBackend()` fallback in `Orchestrator`**: Tests that construct `Orchestrator` directly (without calling `New`) may leave `backend` nil. The `execBackend()` helper returns `DefaultBackend{}` in that case, preventing nil-pointer panics in test helpers without requiring every test to wire up a backend.

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

**Malformed agent result blocks are contract errors, not task failures**: When `ParseSessionResult` returns `ErrMissingOutcome`, `ErrNoFrontmatter`, `*ErrInvalidOutcome`, or another parse error, the main run loop now surfaces that as an explicit agent-reporting/contract error, archives the malformed `ACTIVE_TASK.md`, restores the attempt counter, and exits instead of coercing the iteration into the normal `FAILURE` retry/block flow.

**`ACTIVE_BUG.md` missing for bugfix in `WriteActiveTask`**: warning-only at the package boundary, but the orchestrator's bugfix guard treats this as fatal before the agent is launched. The live file is therefore part of the blocking-bug runtime contract, while durable bug history remains in `.doug/logs/bugs/{epic}/`.

**`ParseSessionResult` does not validate `changelog_entry`**: Only `outcome` is validated. Empty `changelog_entry` is legal.

---

## Related Topics

- [internal/types](types.md) — `SessionResult`, `TaskType`, `Outcome` constants
- [internal/templates](templates.md) — `Runtime`, `Init` exports; template file contents
- [internal/log](log.md) — `log.Warning` used in graceful-degradation paths
- [Exec Command Pattern](../patterns/pattern-exec-command.md) — no `sh -c`, streaming output
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — when to use (state files) vs. when not to (session files)
- [Go Infrastructure](../infrastructure/go.md) — project structure and approved dependencies
