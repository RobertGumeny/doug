# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Integration smoke test verifies doug init and doug run reach agent invocation without panics
- agent.RunAgent now accepts context.Context; cancelling the context kills the subprocess and returns promptly
- Orchestrator.Run now checks ctx.Done() at each iteration for clean cancellation; cmd/run.go reduced to 46 lines by extracting loadConfig into cmd/config.go
- Move pre-loop orchestration setup into Orchestrator.Run; cmd/run.go reduced to flag parsing and construction
- Eliminate post-construction LoopContext mutations: AgentDurationSeconds and SessionResult are now passed explicitly to handlers
- Introduce Orchestrator struct and Paths type in internal/orchestrator
- Introduce Logger interface in internal/log with stderr default; thread through orchestrator, handlers, and agent

### Changed
- docs: update KB for EPIC-12 orchestrator refactor — Logger interface, Orchestrator struct, Paths, LoopContext move to types, context.Context on RunAgent, ValidateTaskTypes

### Fixed

### Removed

## [0.6.0]

### Added

- On doug run startup, epic rollover is now detected automatically — when tasks.yaml declares a new epic ID, runtime state is reset without requiring manual edits to project-state.yaml, and a clear log message confirms the transition.
- Automatic epic rollover: doug run now detects a new epic ID in tasks.yaml and re-initializes project state without manual edits
- Improved YAML parse error messages for tasks.yaml with field-level detail and formatting hints; added corrective action hints to key orchestrator errors
- On test failure after SUCCESS, retry once with test output injected into next ACTIVE_TASK.md briefing; second consecutive test failure routes to PAUSED
- Implement PAUSED state resume: doug run now runs build verification on a paused project instead of exiting, marks the task DONE and continues the loop on pass, or re-pauses on fail
- Introduce PAUSED project state and BUILD_FAILURE outcome: build or test verification failure after agent SUCCESS now pauses the loop with working tree preserved instead of rolling back and retrying
- Add unconditional ArchiveActiveTask that copies ACTIVE_TASK.md to session archive before every state change
- Move agent result block into ACTIVE_TASK.md; ParseSessionResult now reads from ACTIVE_TASK.md instead of a separate session file

### Changed

- `RunAgent` now accepts an `output io.Writer` parameter; `nil` preserves the previous behaviour (forwarding to `os.Stdout`/`os.Stderr`). The `doug run` loop passes a log file so agent stdout/stderr is captured silently rather than printed to the terminal — this prevents agents such as `codex exec` that unconditionally stream output from polluting the orchestrator display.
- Agent raw output logs are written to `.doug/logs/output/{epic}/output-{taskID}_attempt-{N}.log`, separate from the session archive at `.doug/logs/sessions/{epic}/`, so the KB synthesis scan is not affected.

## [0.5.6]

### Added

- Added `static` build system type for plain HTML/CSS/JS projects with no build step — all build/install/test operations are no-ops and the project is always considered initialized
- `doug init` auto-detects `static` when `index.html` is present (lowest priority, checked after `go.mod`, `pnpm-workspace.yaml`, and `package.json`)
- `--build-system static` flag support in `doug init` and `doug run`
- `CheckDependencies` skips toolchain binary check when `build_system: static`
- `doug init` now injects build-system-specific Bash permissions into Claude Code's `settings.json` when Claude is selected as an agent and a build system is detected
- `doug init` prompts interactively to select a build system when none is auto-detected and Claude is selected; falls back to `"go"` with a warning when stdin is not a TTY

### Changed

- `DetectBuildSystem` now returns `""` (empty string) instead of `"go"` when no marker file is found — callers determine their own fallback
- Moved build system metadata (permissions, install cmd, verify commands, init markers, common pitfalls) into a `BuildSystems` registry map in `internal/config`, replacing scattered per-type constants
- `WriteActiveTask` now injects a `## Build System` briefing section into `ACTIVE_TASK.md` when `ActiveTaskConfig.BuildSystem` is set and recognized, giving agents install commands, verify steps, and common pitfalls
- Updated settings.json template for Claude Code to better handle permissions in sandboxed environments

## [0.5.5]

### Added

- Added .gitignore template for `doug init`. If no gitignore exists, doug creates one. If one already exists, it simply adds the .doug directory to the existing file.

### Fixed

- Fixed issue with copying of skill files from templates to provider directories.

## [0.5.4]

### Fixed

- Fixed the task verification step to run the build system's install command before verifying that the build succeeds if `IsInitialized()` is false

## [0.5.3]

### Fixed

- Fixed failure during session outcome parsing if outcome was lowercase, parser now allows for lowercase or UPPERCASE session outcomes from agents.

## [0.5.2]

### Added

- Added information to README.md about the importance of the knowledge base
- Added 'Removed' section to CHANGELOG update step in `changelog.go`
- Added git init as a default step when running `doug init` with an optional --no-git-init flag to skip

### Changed

- Updated AGENTS.md to be leaner and project-agnostic. CLAUDE.md now loads AGENTS.md to ensure consistent behavior across providers and a single source of truth
- Updated README.md to include all 4 default skills: implement-feature, implement-bugfix, implement-documentation, and research
- Overall enhancements and updates to documentation

### Fixed

### Removed

## [0.5.1]

### Added

- Added support for pnpm build system

### Changed

- Updated `doug init` command to support pnpm flag

## [0.5.0]

### Added

- Added Codecov coverage upload to CI and placed CI, coverage, license, and Go 1.26 badges at the top of the README.
- Added gofmt and golangci-lint checks to CI, aligned make lint with CI, and added a conservative root golangci-lint config.
- Added GitHub issue templates for bug reports and feature requests plus a pull request template.
- Added a root SECURITY.md directing vulnerability reports to GitHub's private advisory flow with a 2-day acknowledgment commitment.
- Add a root CODE_OF_CONDUCT.md using Contributor Covenant v2.1 with a project-specific enforcement contact.
- Added CONTRIBUTING.md with PR, testing, style, commit, and AI-assisted contribution guidance; updated lint docs and Makefile behavior to include gofmt, golangci-lint, and go vet.
- Added a standard MIT LICENSE file at the repo root for OSS release readiness.

### Changed

- Refined repository contributor guidance: `AGENTS.md` is now a concise repo-specific agent brief, `CONTRIBUTING.md` now points humans to the KB and task-design guidance, and the README build instructions now use `make build` with the `bin/doug` output path.
- Updated the pull request template to require concrete change summaries, rationale, explicit validation details, follow-up notes, and local `make test`/`make lint`/`make build` accountability.
- Improved local build and lint workflow defaults by writing build output to `bin/doug`, ignoring `bin/` in Git, and isolating `go vet` and `golangci-lint` caches under `/tmp/doug-cache`.
- Upgraded GitHub Actions workflow dependencies to newer `actions/checkout`, `actions/setup-go`, and `golangci-lint-action` releases in CI and release pipelines.
- Clarified contributor documentation around using the `research` skill and removed the explicit enforcement contact line from the code of conduct.

### Fixed

- Hardened test cleanup and small formatting/test hygiene details, including checked cleanup errors in integration and template tests and `fmt.Fprintf` use in active task generation.

### Removed

## [0.4.11]

### Added

- Add integration tests for doug revert flow and unit tests for git.CurrentSHA and git.ResetHard
- doug revert now deletes session logs for reverted tasks, prints a short-SHA success message with next-steps guidance, and warns when a remote tracking branch requires a force-push
- Add doug revert command with full validation sequence before executing git reset --hard
- Add git.ResetHard helper for deliberate history rewind to a specific SHA
- Add Attempts, TaskType, and AgentDurationSeconds fields to TaskMetric metrics
- Capture git commit SHA in task metrics for traceability
- Fix outcome casing in task metrics (SUCCESS/BUG/FAILURE) and persist failure metrics on retry path

## [0.4.10]

### Added

- Scaffold per-agent autonomous settings during `doug init` for selected agents: `.claude/settings.json`, `.codex/config.toml`, `.gemini/settings.json`, and `.gemini/policies/doug-default.json`
- Add managed settings merge behavior in `doug init` so existing Claude/Codex/Gemini settings are updated non-destructively by default (with `--force` still doing full overwrite)

### Changed

- Update default `agent_command` templates and switch targets: Codex now uses `codex exec ...` and Gemini uses `--output-format json --sandbox ...`
- Update `AGENTS.md` deny list guidance to allow read-only Git context (`status`, `diff`, `log`, `show`) while continuing to block Git write/remote operations

### Fixed

### Removed

## [0.4.9]

### Added

- Remove dead skills_dir config field from OrchestratorConfig, doug.yaml templates, agentRegistry, cmd/switch.go, and cmd/init.go
- Fix doug switch YAML parse error caused by unquoted agent_command in generated doug.yaml template
- Remove stray .gemini/settings.json from doug init scaffold
- feat: add CHANGELOG.md scaffold to doug init
- Move PRD.md into .doug/ directory; update init scaffolding and agent briefing to reference .doug/PRD.md
- Move skills-config.yaml from .agents/ to .doug/ directory
- Fix doug init defaults: remove -p flag from claude agent_command and set skills_dir to .agents/skills
- Add automatic epic rollover prep that resets runtime state when `.doug/tasks.yaml` switches to a new epic ID after completion
- Add agent heartbeat support with `agent_heartbeat_seconds` config and `--agent-heartbeat-seconds` CLI override for headless liveness logs

### Changed

- Standardize Claude default command to headless mode in `doug switch` to match `doug init` behavior

### Fixed

- Ensure `HandleEpicComplete` backfills and persists `current_epic.completed_at` when missing

## [0.4.8]

### Removed

- Removed `ARCHITECTURE.md` and `PRD.md` as these are artifacts from previous sessions

## [0.4.7]

### Added

- Fix UpdateChangelog to scope subsection search and idempotency check to ## [Unreleased] block only
- Move kb_enabled from project-state.yaml into doug.yaml as a first-class config field
- Add commented Codex and Gemini agent_command examples to generated doug.yaml
- AGENTS.md rewritten as terse agent-facing instructions with deny list; now included in doug init scaffolding
- Verified SKILL.md template files are correctly placed under .agents/skills/ path and skills-config.yaml comment block references .agents/skills/
- doug init now scaffolds skills to shared .agents/skills/ and creates .gemini/settings.json
- Moved skill resolution from .claude/skills/ to .agents/skills/; renamed GetSkillName to GetSkillForTaskType
- add research skill to templates for codebase analysis and documentation generation
- added task type validation at startup
- added guard to check for `ACTIVE_BUG.md` file when task is type `bugfix`
- added task id to `agent_command` in `run.go` for better context and metric aggregation

### Changed

- Updated KB documentation for EPIC-7: agents/skills migration, kb_enabled config move, UpdateChangelog scoping

### Fixed

- Fixed UpdateChangelog to scope subsection search and idempotency check to the ## [Unreleased] block only
- Fixed bug in `run.go` that caused loops beyond max attempts for some task types

## [0.4.6]

### Changed

- Moved `tasks.yaml` into `.doug` directory on `doug init`

### Fixed

- Fixed issue with `doug --version` not showing correct version information

## [0.4.5]

### Changed

- Refactored orchestrator state paths from project root to `.doug/` directory; updated config (`SkillsDir`), handlers (active task, bug, failure report paths), tests, and skill documentation accordingly
- Consolidated agent information and improved init logic when using `doug init`

### Removed

- Removed old doug YAML files
- Removed settings.json template

## [0.4.4]

### Fixed

- add `--version` and `-v` flags to check doug version

## [0.4.3]

### Fixed

- adjust rollback logic to preserve untracked protected files and update test cases

## [0.4.2]

### Fixed

- update agent command handling in configuration and dependency checks

## [0.4.1]

### Added

- Added integration smoke test exercising full orchestrator loop end-to-end with mock agent
- Split internal/templates into runtime/ and init/ subdirectories; init command now copies CLAUDE.md, AGENTS.md, template files, and skill files into new projects
- Implemented doug init subcommand with build system detection, project scaffolding, and --force flag
- Implemented full orchestration loop in cmd/run.go, wiring all handlers, startup checks, and agent dispatch
- Added HandleEpicComplete handler, CheckDependencies, and EnsureProjectReady startup functions
- Added HandleBug handler with nested bug protection, bug ID generation, archive, and CI-5 synthetic task type fix
- Added HandleFailure handler with retry logic, failure archiving, and task blocking
- Added LoopContext struct and HandleSuccess orchestration handler with build/test verification, metrics recording, and KB synthesis injection
- Added ParseSessionResult to extract and validate YAML frontmatter from agent session files
- Added RunAgent function to invoke agent commands with live stdout/stderr streaming and duration tracking
- Added WriteActiveTask and GetSkillForTaskType to the agent layer for writing ACTIVE_TASK.md with skill instructions and bug context
- Added CreateSessionFile to copy and hydrate the session results template before each agent invocation
- Added metrics recording (RecordTaskMetrics, UpdateMetricTotals, PrintEpicSummary) and idempotent CHANGELOG update (UpdateChangelog) packages
- Added YAML structure and state-sync validation with tiered auto-correction
- Added task pointer management functions for the Go orchestrator (InitializeTaskPointers, AdvanceToNextTask, FindNextActiveTask, IncrementAttempts, UpdateTaskStatus)
- Added orchestrator bootstrap package with BootstrapFromTasks, NeedsKBSynthesis, and IsEpicAlreadyComplete
- Added git package with EnsureEpicBranch, RollbackChanges, and Commit operations
- Added NpmBuildSystem with package.json test-script guard and NewBuildSystem factory
- Added BuildSystem interface and GoBuildSystem implementation for go build lifecycle management
- Added internal log package with Info, Success, Warning, Error, Fatal, and Section functions using ANSI color codes
- Added OrchestratorConfig with sane defaults, LoadConfig with partial-file support, and DetectBuildSystem for go/npm detection
- Added atomic state I/O package with LoadProjectState, SaveProjectState, LoadTasks, and SaveTasks
- Added core type definitions for the doug orchestrator with full YAML round-trip support
- Verified project scaffold is correct and production-ready; updated go.mod to Go 1.26 per project standard

### Changed

- Updated KB with EPIC-6 content: cmd/init and internal/templates articles; corrected stale agent.md; updated project structure in go.md
- Added internal/handlers KB article covering HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete, LoopContext, and run loop integration; updated orchestrator.md with LoopContext and startup functions
- Added internal/agent KB article covering CreateSessionFile, WriteActiveTask, RunAgent, and ParseSessionResult
- Synthesized EPIC-2 session logs into three new KB articles (internal/log, internal/build, internal/git), created KB index README, and corrected stale content in infrastructure/go.md
- Synthesized EPIC-1 session logs into three new KB articles (internal/types, internal/state, internal/config) and updated infrastructure/go.md with module path and cross-references

### Fixed

### Removed
