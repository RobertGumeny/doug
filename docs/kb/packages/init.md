---
title: cmd/init — Project Scaffolding Subcommand
updated: 2026-03-06
category: Packages
tags: [init, scaffold, subcommand, templates, build-system, cobra, changelog]
related_articles:
  - docs/kb/packages/templates.md
  - docs/kb/packages/config.md
  - docs/kb/packages/switch.md
  - docs/kb/infrastructure/go.md
---

# cmd/init — Project Scaffolding Subcommand

## Overview

`cmd/init.go` implements the `doug init` subcommand. It scaffolds a new doug project by:

1. Generating `.doug/doug.yaml`, `.doug/tasks.yaml`, `.doug/project-state.yaml`, `.doug/PRD.md`, and `CHANGELOG.md` from inline content
2. Copying embedded `init/` template files into the target directory
3. Prompting for agent selection (TTY) or defaulting to `claude` (non-TTY / `--agents` flag)

The testable core is `initProject(dir, force, buildSystem string, selectedAgents []string) error`. The Cobra command handler (`runInit`) calls `os.Getwd()`, resolves agent selection, and delegates.

---

## Guard Check

Before writing any files, `initProject` checks whether the project is already initialized:

```go
if _, statErr := os.Stat(filepath.Join(dougDir, "project-state.yaml")); statErr == nil {
    return fmt.Errorf(".doug/project-state.yaml already exists — ...")
}
```

- Triggered by: `.doug/project-state.yaml` already present
- `--force` skips this check entirely
- Other generated files emit a `log.Warning` and skip if they already exist — they do not error

---

## Agent Selection

`doug init` prompts for agent selection interactively (on a TTY) or accepts `--agents` flag values.

- Default: `claude` when no input is provided or in non-TTY mode
- `--agents claude,gemini` to select multiple agents non-interactively
- All skill files are copied to the shared `.agents/skills/` directory regardless of selected agents
- No per-agent config files (e.g., `.gemini/settings.json`) are created by `doug init` — these are the user's responsibility

---

## Build System Detection

Build system precedence: `--build-system` flag > `config.DetectBuildSystem(dir)` > default `"go"`.

```go
bs := buildSystem           // flag value
if bs == "" {
    bs = config.DetectBuildSystem(dir)
}
```

`DetectBuildSystem` checks for `go.mod` (→ `"go"`) and `package.json` (→ `"npm"`). See [internal/config](config.md).

---

## Generated Files

| File | Content source | Notes |
|------|----------------|-------|
| `.doug/doug.yaml` | `dougYAMLContent(bs)` | All config fields with inline YAML comments; build system interpolated; includes commented Codex/Gemini `agent_command` examples; `agent_command` value is single-quoted to avoid YAML parse errors |
| `.doug/tasks.yaml` | `tasksYAMLContent()` | One example epic, two tasks, all required fields |
| `.doug/project-state.yaml` | `"{}\n"` | Empty YAML; `BootstrapFromTasks` populates on first run |
| `.doug/PRD.md` | `prdContent()` | Blank template with section headers |
| `CHANGELOG.md` | `changelogContent()` | Keep a Changelog format; `[Unreleased]` section; **never overwritten** even with `--force` |

All are written with `os.WriteFile` (not atomic rename — new files, no corruption risk). `CHANGELOG.md` is skipped entirely if it already exists, regardless of `--force`.

### Commented agent_command examples

`dougYAMLContent` generates a `doug.yaml` with the default `agent_command` single-quoted, plus two commented-out alternatives immediately after:

```yaml
agent_command: 'claude --dangerously-skip-permissions -p "[DOUG_TASK_ID: {{task_id}}] ..."'
# agent_command: codex
# agent_command: gemini {project_dir}
```

Single-quoting is required because the value contains `[DOUG_TASK_ID: ` (colon-space), which YAML interprets as a key-value separator in plain scalars. Single-quoted scalars allow embedded double-quotes and colons without escaping. See [cmd/switch](switch.md) for the matching fix applied to the write path.

---

## copyInitTemplates

```go
func copyInitTemplates(dir string, force bool, selectedAgents []string) error
```

Walks `templates.Init` (embedded `init/` FS) and routes each file to its destination:

| Pattern | Destination |
|---------|-------------|
| `CLAUDE.md` | **skipped** (not copied to new projects) |
| `AGENTS.md` | `{dir}/AGENTS.md` |
| `skills-config.yaml` | `{dir}/.doug/skills-config.yaml` |
| `skills/**` | `{dir}/.agents/skills/{rel}` |
| `.gitignore` | `{dir}/.gitignore` |
| `*_TEMPLATE.md` | `{dir}/.doug/logs/{filename}` |
| anything else | logged warning, silently skipped |

**No filename transformations.** Files land at their exact source names — no `_TEMPLATE` suffix stripping.

Parent directories are created with `os.MkdirAll(filepath.Dir(dst), 0o755)` before each write.

---

## init/ Template Inventory

Files embedded in `internal/templates/init/`:

| File | Destination in new project |
|------|---------------------------|
| `CLAUDE.md` | **skipped** |
| `AGENTS.md` | `{dir}/AGENTS.md` |
| `skills-config.yaml` | `{dir}/.doug/skills-config.yaml` |
| `skills/implement-feature/SKILL.md` | `{dir}/.agents/skills/implement-feature/SKILL.md` |
| `skills/implement-bugfix/SKILL.md` | `{dir}/.agents/skills/implement-bugfix/SKILL.md` |
| `skills/implement-documentation/SKILL.md` | `{dir}/.agents/skills/implement-documentation/SKILL.md` |
| `.gitignore` | `{dir}/.gitignore` |
| `SESSION_RESULTS_TEMPLATE.md` | `{dir}/.doug/logs/SESSION_RESULTS_TEMPLATE.md` |
| `BUG_REPORT_TEMPLATE.md` | `{dir}/.doug/logs/BUG_REPORT_TEMPLATE.md` |
| `FAILURE_REPORT_TEMPLATE.md` | `{dir}/.doug/logs/FAILURE_REPORT_TEMPLATE.md` |

---

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| `--force` | `false` | Skip guard check; overwrite all existing files |
| `--build-system` | `""` | Override auto-detection (`go` or `npm`) |
| `--agents` | `""` | Comma-separated agent names (e.g. `claude,gemini`) |

---

## Key Decisions

**Guard on `.doug/project-state.yaml` only**: This is the canonical state file. Other files (`doug.yaml`, `.doug/PRD.md`) are user-editable config — they get a warning + skip rather than a hard error.

**`initProject` as the testable core**: Avoids `os.Chdir` in tests. Tests call `initProject(t.TempDir(), ...)` directly. Mirrors the pattern used in `cmd/run.go` with `runOrchestrate`.

**`os.WriteFile` for all generated files**: Not atomic rename. These files are new (never updating in-place), so partial-write corruption is not a risk.

**`--force` skips guard entirely**: With `--force`, `initProject` does not check for `.doug/project-state.yaml` at all.

**Shared `.agents/skills/` for all agents**: All skill files are copied to a single `.agents/skills/` directory regardless of which agents are selected. No per-agent config files are created by `doug init`.

**`CHANGELOG.md` is never overwritten**: Uses `os.IsNotExist` to guard creation — permission errors or other stat failures do not silently skip it. `--force` does not override this guard; the changelog is user-maintained.

**`PRD.md` lives in `.doug/`**: All orchestrator-owned files are consolidated under `.doug/`. The `ACTIVE_TASK.md` briefing header includes an explicit `**PRD File**: {dougDir}/PRD.md` line so agents always have the correct path.

**CLAUDE.md is skipped**: `CLAUDE.md` exists in the template tree but is explicitly skipped by `copyInitTemplates`. New projects generate their own `CLAUDE.md` from scratch (not from a template). `AGENTS.md` is the agent-facing instruction file that IS scaffolded.

---

## Edge Cases & Gotchas

**`--force` with `copyInitTemplates`**: The `force` flag is threaded through to `copyInitTemplates`. All existing template files are overwritten when `--force` is set.

**Unknown `init/` files are warned and skipped**: If a new file is added to `internal/templates/init/` without a matching case in the routing switch, it logs a warning and continues. Add a case for any new file type.

**`doug.yaml` not in the guard list**: `initProject` checks only `.doug/project-state.yaml` for the guard. If `doug.yaml` exists without that file, init proceeds — the existing `doug.yaml` gets a warning and is skipped (or overwritten with `--force`).

---

## Related Topics

- [internal/templates](templates.md) — embedded `init/` and `runtime/` FSes
- [internal/config](config.md) — `DetectBuildSystem` used by `--build-system` detection
- [Go Infrastructure](../infrastructure/go.md) — project structure and cmd/ conventions
