---
title: doug scaffold — Manifest-Driven Project Scaffold
updated: 2026-05-01
category: Features
tags: [scaffold, manifest, init, run, agent, cobra]
related_articles:
  - docs/kb/packages/init.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/types.md
---

# doug scaffold — Manifest-Driven Project Scaffold

## Overview

`doug scaffold` materializes a day-0 application scaffold from `.doug/plan/manifest.yaml`.

The command is intentionally narrow:

- It requires an existing doug project created by `doug init`
- It reads and validates manifest schema v1
- It writes a synthetic `.doug/ACTIVE_TASK.md` briefing with the full manifest injected as structured context
- It emits one Doug scaffold interaction through the backend with the `scaffold` skill
- It routes the result through the existing success/failure handlers

The intended sequence is:

1. `doug init`
2. Create or generate `.doug/plan/manifest.yaml`
3. `doug scaffold`
4. `doug run`

See [cmd/init — Project Scaffolding Subcommand](../packages/init.md) for the boundary that `doug init` preserves: `doug init` creates doug control files and skills, but it does not create application project files.

## Preconditions And Guards

`doug scaffold` fails fast before agent execution when either prerequisite is missing:

- `.doug/project-state.yaml` must exist, otherwise the command exits with guidance to run `doug init` first
- `.doug/plan/manifest.yaml` must exist and pass schema validation, otherwise the command exits non-zero with an actionable manifest error

There is no retry loop inside `doug scaffold`. If the manifest or project state is wrong, the user fixes it and reruns the command.

## Manifest V1 Contract

The loader uses `yaml.Decoder.KnownFields(true)`, so unknown fields are rejected during parsing.

Required top-level fields:

- `schema_version`
- `project`
- `scaffold`
- `dependencies`
- `constraints`

Required nested fields:

- `project.name`
- `project.mode`
- `scaffold.language`
- `scaffold.runtime`
- `scaffold.framework`
- `scaffold.package_manager`
- `scaffold.build_system`
- `dependencies.runtime`
- `dependencies.development`

Only `schema_version: 1` is accepted.

Reference shape:

```yaml
schema_version: 1
project:
  name: "Acme App"
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
    - "react"
    - "react-dom"
  development:
    - "typescript"
    - "eslint"
constraints:
  - "Deploy on Vercel"
```

## Agent-Driven Interaction Model

`doug scaffold` constructs a synthetic task with:

- task id `SCAFFOLD`
- task type `scaffold`
- one attempt only
- acceptance criteria focused on creating the scaffold, installing dependencies, and honoring manifest constraints

The command writes `.doug/ACTIVE_TASK.md` with:

- the synthetic scaffold description
- the resolved build-system section used for verification/install guidance
- a `## Manifest Context` section containing the full manifest YAML

After that, doug resolves the `scaffold` skill via `policy.tasks.scaffold.skill` in `doug.yaml` (falling back to the hardcoded `scaffold` default), builds the Doug-owned scaffold prompt in code, and dispatches exactly one scaffold interaction through the resolved backend. In the default post-init path, Pi receives that prompt plus policy and chooses the downstream provider/model configuration. The manifest remains the source of truth for the generated project files; doug itself does not template framework files directly.

## Statelessness And Outcome Handling

`doug scaffold` is stateless with respect to the orchestration loop:

- it does not append a task to `.doug/tasks.yaml`
- it does not write scaffold progress or completion state into `.doug/project-state.yaml`
- it does not persist scaffold metadata into `.doug/doug.yaml`

Instead, it reuses the existing handler pipeline for the single invocation:

- success goes through `HandleSuccess`
- failure goes through `HandleFailure`

To preserve the stateless model, scaffold uses temporary state/task paths when dispatching handlers instead of mutating the real `.doug/project-state.yaml` or `.doug/tasks.yaml`.

KB synthesis is also disabled for scaffold runs, so the command stays focused on one manifest-driven materialization pass.

## Build System Resolution

The scaffold command resolves the build system used for install and verification in this order:

1. `scaffold.build_system` when it is one of `go`, `npm`, `pnpm`, or `static`
2. `scaffold.package_manager` when it is `npm` or `pnpm`
3. `scaffold.runtime` fallback: `node -> npm`, `go -> go`
4. doug's default build system fallback

This resolution affects the `## Build System` block written into `.doug/ACTIVE_TASK.md` and the handler-driven verification path after agent execution.

## User-Facing Notes

- `doug scaffold` has no command-specific flags today; usage is simply `doug scaffold`
- The manifest file lives at `.doug/plan/manifest.yaml`
- The command is meant for the initial app scaffold only; ongoing task execution still flows through `doug run`
- `doug init` remains the owner of doug-managed control files and Pi-side skill scaffolding
