---
title: CLI Discoverability And Config Diagnostics
updated: 2026-07-04
category: Features
tags: [cli, init, config, diagnostics, help, mcp]
related_articles:
  - docs/kb/packages/init.md
  - docs/kb/packages/config.md
  - docs/kb/packages/state.md
  - docs/kb/packages/cmd-mcp.md
  - docs/kb/features/module-root.md
  - docs/kb/features/post-epic-finalization.md
---

# CLI Discoverability And Config Diagnostics

## Overview

Doug's first-run and command-discovery surfaces are designed to help operators recover without reading source code. The main surfaces are:

- generated `.doug/doug.yaml` comments from `doug init`
- `doug init` prompt labels and completion epilogue
- Cobra `Long` descriptions, examples, and flag help for workflow commands
- structured parse diagnostics from `.doug/doug.yaml` and `.doug/project-state.yaml`
- early `.doug/doug.yaml` validation before `doug mcp` starts its stdio server

These surfaces should stay representative and workflow-focused. Avoid turning command help into a full manual; link or point users to the next concrete command or file they should inspect.

## First-Run Config Discoverability

`doug init` writes a small `.doug/doug.yaml` that exposes only project/runtime settings. The generated config includes:

- `build_system` with supported values `go | npm | pnpm | static`
- `module_root: ""` as a discoverable optional build-system subdirectory
- retry and iteration limits with inline numeric bounds
- post-epic `kb_enabled` and `review_enabled` toggles
- heartbeat, first-response, and lint settings

`module_root` is generated as an empty string so existing repo-root behavior remains unchanged. See [cmd/init](../packages/init.md) for generated-file details and [Build-System Module Root](module-root.md) for runtime semantics.

## Init Prompts And Epilogue

Interactive `doug init` prompts include a one-line explanation in the prompt label so users know what each default controls:

- `max_retries` controls task failure retries before a task is blocked
- `max_iterations` caps orchestrator loop iterations
- `kb_enabled` controls post-epic KB synthesis

When init finishes, the epilogue points users to three supported next steps:

1. read `.doug/README.md`
2. run `doug plan`, then `doug handoff`
3. or manually edit `.doug/PRD.md` and `.doug/tasks.yaml`, then run `doug run`

The selected non-interactive init defaults are source-owned constants in `cmd/init_workflow.go` and are covered by tests.

## Command Help Expectations

Commands that are common entry points or previously under-described should have concise Cobra `Long` text and examples that explain the workflow outcome, not every implementation detail.

Current notable examples:

| Command | Help expectation |
|---------|------------------|
| `doug run [EPIC-ID]` | Explains the implementation loop, optional planned-epic loading, and includes examples including `--build-system static`. |
| `doug mcp` | Explains local stdio MCP lifecycle tooling and states that config is loaded and validated before serving. |
| `doug review <EPIC-ID>` | Explains advisory review reruns against archived completed epics without reopening implementation work. |
| `doug stats [epic_id]` | Explains local telemetry summary and optional epic narrowing. |
| `doug init --build-system` and `doug run --build-system` | Flag help lists `go|npm|pnpm|static`. |

Tests in `cmd/help_test.go` assert representative strings so future help edits preserve discoverability without requiring brittle full-output snapshots.

## Parse Diagnostics

Config and state parsing return structured errors that can be rendered directly to operators.

`.doug/doug.yaml` parse failures use `config.ParseError` with:

- `Path` for the bad file
- `Fields` from `*yaml.TypeError` when the YAML parser reports field-level type mismatches
- a hint describing scalar values, integer retry limits, boolean KB/review flags, and stale-field recovery

`.doug/project-state.yaml` parse failures use `state.ParseError` with:

- `Path` for the bad file
- `Fields` from `*yaml.TypeError` when available
- a project-state-specific hint naming expected blocks and common type expectations

`execution_mode` is a retired top-level config field. `LoadConfig` rejects it with `config.ParseError` and guidance to remove the field or run `doug upgrade`.

## MCP Startup Validation

`doug mcp` loads and validates `.doug/doug.yaml` before serving JSON-RPC frames. This intentionally fails early for invalid local config instead of starting a stdio server that cannot safely operate on the project.

## Related Topics

- [cmd/init](../packages/init.md) — generated config, prompts, and first-run epilogue
- [internal/config](../packages/config.md) — config parsing, defaults, validation, and stale-field diagnostics
- [internal/state](../packages/state.md) — project-state and tasks parse diagnostics
- [cmd/mcp](../packages/cmd-mcp.md) — stdio server startup and config validation
