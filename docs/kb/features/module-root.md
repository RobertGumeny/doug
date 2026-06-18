---
title: Build-System Module Root
author: doug KB
updated: 2026-06-16
category: Features
tags: [build-system, module-root, go, config, preflight]
related_articles:
  - docs/kb/packages/config.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/build.md
  - docs/kb/infrastructure/go.md
---

# Build-System Module Root

## Overview

Doug supports projects where the orchestrated module is not at the repository root. Configure `.doug/doug.yaml` with optional `module_root` to move the build-system working directory while keeping all `.doug/` runtime files anchored at the project root.

```yaml
build_system: go
module_root: engine
```

With this configuration, Doug still reads and writes `.doug/PRD.md`, `.doug/tasks.yaml`, `.doug/project-state.yaml`, active handoff files, logs, and archives from the repository root. Only build-system operations are rooted at `<ProjectRoot>/engine`.

## Construction Contract

The module-root anchor is applied once in `internal/orchestrator.New`:

```go
modulePath := filepath.Join(paths.ProjectRoot, cfg.ModuleRoot)
buildSys, err := build.NewBuildSystem(cfg.BuildSystem, modulePath)
```

Do not add per-command `module_root` plumbing inside `internal/build`. Build implementations should continue to run commands in the root they receive from `NewBuildSystem`.

Important properties:

- omitted `module_root` defaults to `""`
- `filepath.Join(paths.ProjectRoot, "")` preserves the previous root behavior
- all build systems inherit the resolved anchor because the root is passed through the shared factory
- `.doug/` path derivation remains owned by `orchestrator.Paths` and is unaffected by `module_root`

## Go Initialization Sentinel

`GoBuildSystem.IsInitialized()` checks for `go.mod` in the resolved build root. `go.sum` is not an initialization sentinel: valid Go modules with no external dependencies may have `go.mod` and no `go.sum`, and Doug should still run pre-flight build/test checks.

For a subdirectory Go module, this means:

```text
repo/
├── .doug/
└── engine/
    └── go.mod
```

with `module_root: engine`, `IsInitialized()` returns true because Doug probes `repo/engine/go.mod`.

## Missing Module Warning

When `module_root` is non-empty, `orchestrator.New` checks whether `<ProjectRoot>/<module_root>/go.mod` exists. If not, Doug logs a non-fatal warning naming the configured module root and resolved module path, then continues.

This catches likely Go subdirectory-module misconfiguration early without changing terminal behavior:

- empty `module_root`: no warning
- non-empty `module_root` with a `go.mod`: no warning
- non-empty `module_root` without a `go.mod`: warning only, then normal startup continues

`EnsureProjectReady` may still emit its usual “project is not initialized” warning if the selected build system reports `IsInitialized() == false`.

## Unsupported Scope

Doug still targets one build root per project. Multiple independent modules, per-task module selection, relocating `.doug/` under the module directory, and `go.work`-based orchestration are out of scope for the current contract.

## Related

- [internal/config](../packages/config.md) — `module_root` parsing and defaults
- [internal/orchestrator](../packages/orchestrator.md) — single construction site and startup warning
- [internal/build](../packages/build.md) — build-system root contract and Go sentinel
- [Go Infrastructure](../infrastructure/go.md) — Go module and `go.sum` guidance
