# PRD: Big 3 Agent Support (Claude, Codex, Gemini)

**Version**: 1.0
**Status**: Ready

---

## Problem

doug currently assumes Claude Code as the agent. Context files, skill directories, and init scaffolding are all Claude-specific. Users who want to run Codex CLI or Gemini CLI must manually restructure the project, and there is no guidance or tooling to support them. Additionally, `kb_enabled` lives in `project-state.yaml` — a runtime state file — when it is a project configuration decision that belongs in `doug.yaml`. A latent bug in `UpdateChangelog` inserts entries into the first matching subsection found anywhere in the file rather than explicitly targeting `## [Unreleased]`, which could silently corrupt changelogs if the file structure deviates from convention.

---

## Goal

`doug init` scaffolds a project that works out of the box with Claude Code, Codex CLI, and Gemini CLI. Skill files live in a shared `.agents/skills/` directory. A single `AGENTS.md` serves both Codex and Gemini. `kb_enabled` is a first-class field in `doug.yaml`. `UpdateChangelog` is explicitly `## [Unreleased]`-aware.

---

## Non-Goals

- README updates (handled via manual research skill run-through post-epic)
- Codex or Gemini-specific skill variants (shared SKILL.md content serves all three agents)
- Any new orchestration loop behavior
- Agent sandboxing or hard enforcement for Codex and Gemini (instruction-based only)

---

## Architecture

No architectural changes. All changes are confined to:

- `internal/agent/activetask.go` — skill path resolution
- `internal/config/config.go` — KBEnabled field
- `internal/types/types.go` — remove KBEnabled from ProjectState
- `internal/changelog/changelog.go` — Unreleased-aware insertion
- `cmd/init.go` — routing and template generation
- `internal/templates/init/` — new and updated template files

---

## Task Summary

| ID         | Description                                               |
| ---------- | --------------------------------------------------------- |
| EPIC-7-001 | Move skill resolution to .agents/skills/                  |
| EPIC-7-002 | Update init routing and add .gemini/settings.json         |
| EPIC-7-003 | Move SKILL.md templates to .agents/skills/ tree           |
| EPIC-7-004 | Update AGENTS.md content and include it in init scaffolding |
| EPIC-7-005 | Add Codex and Gemini examples to doug.yaml template       |
| EPIC-7-006 | Move kb_enabled from project-state.yaml to doug.yaml      |
| EPIC-7-007 | Fix UpdateChangelog to scope insertion to ## [Unreleased] |

---

## Task Notes

### EPIC-7-004 — AGENTS.md

`internal/templates/init/AGENTS.md` exists but has two problems:

1. **Stale paths**: references `.claude/skills/` (should be `.agents/skills/` after 001–003) and `logs/ACTIVE_TASK.md` (should be `.doug/ACTIVE_TASK.md`).
2. **Not stamped**: `copyInitTemplates` in `cmd/init.go` has an explicit skip for `AGENTS.md` — so it is never written to new projects. Remove it from the skip list so `doug init` copies it to `{project}/AGENTS.md`.

The file structure (skills table, agent contract, failure escalation) is correct and should be preserved. Only update paths and remove from the skip list. No new file needed.

---

## Definition of Done

- [ ] All tasks are DONE
- [ ] Build passes
- [ ] Tests pass
- [ ] `doug init` output reflects .agents/skills/ paths, AGENTS.md
- [ ] Claude, Codex, and Gemini can be set as agent_command and run successfully
- [ ] kb_enabled is absent from project-state.yaml and present in doug.yaml
