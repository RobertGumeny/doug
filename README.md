# doug

[![CI](https://github.com/robertgumeny/doug/actions/workflows/ci.yml/badge.svg)](https://github.com/robertgumeny/doug/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/robertgumeny/doug/graph/badge.svg)](https://codecov.io/gh/robertgumeny/doug)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/robertgumeny/doug/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)

`doug` is a CLI orchestrator for AI coding agents. It scaffolds a repo, keeps orchestration state under `.doug/`, can materialize a day-0 application scaffold from a manifest, emits Doug-owned prompts plus execution policy to Pi, verifies the result, updates project state, and records the work in `CHANGELOG.md`.

The current CLI supports `init`, `plan`, `handoff`, `scaffold`, `run`, `revert`, and `completion`.

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

`doug init` walks you through an interactive setup: build system and key config values (max retries, max iterations, KB enabled). Press Enter at each prompt to accept the default. The resulting `.doug/doug.yaml` is written from your choices, and Doug always emits Pi RPC execution policy for every workflow phase.

`doug init` does not create your app's project files. It creates doug control files, KB scaffolding, and Pi-side skill/helper scaffolding only. The actual day-0 app scaffold comes from `doug scaffold` after you provide a manifest.

Then:

1. Edit `AGENTS.md` — fill in your project name and tech stack; this is what every agent reads before starting a task
2. Choose a planning path:
   Edit root `.doug/PRD.md` and root `.doug/tasks.yaml` directly for the manual runtime path, or use `.doug/plan/PLAN.md` via `doug plan`
3. Run `doug handoff` when you want deterministic backlog epics and optional scaffold manifest output
4. Run `doug scaffold` when the plan is greenfield and `.doug/plan/manifest.yaml` was generated
5. Run `doug run [EPIC-ID]` to promote a backlog epic into runtime, or plain `doug run` when using the direct root-level runtime path

The root-level `.doug/PRD.md` and `.doug/tasks.yaml` workflow remains fully supported. Planning under `.doug/plan/` is an optional path that feeds the same runtime model rather than replacing direct root-level usage.

Typical scaffolded layout:

```text
.
├── .pi/
│   ├── extensions/
│   │   └── handoff.ts
│   └── skills/
├── .doug/
│   ├── ACTIVE_TASK.md
│   ├── PRD.md
│   ├── doug.yaml
│   ├── plan/
│   │   ├── PLAN.md
│   │   ├── history/
│   │   │   └── PLAN-{timestamp}.md
│   │   ├── manifest.yaml
│   │   └── epics/
│   │       └── {EPIC-ID}/
│   │           ├── PRD.md
│   │           ├── metadata.yaml
│   │           └── tasks.yaml
│   ├── project-state.yaml
│   ├── tasks.yaml
│   └── logs/
│       ├── archives/{epic}/   # final runtime snapshots on epic completion
│       ├── sessions/{epic}/   # ACTIVE_TASK.md archives (KB source)
│       ├── bugs/{epic}/       # bug report archives
│       ├── failures/{epic}/   # failure report archives
│       └── output/{epic}/     # raw agent stdout/stderr logs
├── AGENTS.md
├── CHANGELOG.md
├── CLAUDE.md
└── docs/kb/
```

`doug init` scaffolds skills under `.pi/skills/` and the optional Pi helper `.pi/extensions/handoff.ts`. Skill selection is configured via `policy.tasks[type].skill` in `.doug/doug.yaml`.

Doug no longer stores provider CLI templates in `.doug/doug.yaml`. Repo-owned prompts come from built-in Doug command text, while `.doug/doug.yaml` owns execution policy such as `policy.phases.*.execution_mode: rpc`. Once Pi is active, Pi chooses the underlying provider/model/tooling configuration; Doug does not select provider CLIs directly.

## Execution Model

Doug has one runtime model and one optional planning path into it:

- root `.doug/` is the live runtime workspace
- `.doug/plan/` is the optional planning and backlog workspace

You can work entirely in root `.doug/PRD.md` plus root `.doug/tasks.yaml` and run plain `doug run`, or you can use `doug plan` and `doug handoff` to materialize backlog epics before promotion with `doug run <EPIC-ID>`. Both paths converge on the same runtime loop; backlog promotion is not a second execution system.

Doug supports two backend transports:

- direct subprocess execution when `execution_mode` resolves to `subprocess`
- Pi RPC execution when `policy.phases.*.execution_mode` or `policy.tasks.*.execution_mode` resolves to `rpc`

`doug init` now emits `execution_mode: rpc` for every Doug workflow phase. In that path, Doug writes the canonical brief to `.doug/ACTIVE_TASK.md`, resolves run policy, and sends the built-in Doug prompt through the Pi adapter. Pi launches `pi --mode rpc`, owns provider/model/tool configuration, and keeps Doug's workflow semantics unchanged: `ACTIVE_TASK.md` remains the canonical brief, and `SUCCESS`, `FAILURE`, `BUG`, and `EPIC_COMPLETE` are still read from that file after the run.

The supported operator story is:

- `execution_mode` chooses the backend transport
- `.pi/extensions/` is an optional Pi-native integration surface scaffolded by `doug init`, not a Doug runtime authority surface

Today, the only scaffolded Pi extension surface is `.pi/extensions/handoff.ts`. It is a Pi-side helper for interactive handoff sessions. Doug does not load `.pi/extensions/*` as part of `doug run`, and those extension files do not replace `.doug/ACTIVE_TASK.md`, session-result parsing, or Doug's artifact ownership rules.

Follow-up notes:

- If Pi later owns additional artifact surfaces beyond the RPC transport and the handoff helper, those surfaces should be introduced explicitly. They are not implied by today's `.pi/` scaffolding.

## Current Compatibility Surfaces

Doug is Pi-first after `doug init`, but the repository still supports a few transitional surfaces. They are supported compatibility behavior, not the preferred steady-state model.

- `execution_mode: subprocess` remains available as an explicit compatibility transport for non-Pi environments or fallback setups. It keeps direct agent subprocess execution working, but Pi RPC is the default path emitted by `doug init`.
- `tool_policy` and `session_defaults` are resolved in Doug's execution contract already, but the current Pi adapter does not translate them into the private Pi RPC payload yet. They are reserved compatibility fields rather than active Pi controls today.
- Planning and runtime still coexist in two workspace shapes: manual root `.doug/PRD.md` plus `.doug/tasks.yaml` remains supported, while `.doug/plan/` is the newer optional planning/backlog path that feeds the same runtime loop.
- `doug plan` and `doug research` already route through the same backend abstraction and default Pi policy, but they are not identical to runtime task execution. Planning is a one-shot workbook-oriented session, and research is a one-shot read-only analysis flow outside the retry/state-machine loop.
- Additional write-scope restrictions are enforced natively in Pi RPC mode. In `subprocess` mode Doug can only inject the restriction into `.doug/ACTIVE_TASK.md` as briefing guidance, so enforcement is not symmetric across transports yet.

## Planning Lifecycle Contract

The integrated planning model uses two separate ownership zones:

- root `.doug/` is the single active runtime workspace
- `.doug/plan/` is the planning and backlog workspace

Backlog epics live at `.doug/plan/epics/<EPIC-ID>/` and are expected to contain `PRD.md`, `tasks.yaml`, and `metadata.yaml`. Backlog metadata supports exactly three statuses:

- `PLANNED`
- `ACTIVE`
- `COMPLETED`

Allowed lifecycle transitions are intentionally narrow:

- `doug handoff` creates new backlog epics as `PLANNED`
- `doug run <EPIC-ID>` promotes a `PLANNED` epic into root `.doug/` and marks it `ACTIVE`
- the runtime completion path marks an `ACTIVE` epic `COMPLETED`

Only one epic may be active in the root `.doug/` workspace at a time. During execution, root `.doug/project-state.yaml` and root `.doug/tasks.yaml` are authoritative; backlog packages remain the handed-off planning artifacts. Completed work is retired history and is never revised in place; follow-up work becomes a new epic. Planning is optional, and manual editing of root `.doug/PRD.md` plus root `.doug/tasks.yaml` remains a supported runtime path.

`metadata.yaml` also records lifecycle provenance and timestamps: `epic_id`, `status`, `created_at`, `source_plan_path`, and optional `activated_at` / `completed_at`.

See [docs/kb/features/planning-lifecycle.md](docs/kb/features/planning-lifecycle.md) for the full ownership and transition contract.

## Planning Workflows

The planning surface is split on purpose:

- `doug plan` is for authoring and iterating on `.doug/plan/PLAN.md`
- `doug handoff` is for deterministic derivative output: backlog epic packages and, when applicable, `.doug/plan/manifest.yaml`
- `doug run EPIC-X` is for controlled epic checkout from backlog into the active root `.doug/` runtime workspace

The manual root-level path remains valid. If you already have root `.doug/PRD.md` and root `.doug/tasks.yaml`, you can skip planning entirely and run plain `doug run`.

### Example: Plan And Handoff

```bash
doug plan --mode definition --epic EPIC-19 "Add a first-class planning-intent input surface"
doug handoff
```

This flow keeps `.doug/plan/PLAN.md` as the editable source document until handoff, then materializes deterministic backlog epics under `.doug/plan/epics/<EPIC-ID>/`, archives the handed-off workbook under `.doug/plan/history/`, and reseeds a fresh active `PLAN.md` for the next planning cycle.

### Example: Greenfield Plan To Scaffold

```bash
doug plan
doug handoff
doug scaffold
```

Use this when the handoff payload includes greenfield scaffold data. `doug handoff` generates `.doug/plan/manifest.yaml`, and `doug scaffold` consumes that manifest exactly once to create the day-0 application structure. After scaffold generation, continue ongoing implementation with `doug run [EPIC-ID]` or plain `doug run`, depending on whether you are using backlog promotion or the direct root-level path.

### Example: Epic Checkout Into Runtime

```bash
doug run EPIC-17
```

This promotes `.doug/plan/epics/EPIC-17/PRD.md` and `.doug/plan/epics/EPIC-17/tasks.yaml` into root `.doug/`, marks the backlog epic `ACTIVE`, and then continues through the normal orchestration loop. It fails fast if another epic is already active in the runtime workspace.

## Commands

```text
doug init
doug plan
doug handoff
doug scaffold
doug run [EPIC-ID]
doug revert <task_id>
doug completion [bash|zsh|fish|powershell]
```

### `doug init`

Initializes a project with:

- `.doug/doug.yaml`
- `.doug/project-state.yaml`
- `.doug/tasks.yaml`
- `.doug/PRD.md`
- `AGENTS.md`
- `CLAUDE.md`
- `CHANGELOG.md`
- `docs/kb/`
- Pi-side skill files under `.pi/skills/` plus `.pi/extensions/handoff.ts`

After init, open `AGENTS.md` and replace the `[Project Name]` and tech stack placeholders with a one- or two-sentence description of your project. Agents read this file before every task — it's the fastest way to give them accurate project context without duplicating your PRD.

**Interactive prompt flow**: Running `doug init` with no flags starts a guided setup sequence:

1. **Build system** — auto-detected from marker files (`go.mod`, `pnpm-workspace.yaml`, `package.json`, `index.html`); shown as default at the prompt; falls back to `go` if nothing is detected
2. **max_retries** — max `FAILURE` outcomes before a task is blocked (default: 3)
3. **max_iterations** — max loop iterations before `doug run` exits (default: 10)
4. **kb_enabled** — whether to synthesize KB articles after feature work (default: true)

The resulting `.doug/doug.yaml` reflects your choices. Doug also writes Pi RPC execution policy for each workflow phase, so the post-init runtime path is consistent without extra config edits.

**Non-interactive and CI use**: All prompts are bypassed when the corresponding flag is provided. Use flags when running `doug init` in a script, CI pipeline, or any non-TTY environment:

- `--build-system string` override auto-detection: `go|npm|pnpm|static`
- `--force` overwrite existing scaffolded files
- `--no-git-init` skip running `git init` after scaffolding

### `doug plan`

Creates or refreshes `.doug/plan/PLAN.md`, then emits Doug's planning brief and planning prompt through Pi so planning happens directly in that workbook.

For Doug-managed planning runs, `.doug/ACTIVE_TASK.md` is the canonical brief and `PLAN.md` is the editable planning workbook. Doug refreshes a planning context block at the top of `PLAN.md` on each planning run, persists the resolved planning intent and related run context there, and leaves the rest of the file as the collaborative workbook for planning notes, scope, risks, epic sequencing, and handoff-ready data. `doug plan` does not generate backlog epic packages or `.doug/plan/manifest.yaml`; those derivative artifacts are owned by `doug handoff`.

On each planning run, Doug also injects unresolved archived bug reports from `.doug/logs/bugs/{epic}/` into the Doug-owned briefing block at the top of `PLAN.md`. That keeps deferred bug rediscovery in the canonical archive instead of requiring a second manual intake file.

Planning context can be provided directly on the CLI:

- positional text after `doug plan` becomes the planning intent for that run
- `--intent` provides the planning intent explicitly
- `--mode` hints the planning lens: `discovery`, `roadmapping`, `definition`, `feature`, `refactor`, `bugfix`, or `greenfield`
- `--epic` records a target epic hint in the Doug-owned brief

If no positional intent or `--intent` is provided, an interactive `doug plan` run opens the shared composer-style planning-intent capture surface before the agent launches. In non-interactive mode, missing planning intent is a hard error rather than silently falling back to stale workbook text.

`.doug/ACTIVE_TASK.md` stays the canonical run brief, but the planning intent itself is PLAN-owned run context. Doug resolves that intent from the CLI or interactive capture and writes it into the Doug-owned planning block in `PLAN.md` before agent launch so the current run does not depend on stale workbook prose alone.

Archived bug follow-up should become explicit planning work in `PLAN.md`. If the source epic is still `PLANNED`, you can update that planned package when the scope still matches. If the source epic is `ACTIVE` or `COMPLETED`, plan the follow-up as new work instead of reopening the historical backlog package.

### `doug handoff`

Parses the structured handoff payload in `.doug/plan/PLAN.md` and generates backlog epic packages under `.doug/plan/epics/`. For greenfield plans that include scaffold data, it also derives `.doug/plan/manifest.yaml`.

On successful handoff, Doug also archives the exact pre-handoff workbook to `.doug/plan/history/PLAN-{timestamp}.md` and rewrites `.doug/plan/PLAN.md` with a fresh seeded workbook plus Doug-owned context about the completed handoff. That keeps handed-off epic definitions out of the active intake surface while preserving inspectable planning history.

`PLAN.md` remains a markdown document, but the deterministic payload must live in a `## Handoff Data` section with a fenced YAML block:

````md
## Handoff Data

```yaml
schema_version: 1
project:
  name: "Acme Planner"
  mode: "greenfield"
manifest:
  schema_version: 1
  project:
    name: "Acme Planner"
    mode: "greenfield"
  scaffold:
    language: "typescript"
    runtime: "node"
    framework: "nextjs"
    package_manager: "pnpm"
    build_system: "npm-scripts"
  dependencies:
    runtime:
      - "next"
    development:
      - "typescript"
  constraints:
    - "Deploy on Vercel"
epics:
  - id: "EPIC-17"
    name: "Planning Lifecycle"
    prd: |
      # PRD

      Deterministically generate backlog packages.
    tasks:
      - id: "EPIC-17-003"
        description: "Implement deterministic handoff output."
        acceptance_criteria:
          - "Generated tasks.yaml always quotes descriptions."
```
````

Each generated backlog epic package contains:

- `PRD.md`
- `tasks.yaml`
- `metadata.yaml`

The generated `tasks.yaml` files deterministically quote `description` and `acceptance_criteria` string values so they continue to parse reliably through the existing loader. This quoting is parser-sensitive and should be preserved whenever handoff output is regenerated or reviewed.

### `doug scaffold`

Builds the synthetic scaffold task from `.doug/plan/manifest.yaml`, emits one Doug scaffold interaction through Pi with the `scaffold` skill, and dispatches the outcome through the existing success/failure handlers.

Preconditions:

- `.doug/project-state.yaml` must already exist, so run `doug init` first
- `.doug/plan/manifest.yaml` must exist and pass manifest v1 validation

Behavior:

- writes a synthetic `.doug/ACTIVE_TASK.md` with the full manifest injected as context
- uses the manifest to resolve build/install guidance for the single scaffold run
- does not write scaffold state into `.doug/project-state.yaml`
- leaves `doug run` as the next step for ongoing task execution

Flags:

- none

### `doug run [EPIC-ID]`

Runs the orchestration loop against root `.doug/tasks.yaml`. When `EPIC-ID` is provided, `doug run` first promotes `.doug/plan/epics/<EPIC-ID>/` into the root runtime workspace, marks that backlog epic `ACTIVE`, and then continues through the existing rollover/bootstrap path.

This is an epic checkout flow, not a separate execution mode. The root `.doug/` files become the active working set, while the backlog package remains the immutable handoff artifact and lifecycle record. When an epic completes, the runtime updates backlog metadata to `COMPLETED`; any later follow-up should be planned as a new epic rather than revising the completed package in place.

Epic completion also archives the executed root `.doug/` working set under `.doug/logs/archives/{epic}/` so the final runtime snapshot remains inspectable without mutating the original backlog package. That archive is the durable record; the live root `.doug/ACTIVE_TASK.md` briefing is removed after outcome handling. Manual root-level runs without backlog metadata still create this runtime archive.

Terminal output is structured for long-running loops: each iteration starts with a visible `[taskID] attempt N/M (type)` header, heartbeat lines print as `[taskID] +elapsed`, and success output includes the changelog summary reported by the agent.

High-level flow:

1. Load `.doug/doug.yaml`
2. Verify dependencies and toolchain
3. Load `.doug/project-state.yaml` and `.doug/tasks.yaml`
4. Bootstrap or roll over epic state
5. Run pre-flight build and test checks
6. Ensure the epic branch is checked out
7. Write `.doug/ACTIVE_TASK.md`
8. Emit the Doug prompt and resolved policy to the active backend (Pi by default)
9. Parse the result written into `.doug/ACTIVE_TASK.md` and dispatch `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`
10. Archive `.doug/ACTIVE_TASK.md` into `.doug/logs/sessions/{epic}/` before state changes
11. Remove the live root `.doug/ACTIVE_TASK.md` after handler finalization so stale task briefs do not linger between runs

Flags:

- `--agent string`
- `--agent-heartbeat-seconds int`
- `--build-system string`
- `--kb-enabled`
- `--max-iterations int`
- `--max-retries int`

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

Main config lives in `.doug/doug.yaml`. The interactive `doug init` flow writes this file from your prompt selections — you do not need to edit it manually for a standard setup. In normal use, most users interact with top-level runtime settings while Doug keeps the run prompt in code and Pi owns downstream provider selection; the `policy:` block is the advanced override surface.

Scaffolded example:

```yaml
build_system: go
max_retries: 3
max_iterations: 10
kb_enabled: true
agent_heartbeat_seconds: 30
policy:
  phases:
    runtime:
      execution_mode: rpc
    planning:
      execution_mode: rpc
    scaffold:
      execution_mode: rpc
    research:
      execution_mode: rpc
    post_epic_kb:
      execution_mode: rpc
```

Doug-owned prompts are built into the binary. `AGENTS.md` carries stable repository policy, `.doug/ACTIVE_TASK.md` carries the run-specific brief, and `.doug/doug.yaml` carries execution policy such as `execution_mode`, skill overrides, and restriction metadata. When `execution_mode: rpc` is active, Doug sends that prompt-plus-policy contract to Pi rather than choosing a provider CLI itself.

Fields most users care about:

- `build_system`: `go`, `npm`, `pnpm`, or `static` (no-op for plain HTML/CSS/JS projects)
- `max_retries`: max `FAILURE` outcomes before a task becomes `BLOCKED`
- `max_iterations`: max orchestration loop iterations before `doug run` exits
- `kb_enabled`: inject a documentation synthesis task after feature work completes
- `agent_heartbeat_seconds`: periodic liveness logging while the agent runs; `0` disables it
- `policy`: advanced overrides for skill mapping, execution mode, routing/tool policy, and read/write restrictions

`policy:` is Doug's execution-policy surface. Add entries when you need to override the default skill mapping, fall back to direct subprocess execution, or tighten execution/read-write behavior for a specific workflow.

For backend selection, `execution_mode` is the key override:

- empty or `subprocess` uses the normal CLI subprocess backend
- `rpc` uses the Pi adapter

Example:

```yaml
policy:
  phases:
    runtime:
      execution_mode: rpc
    planning:
      execution_mode: rpc
    scaffold:
      execution_mode: rpc
    research:
      execution_mode: rpc
    post_epic_kb:
      execution_mode: rpc
```

## Skills

Doug bundles built-in skills out of the box:

| Skill | Task type | Output | Notes |
|-------|-----------|--------|-------|
| `implement-feature` | `feature` | Code + session result | Standard feature implementation workflow |
| `implement-bugfix` | `bugfix` | Code + session result | Root cause analysis, fix, regression test |
| `implement-documentation` | `documentation` | `docs/kb/` articles | Synthesizes session logs into KB; can also be pointed at a specific feature or file manually |
| `plan` | `plan` | Planning workbook updates | Used by `doug plan` for interactive planning sessions |
| `scaffold` | `scaffold` | Project scaffold + session result | Used by `doug scaffold` for manifest-driven bootstrap work |
| `research` | `research` | `.doug/logs/research/` report | Read-only codebase analysis; point at a feature, module, file, or the full codebase; does not modify code |

### Adding a Custom Skill

To add your own workflow, wire up both the skill file and the task-type mapping:

1. Pick a task type and skill name, for example `refactor` -> `implement-refactor`.
2. Add `policy.tasks.refactor.skill: implement-refactor` to `.doug/doug.yaml` under the `policy:` block.
3. Create the skill file under `.pi/skills/implement-refactor/SKILL.md`.
4. Add tasks using that task type in `.doug/tasks.yaml`.
5. Keep repository-specific rules in `AGENTS.md`; keep the skill itself focused on the workflow.

Example `doug.yaml` fragment:

```yaml
policy:
  tasks:
    refactor:
      skill: implement-refactor
```

`doug` resolves the skill name from `policy.tasks[type].skill` in `doug.yaml`, then expects Pi-side skill scaffolding at `.pi/skills/<skill-name>/SKILL.md`.

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

Built-in task types:

Backlog task types:
- `feature`
- `bugfix`
- `documentation`

Synthetic runtime-only:
- `scaffold`

Custom backlog task types can still be introduced via `policy.tasks[type].skill`. Use `BLOCKED` status on a backlog task when work needs human intervention. When retries are exhausted, doug leaves `project-state.active_task` on the original backlog task and marks that task `BLOCKED` instead of switching to a separate `manual_review` task type.


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
- **Automatic growth**: `kb_enabled: true` (the default) causes doug to run a synthetic `POST_EPIC_KB` documentation pass after epic finalization. That pass routes the agent through the `implement-documentation` workflow, starts from `docs/kb/README.md`, and only accepts KB output under `docs/kb/`.
- **Manual updates**: For targeted KB work, either edit `docs/kb/` directly or run a manual agent session using the `implement-documentation` skill against a feature, module, or file. Do not add `documentation` tasks to `.doug/tasks.yaml`; that task type is reserved for orchestrator-injected KB synthesis.
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
