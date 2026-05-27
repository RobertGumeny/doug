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

- `doug init` sets up Doug and Pi-facing repo scaffolding.
- `doug run` is the main command: Doug writes the brief, then executes the task through Pi RPC and validates the result.
- `doug run EPIC-ID` promotes a planned epic into runtime and executes it through the same Pi-backed path.

## Optional Workflows

- `doug plan` launches a true interactive Pi planning session against `.doug/ACTIVE_TASK.md` and `.doug/plan/PLAN.md`.
- `doug handoff` packages approved plan output into execution-ready epics.
- `doug research` runs a one-shot Pi RPC read-only analysis pass and saves the report under `.doug/logs/research/`.
- `doug scaffold` runs a one-shot Pi RPC scaffold pass from a generated manifest.
- `doug upgrade` refreshes Doug-managed setup in an existing repo.

## Commands

```text
doug run [EPIC-ID]
doug init
doug plan
doug handoff
doug research [topic...]
doug scaffold
doug revert <task_id>
doug upgrade [--dry-run] [--force]
```

## Configuration

Main config lives in `.doug/doug.yaml`, but most users should not need to edit it directly. `doug init` walks you through the normal setup interactively and writes the config for you.

Doug always uses Pi: `doug plan` launches true interactive Pi, while runtime, scaffold, research, and post-epic KB passes use Pi RPC one-shot execution.

More detail:

- [Execution model details](docs/kb/features/execution-model.md)
- [Upgrade workflow details](docs/kb/features/upgrade.md)

## Knowledge Base

`docs/kb/` is a shared reference layer for humans and agents. Keep durable project knowledge there so later runs can reuse it.

When `kb_enabled` is true, Doug also runs a dedicated post-epic KB documentation pass through Pi RPC at the end of each epic to update and maintain the KB automatically.

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
