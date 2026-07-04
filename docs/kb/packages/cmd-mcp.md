---
title: cmd/mcp — Local Stdio MCP Server
updated: 2026-07-03
category: Packages
tags: [cmd, mcp, interactive, json-rpc, stdio]
related_articles:
  - docs/kb/features/interactive-implement.md
  - docs/kb/packages/mcp.md
  - docs/kb/packages/config.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/features/cli-discoverability.md
---

# cmd/mcp — Local Stdio MCP Server

## Overview

`cmd/mcp.go` implements the `doug mcp` command. It serves Doug's interactive Implement tools over local stdio using MCP-style JSON-RPC framing.

The command layer is intentionally thin:

1. resolve the current working directory as the project root
2. load and validate `.doug/doug.yaml`
3. construct `internal/mcp.ToolHandler`
4. read/write framed JSON-RPC messages on stdin/stdout

Lifecycle semantics live in `internal/mcp`, not in the Cobra command. Invalid config is rejected before the stdio server begins serving frames, matching the validation behavior of other Doug command paths and avoiding a half-started lifecycle tool surface.

## Command

```text
doug mcp
```

The command starts a local stdio server. It does not claim work by itself; clients call MCP tools after initialization.

## Supported RPC Methods

| Method | Behavior |
|--------|----------|
| `initialize` | Returns protocol/server metadata and tool capability declaration. |
| `tools/list` | Lists Doug lifecycle tools exposed by `internal/mcp.ToolNames()`. |
| `tools/call` | Dispatches to `get_status`, `diagnose_lifecycle`, `reconcile_lifecycle`, `get_next_task`, `report_task_complete`, or `report_task_blocked`. |

Unsupported methods return JSON-RPC error `-32000`.

## Framing

`readMCPFrame` expects `Content-Length` headers followed by a JSON payload. `writeMCPFrame` writes the same framing and JSON-encodes the response.

The server ignores notifications by not writing a response when the request has no `id`.

## Tool Arguments

`reconcile_lifecycle` accepts a string `mode` argument. Use `mode: "repair"` for the only supported call path; omitted or other modes return an unsupported-mode JSON-RPC error before applying any lifecycle changes. Supported but ambiguous drift still returns manual-review information without changing files.

`report_task_complete` and `report_task_blocked` accept an optional string `task_id` argument. When omitted, `internal/mcp` uses the active task from `project-state.yaml` and still validates that it matches Doug's active assignment.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [internal/mcp](mcp.md)
- [internal/config](config.md)
- [internal/orchestrator](orchestrator.md)
