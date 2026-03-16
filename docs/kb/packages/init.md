---
title: cmd/init — Project Scaffolding Subcommand
updated: 2026-03-15
category: Packages
tags: [init, scaffold, subcommand, templates, build-system, cobra, changelog]
related_articles:
  - docs/kb/packages/templates.md
  - docs/kb/packages/config.md
  - docs/kb/packages/switch.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# cmd/init — Project Scaffolding Subcommand

## Overview

`cmd/init.go` implements the `doug init` subcommand. It scaffolds a new doug project by:

1. Generating `.doug/doug.yaml`, `.doug/tasks.yaml`, `.doug/project-state.yaml`, `.doug/PRD.md`, and `CHANGELOG.md` from inline content
2. Copying embedded `init/` template files into the target directory, including appending a clearly delimited doug-specific section to `AGENTS.md`
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
- Skill files are copied into each selected provider's local `skills/` directory
- Per-agent settings are scaffolded for selected agents:
  - `claude` → `.claude/settings.json`
  - `codex` → `.codex/config.toml`
  - `gemini` → `.gemini/settings.json` and `.gemini/policies/doug-default.json`
  Existing settings files are merged non-destructively unless `--force` is used.

Interactive prompt text is written with package-local best-effort helpers (`writef` / `writeln`). Prompt rendering must not affect the command's real success/failure path. See [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md).

---

## Build System Detection

Build system precedence (four steps, first non-empty value wins):

1. `--build-system` flag
2. `config.DetectBuildSystem(dir)` — reads marker files (`go.mod`, `pnpm-workspace.yaml`, `package.json`, `index.html`)
3. Interactive prompt on a TTY (when claude is selected and no marker files found)
4. `"go"` — final fallback

```go
bs := buildSystem                      // 1. flag
if bs == "" {
    bs = config.DetectBuildSystem(dir) // 2. marker files; returns "" when none found
}
if bs == "" && claudeSelected {
    if isTTY {
        bs = promptBuildSystemSelection() // 3. interactive prompt
    } else {
        log.Warning("..."); bs = "go"   // non-TTY: warn + default
    }
}
if bs == "" { bs = "go" }              // 4. final fallback (claude not selected)
```

The resolved `bs` value is passed to `copyInitTemplates` for permission injection and written into `build_system:` in `doug.yaml`. See [internal/config](config.md).

 **`doug init` does not create project files.** It never generates `go.mod`, `go.sum`, `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml`, a `Makefile`, or any source code. The human (or a coding agent) is responsible for initializing the actual project (`go mod init`, `npm init`, etc.) before or after running `doug init`. The `build_system` field only tells the orchestrator which toolchain commands to run — it does not scaffold the project itself.

---

## Generated Files

| File | Content source | Notes |
|------|----------------|-------|
| `.doug/doug.yaml` | `dougYAMLContent(bs)` | All config fields with inline YAML comments; build system interpolated; includes commented Codex/Gemini `agent_command` examples; `agent_command` value is single-quoted to avoid YAML parse errors |
| `.doug/tasks.yaml` | `tasksYAMLContent()` | One example epic, two tasks, all required fields |
| `.doug/project-state.yaml` | `"{}\n"` | Empty YAML; `BootstrapFromTasks` populates on first run |
| `.doug/PRD.md` | `prdContent()` | Blank template with section headers |
| `.gitignore` | `init/.gitignore` merged into any existing root `.gitignore` | Guarantees `.doug/` is ignored without clobbering existing project ignore rules |
| `CHANGELOG.md` | `changelogContent()` | Keep a Changelog format; `[Unreleased]` section; **never overwritten** even with `--force` |

All are written with `os.WriteFile` (not atomic rename — new files, no corruption risk). `CHANGELOG.md` is skipped entirely if it already exists, regardless of `--force`.

### Commented agent_command examples

`dougYAMLContent` generates a `doug.yaml` with the default `agent_command` single-quoted, plus two commented-out alternatives immediately after:

```yaml
agent_command: 'claude -p "[DOUG_TASK_ID: {{task_id}}] ..."'
# agent_command: codex exec "[DOUG_TASK_ID: {{task_id}}] ..."
# agent_command: gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] ..."
```

Single-quoting is required because the value contains `[DOUG_TASK_ID: ` (colon-space), which YAML interprets as a key-value separator in plain scalars. Single-quoted scalars allow embedded double-quotes and colons without escaping. See [cmd/switch](switch.md) for the matching fix applied to the write path.

---

## injectBuildSystemPermissions

```go
func injectBuildSystemPermissions(template []byte, bs string) ([]byte, error)
```

Appends build-system-specific Bash permissions to the `permissions.allow` array in a `settings.json` template:

1. Looks up `config.BuildSystems[bs]` — if `bs` is empty, unknown, or has no permissions, returns `template` unchanged
2. Unmarshals template JSON into `map[string]interface{}`
3. Navigates/creates `permissions.allow` (creates the `permissions` object if absent)
4. Calls `mergeStringArrays` to union-merge existing + new permissions (deduplicates)
5. Re-serialises with `json.MarshalIndent` + trailing newline

Returns an error only when the template JSON is malformed. Non-fatal — callers log a warning and proceed with the unmodified template.

---

## copyInitTemplates

```go
func copyInitTemplates(dir string, force bool, selectedAgents []string, buildSystem string) error
```

Walks `templates.Init` (embedded `init/` FS) and routes each file to its destination:

| Pattern | Destination |
|---------|-------------|
| `CLAUDE.md` | `{dir}/CLAUDE.md` |
| `AGENTS.md` | `{dir}/AGENTS.md` (append doug-specific section if absent) |
| `skills-config.yaml` | `{dir}/.doug/skills-config.yaml` |
| `skills/**` | `{dir}/{provider}/skills/{rel}` for each selected provider (`.claude`, `.codex`, `.gemini`) |
| `.claude/**` | `{dir}/.claude/**` (selected agents only) |
| `.codex/**` | `{dir}/.codex/**` (selected agents only) |
| `.gemini/**` | `{dir}/.gemini/**` (selected agents only) |
| `.gitignore` | `{dir}/.gitignore` (created if missing; otherwise merged to ensure `.doug/` is ignored) |
| `*_TEMPLATE.md` | `{dir}/.doug/logs/{filename}` |
| anything else | logged warning, silently skipped |

**No filename transformations.** Files land at their exact source names — no `_TEMPLATE` suffix stripping.

**`AGENTS.md` is merged, not blindly overwritten**: `copyInitTemplates` treats `AGENTS.md` specially. If the file does not exist, it writes the doug section as the full file. If the file exists and the doug marker is absent, it appends the doug section after the existing content. If the marker is already present, it leaves the file unchanged. This keeps user-authored agent guidance intact while ensuring doug's contract is present exactly once.

**Permission injection for `.claude/settings.json`**: Before `copyOrMergeAgentSettings` is called for `.claude/settings.json`, `injectBuildSystemPermissions(data, buildSystem)` is applied to the template bytes. This means:
- New install: template with injected permissions is written
- Existing file: `mergeJSONSettings` appends injected permissions (dedup union)
- `--force`: injected template is written directly

Parent directories are created with `os.MkdirAll(filepath.Dir(dst), 0o755)` before each write.

---

## init/ Template Inventory

Files embedded in `internal/templates/init/`:

| File | Destination in new project |
|------|---------------------------|
| `CLAUDE.md` | `{dir}/CLAUDE.md` |
| `AGENTS.md` | `{dir}/AGENTS.md` with a delimited `Doug-Specific Instructions` section |
| `skills-config.yaml` | `{dir}/.doug/skills-config.yaml` |
| `skills/implement-feature/SKILL.md` | `{dir}/.claude/skills/implement-feature/SKILL.md`, `{dir}/.codex/skills/implement-feature/SKILL.md`, and/or `{dir}/.gemini/skills/implement-feature/SKILL.md` depending on selected agents |
| `skills/implement-bugfix/SKILL.md` | `{dir}/.claude/skills/implement-bugfix/SKILL.md`, `{dir}/.codex/skills/implement-bugfix/SKILL.md`, and/or `{dir}/.gemini/skills/implement-bugfix/SKILL.md` depending on selected agents |
| `skills/implement-documentation/SKILL.md` | `{dir}/.claude/skills/implement-documentation/SKILL.md`, `{dir}/.codex/skills/implement-documentation/SKILL.md`, and/or `{dir}/.gemini/skills/implement-documentation/SKILL.md` depending on selected agents |
| `skills/research/SKILL.md` | `{dir}/.claude/skills/research/SKILL.md`, `{dir}/.codex/skills/research/SKILL.md`, and/or `{dir}/.gemini/skills/research/SKILL.md` depending on selected agents |
| `.claude/settings.json` | `{dir}/.claude/settings.json` (selected agents only) |
| `.codex/config.toml` | `{dir}/.codex/config.toml` (selected agents only) |
| `.gemini/settings.json` | `{dir}/.gemini/settings.json` (selected agents only) |
| `.gemini/policies/doug-default.json` | `{dir}/.gemini/policies/doug-default.json` (selected agents only) |
| `.gitignore` | `{dir}/.gitignore` |
| `SESSION_RESULTS_TEMPLATE.md` | `{dir}/.doug/logs/SESSION_RESULTS_TEMPLATE.md` |
| `BUG_REPORT_TEMPLATE.md` | `{dir}/.doug/logs/BUG_REPORT_TEMPLATE.md` |
| `FAILURE_REPORT_TEMPLATE.md` | `{dir}/.doug/logs/FAILURE_REPORT_TEMPLATE.md` |

---

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| `--force` | `false` | Skip guard check; overwrite all existing files |
| `--build-system` | `""` | Override auto-detection and prompt: `go`, `npm`, or `pnpm` |
| `--agents` | `""` | Comma-separated agent names (e.g. `claude,gemini`) |
| `--no-git-init` | `false` | Skip running `git init` after scaffolding |

---

## Key Decisions

**Guard on `.doug/project-state.yaml` only**: This is the canonical state file. Other files (`doug.yaml`, `.doug/PRD.md`) are user-editable config — they get a warning + skip rather than a hard error.

**`initProject` as the testable core**: Avoids `os.Chdir` in tests. Tests call `initProject(t.TempDir(), ...)` directly. Mirrors the pattern used in `cmd/run.go` with `runOrchestrate`.

**`os.WriteFile` for all generated files**: Not atomic rename. These files are new (never updating in-place), so partial-write corruption is not a risk.

**`--force` skips guard entirely**: With `--force`, `initProject` does not check for `.doug/project-state.yaml` at all.

**Per-provider skill directories**: Skill files are copied only for the agents selected during `doug init`, and each selected provider gets its own local directory (`.claude/skills/`, `.codex/skills/`, `.gemini/skills/`). Provider settings files are also scaffolded only for selected agents.

**`.claude/settings.json` template is base-only**: The embedded template contains only non-build-system permissions (Read, Write, Edit, Glob, Grep, git commands, make, etc.). Build-system-specific Bash permissions (`go build *`, `npm ci`, etc.) are injected at runtime by `injectBuildSystemPermissions` so the file is scoped to the actual project toolchain.

**`.gitignore` is merged, not skipped**: `doug init` always ensures the root `.gitignore` contains `.doug/`. If a `.gitignore` already exists, its contents are preserved and the missing `doug` ignore entry is appended idempotently.

**`CHANGELOG.md` is never overwritten**: Uses `os.IsNotExist` to guard creation — permission errors or other stat failures do not silently skip it. `--force` does not override this guard; the changelog is user-maintained.

**`PRD.md` lives in `.doug/`**: All orchestrator-owned files are consolidated under `.doug/`. The `ACTIVE_TASK.md` briefing header includes an explicit `**PRD File**: {dougDir}/PRD.md` line so agents always have the correct path.

**`AGENTS.md` owns doug policy, skills stay generic**: `doug init` appends a clearly delimited doug-specific section to `AGENTS.md` covering progressive disclosure (`ACTIVE_TASK.md` → `PRD.md` → `docs/kb/README.md`), reporting rules, and the agent-facing file contract. Skill files remain task workflows rather than repeating repo policy.

**CLAUDE.md is scaffolded as `@AGENTS.md`**: `CLAUDE.md` is scaffolded as a single-line include (`@AGENTS.md`) so any agent reading `CLAUDE.md` picks up the repository's `AGENTS.md` instructions.

**`git init` runs by default**: After all scaffolding completes, `initProject` runs `git init` on the target directory unless `.git/` already exists (silent skip) or `--no-git-init` is passed.

---

## Edge Cases & Gotchas

**`--force` with `copyInitTemplates`**: The `force` flag is threaded through to `copyInitTemplates`. Existing template files are overwritten when `--force` is set, except `AGENTS.md`, which still uses append-if-missing-marker semantics to preserve user-authored instructions.

**Unknown `init/` files are warned and skipped**: If a new file is added to `internal/templates/init/` without a matching case in the routing switch, it logs a warning and continues. Add a case for any new file type.

**`doug.yaml` not in the guard list**: `initProject` checks only `.doug/project-state.yaml` for the guard. If `doug.yaml` exists without that file, init proceeds — the existing `doug.yaml` gets a warning and is skipped (or overwritten with `--force`).

---

## Related Topics

- [internal/templates](templates.md) — embedded `init/` and `runtime/` FSes
- [internal/config](config.md) — `DetectBuildSystem` used by `--build-system` detection
- [Go Infrastructure](../infrastructure/go.md) — project structure and cmd/ conventions
