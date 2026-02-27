# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Removed

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
