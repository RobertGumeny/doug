<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->
<!-- Managed by doug init -->
DOUG_PROJECT_ID: {{DOUG_PROJECT_ID}}
DOUG_PROJECT_NAME: {{DOUG_PROJECT_NAME}}

During Doug-managed runs, `.doug/ACTIVE_TASK.md` is the canonical task brief. The `## Agent Result` block in that file is the only workflow-control surface agents should edit during a managed run.

**Bug and failure capture:** Report `outcome: BUG` only when you must stop — i.e., continuing would make this task's acceptance criteria impossible to verify, completing this task would require committing a change that violates its acceptance criteria, or the only path forward would directly introduce a regression. A `BUG` outcome must include `bugs: [{severity: blocking, body: "..."}]`. For all other findings, use `bugs: [{severity: non-blocking, body: "..."}]` in the result and finish the task. Agents not working from an active task brief may use `.doug/logs/BUG_REPORT_TEMPLATE.md` and write non-blocking findings to `.doug/logs/bugs/`.
<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->
