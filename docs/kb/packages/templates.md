---
title: internal/templates — Embedded Template Files
updated: 2026-05-01
category: Packages
tags: [templates, embed, go-embed, session-result, init, runtime]
related_articles:
  - docs/kb/packages/agent.md
  - docs/kb/packages/init.md
  - docs/kb/infrastructure/go.md
---

# internal/templates — Embedded Template Files

## Overview

`internal/templates/templates.go` embeds all template files into the binary at build time using `//go:embed`. No runtime disk paths — all templates are compiled in.

Two subdirectories serve distinct purposes:

| Directory | Purpose |
|-----------|---------|
| `runtime/` | Templates used internally by the orchestrator (never copied to user projects) |
| `init/` | Files stamped into a new project by `doug init` |

---

## Exports

```go
// Runtime holds templates used by the orchestrator at runtime.
//go:embed runtime
var Runtime embed.FS

// Init holds files copied to the target project by `doug init`.
// Uses "all:" prefix to include hidden directories (e.g., .gemini/).
//go:embed all:init
var Init embed.FS

// SessionResult is the content of runtime/session_result.md.
// Convenience accessor used by CreateSessionFile.
//go:embed runtime/session_result.md
var SessionResult string
```

`SessionResult` is a convenience `string` for `internal/agent.CreateSessionFile` — it avoids a `ReadFile` call on every session creation. `Runtime` and `Init` are `embed.FS` for directory-level access.

---

## runtime/session_result.md

The pre-filled template the orchestrator writes before invoking the agent. The agent fills it out and the orchestrator reads it back via `ParseSessionResult`.

**Exact frontmatter** (3 fields only):

```yaml
---
outcome: ""
changelog_entry: ""
dependencies_added: []
---
```

No `task_id`, no `timestamp`, no `files_modified`, no `tests_run`, no `build_successful`. Those are **dead fields** — the orchestrator never reads them. Agents may write them as self-documentation, but `ParseSessionResult` (via `yaml.Unmarshal`) silently discards any key not in `types.SessionResult`.

---

## init/ Contents

Files in `init/` are copied verbatim by `cmd/init.copyInitTemplates`. See [cmd/init](init.md) for destination routing.

| File | Destination in new project |
|------|---------------------------|
| `CLAUDE.md` | `{project}/CLAUDE.md` |
| `AGENTS.md` | `{project}/AGENTS.md` with a delimited doug-specific section |
| `skills-config.yaml` | — (removed; skill selection is handled by `policy.tasks[type].skill` in `doug.yaml`) |
| `skills/implement-feature/SKILL.md` | `{project}/.claude/skills/implement-feature/SKILL.md`, `{project}/.codex/skills/implement-feature/SKILL.md`, and/or `{project}/.gemini/skills/implement-feature/SKILL.md` depending on selected agents |
| `skills/implement-bugfix/SKILL.md` | `{project}/.claude/skills/implement-bugfix/SKILL.md`, `{project}/.codex/skills/implement-bugfix/SKILL.md`, and/or `{project}/.gemini/skills/implement-bugfix/SKILL.md` depending on selected agents |
| `skills/implement-documentation/SKILL.md` | `{project}/.claude/skills/implement-documentation/SKILL.md`, `{project}/.codex/skills/implement-documentation/SKILL.md`, and/or `{project}/.gemini/skills/implement-documentation/SKILL.md` depending on selected agents |
| `skills/manual-review/SKILL.md` | `{project}/.claude/skills/manual-review/SKILL.md`, `{project}/.codex/skills/manual-review/SKILL.md`, and/or `{project}/.gemini/skills/manual-review/SKILL.md` depending on selected agents |
| `skills/plan/**` | `{project}/.claude/skills/plan/**`, `{project}/.codex/skills/plan/**`, and/or `{project}/.gemini/skills/plan/**` depending on selected agents |
| `skills/scaffold/SKILL.md` | `{project}/.claude/skills/scaffold/SKILL.md`, `{project}/.codex/skills/scaffold/SKILL.md`, and/or `{project}/.gemini/skills/scaffold/SKILL.md` depending on selected agents |
| `skills/research/SKILL.md` | `{project}/.claude/skills/research/SKILL.md`, `{project}/.codex/skills/research/SKILL.md`, and/or `{project}/.gemini/skills/research/SKILL.md` depending on selected agents |
| `.claude/settings.json` | `{project}/.claude/settings.json` |
| `.codex/config.toml` | `{project}/.codex/config.toml` |
| `.gitignore` | `{project}/.gitignore` (created if missing; otherwise merged to ensure `.doug/` is ignored) |
| `.gemini/settings.json` | `{project}/.gemini/settings.json` |
| `.gemini/policies/doug-default.json` | `{project}/.gemini/policies/doug-default.json` |
| `.pi/extensions/handoff.ts` | `{project}/.pi/extensions/handoff.ts` (always) |
| `SESSION_RESULTS_TEMPLATE.md` | `{project}/.doug/logs/SESSION_RESULTS_TEMPLATE.md` |
| `BUG_REPORT_TEMPLATE.md` | `{project}/.doug/logs/BUG_REPORT_TEMPLATE.md` |
| `FAILURE_REPORT_TEMPLATE.md` | `{project}/.doug/logs/FAILURE_REPORT_TEMPLATE.md` |

**`AGENTS.md` carries repo policy; launch prompts carry transient routing; skills carry workflow**: The init `AGENTS.md` template is a delimited doug-specific section that is created or appended into the project root `AGENTS.md`. It defines stable repository operating rules, including the conditional rule that `.doug/ACTIVE_TASK.md` is authoritative only for doug-managed runs. The skill templates are intentionally workflow-centric, including the `plan` and `scaffold` skills, and launch prompts are where doug points the agent at the active briefing artifact for a specific orchestrated run. Planning follows the same universal brief contract: root `.doug/ACTIVE_TASK.md` is the canonical brief, while `.doug/plan/PLAN.md` remains the editable downstream workbook.

**Skill packages may include supporting files**: Files under `init/skills/**` are copied into each selected provider's local skills directory with relative paths preserved. This allows complex skills such as `plan` to ship `references/` files and other supporting material for progressive disclosure without adding provider-specific content.

**`SESSION_RESULTS_TEMPLATE.md` vs `runtime/session_result.md`**: These are distinct files serving different purposes. Both share the 3-field frontmatter shape, but `SESSION_RESULTS_TEMPLATE.md` is for human agents to reference in the target project, while `runtime/session_result.md` is used internally by session-file creation and compatibility helpers.

---

## Adding New Templates

**New runtime template**: Add the file to `internal/templates/runtime/`. Access via `templates.Runtime.ReadFile("runtime/filename.md")` or add a new `string` convenience var if used frequently.

**New init template**: Add the file to `internal/templates/init/`. Then add a routing case in `cmd/init.copyInitTemplates` — unknown files emit a warning and are skipped, so the file will not be copied without a matching case. If the new file is in a hidden directory (dot-prefix), ensure the `//go:embed all:init` directive is already present (it is).

**Hidden directories require `all:init`**: Go's `//go:embed` skips hidden directories (dot-prefix like `.gemini/`) without the `all:` prefix. The embed directive is `//go:embed all:init` for this reason.

**No `..` paths in embed directives**: Go's `//go:embed` does not allow `..` in paths. Templates must live inside the `internal/templates/` package directory.

---

## Key Decisions

**Two separate `embed.FS` vars**: `Runtime` and `Init` are kept separate so the `agent` package can import only `templates.SessionResult` (a string) without carrying the entire `init/` tree. The compiler does not tree-shake `embed.FS` contents.

**`SessionResult` as `string`, not `[]byte`**: `os.WriteFile` accepts `[]byte`, so callers do `[]byte(templates.SessionResult)`. The string form is more readable in tests and avoids the `embed.FS.ReadFile` call overhead on the hot path.

**Template written as-is**: `CreateSessionFile` writes `templates.SessionResult` directly without string substitution. There are no `{{placeholder}}` tokens and no `strings.ReplaceAll` calls. The 3-field frontmatter is always identical; the agent fills in the actual values.

---

## Edge Cases & Gotchas

**`AGENTS.md` is merged by `cmd/init`**: Unlike most init templates, `AGENTS.md` is not treated as a simple copy. `cmd/init` appends the doug-specific section only when its marker is absent.

**`embed.FS` paths use forward slashes**: Always use `/` separators with `embed.FS.ReadFile`, even on Windows. `filepath.Join` is wrong here — use explicit forward-slash strings.

---

## Related Topics

- [internal/agent](agent.md) — `CreateSessionFile` uses `templates.SessionResult`
- [cmd/init](init.md) — `copyInitTemplates` uses `templates.Init`
- [Go Infrastructure](../infrastructure/go.md) — project structure, `//go:embed` placement rules
