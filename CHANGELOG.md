# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Removed

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
