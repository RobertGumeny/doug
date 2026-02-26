---
title: internal/templates — Embedded Template Files
updated: 2026-02-25
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
//go:embed init
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
| `AGENTS.md` | `{project}/AGENTS.md` |
| `SESSION_RESULTS_TEMPLATE.md` | `{project}/logs/SESSION_RESULTS_TEMPLATE.md` |
| `BUG_REPORT_TEMPLATE.md` | `{project}/logs/BUG_REPORT_TEMPLATE.md` |
| `FAILURE_REPORT_TEMPLATE.md` | `{project}/logs/FAILURE_REPORT_TEMPLATE.md` |
| `skills/implement-feature/SKILL.md` | `{project}/.claude/skills/implement-feature/SKILL.md` |
| `skills/implement-bugfix/SKILL.md` | `{project}/.claude/skills/implement-bugfix/SKILL.md` |
| `skills/implement-documentation/SKILL.md` | `{project}/.claude/skills/implement-documentation/SKILL.md` |

**`SESSION_RESULTS_TEMPLATE.md` vs `runtime/session_result.md`**: These are distinct files serving different purposes. Both share the 3-field frontmatter shape, but `SESSION_RESULTS_TEMPLATE.md` is for human agents to reference in the target project, while `runtime/session_result.md` is used internally by `CreateSessionFile`.

---

## Adding New Templates

**New runtime template**: Add the file to `internal/templates/runtime/`. Access via `templates.Runtime.ReadFile("runtime/filename.md")` or add a new `string` convenience var if used frequently.

**New init template**: Add the file to `internal/templates/init/`. Then add a routing case in `cmd/init.copyInitTemplates` — unknown files are silently skipped, so the file will not be copied without a matching case.

**No `..` paths in embed directives**: Go's `//go:embed` does not allow `..` in paths. Templates must live inside the `internal/templates/` package directory.

---

## Key Decisions

**Two separate `embed.FS` vars**: `Runtime` and `Init` are kept separate so the `agent` package can import only `templates.SessionResult` (a string) without carrying the entire `init/` tree. The compiler does not tree-shake `embed.FS` contents.

**`SessionResult` as `string`, not `[]byte`**: `os.WriteFile` accepts `[]byte`, so callers do `[]byte(templates.SessionResult)`. The string form is more readable in tests and avoids the `embed.FS.ReadFile` call overhead on the hot path.

**Template written as-is**: `CreateSessionFile` writes `templates.SessionResult` directly without string substitution. There are no `{{placeholder}}` tokens and no `strings.ReplaceAll` calls. The 3-field frontmatter is always identical; the agent fills in the actual values.

---

## Edge Cases & Gotchas

**Stale `init/skills/` copies**: The skill files in `init/skills/` are copied from `internal/templates/` at build time. If you update the top-level `internal/templates/skills/` files, you must also update the copies in `internal/templates/init/skills/` — they are not symlinked.

**`embed.FS` paths use forward slashes**: Always use `/` separators with `embed.FS.ReadFile`, even on Windows. `filepath.Join` is wrong here — use explicit forward-slash strings.

---

## Related Topics

- [internal/agent](agent.md) — `CreateSessionFile` uses `templates.SessionResult`
- [cmd/init](init.md) — `copyInitTemplates` uses `templates.Init`
- [Go Infrastructure](../infrastructure/go.md) — project structure, `//go:embed` placement rules
