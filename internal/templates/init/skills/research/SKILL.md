---
name: research
description: Perform read-only codebase analysis and generate a portable research document. Use when exploring a feature, module, file, function, or the entire codebase to understand current state, patterns, dependencies, and technical debt. Does not modify code or create branches.
---

# Research Workflow

This skill performs deep codebase analysis and produces a `RESEARCH_REPORT.md` that can be used by humans, passed to LLMs for planning, or provided to agents as context for future tasks.

**This is a read-only skill.** Do not create branches, modify code, or make commits.

## Phase 1: Clarify Scope

Before beginning research, confirm the scope is clear. Valid scope types:

1. **Feature/Module**: A specific capability (e.g., "the authentication flow", "story generation")
2. **File/Function Origin**: Start from a specific file or function and trace what uses or touches it
3. **Full Codebase**: A high-level map of the entire project (use sparingly)

If the scope is ambiguous, ask for clarification before proceeding.

## Phase 2: Gather Context

1. Read `PRD.md` to understand product context and priorities
2. Read `project-state.yaml` for current epic and recent activity
3. Read `tasks.yaml` to identify related tasks (past, current, or planned)
4. Read `CLAUDE.md` or `AGENTS.md` for architectural rules and patterns

This context helps you relate code findings back to product goals.

## Phase 3: Explore the Codebase

Use read-only tools to map the target scope:

- **Glob**: Find files matching patterns (e.g., `src/**/*Service.ts`)
- **Grep**: Search for function names, imports, and references
- **Read**: Examine file contents
- **LS**: List directory structures

For **Feature/Module** research:

1. Identify the entry point(s)
2. Trace the data flow through components, hooks, services, and API routes
3. Map all files that participate in the feature

For **File/Function Origin** research:

1. Start with the specified file or function
2. Search for all imports and usages across the codebase
3. Build a dependency tree (what it uses, what uses it)

For **Full Codebase** research:

1. Map the top-level directory structure
2. Identify major modules and their responsibilities
3. Document the overall architecture pattern

## Phase 4: Archive Existing Report

Before writing, check if `RESEARCH_REPORT.md` already exists in the project root.

If it exists:

1. Read the existing report to determine its scope
2. Create the archive directory if needed: `logs/research/`
3. Move the existing file to: `logs/research/report_[scope]-[NNN].md`
   - `[scope]` = kebab-case descriptor of the OLD report's scope (e.g., `authentication`, `full-codebase`, `storyService`)
   - `[NNN]` = incremented number based on existing archives of that scope (001, 002, etc.)

## Phase 5: Write the Report

Create `RESEARCH_REPORT.md` in the project root with the following structure:

```markdown
# Research Report: [Scope Description]

**Generated**: [YYYY-MM-DD]
**Scope Type**: [Feature/Module | File/Function Origin | Full Codebase]
**Related Epic**: [Current epic from project-state.yaml, if relevant]
**Related Tasks**: [List any task IDs from tasks.yaml that touch this scope]

---

## Overview

[2-3 sentences MAX. What is this and what does it do?]

---

## File Manifest

| File                 | Purpose           |
| -------------------- | ----------------- |
| `path/to/file.ts`    | Brief description |
| `path/to/another.ts` | Brief description |

---

## Data Flow

[Describe how data moves through the system for this scope. Use a simple diagram if helpful:]
```

Component → Hook → Service → API → Database
↑
State Update

```

---

## Dependencies

### Internal Dependencies
- `src/services/someService.ts` — Used for X
- `src/hooks/useSomeHook.ts` — Provides Y

### External Dependencies
- `package-name` — Purpose

---

## Patterns Observed

[Document coding patterns found in this scope]

- **Pattern Name**: Description of how it's applied
- **Pattern Name**: Description of how it's applied

---

## Anti-Patterns & Tech Debt

[Document any problematic patterns, inconsistencies, or areas needing improvement]

- **Issue**: Description and location
- **Issue**: Description and location

---

## State Management

[How is state handled in this scope? Local state, context, external stores?]

---

## PRD Alignment

[How does this code relate to requirements in PRD.md? Any gaps or drift?]

---

## Raw Notes

[Optional: Any additional observations, questions, or context that didn't fit above]
```

## Phase 6: Finalize

1. Review the report for completeness
2. Ensure all file paths are accurate
3. Confirm the report is saved to the project root as `RESEARCH_REPORT.md`
4. Summarize key findings to the user
