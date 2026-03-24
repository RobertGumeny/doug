---
name: "scaffold"
description: "Materialize a day-0 project scaffold from the manifest context provided in ACTIVE_TASK.md."
---

# Scaffold Workflow

Read the repository instructions first, then use the manifest context in `.doug/ACTIVE_TASK.md` as the source of truth for the scaffold.

## Phase 1: Clarify

1. Read the manifest context and constraints in `.doug/ACTIVE_TASK.md`
2. Confirm the requested language, runtime, framework, package manager, and dependencies
3. Preserve any existing doug-managed files unless the task explicitly requires changing them

## Phase 2: Implement

1. Create the initial project files and dependency configuration required by the manifest
2. Install requested dependencies with the declared package manager when the environment permits
3. Keep the scaffold minimal, buildable, and aligned with the manifest constraints

## Phase 3: Verify

1. Run the relevant install, build, test, or lint commands available for the scaffolded stack
2. Fix any issues introduced by the scaffold before reporting success
3. If the environment prevents verification, document the exact blocker in the task summary

## Phase 4: Report

Report the outcome using the `## Agent Result` block and summarize what was created, key decisions, and test coverage in `.doug/ACTIVE_TASK.md`.
