# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added transport_failure run status classification for Pi CLI transport failures.
- Warn when module_root has no go.mod
- Treat Go modules with go.mod but no go.sum as initialized during pre-flight checks.
- Add module_root build-system anchoring and Go go.mod initialization detection

### Changed

- Improved the planning skill and generated planning workbook for `doug plan` to enforce more robust PRDs and better task decomposition and acceptance criteria

### Fixed

### Removed

## [0.7.0]

### Added
- Added a fuller workflow around `doug plan`, `doug handoff`, `doug research`, `doug scaffold`, and `doug upgrade`, giving Doug stronger support for planning, greenfield bootstrap, repo analysis, and workspace maintenance.
- Added a structured planning lifecycle with backlog epic packaging, planning workbooks, handoff validation, and post-epic knowledge-base synthesis.
- Added a shared backend execution contract that supports Pi-backed runs across runtime, planning, scaffold, research, and post-epic workflows.

### Changed
- Doug now uses Pi exclusively: `doug plan` launches true interactive Pi, while `doug run`, `doug research`, `doug scaffold`, and post-epic KB synthesis use Pi RPC one-shot execution.
- `doug init` now scaffolds the current Pi-era workspace model, including `.pi/skills`, `.pi/extensions/handoff.ts`, updated `.doug` project files, interactive setup prompts, and stronger AGENTS/briefing guidance.
- Project config and execution docs were simplified around the Pi-only contract, removing stale config-driven routing narratives and aligning README/KB/package docs with the current model.
- `doug upgrade` now does a better job identifying Pi-era drift by flagging retired provider directories/config fields and reinstalling outdated managed `.pi` surfaces.
- Planning and runtime workflows were restructured around clearer artifact ownership: Doug owns execution briefs and runtime state, while planning/handoff own backlog packaging and lifecycle transitions.
- Runtime validation and task handling were hardened with clearer result contracts, write-scope enforcement, optional lint validation, better pause/retry behavior, improved epic rollover/finalization, and stronger protection against malformed workspace state.
- Bundled skill templates were decoupled from Doug-specific workflow contracts so installed skills stay reusable outside Doug-managed runs.

### Fixed
- Fixed multiple planning and handoff edge cases, including stale workbook content, placeholder handoff data, task-type validation, and planning-intent handling.
- Fixed `doug plan` intent capture UX and launch flow so Doug prepares `PLAN.md`/`ACTIVE_TASK.md` first and then opens a true terminal-interactive Pi session bootstrapped from `.doug/ACTIVE_TASK.md`.
- Fixed Pi-era migration and workspace drift issues by improving upgrade detection, managed-surface reinstall behavior, and config validation/reporting.
- Fixed several orchestration correctness issues around blocked tasks, synthetic task handling, backend selection, session/result parsing, changelog categorization, and erroneous Pi dependency checks in CI.

### Removed
- Removed legacy provider-specific scaffolding and compatibility surfaces such as `.claude/`, `.codex/`, `.gemini/`, `doug switch`, and older command/config wiring that no longer fits the Pi-first model.
- Removed older runtime and planning implementation paths that were superseded by the shared backend, execution-policy, and planning-lifecycle architecture.

## [0.6.7]

### Added
- Add regression tests for EPIC-18 planning and init changes: provider command routing in doug.yaml, AGENTS.md bug-report path, skills-config content, and per-provider workflow coverage for codex and gemini.
- - Rewrote `internal/templates/init/AGENTS.md` to distinguish auto-generated metadata from editable instructional content; added bug report template path to Working Rules.
- Removed commented-out alternative provider command blocks from generated `doug.yaml` (`dougYAMLContent` in `cmd/init.go`).
- Rewrote `internal/templates/init/skills-config.yaml` comments to explain skill-mapping behavior and clarify the selected-agent install model.
- Refactored `doug init` template installation into explicit install-plan and focused merge seams: extracted all merge algorithms into `cmd/init_merge.go`, introduced `installEntry`/`buildInstallPlan`/`executeInstallPlan` in `cmd/init_install.go`, and replaced the monolithic walk callback in `copyInitTemplates` with a thin two-step wrapper. Added 30+ focused unit tests covering `mergeJSONSettings`, `mergeCodexConfigTOML`, `mergeStringArrays`, `mergeGitignore` edge cases, and `buildInstallPlan` routing.
- Refactored `cmd/init.go` into a thin entrypoint by extracting prompt orchestration into `cmd/init_workflow.go`. Prompt helpers now accept `io.Writer`/`io.Reader` instead of depending on `os.Stdin`/`os.Stdout`. Added `cmd/init_workflow_test.go` with 25 new tests covering prompt functions, `runInitWorkflow` (interactive and non-interactive paths), and the cobra command entry path.
- Add placeholder-safety validation to `internal/plan/handoff.go` so seed-template values from the default PLAN.md workbook are rejected at handoff with actionable error messages; document the handoff data structure and `prd` content source in `docs/kb/features/planning-lifecycle.md`.
- Replace minimal PLAN.md handoff seed with full schema-shaped YAML example exposing project, epic, PRD, task, and acceptance-criteria structure.
- Expand the seeded `PLAN.md` handoff template and Doug-owned planning brief to show the exact supported YAML schema, warn that unknown fields break `doug handoff`, and route greenfield scaffold metadata into `manifest` instead of `project`.
- Polish `doug plan` interactive flow: suppress heartbeat logging during planning sessions and add blank-workbook imperative to the Doug-owned brief.

### Changed

### Fixed

### Removed

## [0.6.6]

### Added
- Expanded README and KB documentation for the integrated planning lifecycle, backlog package contract, quoting rules, and epic checkout flow.
- Propagate epic completion into backlog metadata, archive the runtime snapshot, and add a terminal completion path that works with or without KB synthesis.
- Extend doug run to promote backlog epics into the root runtime workspace before the existing orchestrator loop starts.
- Added `doug plan` plus a provider-installed `plan` skill so planning now centers on `.doug/plan/PLAN.md` and leaves deterministic backlog artifacts to `doug handoff`.
- Added `doug handoff` with a strict `PLAN.md` handoff parser, deterministic backlog package generation, manifest derivation for greenfield plans, and quoted `tasks.yaml` rendering for parser-safe output.
- Added typed backlog epic metadata plus deterministic epic package path and metadata IO helpers for .doug/plan/epics.
- Define the planning and execution lifecycle contract for root runtime files, backlog epics, lifecycle states, and command ownership.
- Added a provider-installed `scaffold` skill template to init output so scaffold workflows can be shipped alongside the existing default skill set.
- Added best-effort post-epic KB synthesis that runs from archived runtime/session artifacts after epic finalization without reopening runtime task state.

### Changed
- Update planning lifecycle documentation to cover metadata fields, runtime snapshot archives, and epic completion finalization behavior.
- `doug plan` now refreshes a Doug-owned briefing block at the top of `.doug/plan/PLAN.md`, treats that file as both the planning brief and workbook, and rewrites the provider prompt to operate on `PLAN.md` instead of `.doug/ACTIVE_TASK.md`.
- Refactored the `plan` skill into a stronger discovery/scoping workflow with progressive-disclosure planning lenses for discovery, roadmapping, definition, feature, refactor, bugfix, and greenfield planning.
- Removed synthetic `KB_UPDATE` task injection from the run loop; epics now finalize immediately after the last user task, clear runtime task pointers, and optionally run KB synthesis afterward as a separate post-epic step.
- Outcome handlers now remove the live root `.doug/ACTIVE_TASK.md` after processing so stale briefings do not linger between runs while archived session and runtime snapshots remain available.

### Fixed

### Removed

## [0.6.5]

### Added
- Added a KB article for doug scaffold and updated README to document the manifest-driven init -> scaffold -> run flow.
- Tightened the scaffold skill template so scaffold tasks explicitly read manifest context from ACTIVE_TASK.md, create stack-appropriate minimum project definition files, run package-manager install as the final step, and only report SUCCESS after install completes cleanly.
- Wire doug scaffold to invoke the agent once and dispatch success/failure through the existing handlers without touching the real project state files.
- Implemented synthetic scaffold task construction, manifest context injection into ACTIVE_TASK.md, and scaffold skill resolution through the existing agent path.
- Add the doug scaffold command shell with init/manifest precondition guards and tests.
- Added manifest v1 typed structs and validated loader, plus a derived orchestrator manifest path.

### Changed
- Updated KB and README docs to reflect scaffold as a synthetic task and documented the newer ACTIVE_TASK context/manifest path behavior.
- Moved manifest build system resolution logic from `cmd/scaffold.go` into `internal/config` as `ResolveManifestBuildSystem`; the mapping now consults the `BuildSystems` registry directly instead of a hardcoded string set.

## [0.6.4]

### Added
- Updated README Quick Start and doug init docs to describe the interactive prompt flow; flags documented as CI/scripted path; removed manual doug.yaml editing step
- Improve doug run per-iteration output: iteration header shows [taskID] attempt N/M (type), heartbeat lines shortened to [taskID] +elapsed, outcome line includes changelog entry summary
- Improved `doug init` terminal output: startup header, per-file ✓ checkmarks with relative paths, and structured 3-step next-steps completion summary.
- feat: EPIC-15-003 — doug init now prompts for build system (with auto-detect as default), max_retries, max_iterations, and kb_enabled on TTY; --build-system flag bypasses the build system prompt; generated doug.yaml reflects all selections
- feat: write selected agent_command into doug.yaml during doug init
- feat: add internal/prompt package with SelectOne, Confirm, and Text helpers

### Changed
- Updated KB articles for EPIC-15 interactive init and run logging improvements

## [0.6.3]

### Added
- `doug init` now writes `DOUG_PROJECT_ID` and `DOUG_PROJECT_NAME` into the managed AGENTS.md block; the ID is generated once (slugified dir name + 6-char random hex suffix) and preserved on all subsequent re-inits, including `--force`
- Add integration smoke tests for BUG→bugfix→SUCCESS, FAILURE→retry→BLOCKED, and SUCCESS→build-fail retry paths
- Add Build() error-path tests for NpmBuildSystem and PnpmBuildSystem, NewBuildSystem("static") factory coverage, and agentRegistry placeholder validation
- Fill handler unit test gaps: add coverage for HandleSuccess changelog writes and SaveProjectState failure paths in HandleBug and HandleFailure
- Add `docs/testing-strategy.md` with a package-by-package test audit, current strengths, misleading tests, and prioritized coverage gaps
- Add shared `internal/testutil.WriteFile` helper and reuse it across test packages to remove duplicate filesystem setup helpers

### Changed
- docs: update testing-strategy.md and add internal/testutil KB article to reflect EPIC-14 completion
- Rename the misleading orchestrator dependency test to reflect that it validates a happy-path configuration instead of proving git is always required
- Refactor affected tests to use the shared `testutil.WriteFile` helper for clearer, more consistent setup

## [0.6.2]

### Added
- Apply low-severity cleanups: max() for log tail, single-pass task scan, remove double-logging before pauseProject, explain 3-clause loop, EnsureProjectReady accepts string
- Consolidate protected-path and git-clean-exclude literals into git.DefaultProtectedPaths and git.defaultCleanExcludes, removing the duplicated .doug/ literals from handlers/success.go
- metrics.PrintEpicSummary now accepts an io.Writer as its first parameter, enabling testable output and caller-controlled destinations
- Changelog writes now use atomic write pattern to prevent partial file corruption
- Add fmt.Errorf context wrapping to bare error returns in internal packages
- Route all log output in internal/metrics to stderr instead of stdout

### Changed
- Update KB articles to reflect EPIC-13 hardening changes

## [0.6.1]

### Added
- Integration smoke test verifies doug init and doug run reach agent invocation without panics
- agent.RunAgent now accepts context.Context; cancelling the context kills the subprocess and returns promptly
- Orchestrator.Run now checks ctx.Done() at each iteration for clean cancellation; cmd/run.go reduced to 46 lines by extracting loadConfig into cmd/config.go
- Move pre-loop orchestration setup into Orchestrator.Run; cmd/run.go reduced to flag parsing and construction
- Eliminate post-construction LoopContext mutations: AgentDurationSeconds and SessionResult are now passed explicitly to handlers
- Introduce Orchestrator struct and Paths type in internal/orchestrator
- Introduce Logger interface in internal/log with stderr default; thread through orchestrator, handlers, and agent
- doug init now appends a clearly delimited Doug-specific instructions block to AGENTS.md when the marker is absent, preserving any existing project-authored AGENTS content

### Changed
- docs: update KB after orchestrator refactor — Logger interface, Orchestrator struct, Paths, LoopContext move to types, context.Context on RunAgent, ValidateTaskTypes
- Move doug-specific agent policy into AGENTS.md and make the default feature, bugfix, documentation, and research skill templates task-generic
- Update init scaffolding, skills-config guidance, and KB docs to document the AGENTS-first instruction split and the progressive disclosure path of ACTIVE_TASK.md → PRD.md → docs/kb/README.md

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
