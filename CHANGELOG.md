# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added internal log package with Info, Success, Warning, Error, Fatal, and Section functions using ANSI color codes
- Added OrchestratorConfig with sane defaults, LoadConfig with partial-file support, and DetectBuildSystem for go/npm detection
- Added atomic state I/O package with LoadProjectState, SaveProjectState, LoadTasks, and SaveTasks
- Added core type definitions for the doug orchestrator with full YAML round-trip support
- Verified project scaffold is correct and production-ready; updated go.mod to Go 1.26 per project standard

### Changed
- Synthesized EPIC-1 session logs into three new KB articles (internal/types, internal/state, internal/config) and updated infrastructure/go.md with module path and cross-references

### Fixed

### Removed

