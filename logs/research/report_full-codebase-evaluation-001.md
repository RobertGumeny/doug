# Research Report: Full Codebase Evaluation

**Generated**: 2026-02-25
**Scope Type**: Full Codebase
**Related Epic**: EPIC-6 (final)
**Related Tasks**: All 24 feature tasks, EPIC-1 through EPIC-6

---

## Overview

`doug` is a compiled, cross-platform Go binary that orchestrates AI coding agents (Claude Code, Aider, or any CLI tool) across structured task lists. It reads `tasks.yaml` and `project-state.yaml`, dispatches the configured agent, verifies build/tests, commits each completed task, and handles failures and bugs automatically. It is a faithful Go port of a prior Bash orchestrator, adding type safety, cross-platform support, and the `doug init` onboarding subcommand.

---

## File Manifest

| File | Purpose |
|------|---------|
| `main.go` | Entry point — calls `cmd.Execute()` |
| `cmd/root.go` | Cobra root command, version string (`v0.0.1`), subcommand registration |
| `cmd/run.go` | `doug run` — full orchestration loop (10-step pre-loop, main loop, dispatch) |
| `cmd/init.go` | `doug init` — project scaffolding, build system detection, template copying |
| `cmd/init_test.go` | Unit tests for init command (comprehensive) |
| `internal/types/types.go` | All shared structs and typed constants (Status, Outcome, TaskType) |
| `internal/config/config.go` | `OrchestratorConfig`, `LoadConfig`, `DetectBuildSystem` |
| `internal/state/state.go` | Atomic YAML I/O: `LoadProjectState`, `SaveProjectState`, `LoadTasks`, `SaveTasks` |
| `internal/log/log.go` | Leveled console logging with ANSI colors (Info, Success, Warning, Error, Fatal, Section) |
| `internal/build/build.go` | `BuildSystem` interface + `GoBuildSystem` implementation |
| `internal/build/npm.go` | `NpmBuildSystem` + `NewBuildSystem` factory |
| `internal/git/git.go` | `EnsureEpicBranch`, `RollbackChanges`, `Commit` |
| `internal/orchestrator/bootstrap.go` | `BootstrapFromTasks`, `NeedsKBSynthesis`, `IsEpicAlreadyComplete` |
| `internal/orchestrator/taskpointers.go` | `InitializeTaskPointers`, `AdvanceToNextTask`, `FindNextActiveTask`, `IncrementAttempts`, `UpdateTaskStatus` |
| `internal/orchestrator/validation.go` | `ValidateYAMLStructure`, `ValidateStateSync` with tiered recovery |
| `internal/orchestrator/startup.go` | `CheckDependencies`, `EnsureProjectReady` |
| `internal/orchestrator/context.go` | `LoopContext` struct — carries all per-iteration state for handlers |
| `internal/agent/session.go` | `CreateSessionFile` — pre-creates session result file from embedded template |
| `internal/agent/activetask.go` | `WriteActiveTask`, `GetSkillForTaskType` — writes `logs/ACTIVE_TASK.md` |
| `internal/agent/invoke.go` | `RunAgent` — exec-based agent invocation with live stdout/stderr |
| `internal/agent/parse.go` | `ParseSessionResult` — YAML frontmatter extraction with typed errors |
| `internal/handlers/success.go` | `HandleSuccess` — install deps → build → test → metrics → changelog → commit → advance |
| `internal/handlers/failure.go` | `HandleFailure` — rollback → retry or block + archive |
| `internal/handlers/bug.go` | `HandleBug` — nested bug check → rollback → inject bugfix task |
| `internal/handlers/epic.go` | `HandleEpicComplete` — metrics summary → final commit → banner |
| `internal/metrics/metrics.go` | `RecordTaskMetrics`, `UpdateMetricTotals`, `PrintEpicSummary` |
| `internal/changelog/changelog.go` | Idempotent CHANGELOG.md update via pure Go string manipulation |
| `internal/templates/templates.go` | `//go:embed` for `runtime/` and `init/` template trees |
| `internal/templates/runtime/session_result.md` | Session file template used by `CreateSessionFile` at runtime |
| `internal/templates/init/**` | Files copied to user projects by `doug init` (CLAUDE.md, AGENTS.md, templates, skill files) |
| `integration/doc.go` | Package declaration for integration test package |
| `integration/smoke_test.go` | **EMPTY** — contains only `package integration` (see Anti-Patterns) |
| `.github/workflows/ci.yml` | CI: `go test ./...` and `go vet ./...` on ubuntu + macos |
| `.github/workflows/release.yml` | Release: GoReleaser on `v*` tag push |
| `.goreleaser.yaml` | Builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |
| `Makefile` | `build`, `test`, `lint`, `release-dry` targets |
| `doug.yaml` | Config for this project's own development runs |
| `backlog.yaml` | Full multi-epic backlog reference (used as source material, not live state) |
| `orchestrator/` | **Bash orchestrator source** — the v0.3.0 predecessor (still present) |
| `agent_loop` | Bash orchestrator entry point script (still present) |

---

## Data Flow

```
User runs: doug run
     │
     ▼
cmd/run.go: runOrchestrate()
     │
     ├─ LoadConfig (doug.yaml → OrchestratorConfig)
     ├─ CheckDependencies (agent binary, git, toolchain on PATH)
     ├─ LoadProjectState (project-state.yaml)
     ├─ LoadTasks (tasks.yaml)
     ├─ BootstrapFromTasks (first-run init of epic/task pointers)
     ├─ IsEpicAlreadyComplete → exit 0 if done
     ├─ NewBuildSystem (go | npm)
     ├─ EnsureProjectReady (pre-flight build + test)
     ├─ ValidateYAMLStructure (required fields, valid status enums)
     ├─ EnsureEpicBranch (create/checkout feature/{epic-id})
     ├─ InitializeTaskPointers (align active/next with task statuses)
     ├─ ValidateStateSync (state drift detection, Tier 1/3 recovery)
     └─ SaveProjectState
          │
          ▼
     ┌─── MAIN LOOP (max_iterations) ──────────────────────────────┐
     │ IncrementAttempts → SaveState                               │
     │ CreateSessionFile → logs/sessions/{epic}/session-{id}_attempt-{n}.md │
     │ WriteActiveTask → logs/ACTIVE_TASK.md (task ID + skill + bug ctx) │
     │ RunAgent (agentCommand via exec.Command, live stdout/stderr) │
     │ ParseSessionResult (YAML frontmatter from session file)     │
     │         │                                                   │
     │    ┌────┴────┐                                              │
     │    ▼         ▼                                              │
     │ SUCCESS    FAILURE                                          │
     │   │         │                                               │
     │   │       rollback → retry or block                        │
     │   ▼                                                         │
     │ install deps → build → test → metrics → changelog          │
     │ → mark DONE → SaveState → Commit → advance pointers        │
     │   │                                                         │
     │   ├─ Retry (build/test failed, rollback done)              │
     │   ├─ Continue (normal forward progress)                     │
     │   └─ EpicComplete → HandleEpicComplete → exit 0            │
     │                                                             │
     │ BUG → nested bug check → rollback → inject bugfix task     │
     │ EPIC_COMPLETE → HandleEpicComplete → exit 0                │
     └─────────────────────────────────────────────────────────────┘
```

---

## Dependencies

### Internal Dependencies
All packages are internal (`github.com/robertgumeny/doug/internal/*`). The dependency graph flows:
- `types` → no internal deps (leaf)
- `config`, `state`, `log`, `metrics`, `changelog`, `templates` → `types` only
- `build`, `git` → `types`, `log`
- `orchestrator` → `types`, `build`, `config`, `log`, `state`
- `agent` → `types`, `log`, `templates`
- `handlers` → `types`, `orchestrator`, `git`, `log`, `metrics`, `changelog`, `state`
- `cmd` → all of the above

### External Dependencies
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework — commands, flags, help text |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML serialization for state files |
| `github.com/spf13/pflag` | v1.0.9 | Indirect dep of cobra |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | Windows cobra helper (indirect) |

Only 2 direct dependencies. No unnecessary imports.

---

## Patterns Observed

- **Atomic YAML writes**: `state.go` writes to `path.tmp` then `os.Rename` — prevents partial-write corruption across all state file saves.
- **exec.Command only**: Zero shell wrapping. Every external process uses an explicit args slice (`exec.Command("git", "checkout", branchName)`).
- **Defaults-merge config loading**: `partialConfig` with pointer fields allows distinguishing absent vs zero-valued config fields; missing fields get sane defaults.
- **Tiered failure recovery**: Tier 1 = silent self-correct, Tier 2 = self-correct + warning, Tier 3 = fatal exit. Every error path is classified.
- **IsSynthetic() dispatch**: `TaskType.IsSynthetic()` cleanly gates logic that only applies to user-defined tasks, avoiding scattered string comparisons.
- **LoopContext pass-through**: All per-iteration state passed as `*LoopContext` to handlers — no package globals, no function parameter sprawl.
- **Protected paths in rollback**: `project-state.yaml` and `tasks.yaml` are backed up before `git reset --hard` and restored after, so orchestrator state survives agent misbehavior.
- **Embedded templates**: `//go:embed` compiles all template files into the binary — no runtime disk dependencies, no install path issues.
- **Hardcoded fallback skills**: `activetask.go` defines minimal fallback skill content so the orchestrator works even without a `skills-config.yaml` file.

---

## Anti-Patterns & Tech Debt

- **CRITICAL — Missing `project-state.yaml` creation in `doug init`**: `cmd/init.go` creates `doug.yaml`, `tasks.yaml`, `PRD.md`, `CLAUDE.md`, and `AGENTS.md` but NOT `project-state.yaml`. `cmd/run.go:108` calls `state.LoadProjectState` and returns an error on `ErrNotFound` with no special handling. **A user following the intended workflow (`doug init` → `doug run`) will get a fatal error on first run.** This is a workflow-breaking bug. Fix: either `doug init` creates a minimal `project-state.yaml`, or `runOrchestrate` creates a default one when not found.

- **CRITICAL — Integration smoke test is empty**: `integration/smoke_test.go` contains only `package integration`. The EPIC-6-003 task was marked DONE but both implementation attempts were rolled back (the agent couldn't verify via Bash tool in the Windows/MSYS2 environment, and the smoke test code failed go test verification). The PRD's Definition of Done requires a passing integration smoke test. This is a significant gap.

- **Stale README**: `README.md` still contains the Epic 1 scaffold description ("This is a scaffold — the core commands are stubs today"). It has no documentation of the actual functionality, installation, usage, configuration, agent contract, or trust boundary. This must be replaced entirely before open-source release.

- **Version inconsistency**: `cmd/root.go:10` hardcodes `version = "v0.0.1"` but the PRD targets `v0.4.0`. The version string needs to be bumped to reflect the actual release. Ideally, GoReleaser should inject the version at build time via `-ldflags "-X cmd.version={{.Version}}"` rather than hardcoding.

- **Unused `internal/templates/skills/` directory**: This directory (`internal/templates/skills/implement-{feature,bugfix,documentation}/SKILL.md`) exists on disk but is NOT embedded in `templates.go` (only `init/` and `runtime/` are embedded). These files appear to be a vestigial artifact from an earlier implementation design. They are dead code — never read, never embedded.

- **Stale `logs/sessions/SESSION_RESULTS_TEMPLATE.md`**: A copy of the session results template was placed at `logs/sessions/SESSION_RESULTS_TEMPLATE.md` during the project's development. This is the wrong location (the canonical template is embedded in `internal/templates/init/SESSION_RESULTS_TEMPLATE.md`). This file is not gitignored and would appear in the repository.

- **Bash orchestrator still present**: The `orchestrator/` directory and `agent_loop` file in the project root are the Bash v0.3.0 predecessor. For a Go port release, these should be removed or explicitly archived in a `_legacy/` directory with a README note.

- **`doug.exe` binary in root**: The compiled binary is gitignored but present on disk. Minor cosmetic issue for development.

- **`settings.json` rule typo**: `".claude/settings.json"` contains `"Bash(git commit:*)"` (colon, not space). This deny rule would never match a git commit command. The correct pattern is `"Bash(git commit *)"`. The rule `"Bash(git commit -m *)"` below it does work correctly, but the generic pattern is silently broken.

- **`InitializeTaskPointers` clobbers synthetic tasks on restart**: If a bugfix or KB synthesis task is active and `doug run` is restarted, `InitializeTaskPointers` overwrites the active task with the first TODO/IN_PROGRESS user task, losing the synthetic task context. This is inherited from the Bash orchestrator and is a known limitation, but should be documented.

- **Go 1.26 requirement**: `go.mod` requires Go 1.26 (released ~Feb 2026). Users on older Go versions cannot build the tool. The README must state this requirement clearly. Consider whether the tool actually uses any Go 1.26-specific features; if not, lowering to 1.22 or 1.23 would broaden compatibility.

---

## State Management

The orchestrator manages two YAML state files with distinct ownership:

| File | Owner | Mutation Pattern |
|------|-------|-----------------|
| `project-state.yaml` | Orchestrator | Loaded into memory once per run, mutated in-memory throughout the loop, atomically written via `SaveProjectState` at each critical point |
| `tasks.yaml` | User (status fields only) | Loaded once per run, status fields updated in-memory by `UpdateTaskStatus`, saved via `SaveTasks` |
| `doug.yaml` | User | Read-only at runtime; CLI flags override at highest precedence |

State is held as `*types.ProjectState` and `*types.Tasks` in `LoopContext` and mutated in-place. All writes are atomic via the `atomicWrite` helper (`path.tmp` → `os.Rename`). The orchestrator persists state at 6 points per iteration:
1. After `IncrementAttempts` (before agent invocation — preserves attempt count across crashes)
2. After `HandleSuccess` updates task pointers
3. After `HandleFailure` sets manual_review state
4. After `HandleBug` schedules bugfix task
5. After documentation task sets `completed_at`
6. Final `SaveProjectState` at loop start

---

## PRD Alignment

### Definition of Done — Status

| Requirement | Status | Notes |
|-------------|--------|-------|
| All 24 feature tasks DONE | ✅ | All marked DONE in project-state.yaml metrics |
| `go test ./...` passes with meaningful coverage | ✅ (partial) | Unit tests exist for all packages; integration test is empty |
| Integration smoke test passes | ❌ | `integration/smoke_test.go` is empty — not implemented |
| `doug init` produces usable scaffold | ✅ (partial) | Works but doesn't create `project-state.yaml`; first `doug run` fails |
| `cmd/run.go` human-reviewed after EPIC-5-005 | ✅ | Marked done |
| Task 6-4 Final Integration QA signed off | ⚠️ | QA was static-analysis-only due to Bash tool failure in Windows/MSYS2 |
| README documents agent trust boundary | ❌ | README is scaffold placeholder from Epic 1 |
| No `sed`, `awk`, `yq`, `eval` in Go codebase | ✅ | All shell-specific operations replaced |

### Behaviors from PRD

| Behavior | Implemented | Location |
|----------|-------------|----------|
| Atomic YAML writes via os.Rename | ✅ | `internal/state/state.go:atomicWrite` |
| exec.Command replaces eval | ✅ | All exec calls use explicit args slice |
| SessionResult has exactly 3 fields | ✅ | `internal/types/types.go:SessionResult` |
| Epic commit failure is Tier 3 exit (CI-6) | ✅ | `internal/handlers/epic.go:HandleEpicComplete` |
| next_task.type preserved for synthetic tasks in bug handler (CI-5) | ✅ | `internal/handlers/bug.go:resolveInterruptedType` |
| ACTIVE_BUG.md and ACTIVE_FAILURE.md flat paths (CI-1, CI-2) | ✅ | Both handlers read from `logs/` root |
| IsInitialized() checks go.sum | ✅ | `internal/build/build.go:GoBuildSystem.IsInitialized` |
| handle_early_exit dead code removed | ✅ | Not ported |
| Python build system not ported | ✅ | Only go and npm implemented |
| Failure recovery tier system | ✅ | Documented in `orchestrator/validation.go`, handlers |
| Protected paths during rollback | ✅ | `handlers/success.go:protectedPaths` |
| Session file pre-creation | ✅ | `agent/session.go:CreateSessionFile` |
| Agent boundary by instruction only | ✅ (design) | Not yet documented in README |

---

## Extensibility Analysis

### Adding a New Coding Agent

This is the simplest extension. Change one line in `doug.yaml`:
```yaml
agent_command: aider  # or: cursor, copilot-cli, custom-script.sh
```
No code changes needed. The agent just needs to:
1. Read `logs/ACTIVE_TASK.md` for its briefing
2. Write a session result file (path given in ACTIVE_TASK.md) with YAML frontmatter

**Hook point**: `internal/agent/invoke.go:RunAgent` — the command is split by whitespace into executable + args, so `agent_command: claude --dangerously-skip-permissions` works correctly.

### Adding a New Build System

Implement the `BuildSystem` interface in `internal/build/`:
```go
type BuildSystem interface {
    Install() error
    Build() error
    Test() error
    IsInitialized() bool
}
```
Then add a case to `NewBuildSystem` in `internal/build/npm.go`. The `CheckDependencies` function in `startup.go` would also need a case for the new toolchain binary.

**Effort**: ~50 lines of code, one Makefile update, one switch case.

### Adding a New Task Type

1. Add a constant to `internal/types/types.go:TaskType`
2. Update `IsSynthetic()` if the new type is orchestrator-injected
3. Add a default skill name to `hardcodedSkillNames` in `agent/activetask.go`
4. Add minimal fallback content to `hardcodedSkillContent`
5. Create the skill file at `.claude/skills/{name}/SKILL.md`
6. Add the mapping to `.claude/skills-config.yaml`

The `sectionHeader` in `changelog/changelog.go` would need updating for changelog entries.

### Custom Skill Routing

The `.claude/skills-config.yaml` file maps task types to skill names with zero code changes:
```yaml
skill_mappings:
  refactor: implement-refactor
  security-audit: security-scan-and-report
```
Then create the skill file and add tasks with the new type.

### Configuration Options

`doug.yaml` has 5 fields, all with sane defaults:
```yaml
agent_command: claude
build_system: go
max_retries: 5
max_iterations: 20
kb_enabled: true
```
All fields can be overridden by CLI flags at highest precedence.

---

## Architecture & Installation Summary (for README)

### How It Works

`doug` implements a three-layer model:
```
doug (orchestrator binary)
  ↓ writes logs/ACTIVE_TASK.md (task briefing + skill instructions)
Agent (claude, aider, or any CLI tool)
  ↓ writes session file (result: SUCCESS | BUG | FAILURE | EPIC_COMPLETE)
State (tasks.yaml, project-state.yaml)
```

The orchestrator is the only process that touches git and state files. The agent reads code and briefing files, writes code and a session result — nothing else.

### Installation

**From source:**
```bash
go install github.com/robertgumeny/doug@latest
```

**From binary release:**
Download the appropriate archive from GitHub Releases (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64), extract, and place `doug` on your PATH.

**Requirements:**
- Go 1.26+ (if building from source)
- `git` on PATH
- Your build system toolchain on PATH (go for Go projects, npm for Node.js)
- Your agent of choice on PATH (e.g., `claude` from Claude Code)

### Commands

**`doug init`** — Initialize a new project:
```bash
cd my-project
doug init                        # auto-detects go.mod / package.json
doug init --build-system npm    # explicit build system
doug init --force                # overwrite existing files
```

Creates:
- `doug.yaml` — orchestrator config with inline comments
- `tasks.yaml` — example epic with two tasks
- `PRD.md` — product requirements template
- `CLAUDE.md` — agent onboarding guide
- `AGENTS.md` — agent contract documentation
- `logs/SESSION_RESULTS_TEMPLATE.md` — template for agent session results
- `logs/BUG_REPORT_TEMPLATE.md` — template for bug reports
- `logs/FAILURE_REPORT_TEMPLATE.md` — template for failure reports
- `.claude/skills/implement-{feature,bugfix,documentation}/SKILL.md` — agent skill files

**`doug run`** — Run the orchestration loop:
```bash
doug run                          # uses doug.yaml config
doug run --agent aider            # override agent command
doug run --build-system npm       # override build system
doug run --max-iterations 10      # override iteration limit
doug run --max-retries 3          # override retry limit
doug run --kb-enabled=false       # skip KB synthesis
```

**`doug --version`** — Print version.

### State Machine

```
Agent reports SUCCESS
  → install deps (if any) → build → test → mark DONE → commit → advance
  → if build/test fails: rollback → retry on next iteration
  → if all tasks DONE + kb_enabled: inject KB_UPDATE → EpicComplete

Agent reports FAILURE
  → rollback → if attempts < max_retries: retry
  → if attempts >= max_retries: BLOCK task → manual_review → exit 1

Agent reports BUG
  → check for nested bug (exit 1 if bugfix task) → rollback
  → inject BUG-{taskID} bugfix task → resume original task after

Agent reports EPIC_COMPLETE
  → print summary → final commit → exit 0
```

### Task Structure (`tasks.yaml`)

```yaml
epic:
  id: "EPIC-1"
  name: "First Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"         # feature | bugfix | documentation | manual_review
      status: "TODO"          # TODO | IN_PROGRESS | DONE | BLOCKED
      description: "..."
      acceptance_criteria:
        - "..."
```

### Config Reference (`doug.yaml`)

```yaml
agent_command: claude   # command to invoke the agent
build_system: go        # go | npm
max_retries: 5          # FAILURE attempts before task is BLOCKED
max_iterations: 20      # loop iterations before exit
kb_enabled: true        # inject KB synthesis task after epic completion
```

---

## Safety & "Running Amok" Analysis

### What the Agent CAN Do

- Read and write any file in the project directory (and potentially outside it)
- Run arbitrary code within the permissions granted by the agent's own settings
- Write to `logs/ACTIVE_TASK.md`, `logs/ACTIVE_BUG.md`, `logs/ACTIVE_FAILURE.md`
- Modify source files, tests, documentation

### What the Agent CANNOT Do (by design)

- Run `git` commands — the orchestrator manages all git operations
- Modify `project-state.yaml` or `tasks.yaml` — orchestrator owns these
- Trigger more than `max_iterations` loop runs
- Escape a bugfix death spiral — nested bug check prevents it
- Corrupt state through partial writes — atomic YAML I/O and protected paths prevent this

### Trust Boundary Enforcement

The boundary is enforced by **instruction only**, not sandboxing:
- `CLAUDE.md` (copied to user projects) instructs the agent not to run git or modify state files
- Skill files reinforce these constraints
- `.claude/settings.json` (for Claude Code specifically) uses `deny` rules to block git and yaml writes

**Important**: The `settings.json` deny rules in this repo are the **development project's settings**. They are not automatically applied to user projects. When users install `doug` and run it against their own projects, they get the CLAUDE.md and skills constraints (via `doug init`) but NOT the settings.json restrictions unless they configure them. This is by design (agent-agnostic), but it must be prominently documented.

### Safety Mechanisms Against Runaway Behavior

| Mechanism | Protects Against |
|-----------|-----------------|
| `max_iterations` limit | Infinite loops |
| `max_retries` (5-strike) limit | Persistent failure loops |
| Nested bug check in `HandleBug` | Bugfix death spiral |
| `git reset --hard HEAD` rollback on failure | Bad agent output persisting |
| Protected paths during rollback | State corruption from rollback |
| Atomic YAML writes | Partial-write state corruption |
| `exec.Command` (no `eval`) | Shell injection from malformed agentCommand |
| Build + test verification after each SUCCESS | Broken code being committed |

### Residual Risks

1. **Full filesystem access**: An agent with instructions to "implement the feature" technically has access to files outside the project. No path restriction is enforced at the orchestrator level.
2. **Agent exit code non-fatal**: `agent.RunAgent` logs a warning but reads the session file regardless of exit code. A malicious or broken agent could write a forged session result.
3. **`agentCommand` injection** (low risk): `agentCommand` is split by whitespace only, not shell-parsed. A config value like `claude; rm -rf /home` would try to execute `claude;` as a binary (which would fail), not be shell-interpolated. This is safe.

---

## Production Readiness Checklist (for GitHub Open-Source Release)

### Blocking (must fix before release)

- [ ] **Fix `doug init` missing `project-state.yaml`**: After `doug init`, `doug run` fails with "state file not found". Either create a minimal `project-state.yaml` in `initProject`, or handle `ErrNotFound` gracefully in `runOrchestrate` by initializing a zero state.
- [ ] **Replace README.md entirely**: Current README is the Epic 1 scaffold ("stubs today"). Needs full documentation of: what doug is, installation, `doug init` walkthrough, `doug run` usage, `doug.yaml` reference, `tasks.yaml` format, agent contract, trust boundary warning, build system support, extension guide.
- [ ] **Document agent trust boundary in README**: Must prominently state that the agent has full filesystem access enforced by instruction, not sandboxing. Quote the CLAUDE.md/skills contract. Link to relevant settings.json pattern for Claude Code users.
- [ ] **Bump version to v0.4.0**: Update `cmd/root.go:10` from `v0.0.1` to `v0.4.0`. Ideally inject via GoReleaser ldflags: `ldflags: ["-X github.com/robertgumeny/doug/cmd.version={{.Version}}"]`.
- [ ] **Remove or archive Bash orchestrator**: The `orchestrator/` directory and `agent_loop` script are the v0.3.0 predecessor. For a Go port release, move to `_legacy/` with a README or remove entirely to avoid confusion.
- [ ] **Fix `settings.json` deny rule typo**: `"Bash(git commit:*)"` should be `"Bash(git commit *)"`. This is in the init template so it affects all users.

### Important (strongly recommended)

- [ ] **Implement integration smoke test**: `integration/smoke_test.go` is empty. The test was designed in EPIC-6-003 but both implementation attempts were rolled back due to Bash tool failures in the Windows/MSYS2 build environment. The attempt-2 design (self-referential test binary via `MOCK_AGENT_MODE=1` env var) was sound — implement it in a non-Windows environment where `go test ./integration/...` can actually run.
- [ ] **Remove `internal/templates/skills/` directory**: This directory is not embedded in `templates.go` and serves no function. It is dead code from an earlier design draft.
- [ ] **Remove stale `logs/sessions/SESSION_RESULTS_TEMPLATE.md`**: This file is at the wrong path and is a development artifact.
- [ ] **Create `.gitignore` entry for `logs/sessions/`**: Session logs are project-specific; new users should probably gitignore these or the repo will accumulate session logs.
- [ ] **Go version consideration**: `go 1.26` is the newest Go release. Consider whether to require 1.26 or lower to 1.22/1.23 for broader compatibility. If no 1.26-specific features are used, lower the minimum. Document the requirement in README.
- [ ] **Inject version via GoReleaser ldflags**: Avoid hardcoding the version string in source. Add to `.goreleaser.yaml` builds section: `ldflags: ["-s", "-w", "-X github.com/robertgumeny/doug/cmd.version={{.Version}}"]`.

### Nice-to-Have (post-release)

- [ ] **Add `CONTRIBUTING.md`**: Given potential community contributions, document: how to add a build system, how to add a task type, how to write a new skill, PR guidelines.
- [ ] **Add `LICENSE` file**: Required for an open-source release. MIT is a natural choice for a developer tool.
- [ ] **Clean up `backlog.yaml`**: This was the development backlog used to build the tool. For an end-user repo, it is confusing. Either remove or rename to `DEVELOPMENT_HISTORY.md`.
- [ ] **Add `project-state.yaml` to `.gitignore`**: Users probably don't want to commit their project state. Consider adding it to the default `.gitignore` generated by `doug init` (though the current tool gitignore may differ from a user's project gitignore).
- [ ] **Add `--version` to GoReleaser release notes**: The release.yml workflow currently uses `args: release --clean`. Add `--release-notes` or configure GoReleaser changelog to produce better release notes.
- [ ] **Consider adding `doug status`**: A read-only command to display current epic, active task, attempt count, and metrics without running the loop. Very useful for debugging.
- [ ] **Fix `InitializeTaskPointers` synthetic task clobbering**: On restart, synthetic tasks (bugfix in progress, KB synthesis in progress) are lost because `InitializeTaskPointers` only looks at `tasks.yaml` statuses. A future fix would check `state.ActiveTask.Type.IsSynthetic()` and skip re-initialization when a synthetic task is already active.

---

## Raw Notes

### On the Module Path
The module is `github.com/robertgumeny/doug`. The `[org]` placeholder from the PRD was filled in. This is fine for release — the module path is baked into all imports and cannot be changed without breaking all consuming packages.

### On the Bash Orchestrator
The `orchestrator/` directory and `agent_loop` script represent the prior Bash implementation. They are interesting as historical context but will confuse new GitHub visitors. The PRD says the Go port is "not a feature addition — it is a faithful translation." Keeping the Bash source in the same repo conflates the two. For a clean open-source release, archive them.

### On the CI Workflow
The CI runs on ubuntu-latest and macos-latest but not windows-latest. Given the tool is cross-platform and the development environment is Windows, adding a windows-latest matrix entry would catch Windows-specific issues (e.g., the EINVAL Bash tool problem that caused the smoke test failures).

### On the `EPIC_COMPLETE` Exit Path
There are two paths to `HandleEpicComplete`:
1. `HandleSuccess` returns `EpicComplete` (documentation task finished)
2. Agent directly reports `EPIC_COMPLETE` outcome

The second path is rarely used but allows an agent to signal early completion (e.g., "all tasks were actually already done"). Both paths are handled identically.

### On the skills-config.yaml in Init Templates
`doug init` copies `.claude/skills-config.yaml` — wait, actually it does NOT. Looking at `copyInitTemplates`, it only copies CLAUDE.md, AGENTS.md, `*_TEMPLATE.md` files, and `skills/**`. The `skills-config.yaml` is NOT copied. New users would need to create this file manually or rely on hardcoded fallback mappings. The hardcoded fallbacks cover all four built-in types, so this works — but extending with custom task types requires manually creating `skills-config.yaml`. This should be documented.
