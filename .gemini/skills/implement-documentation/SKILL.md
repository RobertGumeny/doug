---
name: implement-documentation
description: Update or synthesize technical documentation using repository context and current code. Use when the task is documentation-focused, including KB maintenance and cross-linking.
allowed-tools: Read, Grep, Glob, LS, Write, Bash
---

# Documentation Workflow

Read the repository instructions first, then use this workflow for KB updates, package docs, feature docs, and other repository documentation changes.

## Documentation Principles

- Prefer updating an existing document over creating a new one
- Optimize for durable technical guidance, not session-by-session narration
- Keep content lean, explicit, and cross-linked where useful
- Reflect the code as it exists now, not how it used to work

## Phase 1: Ingest Context

1. Read the task request and repository guidance
2. Read product context if it matters to the documentation
3. Inspect the relevant code and current docs to identify drift, missing coverage, and the best place to document the change

## Phase 2: Plan

1. Decide which existing docs to update
2. Create a new document only when the topic does not fit an existing one
3. If the task requirements are too ambiguous to document accurately, stop and report the ambiguity clearly

## Phase 3: Write

1. Update the selected docs to match the current implementation
2. Add cross-links when they materially improve navigation
3. Keep examples short and focused
4. If you discover a product or code bug while documenting, report it instead of inventing documentation that papers over the issue

## Phase 4: Verify

1. Re-read changed docs for accuracy against the code
2. Run any relevant doc checks or tests if the repo provides them
3. Confirm links, filenames, commands, and identifiers are correct

## Phase 5: Report

Report what changed, what drift was corrected, and how you verified accuracy using the mechanism defined by the repository instructions.
