---
name: "plan"
description: "Drive interactive planning in .doug/plan/PLAN.md while targeting deterministic handoff outputs."
---

# Planning Workflow

Read the repository instructions first, then use the task brief provided by the user, launch prompt, or repository workflow. When the repository already uses `.doug/plan/PLAN.md`, treat that file as the primary planning artifact.

## Phase 1: Orient

1. Read the planning brief completely, including any product context, constraints, or downstream handoff requirements
2. Review the current contents of `.doug/plan/PLAN.md` when it exists, or the repository's designated planning document if it differs
3. Confirm what is already known, what is undecided, and what must be prepared for deterministic handoff later

## Phase 2: Plan

1. Write or refine the plan directly in `.doug/plan/PLAN.md`
2. Keep planning free-form where useful, but make the eventual handoff contract explicit
3. Treat `PLAN.md` as the single planning source of truth instead of creating extra required stage files

## Phase 3: Handoff Readiness

1. Keep deterministic backlog derivatives out of scope for this workflow
2. Make it clear in `PLAN.md` when the plan is ready for the next downstream handoff step
3. When working in a doug repository, remember that `doug handoff` owns backlog epic packages and `.doug/plan/manifest.yaml`

## Phase 4: Report

1. Report the result using the mechanism defined by the repository instructions or task brief, if one exists. If no specific reporting mechanism is defined, report the result in your current session.
2. Summarize what changed in `PLAN.md`, any open questions, and handoff readiness
