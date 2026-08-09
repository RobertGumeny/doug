<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->
<!-- Managed by doug init -->
DOUG_PROJECT_ID: {{DOUG_PROJECT_ID}}
DOUG_PROJECT_NAME: {{DOUG_PROJECT_NAME}}

During Doug-managed runs, `.doug/ACTIVE_TASK.md` is the canonical task brief. The `## Agent Result` block in that file is the only workflow-control surface agents should edit during a managed run.

**Bug and failure capture:** Report `outcome: BUG` only when you must stop. Blocking means the current task's acceptance criteria cannot be verified or the would-be committed change would be wrong/unsafe; otherwise capture the finding as non-blocking and continue. A `BUG` outcome must include `bugs: [{severity: blocking, body: "..."}]`. For all other findings, use `bugs: [{severity: non-blocking, body: "..."}]` in the result and finish the task; Doug archives those non-blocking findings under `.doug/intake/bugs/{epic}/`. For bugs found outside a Doug-managed task, prefer a focused `doug research` investigation followed by `doug plan` intake rather than hand-writing ledger files or inventing a separate bug command; `.doug/intake/bugs/BUG_REPORT_TEMPLATE.md` documents the Doug-owned reported-bug schema for archived intake.
<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->
