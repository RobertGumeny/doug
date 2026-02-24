---
name: implement-documentation
description: Expert technical document writer that synthesizes session logs into an atomic, cross-linked, in-repo knowledge base (KB) for agentic workflows. Topic-based organization with lean, high-signal articles.
allowed-tools: Read, Grep, Glob, LS, Write, Bash
---

# Knowledge Base Update Workflow

This skill transforms temporary session logs into a durable source of truth within the `docs/kb/` directory. It is designed for easy scanning and reference by autonomous coding agents.

## Agent Boundaries (Critical)

**You ARE allowed to:**

- ✅ Read all session logs in `logs/sessions/{epic}/*.md`
- ✅ Read `PRD.md` for product context
- ✅ Read existing `docs/kb/**/*.md` files
- ✅ Write/update files in `docs/kb/` directory
- ✅ Write session result to `logs/sessions/{epic}/session-KB_UPDATE_attempt-1.md`

**You are NOT allowed to:**

- ❌ Read `project-state.yaml` or `tasks.yaml` (not needed - session logs have all context)
- ❌ Run ANY Git commands
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive session logs
- ❌ Run `npm run dev`

**The orchestrator handles:** Git operations, YAML updates, session log archiving.

## Design Philosophy

**Lean & High-Signal**: Every KB article should answer "What was built and why?" not "How did we build it step-by-step?"

**Progressive Disclosure**: Start with high-level overview, then provide technical details.

**Update-First**: Prefer updating existing articles over creating new ones.

**Cross-Linked**: Every article should point to related topics.

## Phase 1: Ingestion & Audit

### 1.1 Scan Session Logs

Read all session result files in `logs/sessions/{epic}/*.md` where `outcome: SUCCESS`.

**Extract from each session:**

- `task_id` - What task was completed
- `files_modified` - What code changed
- `changelog_entry` - User-facing description
- Session body - Technical decisions, patterns used, dependencies added

**Ignore:**

- Sessions with `outcome: FAILURE` or `outcome: BUG` (incomplete work)
- Bug report files
- Failure report files

### 1.2 Map Existing KB

Scan `docs/kb/` directory structure:

```bash
find docs/kb -name "*.md" -type f
```

For each existing KB article, read **only the frontmatter** to extract:

- `title`
- `category`
- `tags`
- `updated` (last update date)

Build an index of:

- What topics already exist
- When they were last updated
- What tags are in use

### 1.3 Read PRD for Context

Scan `PRD.md` to understand:

- Product goals and constraints
- Features already documented in PRD
- Architectural decisions already stated

**Goal**: Avoid duplicating information that's already in the PRD. KB should focus on _implementation details_ and _lessons learned_, not product requirements.

### 1.4 Check Dependencies

Compare session logs against `package.json`:

- Identify any new libraries added during the epic
- Check if `docs/kb/dependencies/` has articles for them
- Prioritize documenting new/unfamiliar libraries

## Phase 2: Categorization

Group session findings into KB topics. Each topic should be atomic and focused.

### 2.1 Topic Extraction Rules

**Architecture** - How major systems are structured:

- State management approach
- Data flow patterns
- Application shell design
- Module boundaries

**Patterns** - Reusable code patterns:

- Custom hooks
- Service layer patterns
- Error handling strategies
- Testing patterns

**Integration** - How external systems connect:

- API integrations
- Third-party service wrappers
- Authentication flows

**Infrastructure** - Build, deploy, tooling:

- Build configuration
- Testing infrastructure
- Development workflow
- Linting/formatting setup

**Dependencies** - External libraries:

- Why library was chosen
- How it's configured
- Common usage patterns
- Gotchas or limitations

**Features** - User-facing capabilities:

- What the feature does (from user perspective)
- Key implementation decisions
- Edge cases handled
- Related components

### 2.2 Map Sessions to Topics

For each successful session, determine:

1. **Primary topic** - The main thing this session accomplished
2. **Secondary topics** - Other areas it touched

**Example Mapping:**

Session: "Implemented JWT token generation"

- Primary: `dependencies/jsonwebtoken.md` (new library)
- Secondary: `patterns/authentication-service.md` (new pattern)

Session: "Added story reader navigation"

- Primary: `features/story-reader.md` (feature implementation)
- Secondary: `patterns/state-based-routing.md` (pattern used)

### 2.3 Determine Update vs. Create

For each topic:

- **If KB article exists**: Plan to UPDATE it (add new section or revise)
- **If no article exists**: Plan to CREATE it

**Prefer updates** to avoid KB fragmentation.

## Phase 3: Synthesis

Write lean, focused articles. Follow the standard structure for all KB documents.

### 3.1 KB Article Structure

Every article must follow this format:

```markdown
---
title: [Human Readable Title]
updated: [YYYY-MM-DD]
category:
  [
    Architecture | Patterns | Integration | Infrastructure | Dependency | Features,
  ]
tags: [e.g., react, state-management, navigation]
related_articles:
  - docs/kb/path-to-related-1.md
  - docs/kb/path-to-related-2.md
---

# [Title]

## Overview

[2-3 sentence summary of what this is and why it exists]

## Implementation

[Key technical details - what was built, how it works]

## Key Decisions

- **Decision 1**: Rationale
- **Decision 2**: Rationale

## Usage Example (if applicable)

[Brief code snippet showing how to use this pattern/feature/dependency]

## Edge Cases & Gotchas (if applicable)

- Edge case 1 and how it's handled
- Known limitation or quirk

## Related Topics

See [related article 1](../path/to/article.md) for more on X.
```

### 3.2 Writing Guidelines

**DO:**

- ✅ Focus on "what" and "why", not "how we got there"
- ✅ Use concrete code examples (2-5 lines max)
- ✅ Document non-obvious decisions
- ✅ Note edge cases and limitations
- ✅ Keep total article length under 200 lines

**DON'T:**

- ❌ Chronicle the development process ("First we tried X, then Y")
- ❌ Include entire file contents
- ❌ Duplicate information from PRD
- ❌ Document things that are obvious from the code
- ❌ Write tutorials (this is reference material, not teaching)

### 3.3 Update Strategy

When updating an existing article:

1. Read the full article first
2. Identify which section to update:
   - New implementation details? → Add to "Implementation"
   - New decision made? → Add to "Key Decisions"
   - New edge case discovered? → Add to "Edge Cases & Gotchas"
3. Update the `updated` date in frontmatter
4. Add new `related_articles` if relevant
5. Keep the article cohesive - merge related points, don't just append

**Don't create version history inside articles** - Git tracks changes.

## Phase 4: Cross-Linking

Ensure all articles reference related topics.

### 4.1 Identify Relationships

For each article you created/updated, identify:

- **Dependencies**: What libraries or patterns does this rely on?
- **Dependents**: What features or patterns rely on this?
- **Alternatives**: What other approaches exist for the same problem?

### 4.2 Update Frontmatter

Add to `related_articles` array:

- Articles that provide context for this one
- Articles that build on this one
- Articles that solve similar problems differently

**Bidirectional linking**: If Article A links to Article B, make sure Article B links back to Article A (where relevant).

### 4.3 Cross-Link in Body

In the "Related Topics" section, explain the relationship:

```markdown
## Related Topics

- See [State Management](../architecture/state-management.md) for how this pattern fits into the overall architecture
- See [React Router](../dependencies/react-router.md) for the routing library this depends on
- Alternative approach: [Hash-based Routing](../patterns/hash-routing.md)
```

## Phase 5: Verification & Report

### 5.1 Self-Review Checklist

Before finalizing, verify:

- ✅ All new/updated articles have valid frontmatter
- ✅ All articles are in correct category subdirectory
- ✅ No article exceeds ~200 lines
- ✅ All `related_articles` paths are valid (files exist)
- ✅ No information duplicates the PRD
- ✅ Code examples are concise (2-5 lines)
- ✅ Focus is on "what/why" not "how we got here"

### 5.2 Directory Structure Validation

Ensure the KB follows this structure:

```
docs/kb/
├── architecture/
│   └── *.md
├── patterns/
│   └── *.md
├── integration/
│   └── *.md
├── infrastructure/
│   └── *.md
├── dependencies/
│   └── *.md
└── features/
    └── *.md
```

If you created new categories, document why in your session result.

### 5.3 Write Session Result

**Path**: `logs/sessions/{epic}/session-KB_UPDATE_attempt-1.md`

Where `{epic}` is the epic you just synthesized (extract from session log filenames).

**Frontmatter**:

```yaml
---
task_id: "KB_UPDATE"
outcome: "EPIC_COMPLETE"
timestamp: "2025-02-06T10:30:00Z"
duration_seconds: 600
estimated_tokens: 60000
files_modified:
  - docs/kb/features/story-reader.md
  - docs/kb/dependencies/react-router.md
  - docs/kb/patterns/state-routing.md
tests_run: 0
tests_passed: 0
build_successful: true
kb_articles_created: 2
kb_articles_updated: 1
---

## KB Synthesis Summary

Synthesized session logs from EPIC-2 into 3 KB articles covering the Story Reader feature implementation.

## Articles Created

- `docs/kb/features/story-reader.md` - Story reader UI and navigation
- `docs/kb/dependencies/react-router.md` - React Router integration

## Articles Updated

- `docs/kb/patterns/state-routing.md` - Added section on reader view routing

## Key Topics Documented

- Story reader feature with chapter/page navigation
- React Router setup and configuration
- State-based routing pattern for view transitions

## Cross-Links Added

- Story Reader ↔ React Router ↔ State Routing pattern
```

**Body**:

```markdown
## Implementation Summary

Reviewed 4 successful session logs from EPIC-2 and synthesized them into 3 KB articles organized by technical topic.

## Files Changed

- `docs/kb/features/story-reader.md` (created) - Documents story reader implementation
- `docs/kb/dependencies/react-router.md` (created) - Documents React Router setup
- `docs/kb/patterns/state-routing.md` (updated) - Added reader routing patterns

## Synthesis Decisions

- Grouped reader UI + navigation into single feature article (cohesive user-facing capability)
- Created dedicated React Router article (new dependency worth documenting)
- Updated existing state routing pattern article rather than creating new one

## Coverage

- Epic: EPIC-2 (Library & Reader Interface)
- Sessions reviewed: 4
- Articles created: 2
- Articles updated: 1
- Total KB articles: 5
```

**Metrics Tracking (Required):**

The orchestrator tracks task duration and token usage. Include these fields:

- `duration_seconds`: Estimated time spent on KB synthesis (in seconds)
  - Include time reading session logs, researching code, writing articles, cross-linking
  - Documentation tasks vary widely based on epic size
  - Example: 600 (10 minutes), 1800 (30 minutes), 3600 (1 hour)

- `estimated_tokens`: Rough estimate of tokens consumed
  - Count all characters in session logs you read
  - Count all characters in KB articles you wrote
  - Count all characters in code files you referenced
  - Divide total by 4 for rough token count
  - Multiply by 1.5x for documentation (less overhead than features)
  - Example calculation:
    - Read 20,000 chars (session logs) + wrote 10,000 chars (KB articles) = 30,000 chars total
    - 30,000 / 4 = 7,500 base tokens
    - 7,500 × 1.5 = 11,250 estimated tokens (for documentation work)
  - Round to nearest 1000 or 5000 for simplicity
  - Documentation typically uses more tokens due to reading many session logs

**Then exit with `outcome: EPIC_COMPLETE`**.

The orchestrator will:

- Commit the KB updates
- Mark the epic as DONE
- Archive session logs
- Stop execution

## Quick Reference

**Categories:**

- `architecture/` - System structure and design
- `patterns/` - Reusable code patterns
- `integration/` - External system connections
- `infrastructure/` - Build, test, deploy tooling
- `dependencies/` - External libraries
- `features/` - User-facing capabilities

**Article Max Length:** ~200 lines

**Focus:** What was built + why, not how we got there

**Update Strategy:** Prefer updating existing articles over creating new ones

**Outcome:** Always `EPIC_COMPLETE` (signals epic is fully done)

**Session Result Path:** `logs/sessions/{epic}/session-KB_UPDATE_attempt-1.md`

**Required Session Result Fields:**

- `outcome`: EPIC_COMPLETE
- `timestamp`: ISO 8601 format
- `duration_seconds`: (REQUIRED) Time spent on task in seconds
- `estimated_tokens`: (REQUIRED) Rough token count estimate
- `files_modified`: Array of KB articles created/updated
- `tests_run`: Always 0 for documentation
- `tests_passed`: Always 0 for documentation
- `build_successful`: Always true for documentation
- `kb_articles_created`: Number of new articles
- `kb_articles_updated`: Number of updated articles

---

**Version:** 1.0  
**Last Updated:** 2025-02-06
