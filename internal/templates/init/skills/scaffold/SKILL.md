---
name: "scaffold"
description: "Materialize a day-0 project scaffold from the manifest context provided in ACTIVE_TASK.md."
---

# Scaffold Workflow

Read the repository instructions first, then use `.doug/ACTIVE_TASK.md` as the source of truth for the scaffold task. The task brief includes the scaffold request, acceptance criteria, build-system section, and a `## Manifest Context` section containing the full manifest YAML.

## Phase 1: Clarify

1. Read `.doug/ACTIVE_TASK.md` completely, including `## Manifest Context`
2. Confirm the declared language, runtime, framework, package manager, build system, dependencies, and constraints from the manifest
3. Preserve existing doug-managed files unless the task explicitly requires changing them

## Phase 2: Implement

1. Create the minimum project definition files required for the declared stack
2. Write files that match the manifest choices, for example `package.json` for Node-based stacks or `go.mod` for Go stacks
3. Add only the dependency declarations needed for the requested scaffold and keep the result minimal, buildable, and aligned with the manifest constraints

## Phase 3: Verify

1. Run any relevant verification needed to confirm the scaffold files are coherent before dependency installation
2. Run the declared package manager install as the final execution step for the scaffold
3. Do not report `SUCCESS` unless that install step completes without error
4. If install or verification fails, report the failure instead of claiming success

## Phase 4: Report

1. Write the result into the `## Agent Result` block in `.doug/ACTIVE_TASK.md`
2. Report `SUCCESS` only after the install step has completed without error
3. Summarize what was created, key decisions, and verification in the task summary sections at the bottom of `.doug/ACTIVE_TASK.md`
