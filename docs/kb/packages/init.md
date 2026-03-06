---
title: cmd/init — Project Scaffolding Subcommand
updated: 2026-03-04
category: Packages
tags: [init, scaffold, subcommand, templates, build-system, cobra]
related_articles:
  - docs/kb/packages/templates.md
  - docs/kb/packages/config.md
  - docs/kb/infrastructure/go.md
---

# cmd/init — Project Scaffolding Subcommand

## Overview

`cmd/init.go` implements the `doug init` subcommand. It scaffolds a new doug project by:

1. Generating `.doug/doug.yaml`, `.doug/tasks.yaml`, `.doug/project-state.yaml`, and `PRD.md` from inline content
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

`doug init` prompts for agent selection interactively (on a TTY) or accepts `--agents` flag values. The selected agents determine which per-agent config files (e.g., `.gemini/settings.json`) are created.

- Default: `claude` when no input is provided or in non-TTY mode
- `--agents claude,gemini` to select multiple agents non-interactively
- All skill files are copied to the shared `.agents/skills/` directory regardless of selected agents

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
| `.doug/doug.yaml` | `dougYAMLContent(bs, skillsDir)` | All config fields with inline YAML comments; build system interpolated; includes commented Codex/Gemini `agent_command` examples |
| `.doug/tasks.yaml` | `tasksYAMLContent()` | One example epic, two tasks, all required fields |
| `.doug/project-state.yaml` | `"{}\n"` | Empty YAML; `BootstrapFromTasks` populates on first run |
| `PRD.md` | `prdContent()` | Blank template with section headers |

All four are written with `os.WriteFile` (not atomic rename — new files, no corruption risk).

### Commented agent_command examples

`dougYAMLContent` generates a `doug.yaml` with the default `agent_command: claude` plus two commented-out alternatives immediately after:

```yaml
agent_command: claude
# agent_command: codex
# agent_command: gemini {project_dir}
```

This gives users ready-to-use references for switching agents.

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
| `.gemini/settings.json` | `{dir}/.gemini/settings.json` (when gemini selected) |
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
| `.gemini/settings.json` | `{dir}/.gemini/settings.json` |
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

**Guard on `.doug/project-state.yaml` only**: This is the canonical state file. Other files (`doug.yaml`, `PRD.md`) are user-editable config — they get a warning + skip rather than a hard error.

**`initProject` as the testable core**: Avoids `os.Chdir` in tests. Tests call `initProject(t.TempDir(), ...)` directly. Mirrors the pattern used in `cmd/run.go` with `runOrchestrate`.

**`os.WriteFile` for all generated files**: Not atomic rename. These files are new (never updating in-place), so partial-write corruption is not a risk.

**`--force` skips guard entirely**: With `--force`, `initProject` does not check for `.doug/project-state.yaml` at all.

**Shared `.agents/skills/` for all agents**: All skill files are copied to a single `.agents/skills/` directory regardless of which agents are selected. Per-agent config (e.g., `.gemini/settings.json`) is still agent-specific.

**CLAUDE.md is skipped**: `CLAUDE.md` exists in the template tree but is explicitly skipped by `copyInitTemplates`. New projects generate their own `CLAUDE.md` from scratch (not from a template). `AGENTS.md` is the agent-facing instruction file that IS scaffolded.

---

## Edge Cases & Gotchas

**`--force` with `copyInitTemplates`**: The `force` flag is threaded through to `copyInitTemplates`. All existing template files are overwritten when `--force` is set.

**Unknown `init/` files are warned and skipped**: If a new file is added to `internal/templates/init/` without a matching case in the routing switch, it logs a warning and continues. Add a case for any new file type.

**`.gemini/settings.json` requires `all:init` embed**: The `.gemini/` directory is hidden (dot-prefix). The `//go:embed all:init` directive is required to include it — plain `//go:embed init` skips hidden directories.

**`doug.yaml` not in the guard list**: `initProject` checks only `.doug/project-state.yaml` for the guard. If `doug.yaml` exists without that file, init proceeds — the existing `doug.yaml` gets a warning and is skipped (or overwritten with `--force`).

---

## Related Topics

- [internal/templates](templates.md) — embedded `init/` and `runtime/` FSes
- [internal/config](config.md) — `DetectBuildSystem` used by `--build-system` detection
- [Go Infrastructure](../infrastructure/go.md) — project structure and cmd/ conventions
