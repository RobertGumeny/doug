# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
- Synthesized EPIC-2 session logs into three new KB articles (internal/log, internal/build, internal/git), created KB index README, and corrected stale content in infrastructure/go.md
- Synthesized EPIC-1 session logs into three new KB articles (internal/types, internal/state, internal/config) and updated infrastructure/go.md with module path and cross-references

### Fixed

### Removed

