---
title: cmd/mcp — Local Stdio MCP Server
updated: 2026-08-09
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
| `tools/list` | Lists Doug lifecycle tools with names, human-readable descriptions, and JSON object input schemas from `internal/mcp.ToolDefinitions()`. |
| `tools/call` | Dispatches to `get_status`, `diagnose_lifecycle`, `reconcile_lifecycle`, `get_next_task`, `report_task_complete`, or `report_task_blocked`. |

Unsupported methods return JSON-RPC error `-32000`.

## Framing

MCP's stdio transport is newline-delimited JSON: one message per line, no embedded newlines, and no framing headers. `readMCPFrame` reads a single line and trims it; `writeMCPFrame` uses `json.Encoder.Encode`, which emits compact JSON plus a newline.

Two edge cases are deliberate: blank lines are skipped so client padding does not raise a parse error, and a final message arriving without a trailing newline is still answered rather than dropped when its read returns `io.EOF` alongside the data.

This transport is **not** LSP. An earlier implementation expected `Content-Length` headers, which made the server unusable against every conforming client: `strings.Cut` succeeds on the colons inside a JSON line, so each message was consumed as an unrecognized header, no length was ever found, stdin hit EOF, and the server exited 0 without responding or logging. Do not reintroduce header framing.

The server ignores notifications by not writing a response when the request has no `id`.

## Response Shape

`tools/call` results must use MCP's `CallToolResult` shape, not the handler's return value directly:

```json
{"content": [{"type": "text", "text": "<JSON-encoded handler result>"}], "isError": false}
```

`toolCallResult` performs this wrapping and is the only place tool payloads are encoded. Returning a bare domain struct as `result` produces a response that is valid JSON-RPC and passes envelope-level assertions, but clients read `result.content` and render it as empty output — the tool appears to do nothing.

Errors split by kind:

| Condition | Response |
|-----------|----------|
| Handler returned an error | `isError: true` with the message as text content |
| Unknown tool name (`errUnknownTool`) | JSON-RPC error `-32000` |
| Unsupported method, malformed params | JSON-RPC error `-32000` |
| Unparseable request line | JSON-RPC error `-32700` |

The split is intentional. A handler failure is an outcome the calling agent can act on — claim a different task, run reconcile, surface it to the operator — so it belongs in the result as readable content. A JSON-RPC error means the protocol itself faulted and the caller can only give up; reserve it for faults, not for lifecycle outcomes.

Tests for this surface must assert on decoded `result.content[0].text`, not merely on response count or non-empty output. Envelope-only assertions are how both the framing defect and the bare-struct defect reached `main`.

## Tool Arguments

`reconcile_lifecycle` accepts a string `mode` argument. Use `mode: "repair"` for the only supported call path; omitted or other modes return an unsupported-mode JSON-RPC error before applying any lifecycle changes. Supported but ambiguous drift still returns manual-review information without changing files.

`report_task_complete` and `report_task_blocked` accept an optional string `task_id` argument. When omitted, `internal/mcp` uses the active task from `project-state.yaml` and still validates that it matches Doug's active assignment.

`allowed_next_actions` remains a backward-compatible array of strings. Entries use one simple action grammar: a snake_case action token such as `get_next_task` or `manual_review`, optionally followed by parenthesized key/value guidance such as `reconcile_lifecycle(mode=repair)`.

## Related Topics

- [Interactive Implement MCP Surface](../features/interactive-implement.md)
- [internal/mcp](mcp.md)
- [internal/config](config.md)
- [internal/orchestrator](orchestrator.md)
