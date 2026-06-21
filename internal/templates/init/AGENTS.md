<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->
<!-- Managed by doug init -->
DOUG_PROJECT_ID: {{DOUG_PROJECT_ID}}
DOUG_PROJECT_NAME: {{DOUG_PROJECT_NAME}}

During Doug-managed runs, `.doug/ACTIVE_TASK.md` is the canonical task brief. The `## Agent Result` block in that file is the only workflow-control surface agents should edit during a managed run.

**Bug and failure capture:** Agents working from an active `.doug/ACTIVE_TASK.md` brief use that file's structured result contract (`BUG` or `FAILURE` outcome) to surface blocking issues. Agents not working from an active task brief may use `.doug/logs/BUG_REPORT_TEMPLATE.md` and write non-blocking findings to `.doug/logs/bugs/`.
<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->
