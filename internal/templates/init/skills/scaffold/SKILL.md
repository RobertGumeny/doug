---
name: "scaffold"
description: "Materialize a day-0 project scaffold from a manifest or structured scaffold brief."
---

# Scaffold Workflow

Read the repository instructions first, then use the scaffold brief provided by the user, launch prompt, or repository workflow. When the task includes a manifest or structured scaffold context, treat that input as the source of truth for the requested stack, dependencies, and constraints.

## Phase 1: Clarify

1. Read the scaffold brief completely, including any manifest, dependency lists, acceptance criteria, and build-system guidance
2. Confirm the declared language, runtime, framework, package manager, build system, dependencies, and constraints from that brief
3. Preserve repository-owned control files unless the task explicitly requires changing them

## Phase 2: Implement

1. Create the minimum project definition files required for the declared stack
2. Write files that match the manifest choices, for example `package.json` for Node-based stacks or `go.mod` for Go stacks
3. Add only the dependency declarations needed for the requested scaffold and keep the result minimal, buildable, and aligned with the manifest constraints

## Phase 3: Verify

1. Run any relevant verification needed to confirm the scaffold files are coherent before dependency installation
2. Run the declared package manager install as the final execution step when the scaffold brief or repository workflow expects an install-complete result
3. Do not report the scaffold as complete unless that required install step completes without error
4. If install or verification fails, report the failure instead of claiming completion

## Phase 4: Report

1. Report the result using the mechanism defined by the repository instructions or task brief, if one exists. If no specific reporting mechanism is defined, report the result in your current session.
2. Report completion only after any required install step has completed without error
3. Summarize what was created, key decisions, and verification
