<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->
## Doug-Specific Instructions

This section is managed by `doug init`. Keep repository-specific operating rules here, and keep task skills focused on their workflow.

### Progressive Disclosure

1. Read `.doug/ACTIVE_TASK.md` for the active task brief when it exists.
2. Read `.doug/PRD.md` for product context and constraints.
3. Read `docs/kb/README.md` for the knowledge base index.
4. Read only the KB articles relevant to the task at hand.

### Working Rules

- Treat `.doug/ACTIVE_TASK.md` as the canonical task brief for doug-managed work.
- Write your result directly into the `## Agent Result` block and summary sections at the bottom of `.doug/ACTIVE_TASK.md`.
- Do not depend on other internal doug control files. Only `.doug/ACTIVE_TASK.md` and `.doug/PRD.md` are part of the agent-facing contract.
- If you find a bug that is outside the current task scope, report it instead of fixing it opportunistically.
- Use `docs/kb/README.md` as the KB entrypoint instead of scanning the whole KB up front.
<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->
