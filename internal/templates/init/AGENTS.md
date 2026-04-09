<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->
DOUG_PROJECT_ID: {{DOUG_PROJECT_ID}}
DOUG_PROJECT_NAME: {{DOUG_PROJECT_NAME}}

## Doug-Specific Instructions

This section is managed by `doug init`. Keep repository-specific operating rules here, and keep task skills focused on their workflow.

### Progressive Disclosure

1. For doug-managed runs launched by `doug`, read `.doug/ACTIVE_TASK.md` for the active task brief.
2. Read `.doug/PRD.md` for product context and constraints when it is relevant to the task.
3. Read `docs/kb/README.md` for the knowledge base index.
4. Read only the KB articles relevant to the task at hand.

### Working Rules

- Only treat `.doug/ACTIVE_TASK.md` as the canonical task brief when the user request or launch prompt indicates a doug-managed run.
- In doug-managed runs, write your result directly into the `## Agent Result` block and summary sections at the bottom of `.doug/ACTIVE_TASK.md`.
- Do not depend on other internal doug control files. Only `.doug/ACTIVE_TASK.md` and `.doug/PRD.md` are part of the agent-facing contract.
- If you find a bug that is outside the current task scope, report it instead of fixing it opportunistically.
- Use `docs/kb/README.md` as the KB entrypoint instead of scanning the whole KB up front.
<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->
