# doug

[![CI](https://github.com/robertgumeny/doug/actions/workflows/ci.yml/badge.svg)](https://github.com/robertgumeny/doug/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/robertgumeny/doug/graph/badge.svg)](https://codecov.io/gh/robertgumeny/doug)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/robertgumeny/doug/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)

`doug` is a CLI orchestrator for AI coding agents. It scaffolds a repo, keeps orchestration state under `.doug/`, invokes an agent with task-specific instructions, verifies the result, updates project state, and records the work in `CHANGELOG.md`.

The current CLI supports `init`, `run`, `switch`, and `revert`, with built-in agent presets for Claude, Codex, and Gemini.

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
doug init --agents claude
```

Then:

1. Edit `AGENTS.md` — fill in your project name and tech stack; this is what every agent reads before starting a task
2. Edit `.doug/PRD.md`
3. Edit `.doug/tasks.yaml`
4. Review `.doug/doug.yaml`
5. Run `doug run`

Typical scaffolded layout:

```text
.
├── .agents/skills/
├── .claude/                     # if selected during init
├── .codex/                      # if selected during init
├── .gemini/                     # if selected during init
├── .doug/
│   ├── ACTIVE_TASK.md
│   ├── PRD.md
│   ├── doug.yaml
│   ├── project-state.yaml
│   ├── skills-config.yaml
│   ├── tasks.yaml
│   └── logs/
│       ├── sessions/{epic}/   # ACTIVE_TASK.md archives (KB source)
│       ├── bugs/{epic}/       # bug report archives
│       ├── failures/{epic}/   # failure report archives
│       └── output/{epic}/     # raw agent stdout/stderr logs
├── AGENTS.md
├── CHANGELOG.md
├── CLAUDE.md
└── docs/kb/
```

`doug init` always scaffolds shared skills into `.agents/skills/`. Per-agent settings are created only for agents you select.

## Commands

```text
doug init
doug run
doug switch [agent]
doug revert <task_id>
doug completion [bash|zsh|fish|powershell]
```

### `doug init`

Initializes a project with:

- `.doug/doug.yaml`
- `.doug/project-state.yaml`
- `.doug/tasks.yaml`
- `.doug/PRD.md`
- `.doug/skills-config.yaml`
- `.agents/skills/...`
- `AGENTS.md`
- `CLAUDE.md`
- `CHANGELOG.md`
- `docs/kb/`
- selected agent settings such as `.claude/settings.json`, `.codex/config.toml`, and `.gemini/policies/doug-default.json`

After init, open `AGENTS.md` and replace the `[Project Name]` and tech stack placeholders with a one- or two-sentence description of your project. Agents read this file before every task — it's the fastest way to give them accurate project context without duplicating your PRD.

**Build system auto-detection**: `doug init` reads marker files (`go.mod`, `pnpm-workspace.yaml`, `package.json`, `index.html`) to detect the build system. If none are found and you selected claude as your agent, the CLI prompts interactively. The detected build system determines which Bash permissions are injected into `.claude/settings.json` (scoped to your toolchain, not a blanket allow-all list).

Flags:

- `--agents string` comma-separated agent list, for example `claude,codex`
- `--build-system string` override auto-detection: `go|npm|pnpm|static`
- `--force` overwrite existing scaffolded files
- `--no-git-init` skip running `git init` after scaffolding

### `doug run`

Runs the orchestration loop against `.doug/tasks.yaml`.

High-level flow:

1. Load `.doug/doug.yaml`
2. Verify dependencies and toolchain
3. Load `.doug/project-state.yaml` and `.doug/tasks.yaml`
4. Bootstrap or roll over epic state
5. Run pre-flight build and test checks
6. Ensure the epic branch is checked out
7. Write `.doug/ACTIVE_TASK.md`
8. Create a session result file under `.doug/logs/sessions/{epic}/`
9. Invoke the configured agent command
10. Parse the session result and dispatch `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`

Flags:

- `--agent string`
- `--agent-heartbeat-seconds int`
- `--build-system string`
- `--kb-enabled`
- `--max-iterations int`
- `--max-retries int`

### `doug switch [agent]`

Updates `.doug/doug.yaml` to use a supported preset agent command.

Supported agents:

- `claude`
- `codex`
- `gemini`

Use `doug switch --list` to print the list from the current binary.

### `doug revert <task_id>`

Rewinds the repo to the commit boundary recorded for a completed task.

Behavior:

- accepts only `DONE` tasks from `.doug/tasks.yaml`
- looks up the commit SHA from task metrics, with a `git log --grep` fallback
- checks for dirty working tree state unless `--force` is used
- runs `git reset --hard <sha>`
- deletes session logs for tasks after the revert point, plus `KB_UPDATE`
- warns when a remote tracking branch means a force-push is required

Flag:

- `--force` skip dirty-tree validation and confirmation prompt

## Configuration

Main config lives in `.doug/doug.yaml`.

Scaffolded example:

```yaml
agent_command: 'claude -p "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"'
# agent_command: codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"
# agent_command: gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"
build_system: go
max_retries: 3
max_iterations: 10
kb_enabled: true
agent_heartbeat_seconds: 30
```

Fields:

- `agent_command`: command template used to invoke the agent
- `build_system`: `go`, `npm`, `pnpm`, or `static` (no-op for plain HTML/CSS/JS projects)
- `max_retries`: max `FAILURE` outcomes before a task becomes `BLOCKED`
- `max_iterations`: max orchestration loop iterations before `doug run` exits
- `kb_enabled`: inject a documentation synthesis task after feature work completes
- `agent_heartbeat_seconds`: periodic liveness logging while the agent runs; `0` disables it

Skill mapping lives in `.doug/skills-config.yaml`. Shared skill files live in `.agents/skills/`.

## Skills

Doug bundles four skills out of the box:

| Skill | Task type | Output | Notes |
|-------|-----------|--------|-------|
| `implement-feature` | `feature` | Code + session result | Standard feature implementation workflow |
| `implement-bugfix` | `bugfix` | Code + session result | Root cause analysis, fix, regression test |
| `implement-documentation` | `documentation` | `docs/kb/` articles | Synthesizes session logs into KB; can also be pointed at a specific feature or file manually |
| `research` | `research` | `RESEARCH_REPORT.md` at project root | Read-only codebase analysis; point at a feature, module, file, or the full codebase; does not modify code |

## Tasks

Tasks are defined in `.doug/tasks.yaml`.

```yaml
epic:
  id: "EPIC-1"
  name: "First Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement the first feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
```

Supported task types:

External:
- `feature`

Internal*:
- `bugfix`
- `documentation`
- `research`
- `manual_review`

*Right now, the user should set all task types in their `tasks.yaml` to `feature`, doug will handle creating the internal tasks. 


Supported statuses:

- `TODO`
- `IN_PROGRESS`
- `DONE`
- `BLOCKED`

## Agent Contract

Before each iteration, `doug` writes `.doug/ACTIVE_TASK.md` with:

- task ID, type, and attempt count
- bug and failure report paths
- the `.doug/PRD.md` path
- acceptance criteria for user-defined tasks
- an `## Agent Result` YAML stub the agent must fill in

Agents report back by writing YAML frontmatter directly into the `## Agent Result` block in `.doug/ACTIVE_TASK.md`:

```yaml
---
outcome: "SUCCESS"
changelog_entry: "Brief user-facing description of the change"
dependencies_added: []
---
```

Valid outcomes:

- `SUCCESS`
- `FAILURE`
- `BUG`
- `EPIC_COMPLETE`

The orchestrator owns Git operations, YAML state updates, changelog updates, and log archival. Agents are expected to write code, tests, and their session result only.

## Knowledge Base

`docs/kb/` is a living knowledge base — articles about patterns, decisions, and lessons learned — shared between humans and agents. `AGENTS.md` instructs every agent to check `docs/kb/` before starting work, so articles written during one loop become context for every subsequent loop.

Key points:

- **Selective loading via frontmatter**: Every KB article carries YAML frontmatter with `title`, `category`, `tags`, and `related_articles` fields. Agents can scan these fields cheaply — without reading article bodies — and load only the articles relevant to their current task. This keeps context lean as the KB grows.
- **Automatic growth**: `kb_enabled: true` (the default) causes doug to inject a `documentation` task at the end of each epic. That task runs the `implement-documentation` skill, which synthesizes session logs into new or updated KB articles.
- **Manual updates**: Add a `documentation` task to `tasks.yaml` at any time to trigger a targeted KB update — for example, "Document the authentication module" or "Update KB after the storage refactor." Point it at a feature, module, or file and the agent will produce or update the relevant articles.
- **Human updates**: You can add or edit KB articles directly at any time — after a manual refactor, a design decision, or a code review — and the next agent will pick them up automatically.
- **Compounding benefit**: Early agents document the patterns they establish; later agents read those patterns and produce more consistent work without rediscovering them. The KB is what makes agent output compound across loops rather than restart from zero.

Notable articles:

- [docs/kb/README.md](docs/kb/README.md)
- [docs/kb/packages/init.md](docs/kb/packages/init.md)
- [docs/kb/packages/agent.md](docs/kb/packages/agent.md)
- [docs/kb/packages/changelog.md](docs/kb/packages/changelog.md)
- [docs/kb/features/revert.md](docs/kb/features/revert.md)

## Platform Notes

Linux and macOS are supported directly.

Windows native is not supported for agent execution because the Bash tool is unavailable there. Use WSL2 instead:

1. Install WSL2 and a Linux distribution
2. Install your agent CLI, `git`, and language toolchain inside WSL2
3. Run `doug init` and `doug run` from the WSL2 filesystem
