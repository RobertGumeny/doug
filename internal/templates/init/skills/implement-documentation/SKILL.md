---
name: implement-documentation
description: Expert technical document writer that synthesizes session logs into an atomic, cross-linked, in-repo knowledge base (KB) for agentic workflows. Topic-based organization with lean, high-signal articles.
allowed-tools: Read, Grep, Glob, LS, Write, Bash
---

# Knowledge Base Update Workflow

This skill transforms temporary session logs into a durable source of truth within the `docs/kb/` directory. Designed for easy scanning and reference by autonomous coding agents.

## Agent Boundaries (Critical)

**You ARE allowed to:**

- ✅ Read all session logs in `logs/sessions/{epic}/*.md`
- ✅ Read `PRD.md` for product context
- ✅ Read existing `docs/kb/**/*.md` files
- ✅ Write/update files in `docs/kb/` directory
- ✅ Write session result to `logs/sessions/{epic}/session-KB_UPDATE_attempt-1.md`

**You are NOT allowed to:**

- ❌ Read `project-state.yaml` or `tasks.yaml` (not needed — session logs have all context)
- ❌ Run ANY Git commands
- ❌ Modify `CHANGELOG.md`
- ❌ Move or archive session logs

**The orchestrator handles:** Git operations, YAML updates, session log archiving.

## Design Philosophy

**Lean & High-Signal**: Every KB article answers "What was built and why?" — not "How did we build it step-by-step?"

**Update-First**: Prefer updating existing articles over creating new ones.

**Cross-Linked**: Every article should point to related topics.

## Phase 1: Ingestion

1. Read all `outcome: SUCCESS` session logs from `logs/sessions/{epic}/*.md`
2. Scan `docs/kb/` to index existing articles (title, category, tags)
3. Read `PRD.md` — avoid duplicating information already there; KB focuses on implementation details and lessons learned

## Phase 2: Categorization

Group findings into KB topics. Map each to an existing article (update) or a new one (create). Prefer updates.

**Categories:**
- `architecture/` — system structure and design
- `patterns/` — reusable code patterns
- `integration/` — external system connections
- `infrastructure/` — build, test, deploy tooling
- `dependencies/` — external libraries
- `features/` — user-facing capabilities

## Phase 3: Write Articles

Every article must follow this structure:

```markdown
---
title: [Human Readable Title]
updated: [YYYY-MM-DD]
category: [Architecture | Patterns | Integration | Infrastructure | Dependency | Features]
tags: [e.g., go, state-management]
related_articles:
  - docs/kb/path-to-related.md
---

# [Title]

## Overview
[2-3 sentence summary]

## Implementation
[Key technical details]

## Key Decisions
- **Decision**: Rationale

## Usage Example (if applicable)
[Brief code snippet, 2-5 lines]

## Edge Cases & Gotchas (if applicable)
- Known limitation or quirk

## Related Topics
See [related article](../path/to/article.md) for more on X.
```

**Guidelines:** Focus on "what/why" not "how we got there". Keep articles under ~200 lines. No entire file contents. No development process chronicles.

## Phase 4: Cross-Link

Ensure bidirectional linking between related articles. Update `related_articles` frontmatter on both sides.

## Phase 5: Report

**Session result path**: `logs/sessions/{epic}/session-KB_UPDATE_attempt-1.md`

```yaml
---
outcome: "EPIC_COMPLETE"
changelog_entry: ""
dependencies_added: []
---

## KB Synthesis Summary
## Articles Created
## Articles Updated
## Key Topics Documented
```

**Outcome is always `EPIC_COMPLETE`** — KB synthesis is the final step of every epic.
