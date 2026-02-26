---
title: cmd/init — Project Scaffolding Subcommand
updated: 2026-02-25
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

1. Generating `doug.yaml`, `tasks.yaml`, and `PRD.md` from inline content
2. Copying embedded `init/` template files into the target directory

The testable core is `initProject(dir, force, buildSystem string) error`. The Cobra command handler (`runInit`) simply calls `os.Getwd()` and delegates.

---

## Guard Check

Before writing any files, `initProject` checks whether the project is already initialized:

```go
for _, name := range []string{"project-state.yaml", "tasks.yaml"} {
    if _, statErr := os.Stat(filepath.Join(dir, name)); statErr == nil {
        return fmt.Errorf("%s already exists — ...")
    }
}
```

- Triggered by: `project-state.yaml` **or** `tasks.yaml` already present
- `--force` skips this check entirely
- Other generated files (`doug.yaml`, `PRD.md`, template files) emit a `log.Warning` and skip if they already exist — they do not error

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
| `doug.yaml` | `dougYAMLContent(bs)` | All 5 config fields with inline YAML comments; build system interpolated |
| `tasks.yaml` | `tasksYAMLContent()` | One example epic, two tasks, all required fields |
| `PRD.md` | `prdContent()` | Blank template with section headers |

All three are written with `os.WriteFile` (not atomic rename — new files, no corruption risk).

---

## copyInitTemplates

```go
func copyInitTemplates(dir string, force bool) error
```

Walks `templates.Init` (embedded `init/` FS) and routes each file to its destination using a `switch` on the relative path:

| Pattern | Destination |
|---------|-------------|
| `CLAUDE.md`, `AGENTS.md` | `{dir}/` |
| `*_TEMPLATE.md` | `{dir}/logs/` |
| `skills/**` | `{dir}/.claude/skills/` |
| anything else | silently skipped |

**No filename transformations.** Files land at their exact source names — no `_TEMPLATE` suffix stripping.

Parent directories are created with `os.MkdirAll(filepath.Dir(dst), 0o755)` before each write.

---

## init/ Template Inventory

Files embedded in `internal/templates/init/`:

| File | Destination |
|------|-------------|
| `CLAUDE.md` | `{dir}/CLAUDE.md` |
| `AGENTS.md` | `{dir}/AGENTS.md` |
| `SESSION_RESULTS_TEMPLATE.md` | `{dir}/logs/SESSION_RESULTS_TEMPLATE.md` |
| `BUG_REPORT_TEMPLATE.md` | `{dir}/logs/BUG_REPORT_TEMPLATE.md` |
| `FAILURE_REPORT_TEMPLATE.md` | `{dir}/logs/FAILURE_REPORT_TEMPLATE.md` |
| `skills/implement-feature/SKILL.md` | `{dir}/.claude/skills/implement-feature/SKILL.md` |
| `skills/implement-bugfix/SKILL.md` | `{dir}/.claude/skills/implement-bugfix/SKILL.md` |
| `skills/implement-documentation/SKILL.md` | `{dir}/.claude/skills/implement-documentation/SKILL.md` |

---

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| `--force` | `false` | Skip guard check; overwrite all existing files |
| `--build-system` | `""` | Override auto-detection (`go` or `npm`) |

---

## Key Decisions

**Guard on `project-state.yaml` and `tasks.yaml` only**: These are the canonical state files. Other files (`doug.yaml`, `PRD.md`) are user-editable config — they get a warning + skip rather than a hard error.

**`initProject` as the testable core**: Avoids `os.Chdir` in tests. Tests call `initProject(t.TempDir(), ...)` directly. Mirrors the pattern used in `cmd/run.go` with `runOrchestrate`.

**`os.WriteFile` for all generated files**: Not atomic rename. These files are new (never updating in-place), so partial-write corruption is not a risk.

**`--force` skips guard entirely**: With `--force`, `initProject` does not check for `project-state.yaml` or `tasks.yaml` at all — the loop just overwrites whatever it finds.

---

## Edge Cases & Gotchas

**`--force` with `copyInitTemplates`**: The `force` flag is threaded through to `copyInitTemplates`. All existing template files are overwritten when `--force` is set.

**Unknown `init/` files are silently skipped**: If a new file is added to `internal/templates/init/` without a matching case in the `switch`, it will be silently ignored by `copyInitTemplates`. Add a case for any new file type.

**`doug.yaml` not in the guard list**: `initProject` checks only `project-state.yaml` and `tasks.yaml` for the guard. If `doug.yaml` exists without those two, init proceeds — the existing `doug.yaml` gets a warning and is skipped (or overwritten with `--force`).

---

## Related Topics

- [internal/templates](templates.md) — embedded `init/` and `runtime/` FSes
- [internal/config](config.md) — `DetectBuildSystem` used by `--build-system` detection
- [Go Infrastructure](../infrastructure/go.md) — project structure and cmd/ conventions
