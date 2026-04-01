---
name: "plan"
description: "Drive interactive planning in .doug/plan/PLAN.md while targeting deterministic handoff outputs."
---

# Planning Workflow

Read the repository instructions first, then use `.doug/ACTIVE_TASK.md` as the planning brief and `.doug/plan/PLAN.md` as the primary artifact.

## Phase 1: Orient

1. Read `.doug/ACTIVE_TASK.md` completely
2. Review the current contents of `.doug/plan/PLAN.md`
3. Confirm what is already known, what is undecided, and what must be prepared for deterministic handoff later

## Phase 2: Plan

1. Write or refine the plan directly in `.doug/plan/PLAN.md`
2. Keep planning free-form where useful, but make the eventual handoff contract explicit
3. Treat `PLAN.md` as the single planning source of truth instead of creating extra required stage files

## Phase 3: Handoff Readiness

1. Keep deterministic backlog derivatives out of scope for this workflow
2. Make it clear in `PLAN.md` when the plan is ready for `doug handoff`
3. Remember that `doug handoff` owns backlog epic packages and `.doug/plan/manifest.yaml`

## Phase 4: Report

1. Write the result into the `## Agent Result` block in `.doug/ACTIVE_TASK.md`
2. Summarize what changed in `PLAN.md`, any open questions, and handoff readiness in the summary sections at the bottom of `.doug/ACTIVE_TASK.md`
