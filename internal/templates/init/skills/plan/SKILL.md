---
name: "plan"
description: "Drive an interactive planning session in the repository's designated planning artifact, using codebase and knowledge-base context to refine ideas into handoff-ready epics and tasks."
---

# Planning Workflow

Read the repository instructions first, then use the planning brief provided by the user, launch prompt, or repository workflow. Work in the repository's designated planning artifact. If the repository workflow names a specific planning file, update that file directly and treat it as the planning source of truth.

## Mindset

You are running a combined product discovery, technical scoping, and delivery planning session.

Your job is to help the user turn an idea, request, or rough direction into a plan that is:

- grounded in the actual codebase, architecture, and repository constraints
- explicit about goals, scope, assumptions, and risks
- decomposed into a minimal, coherent sequence of epics
- detailed enough that downstream handoff can deterministically generate execution artifacts without guesswork

Do not treat planning as lightweight note-taking. Push vague ideas toward concrete outcomes, but keep the conversation collaborative. When something is still uncertain, make the uncertainty explicit instead of filling gaps with invented detail.

## Default Loop

1. Read the planning brief, current planning artifact, and only the code/docs/KB context needed to understand the work being planned.
2. Clarify the intended outcome, scope boundaries, constraints, and handoff expectations before locking the plan.
3. Shape the work into the smallest coherent set of epics and, when needed, executable tasks with binary acceptance criteria.
4. Keep the planning artifact coherent: narrative rationale, scope notes, risks, and structured handoff data should agree with each other.
5. When the repository is empty or near-empty and the user is clearly asking for day-0/bootstrap setup, bias the plan toward scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic. Add implementation epics only for follow-on work that comes after the initial scaffold.
6. Keep deterministic derivative artifacts out of scope for the planning session unless the repository workflow explicitly says otherwise.

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
- The planning artifact can serve as the single source of truth for the next handoff step.

## Report

1. Report the result using the mechanism defined by the repository instructions or task brief, if one exists. If no specific reporting mechanism is defined, report the result in your current session.
2. Summarize what changed in the planning artifact, the current handoff readiness, and any open questions or decision points that still need user input.
