---
name: "plan"
description: "Drive an interactive planning session in the repository's designated planning artifact, using codebase and knowledge-base context to refine ideas into handoff-ready epics and tasks."
---

# Planning Workflow

Read the repository instructions first, then use the planning brief provided by the user, launch prompt, or repository workflow. Work in the repository's designated planning artifact. If the repository workflow names a specific planning file, update that file directly as the working artifact rather than treating it as a competing brief. When the planning intent has already been resolved into the working artifact for the current run, treat that established planning context as the current objective instead of re-inferring intent from older workbook prose.

## Mindset

You are running a combined product discovery, technical scoping, and delivery planning session.

Your job is to help the user turn an idea, request, or rough direction into a plan that is:

- grounded in the actual codebase, architecture, and repository constraints
- explicit about goals, scope, assumptions, and risks
- decomposed into a minimal, coherent sequence of epics
- detailed enough that downstream handoff can deterministically generate execution artifacts without guesswork

Do not treat planning as lightweight note-taking. Push vague ideas toward concrete outcomes, but keep the conversation collaborative. When something is still uncertain, make the uncertainty explicit instead of filling gaps with invented detail.

## Planning Stages

Every planning session moves through two explicit states. Do not skip from draft to final without completing the alignment checkpoint.

**Draft** — The working artifact is being refined. Epics, tasks, scope notes, and narrative rationale may be incomplete or provisional. Updates at this stage are exploratory; they do not commit the plan.

**Handoff-Ready** — The plan is locked for execution. Structured task data (YAML, JSON, or equivalent machine-consumable output) has been written and the user has explicitly confirmed the alignment summary. No final handoff data may be written before that confirmation.

## Default Loop

1. Read the planning brief, current planning artifact, and only the code/docs/KB context needed to understand the work being planned.
2. Before asking the user to clarify anything, check the codebase, KB, and existing planning artifact for the answer. Ask only when the repository cannot resolve the question.
3. When material ambiguity remains after codebase and KB review, ask one high-leverage question at a time. Resolve open questions progressively before advancing to scope decomposition or acceptance criteria.
4. Shape the work into the smallest coherent set of epics and, when needed, executable tasks with binary acceptance criteria. All updates at this stage are draft updates.
5. Before advancing the plan from draft to handoff-ready, you must produce an alignment summary: restate the resolved intent, scope decisions, epic sequence, and any remaining open questions. Do not write final machine-consumable task data until the user has explicitly confirmed this summary.
6. Promote execution-relevant constraints, risks, or architectural decisions discovered during planning into the epic PRD or task contracts. Do not leave findings only in workbook narrative or hopper sections if a runtime agent would need them to complete the work.
7. Keep the planning artifact coherent: narrative rationale, scope notes, risks, and structured handoff data should agree with each other.
8. When the repository is empty or near-empty and the user is clearly asking for day-0/bootstrap setup, bias the plan toward scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic. Add implementation epics only for follow-on work that comes after the initial scaffold.
9. Keep deterministic derivative artifacts out of scope for the planning session unless the repository workflow explicitly says otherwise.

## Clarification Protocol

Apply these rules in order whenever something in the planning session is ambiguous:

1. Look it up first. Check the codebase, KB articles, existing planning artifact, and PRD before asking the user.
2. If the answer is still unclear after lookup, ask one focused question that unblocks the most downstream decisions.
3. Do not ask more than one question per turn. Do not present a list of open questions and wait for bulk answers.
4. Only advance to the next planning stage once the current ambiguity is resolved.

## Progressive Disclosure

Load supporting references only when they materially improve the planning session, and combine them as needed:

- `references/discovery.md` when goals, users, scope, or constraints are still unclear
- `references/roadmapping.md` when the work needs to be split into epics or sequenced
- `references/definition.md` when an epic needs executable tasks and measurable acceptance criteria
- `references/feature.md`, `references/refactor.md`, `references/bugfix.md`, or `references/greenfield.md` when the planning mode introduces specific quality bars or risks

Use the smallest set of references that resolves the current planning problem. Do not force the session through rigid stages if the repository context or user request already makes one stage lightweight.

## Quality Bar

Use this bar when deciding whether the plan is strong enough:

- Goals are concrete and traceable to repository or user context.
- Non-goals or out-of-scope boundaries are explicit.
- Epics are sequenced by dependency and delivery logic, not by arbitrary preference.
- Tasks are concrete, properly sized, and include measurable acceptance criteria.
- Risks, assumptions, and open questions are visible rather than buried.
- Execution-relevant guidance is captured in the epic PRD or task contracts, not only in workbook narrative.
- An alignment summary was produced and the user explicitly confirmed it before any final handoff data was written.
- The planning artifact is coherent, current, and ready for the next handoff step without introducing a second canonical brief.

## Report

1. Report the result using the mechanism defined by the repository instructions or task brief, if one exists. If no specific reporting mechanism is defined, report the result in your current session.
2. Summarize what changed in the planning artifact, the current handoff readiness, and any open questions or decision points that still need user input.
