# doug

[![CI](https://github.com/robertgumeny/doug/actions/workflows/ci.yml/badge.svg)](https://github.com/robertgumeny/doug/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/robertgumeny/doug/graph/badge.svg)](https://codecov.io/gh/robertgumeny/doug)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/robertgumeny/doug/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)

`doug` runs coding-agent implementation work inside your repo and validates the result deterministically before recording the outcome. Pi is Doug's exclusive agent harness.

The core loop is simple:

1. define the work
2. run the agent in the repo
3. validate the result
4. review a recorded outcome

`plan`, `handoff`, and `scaffold` are available when you want more structure, but the main story is still `implement + validate`.

## Install

### From a release binary

Download the latest archive from the [releases page](https://github.com/robertgumeny/doug/releases), extract it, and move `doug` onto your `PATH`.

```bash
# macOS arm64
curl -L https://github.com/robertgumeny/doug/releases/download/vVERSION/doug_VERSION_darwin_arm64.tar.gz | tar xz
sudo mv doug /usr/local/bin/

# Linux amd64
curl -L https://github.com/robertgumeny/doug/releases/download/vVERSION/doug_VERSION_linux_amd64.tar.gz | tar xz
sudo mv doug /usr/local/bin/
```

### With Go

Requires Go 1.26.

If Go is not installed yet, use the official installer for your platform from `https://go.dev/dl/`, then verify it:

```bash
go version
```

```bash
go install github.com/robertgumeny/doug@latest
```

If needed:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Build from source

```bash
make build
./bin/doug
```

## Quick Start

```bash
mkdir my-project
cd my-project
doug init
```

`doug init` sets up the repo so you can start running work.

Then:

1. Edit `AGENTS.md` with the project name, stack, and any repo guidance every agent should see first.
2. Add or refine the work in `.doug/PRD.md` and `.doug/tasks.yaml`.
3. Run `doug run`.

```bash
doug run
```

If you want a more structured planning flow first:

```bash
doug plan "Add X"
doug handoff
doug run EPIC-1
```

Use `doug scaffold` only for optional greenfield bootstrap work.

## Core Workflow

- `doug init` sets up Doug and Pi-facing repo scaffolding, including the six namespaced built-in skills at `.agents/skills/doug-*/`. Claude receives them through a managed `.claude/skills` bridge (or managed copies when a user-owned Claude skills directory must remain); Pi's local-project trust requirement is unchanged.
- `doug run` is the main headless Implement command: Doug writes the brief, executes the task through Pi RPC, validates the result, and advances lifecycle state.
- `doug run EPIC-ID` promotes a planned epic into runtime and executes it through the same Pi-backed path.
- `doug mcp` starts the MCP-first interactive Implement surface for already-active agent sessions; use its lifecycle tools instead of editing `.doug/project-state.yaml` or `.doug/tasks.yaml`. See [Connecting an agent session](#connecting-an-agent-session) to wire up a client.

## Optional Workflows

- `doug plan` launches a true interactive Pi planning session against `.doug/ACTIVE_TASK.md` and `.doug/plan/PLAN.md`; it also surfaces unresolved reported bugs from Doug-managed intake.
- `doug handoff` packages approved plan output into execution-ready epics.
- `doug research` runs a one-shot Pi RPC read-only analysis pass and saves the report under `.doug/intake/research/`.
- Bugs found outside scheduled implementation can be investigated with `doug research` and converted into scoped work through `doug plan`; Doug does not provide a separate `doug bug` command or ask users to maintain hand-written ledger files.
- `doug review EPIC-ID` reruns the advisory post-epic review for a completed archive and writes under `.doug/logs/epics/{epic}/`.
- `doug stats [EPIC-ID]` summarizes local run statistics from `.doug/logs/epics/`.
- `doug scaffold` runs a one-shot Pi RPC scaffold pass from a generated manifest.
- `doug upgrade` refreshes Doug-managed setup in an existing repo.

## Commands

```text
doug run [EPIC-ID]
doug init
doug plan
doug handoff
doug research [topic...]
doug review <EPIC-ID>
doug stats [EPIC-ID]
doug mcp
doug scaffold
doug revert <task_id>
doug upgrade [--dry-run] [--force]
```

## Connecting an agent session

`doug mcp` is a stdio MCP server: a client starts it, so you do not run it yourself in a terminal. Point your agent at it with an `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "doug": {
      "command": "doug",
      "args": ["mcp"]
    }
  }
}
```

`doug` must be on the `PATH` your client launches with, and the server reads `.doug/doug.yaml` from the working directory it starts in, so run the client from the project root. Invalid config fails immediately rather than serving unusable lifecycle tools.

To verify the connection without a client, pipe a request in directly:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_status","arguments":{}}}' | doug mcp
```

A working server answers with a `result` containing a `content` array. No output at all means the server is not speaking to you — check that `doug mcp` runs from the project root and that config loads.

Once connected, keep the MCP session as a thin dispatcher and hand each `.doug/ACTIVE_TASK.md` brief to a fresh worker context. See [Interactive Implement details](docs/kb/features/interactive-implement.md) for the full lifecycle contract.

## Configuration

Main config lives in `.doug/doug.yaml`, but most users should not need to edit it directly. `doug init` walks you through the normal setup interactively and writes the config for you.

Doug always uses Pi: `doug plan` launches true interactive Pi, while headless Implement (`doug run`), scaffold, research, post-epic review, and post-epic KB/changelog passes use Pi RPC one-shot execution. Interactive Implement is MCP-first through `doug mcp`; Doug still owns lifecycle changes, so `.doug/project-state.yaml` and `.doug/tasks.yaml` are not external write APIs.

Useful config fields include `review_enabled` (default `true`) to run the advisory post-epic review automatically, and `kb_enabled` (default `true`) to run post-epic KB/changelog synthesis. Review artifacts are written under `.doug/logs/epics/{epic}/`; the review is advisory/non-gating and runs before KB/changelog polish.

More detail:

- [Execution model details](docs/kb/features/execution-model.md)
- [Interactive Implement details](docs/kb/features/interactive-implement.md)
- [Upgrade workflow details](docs/kb/features/upgrade.md)

## Knowledge Base

`docs/kb/` is a shared reference layer for humans and agents. Keep durable project knowledge there so later runs can reuse it.

When `kb_enabled` is true, Doug also runs a dedicated post-epic KB documentation pass through Pi RPC at the end of each epic to update and maintain the KB automatically and polish `[Unreleased]` changelog prose without changing facts. When `review_enabled` is true, Doug first runs an advisory post-epic review and writes `.doug/logs/epics/{epic}/epic-review.md`; warnings do not block epic completion.

Start here:

- [docs/kb/README.md](docs/kb/README.md)
- [Planning lifecycle details](docs/kb/features/planning-lifecycle.md)
- [Research workflow details](docs/kb/features/research.md)
- [Scaffold workflow details](docs/kb/features/scaffold.md)

## Platform Notes

Linux and macOS are supported directly.

Windows native is not supported for agent execution because the Bash tool is unavailable there. Use WSL2 instead:

1. Install WSL2 and a Linux distribution
2. Install your agent CLI, `git`, and language toolchain inside WSL2
3. Run `doug init` and `doug run` from the WSL2 filesystem
