---
title: internal/templates — Embedded Template Files
updated: 2026-07-03
category: Packages
tags: [templates, embed, go-embed, init]
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
| `init/` | Files stamped into a new project by `doug init` |

---

## Exports

```go
// Init holds files copied to the target project by `doug init`.
// Uses explicit patterns so only current managed init surfaces are embedded.
//go:embed init/.gitignore init/AGENTS.md init/CLAUDE.md
//go:embed init/BUG_REPORT_TEMPLATE.md
//go:embed init/skills
//go:embed all:init/.pi
var Init embed.FS
```

`Init` is the only exported embed. There is no `Runtime` embed, `SessionResult` string, or scaffolded session-result template. The orchestrator generates `ACTIVE_TASK.md` programmatically via `agent.WriteActiveTask`, and that file is the sole managed result handshake surface.

---

## init/ Contents

Files in `init/` are copied verbatim by `cmd/init.copyInitTemplates`. See [cmd/init](init.md) for destination routing.

| File | Destination in new project |
|------|---------------------------|
| `CLAUDE.md` | `{project}/CLAUDE.md` |
| `AGENTS.md` | `{project}/AGENTS.md` with a delimited doug-specific section |
| `skills/implement-feature/SKILL.md` | `{project}/.pi/skills/implement-feature/SKILL.md` |
| `skills/implement-bugfix/SKILL.md` | `{project}/.pi/skills/implement-bugfix/SKILL.md` |
| `skills/implement-documentation/SKILL.md` | `{project}/.pi/skills/implement-documentation/SKILL.md` |
| `skills/plan/**` | `{project}/.pi/skills/plan/**` |
| `skills/scaffold/SKILL.md` | `{project}/.pi/skills/scaffold/SKILL.md` |
| `skills/research/SKILL.md` | `{project}/.pi/skills/research/SKILL.md` |
| `.gitignore` | `{project}/.gitignore` (created if missing; otherwise merged to ensure `.doug/` is ignored) |
| `.pi/extensions/handoff.ts` | `{project}/.pi/extensions/handoff.ts` (always; optional Pi-native handoff helper, not a Doug runtime authority file) |
| `BUG_REPORT_TEMPLATE.md` | `{project}/.doug/logs/BUG_REPORT_TEMPLATE.md` |

The embedded init inventory matches the supported Pi-first artifact set directly. There is no scaffolded failure-report template; task failures are reported in `ACTIVE_TASK.md` results and infra failures use durable logs.

**`AGENTS.md` carries repo policy; launch prompts carry transient routing; skills carry workflow**: The init `AGENTS.md` template is a delimited doug-specific section that is created or appended into the project root `AGENTS.md`. It defines stable repository operating rules, including the conditional rule that `.doug/ACTIVE_TASK.md` is authoritative only for doug-managed runs. The skill templates are intentionally workflow-centric, including the `plan` and `scaffold` skills, and launch prompts are where doug points the agent at the active briefing artifact for a specific orchestrated run. Planning follows the same universal brief contract: root `.doug/ACTIVE_TASK.md` is the canonical brief, while `.doug/plan/PLAN.md` remains the editable downstream workbook.

**Skill packages may include supporting files**: Files under `init/skills/**` are copied into `.pi/skills/` with relative paths preserved. This allows complex skills such as `plan` to ship `references/` files and other supporting material for progressive disclosure.

**`.pi/extensions/handoff.ts` is a scaffolded extension surface, not an orchestrator input**: The file is copied into every initialized project so Pi users have a ready-made interactive handoff helper, but Doug's runtime does not read `.pi/extensions/*` when executing `doug run`. Doug's canonical runtime inputs remain `.doug/ACTIVE_TASK.md`, the resolved initial Pi prompt, and the Pi-only execution contract.

## Follow-Up Notes

- If additional `.pi/extensions/*` files are added later, document the purpose and runtime authority of each one individually. The template inventory alone is not sufficient product guidance.

---

## Adding New Templates

**New init template**: Add the file to `internal/templates/init/`. Then add an explicit `//go:embed` pattern for the file or its directory in `templates.go` and add a routing case in `cmd/init.routeTemplateFile` — unknown files emit a warning and are skipped, so the file will not be copied without a matching case. If the new file is under a hidden directory (dot-prefix), use `all:<path>` in the embed directive (e.g., `//go:embed all:init/.pi`).

**No `..` paths in embed directives**: Go's `//go:embed` does not allow `..` in paths. Templates must live inside the `internal/templates/` package directory.

---

## Key Decisions

**Single `Init embed.FS`**: Only `Init` is exported. The init tree now contains only supported Doug and Pi artifacts. The orchestrator generates `ACTIVE_TASK.md` programmatically rather than from a separate outcome template.

**Explicit embed patterns, not `all:init`**: The old `//go:embed all:init` directive embedded everything under `init/`. The current directive uses explicit per-path patterns so that new template files must be consciously added to the embed list rather than being silently included.

---

## Edge Cases & Gotchas

**`AGENTS.md` is merged by `cmd/init`**: Unlike most init templates, `AGENTS.md` is not treated as a simple copy. `cmd/init` appends the doug-specific section only when its marker is absent.

**`embed.FS` paths use forward slashes**: Always use `/` separators with `embed.FS.ReadFile`, even on Windows. `filepath.Join` is wrong here — use explicit forward-slash strings.

---

## Related Topics

- [internal/agent](agent.md) — `WriteActiveTask` generates ACTIVE_TASK.md programmatically; `ParseSessionResult` parses it back
- [cmd/init](init.md) — `copyInitTemplates` uses `templates.Init`
- [Go Infrastructure](../infrastructure/go.md) — project structure, `//go:embed` placement rules
