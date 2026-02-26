# Research Report: Full Architecture Review — doug v0.4.0

**Generated**: 2026-02-26
**Scope Type**: Full Codebase
**Related Epic**: Post-EPIC-6 (all 6 epics complete; v0.4.0 production-ready)
**Related Tasks**: All 24 feature tasks DONE; KB_UPDATE cycles complete

---

## Overview

`doug` is a compiled, cross-platform Go binary (v0.4.0) that orchestrates AI coding agents across structured task lists. It replaces a Bash predecessor (v0.3.0) with a typed, testable, and distributable CLI. The binary provides two subcommands: `doug run` (the main orchestration loop) and `doug init` (zero-friction project scaffolding). The system is agent-agnostic — it works with any CLI tool that can read a file and write a file.

---

## File Manifest

| File / Directory | Purpose |
|------------------|---------|
| `main.go` | 7-line entry point — calls `cmd.Execute()` |
| `cmd/root.go` | Cobra root command, version constant, subcommand registration |
| `cmd/run.go` | `doug run` subcommand — full orchestration loop (pre-loop + main loop) |
| `cmd/init.go` | `doug init` subcommand — project scaffolding, template copying |
| `internal/types/types.go` | All shared structs and typed constants (single source of truth) |
| `internal/state/state.go` | Load/save for `project-state.yaml` and `tasks.yaml`; atomic writes |
| `internal/config/config.go` | `OrchestratorConfig`, `LoadConfig` (partial-file pattern), `DetectBuildSystem` |
| `internal/log/log.go` | Colored terminal output (`Info`, `Success`, `Warning`, `Error`, `Fatal`, `Section`) |
| `internal/build/build.go` | `BuildSystem` interface + `NewBuildSystem` factory |
| `internal/build/npm.go` | `NpmBuildSystem` implementation |
| `internal/git/git.go` | `EnsureEpicBranch`, `RollbackChanges`, `Commit`, `ErrNothingToCommit` |
| `internal/orchestrator/bootstrap.go` | `BootstrapFromTasks`, `NeedsKBSynthesis`, `IsEpicAlreadyComplete` |
| `internal/orchestrator/taskpointers.go` | `InitializeTaskPointers`, `AdvanceToNextTask`, `FindNextActiveTask`, `IncrementAttempts`, `UpdateTaskStatus` |
| `internal/orchestrator/validation.go` | `ValidateYAMLStructure`, `ValidateStateSync`; tiered recovery |
| `internal/orchestrator/startup.go` | `CheckDependencies`, `EnsureProjectReady` |
| `internal/orchestrator/context.go` | `LoopContext` struct — per-iteration state container |
| `internal/agent/session.go` | `CreateSessionFile` — pre-creates the file the agent will write |
| `internal/agent/activetask.go` | `WriteActiveTask`, `GetSkillForTaskType` — dispatches skill instructions |
| `internal/agent/invoke.go` | `RunAgent` — streams agent output live, returns duration |
| `internal/agent/parse.go` | `ParseSessionResult` — extracts and validates YAML frontmatter |
| `internal/handlers/success.go` | `HandleSuccess`, `SuccessResult` — install, build, test, commit, advance |
| `internal/handlers/failure.go` | `HandleFailure` — rollback, retry, block, archive |
| `internal/handlers/bug.go` | `HandleBug` — interrupt, schedule bugfix, preserve next task |
| `internal/handlers/epic.go` | `HandleEpicComplete` — print summary, final commit |
| `internal/metrics/metrics.go` | `RecordTaskMetrics`, `UpdateMetricTotals`, `PrintEpicSummary` |
| `internal/changelog/changelog.go` | `UpdateChangelog` — idempotent CHANGELOG.md insert |
| `internal/templates/templates.go` | `//go:embed` for `runtime/` and `init/` template trees |
| `internal/templates/runtime/session_result.md` | 3-field frontmatter template (used by `CreateSessionFile`) |
| `internal/templates/init/` | Files stamped into new projects by `doug init` |
| `internal/templates/skills/` | Skill SKILL.md files (implement-feature, implement-bugfix, implement-documentation) |
| `integration/doc.go` | Package declaration only — smoke test removed in EPIC-6-003 QA pass |
| `go.mod` | Module: `github.com/robertgumeny/doug`; Go 1.26; cobra + yaml.v3 |
| `.goreleaser.yaml` | Cross-platform release: Linux amd64/arm64, macOS amd64/arm64, Windows amd64 |
| `.github/workflows/ci.yml` | `go test ./...` + `go vet ./...` on ubuntu-latest and macos-latest |
| `Makefile` | `build`, `test`, `lint`, `release-dry` targets |
| `docs/kb/` | Human+agent knowledge base: infrastructure, package docs, patterns |
| `PRD.md` | Product requirements, architecture decisions, epic map |

---

## Architecture Overview

### The Three-Layer Model

```
┌────────────────────────────────────────────┐
│              doug binary                   │
│  ┌──────────────────────────────────────┐  │
│  │  Pre-loop: config, validate, branch  │  │
│  └──────────────────────────────────────┘  │
│  ┌──────────────────────────────────────┐  │
│  │  Main loop (up to MaxIterations)     │  │
│  │  ┌────────────────────────────────┐  │  │
│  │  │  CreateSessionFile             │  │  │
│  │  │  WriteActiveTask (briefing)    │  │  │
│  │  │       ↓                        │  │  │
│  │  │  RunAgent ──────────────────►  │  │  │
│  │  │       ↓            (agent     │  │  │
│  │  │  ParseSessionResult  writes    │  │  │
│  │  │       ↓             file)     │  │  │
│  │  │  Dispatch handler              │  │  │
│  │  └────────────────────────────────┘  │  │
│  └──────────────────────────────────────┘  │
└────────────────────────────────────────────┘
         ↕ reads/writes
┌────────────────────────────────────────────┐
│  State Files                               │
│  project-state.yaml   tasks.yaml           │
│  doug.yaml (read-only at runtime)          │
└────────────────────────────────────────────┘
```

### Agent Contract (What the Agent Must Do)

The orchestrator requires exactly three things from an agent:

1. A command to invoke (`agent_command` in `doug.yaml`, defaults to `claude`)
2. Read `logs/ACTIVE_TASK.md` before starting work (briefing + skill instructions + session file path)
3. Write a session result file at the path given in `ACTIVE_TASK.md` (3-field YAML frontmatter: `outcome`, `changelog_entry`, `dependencies_added`)

The agent is not sandboxed at the OS level. The boundary is enforced by instruction (`CLAUDE.md`, skill files) and optionally by Claude Code's `settings.json` deny rules.

### State Machine

```
Task outcomes flow through a state machine per iteration:

SUCCESS      → install deps → build → test → commit → advance task (→ EpicComplete if done)
FAILURE      → rollback → retry (or block after MaxRetries)
BUG          → rollback → schedule bugfix task → preserve interrupted task as next
EPIC_COMPLETE → print summary → final commit → exit 0
parse error  → treated as FAILURE
```

---

## Data Flow — Per Iteration

```
cmd/run.go (main loop)
    │
    ├─ orchestrator.IncrementAttempts(state)     [mutates in memory]
    ├─ state.SaveProjectState(statePath, state)  [atomic write: .tmp → rename]
    │
    ├─ agent.CreateSessionFile(logsDir, epicID, taskID, attempt)
    │   └── writes runtime/session_result.md template → logs/sessions/{epic}/session-{id}_attempt-{n}.md
    │
    ├─ agent.WriteActiveTask(config)
    │   ├── GetSkillForTaskType → reads .claude/skills/{skillName}/SKILL.md (or hardcoded fallback)
    │   └── writes logs/ACTIVE_TASK.md (task ID + skill instructions + bug context if bugfix)
    │
    ├─ agent.RunAgent(agentCommand, projectRoot)
    │   └── exec.Command(parts[0], parts[1:]...) with cmd.Dir=projectRoot, stdout/stderr streaming
    │       [AGENT RUNS — reads ACTIVE_TASK.md, writes session file]
    │
    ├─ agent.ParseSessionResult(sessionPath)
    │   └── reads session file, extracts YAML frontmatter → types.SessionResult{outcome, changelog_entry, dependencies_added}
    │
    └─ switch result.Outcome:
        ├─ SUCCESS   → handlers.HandleSuccess(ctx)
        │               ├─ build.Install() if deps added
        │               ├─ build.Build() + build.Test()
        │               ├─ metrics.RecordTaskMetrics(...)
        │               ├─ changelog.UpdateChangelog(...) if entry present
        │               ├─ orchestrator.UpdateTaskStatus(tasks, taskID, DONE)
        │               ├─ orchestrator.AdvanceToNextTask(state, tasks) [or inject KB_UPDATE]
        │               ├─ state.SaveProjectState + state.SaveTasks
        │               └─ git.Commit("feat: " + taskID, projectRoot)
        │
        ├─ FAILURE   → handlers.HandleFailure(ctx)
        │               ├─ git.RollbackChanges(projectRoot, protectedPaths)
        │               ├─ metrics.RecordTaskMetrics(...)
        │               └─ [retry if < MaxRetries | block task + manual_review if >= MaxRetries]
        │
        ├─ BUG       → handlers.HandleBug(ctx)
        │               ├─ nested bug check (fatal if TaskType == bugfix)
        │               ├─ git.RollbackChanges(...)
        │               ├─ metrics.RecordTaskMetrics(...)
        │               ├─ active_task → {type: bugfix, id: BUG-{taskID}}
        │               └─ next_task → {type: resolveInterruptedType(), id: taskID}
        │
        └─ EPIC_COMPLETE → handlers.HandleEpicComplete(ctx)
                            ├─ metrics.PrintEpicSummary(state)
                            └─ git.Commit("chore: finalize " + epicID)
```

---

## Package Dependency Graph

```
main.go
  └── cmd/
       ├── root.go     (cobra setup)
       ├── run.go      → internal/agent, build, config, git, handlers, log, orchestrator, state, types
       └── init.go     → internal/config, log, templates

internal/ (no cross-package imports except types)
  types/       ← imported by everyone; imports nothing internal
  state/       ← imports types
  config/      ← imports nothing internal
  log/         ← imports nothing internal
  build/       ← imports nothing internal
  git/         ← imports nothing internal
  metrics/     ← imports types
  changelog/   ← imports nothing internal
  templates/   ← imports nothing internal (//go:embed only)
  orchestrator/← imports types, state, config, build, log
  agent/       ← imports types, log, templates
  handlers/    ← imports orchestrator, types, state, git, build, metrics, changelog, log
```

**Key constraint:** `internal/types` is a pure data package — it imports nothing from the project. Every other package depends on it, directly or transitively. This prevents import cycles.

---

## Dependencies

### External
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework — subcommands, flags, help text |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML marshal/unmarshal for state and config files |
| `github.com/spf13/pflag` | v1.0.9 | POSIX-style flags (indirect dep of cobra) |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | Windows console detection (indirect dep of cobra) |

### Internal (stdlib only, no external deps beyond the two above)
All git operations, subprocess invocation, file I/O, metrics, logging, template embedding, and build system detection use Go stdlib exclusively.

### Deliberate Exclusions
| Package | Why excluded |
|---------|-------------|
| `go-git` | All git operations use `exec.Command("git", ...)` — avoids the large dependency, maintains cross-platform parity |
| `logrus` / `zap` | Custom `internal/log` using ANSI codes — matches Bash orchestrator visual style, zero overhead |
| `goccy/go-yaml` / `sigs.k8s.io/yaml` | Only `gopkg.in/yaml.v3` — one YAML library, pinned |
| `text/template` | Templates are written as-is (`CreateSessionFile` does no substitution) |

---

## Patterns Observed

### 1. Typed Constants Over Bare Strings
```go
// Bad (never used in doug)
if task.Status == "DONE" { ... }

// Good (everywhere in doug)
if task.Status == types.StatusDone { ... }
```
All status values, outcome values, and task types are `type Status string`, `type Outcome string`, `type TaskType string` constants. The compiler catches invalid values at build time.

**C# analogy:** This is exactly `enum` semantics. Go uses typed string aliases instead of integer enums because they round-trip cleanly through YAML without custom serializers.

### 2. Atomic File Writes (Write-to-Temp-then-Rename)
```go
tmp := path + ".tmp"
os.WriteFile(tmp, data, 0o644)
os.Rename(tmp, path)   // atomic: either old or new, never partial
```
Used for all state files (`project-state.yaml`, `tasks.yaml`). `os.Rename` is atomic on the same filesystem on all three target platforms. The `.tmp` file is placed in the same directory to guarantee same-filesystem atomicity.

**C# analogy:** Like `File.Replace()` or a `FileStream` + `File.Move()` pattern — same idea, but Go's stdlib makes it two lines.

### 3. Load-Mutate-Save Discipline
State is loaded once, mutated entirely in memory, then saved once per iteration. No sequential partial saves. No dirty-tracking needed.

**C# analogy:** Same as the Unit of Work pattern — collect all changes, commit once.

### 4. `exec.Command` with Explicit Args Slice (No Shell)
```go
// Safe — explicit args, no shell injection
cmd := exec.Command(parts[0], parts[1:]...)
cmd.Dir = projectRoot  // scope to subprocess, not process-global

// Never done in doug
cmd := exec.Command("sh", "-c", "git commit -m " + message)
```
Every external process (git, build tools, agent) is invoked with an explicit args slice. `cmd.Dir` scopes the working directory to the subprocess without calling `os.Chdir` (which would affect the entire process).

**C# analogy:** Like `Process.Start(new ProcessStartInfo { FileName = "git", Arguments = "...", WorkingDirectory = root })` — but Go's `cmd.Dir` is cleaner and `exec.Command` accepts variadic args directly.

### 5. Interface + Factory for Build Systems
```go
type BuildSystem interface {
    Install() error
    Build() error
    Test() error
    IsInitialized() bool
}

bs, err := build.NewBuildSystem(cfg.BuildSystem, projectRoot)
```
The `BuildSystem` interface decouples the orchestrator from any specific toolchain. `GoBuildSystem` and `NpmBuildSystem` implement it. The factory returns an error for unknown types (`"python"` → error). New build systems require only a new implementation + a case in `NewBuildSystem`.

**C# analogy:** Classic `IService` + factory pattern. `NewBuildSystem` is the abstract factory method.

### 6. `//go:embed` for Zero-Dependency Template Distribution
```go
//go:embed runtime/session_result.md
var SessionResult string

//go:embed init
var Init embed.FS
```
All template files are compiled into the binary at build time. No runtime file paths, no missing-file errors, no installation directories. `doug init` copies templates from `Init` (an in-binary filesystem) to the user's project directory.

**C# analogy:** Like embedded resources (`Assembly.GetManifestResourceStream`), but Go's `//go:embed` is declarative, zero-boilerplate, and supports directory trees.

### 7. Tiered Failure Recovery
Three tiers for error handling:
- **Tier 1**: Silent self-correction (unambiguous, zero risk of death spiral)
- **Tier 2**: Self-correct + `log.Warning` (orchestrator can fix it, user should know)
- **Tier 3**: `return fmt.Errorf(...)` → cobra exits with code 1 (ambiguous, involves git state, or would loop)

Before self-correcting, ask: "Could this same condition re-trigger on the next iteration?" If yes → Tier 3 always.

**C# analogy:** Like structured exception handling with explicit escalation policies — but simpler because Go's error-as-value model makes the tier explicit in the return signature.

### 8. `UserDefined` vs `Synthetic` Tasks
```go
type Task struct {
    ...
    UserDefined bool `yaml:"-"`  // set by LoadTasks; never persisted
}

func (t TaskType) IsSynthetic() bool {
    return t == TaskTypeBugfix || t == TaskTypeDocumentation
}
```
Tasks from `tasks.yaml` are `UserDefined = true`. Bugfix and documentation tasks are orchestrator-injected synthetics that exist only in `project-state.yaml.active_task`. The `yaml:"-"` tag ensures `UserDefined` never reaches the YAML file. This distinction is enforced throughout: task status writes, `ValidateStateSync`, and `HandleFailure` all skip synthetics.

### 9. Partial Config with Pointer Fields
```go
type partialConfig struct {
    AgentCommand  *string `yaml:"agent_command"`
    KBEnabled     *bool   `yaml:"kb_enabled"`
    ...
}
```
A pointer-based intermediate struct distinguishes "field absent from file" from "field present as zero/false value." Without this, `kb_enabled: false` in `doug.yaml` would be indistinguishable from a missing field, and the default `true` would win.

**C# analogy:** Like using `Nullable<bool>` for config values — `null` means "not set," `false` means "explicitly set to false."

### 10. Table-Driven Tests
Every package uses `[]struct{ name, input, want, wantErr }` table-driven tests. This is idiomatic Go and makes adding new cases trivial.

**C# analogy:** Like `[TestCase(...)]` in NUnit or `[InlineData(...)]` in xUnit.

### 11. `OsExit` Injection for Fatal Testing
```go
// internal/log/log.go
var OsExit = os.Exit  // replaceable in tests

func Fatal(format string, args ...any) {
    Error(format, args...)
    OsExit(1)
}
```
Tests replace `log.OsExit` with a no-op function, allowing `log.Fatal` to be tested without subprocess overhead.

**C# analogy:** Dependency injection of `Environment.Exit` — same concept, Go just uses a package-level function variable.

### 12. `LoopContext` as a Per-Iteration Parameter Object
```go
type LoopContext struct {
    TaskID, TaskType, Attempts  // snapshotted identity
    SessionResult               // agent output
    Config, BuildSystem         // infrastructure
    State, Tasks                // mutable shared state
    StatePath, TasksPath, ...   // file paths
}
```
Rather than passing 12+ parameters to each handler, `cmd/run.go` constructs one `LoopContext` per iteration and passes it to all handlers. Handlers mutate `ctx.State` and `ctx.Tasks` in memory; those mutations persist to disk at the end of the handler chain.

**C# analogy:** A DTO / Parameter Object pattern — avoiding "telescoping constructors" in the handler methods.

---

## Anti-Patterns & Tech Debt

### 1. README is Stale (Scaffold-Era Content)
`README.md` describes the project as "stubs today" and explains cobra basics for a Go newcomer. The project is now fully implemented (v0.4.0). The README needs a complete rewrite covering: what doug is, installation, quick start, architecture overview, agent trust boundary, config reference, and development workflow.

### 2. Integration Test Package is Empty
`integration/smoke_test.go` was originally planned (EPIC-6-003) but `integration/doc.go` is a package declaration only. The smoke test was removed during the EPIC-6-003/004 QA pass. A real end-to-end integration test with a mock agent is still a gap.

### 3. Version is Hardcoded as `v0.0.1`
`cmd/root.go:10` has `const version = "v0.0.1"`. This should be injected by GoReleaser via `-ldflags "-X main.version=..."`. The `.goreleaser.yaml` does not currently inject this value.

### 4. `settings.json` Not Generated by `doug init`
`doug init` creates `CLAUDE.md`, `AGENTS.md`, skill files, and template files, but does not create `.claude/settings.json`. This means users deploying doug to their own projects get no mechanical Claude Code deny-rule layer — only instructional constraints. (Documented in detail in the archived trust-boundary research report.)

### 5. `skills-config.yaml` Not Generated by `doug init`
The skill resolution in `GetSkillForTaskType` has a two-tier fallback: read from `.claude/skills-config.yaml` first, then use hardcoded names. `doug init` does not create `skills-config.yaml`, so users who want custom skill mappings must create this file manually.

### 6. No `project-state.yaml` Template Generated by `doug init`
`doug run` requires `project-state.yaml` to exist before it can bootstrap. The orchestrator has a `BootstrapFromTasks` function, but `project-state.yaml` must exist (even as an empty file or minimal stub) for `LoadProjectState` to succeed. `doug init` does not create `project-state.yaml`. This is likely intentional (it's auto-created on first run via bootstrap), but it's a potential UX confusion point.

### 7. Leftover `.tmp` Files Not Cleaned on Startup
Both KB articles and the code note that if the process dies after writing `project-state.yaml.tmp` but before the `os.Rename`, a stale `.tmp` file remains. The orchestrator startup does not detect or clean these. Low risk in practice (single-process tool), but noted as a known gap.

### 8. `changelog.UpdateChangelog` Is Not Atomic
`UpdateChangelog` uses `os.WriteFile` directly (not the write-to-temp pattern). A process kill mid-write could corrupt `CHANGELOG.md`. Acceptable for a human-readable file that can be reconstructed from git history, but worth noting as an exception to the project's consistency rule.

### 9. CI Does Not Run on Windows
The CI matrix covers `ubuntu-latest` and `macos-latest` only. Windows is a target platform (GoReleaser builds `windows/amd64`), but no CI run validates the Windows binary. CRLF handling in `ParseSessionResult` is manually implemented but not CI-tested on Windows.

---

## State Management

### Files and Ownership

| File | Owner | Write Pattern |
|------|-------|---------------|
| `project-state.yaml` | Orchestrator | Full ownership; load-mutate-save per iteration; atomic write |
| `tasks.yaml` | User (status fields only) | Orchestrator writes `status` field only; atomic write |
| `doug.yaml` | User | Read-only at runtime; never written by orchestrator |
| `logs/ACTIVE_TASK.md` | Orchestrator | Overwritten before each agent invocation |
| `logs/sessions/{epic}/session-{id}_attempt-{n}.md` | Agent | Pre-created by orchestrator; filled in by agent |
| `CHANGELOG.md` | Orchestrator | Idempotent insert via `UpdateChangelog` |

### In-Memory State Flow

```
LoadProjectState + LoadTasks
        │
        ▼
BootstrapFromTasks     ← first run only: populate epic, active_task, next_task
        │
        ▼
InitializeTaskPointers ← every run: align pointers with actual task statuses
        │
        ▼
[per iteration]:
  IncrementAttempts → SaveProjectState (persist before agent)
  [agent runs]
  [handler runs: mutates state/tasks in memory]
  SaveProjectState + SaveTasks (persist after handler)
```

**Key invariant:** Attempt counter is incremented and persisted *before* the agent runs. If the orchestrator crashes mid-agent, the next startup correctly sees the incremented attempt count.

---

## Infrastructure & Distribution

### Build
```bash
make build      # go build -o doug .
make test       # go test ./...
make lint       # go vet ./...
make release-dry # goreleaser release --snapshot --clean
```

### CI (GitHub Actions)
- Runs on every push and PR (all branches)
- Matrix: `ubuntu-latest`, `macos-latest`
- Steps: `go test ./...`, `go vet ./...`
- Go version pinned to `1.26`

### Release (GoReleaser)
- Triggered on `v*` tags
- Targets: Linux amd64/arm64, macOS amd64/arm64, Windows amd64 (no Windows arm64)
- `CGO_ENABLED=0` — fully static binaries, no C runtime dependency
- `-s -w` ldflags — strips debug symbols for smaller binaries
- Archives: `.tar.gz` for Linux/macOS, `.zip` for Windows
- `checksums.txt` generated for all artifacts

### Go Version
- Go 1.26 (released February 10, 2026)
- Notable for doug: Green Tea GC (default), faster small allocations (<512B), stack-allocated slice backing stores
- Upgrade policy: two most recent stable releases

---

## PRD Alignment

The PRD targeted a "functionally equivalent Go port" of the v0.3.0 Bash orchestrator. All 24 feature tasks across 6 epics are DONE. The binary is fully functional.

### Confirmed Complete
- [x] All 24 feature tasks DONE
- [x] `go test ./...` passes across all `internal/` packages
- [x] `go build ./...` produces a clean binary
- [x] `go vet ./...` reports no issues
- [x] `doug init` produces a complete project scaffold (5 files + skills directory)
- [x] No `sed`, `awk`, `yq`, or `eval` in the codebase
- [x] Atomic YAML writes via `os.Rename`
- [x] `exec.Command` with explicit args (no shell injection)
- [x] Bash CI bugs fixed: CI-1 (ACTIVE_BUG.md path), CI-2 (ACTIVE_FAILURE.md path), CI-5 (synthetic task deadlock in bug handler), CI-6 (epic commit failures silently swallowed)
- [x] Cross-platform: Linux, macOS, Windows (GoReleaser)

### PRD Open Items (Not Started)
- [ ] Agent trust boundary documented in README (PRD explicit requirement)
- [ ] `github.com/[org]/doug` module path → replaced with `github.com/robertgumeny/doug`
- [ ] `LoopContext` ambiguity note (resolved naturally during EPIC-5)

### PRD Non-Goals (Correctly Deferred)
- No TUI (bubbletea/lipgloss)
- No Python build system support
- No agent sandboxing (instructional boundary only)
- No token tracking

---

## Go Architecture Decisions — Rationale for C# Developers

This section explains each major Go-specific decision in terms familiar to a C# background.

### Why Go instead of C#, Python, or Bash?

| Requirement | Go's answer |
|-------------|------------|
| Cross-platform single binary | `go build` produces a static binary with no runtime dependency. No .NET runtime install, no Python venv, no `node_modules`. |
| No `sed`/`awk`/`yq` | Go stdlib has `strings`, `os`, `yaml.v3`. Pure Go parsing, no shell utilities. |
| Testability | Go has a built-in test runner (`go test`), table-driven tests, and subtests. Bash is not unit-testable. |
| Distributable | GoReleaser + GitHub Actions = signed multi-platform binaries with one `git tag`. No package managers. |
| Type safety | All state mutations go through typed structs. Invalid YAML shapes fail at unmarshal, not at runtime. |

### Why Cobra for CLI?

Cobra (`github.com/spf13/cobra`) is the de facto standard Go CLI framework — used by `kubectl`, `hugo`, `docker`, and hundreds of others. It provides:
- Subcommand registration and dispatch
- Flag binding directly to struct fields
- Auto-generated help text and usage
- `RunE` (returns error) vs `Run` (doesn't) — the project uses `RunE` everywhere for consistent error propagation

**C# analogy:** Like `System.CommandLine` or `Cocona` — declarative subcommand trees with flag binding.

### Why `internal/` for everything?

Go enforces that code in `internal/` cannot be imported by packages outside the module root. This is a compile-time guarantee, not a naming convention. For a CLI tool, `internal/` means the API surface is zero — callers can only use the binary, not import it as a library.

**C# analogy:** `internal` access modifier at the assembly level — but Go enforces it by directory path, not by keyword on each type.

### Why typed string constants instead of `iota` enums?

```go
type Status string
const StatusDone Status = "DONE"
// vs
type Status int
const StatusDone Status = iota + 3
```

Typed string constants:
- Round-trip cleanly through YAML without a custom `MarshalYAML`/`UnmarshalYAML`
- Are human-readable in `project-state.yaml` and `tasks.yaml`
- Match the Bash orchestrator schema (which used string literals)

**C# analogy:** This is why you sometimes use `[EnumMember(Value = "DONE")]` with `System.Text.Json` — the serialized value must match an external schema. Go's typed string avoids the need for any serialization attribute.

### Why pointer (`*string`) for `CompletedAt`?

```go
type EpicState struct {
    CompletedAt *string `yaml:"completed_at"`
}
```

YAML's `null` value deserializes to a Go pointer as `nil`. A `string` (non-pointer) would deserialize `null` as `""` (empty string), which would be indistinguishable from "CompletedAt was explicitly set to empty." The pointer makes the nil state explicit.

**C# analogy:** `string?` (nullable reference type) or `Nullable<DateTime>` — `null` and empty string are semantically different.

### Why `errors.Is` / `errors.As` instead of typed exception hierarchies?

```go
var ErrNotFound = errors.New("state file not found")
type ParseError struct { Path string; Err error }

if errors.Is(err, state.ErrNotFound) { ... }     // sentinel comparison
var pe *state.ParseError
if errors.As(err, &pe) { ... }                    // type extraction
```

Go errors are values, not exceptions. `errors.Is` traverses the `Unwrap()` chain to find a specific sentinel. `errors.As` extracts a typed error from the chain (like `catch (ParseException e)` but without stack unwinding).

**C# analogy:**
- `errors.Is` ≈ `catch (SpecificException)` or `ex is SpecificException`
- `errors.As` ≈ `(ParseException)ex` with a null check — but safe because it does the type check for you
- The `Unwrap()` chain ≈ `InnerException` — but Go makes wrapping explicit with `fmt.Errorf("...: %w", err)`

### Why no inheritance / why interfaces are implicit?

Go has no class inheritance. `BuildSystem` is satisfied implicitly:
```go
// GoBuildSystem satisfies BuildSystem if it has all four methods.
// No "implements BuildSystem" declaration needed.
```

This is structural typing (duck typing with compile-time checking). **C# 8+ has a similar concept with default interface implementations**, but Go's approach is simpler: if the method set matches, the interface is satisfied.

**Practical consequence:** You can satisfy a third-party interface without modifying the third-party package. In C# you'd need an adapter class.

### Why `cmd/` just wires, `internal/` just works?

The rule "if a function in `cmd/` is doing more than calling into `internal/`, it belongs in a package" enforces testability. `cmd/run.go:runOrchestrate` has the full orchestration sequence, but every meaningful step delegates to an `internal/` function. Tests for `BootstrapFromTasks` don't need a real CLI invocation.

**C# analogy:** Thin controllers calling service layer — but enforced structurally, not by convention.

### Why streaming stdout/stderr for the agent?

```go
cmd.Stdout = os.Stdout   // stream live
cmd.Stderr = os.Stderr   // stream live
// Never: cmd.CombinedOutput() for the agent
```

`CombinedOutput()` buffers all output until the process exits. For a coding agent that runs for minutes and produces live output, buffering means the user sees nothing until it's done. Streaming gives a live view.

**C# analogy:** `Process.StandardOutput.ReadToEnd()` (buffering) vs subscribing to `Process.OutputDataReceived` event (streaming). Go's approach is simpler: assign `cmd.Stdout = os.Stdout` and the OS handles the pipe.

---

## Future Roadmap Inputs

Based on the PRD open items and tech debt identified above, these are the highest-priority items for a v0.5.0 PRD:

### Must-Have Before v1.0
1. **README rewrite** — the current README is the EPIC-1-001 scaffold stub. The trust boundary section is a PRD explicit requirement.
2. **Version injection** — wire GoReleaser's `ldflags` to inject the version string into `cmd/root.go`. Current hardcode `v0.0.1` is incorrect.
3. **`settings.json` generated by `doug init`** — the highest-impact security improvement with zero runtime changes needed.

### High Value, Low Effort
4. **Windows CI** — add `windows-latest` to the CI matrix. The binary targets Windows but no CI validates it.
5. **`skills-config.yaml` generated by `doug init`** — completes the init scaffold; currently users can't customize skill mappings without manual file creation.
6. **Leftover `.tmp` cleanup on startup** — detect and remove stale `.tmp` state files before the loop begins.

### Medium Effort, High Value
7. **Integration smoke test** — the EPIC-6-003 task planned a mock-agent end-to-end test. `integration/smoke_test.go` is currently empty. This is the highest-confidence correctness check.
8. **TUI with bubbletea/lipgloss** — deferred by PRD, but the architecture is clean enough that a TUI wrapper around `cmd/run.go` is achievable without architectural changes.
9. **`settings.json` hooks** — supplement deny rules with the more precise hooks API (see trust boundary research report).

### Long-Term
10. **Container mode (`--sandbox=container`)** — opt-in Docker/Podman sandbox for CI/CD and shared-team environments.
11. **Landlock filesystem isolation** — Linux-only, no external deps, graceful degradation; see trust boundary research report.
12. **Multi-epic workflow** — currently one epic per `tasks.yaml`. A `projects.yaml` that chains epics would enable longer-horizon automation.
13. **Agent performance metrics** — the `duration_seconds` field is tracked per task but not surfaced in any report beyond the epic summary. A `doug stats` subcommand could provide historical analysis.

---

## Raw Notes

### On the README Gap

The README contains a detailed Go tutorial for new Go developers (`func init()`, cobra basics, `internal/` semantics). This was the EPIC-1-001 scaffold — appropriate for someone learning Go, not for a user installing `doug` from a release binary. A real README needs: what it is, install instructions, 5-minute quickstart, architecture in 3 sentences, config reference, and a trust boundary section.

### On the Smoke Test Removal

EPIC-6-003 was supposed to implement a real end-to-end smoke test in `integration/smoke_test.go`. The final QA pass (EPIC-6-004) removed it (the file is now `integration/doc.go` — just a package declaration). The reason is likely that the test required a real git repo, a mock agent binary, and complex setup that wasn't ready for the time constraint. This is the biggest testing gap.

### On the `IsSynthetic()` Method

`TaskType.IsSynthetic()` is a method on a typed string constant. This is idiomatic Go — putting behavior directly on the type rather than having a standalone function. In C#, you'd use an extension method or a switch expression. The Go approach is cleaner for a closed set of values.

### On the Bash Comparison

The Go port faithfully translates every behavior of the Bash orchestrator. The PRD lists exact behavioral changes (all improvements, not new features): unified `ACTIVE_TASK.md` instead of three separate active files, atomic writes, no `eval`, reduced `SessionResult` fields. Reading the PRD's "What changes from the Bash orchestrator" table is the fastest way to understand the design intent.

### On Module Path

The module is `github.com/robertgumeny/doug`. The PRD had `github.com/[org]/doug` as a placeholder — the org was resolved to `robertgumeny` during EPIC-1-001 setup.

### On `go.sum` as the Initialized Check

`GoBuildSystem.IsInitialized()` checks for `go.sum`, not `go.mod`. This is correct and intentional: a project with `go.mod` but no `go.sum` hasn't had `go mod tidy` run and cannot `go build`. The check prevents a confusing "package not found" error from `go build` on fresh checkouts.
