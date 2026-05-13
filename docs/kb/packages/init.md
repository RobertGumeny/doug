---
title: cmd/init — Project Scaffolding Subcommand
updated: 2026-05-11
category: Packages
tags: [init, scaffold, subcommand, templates, build-system, cobra, changelog, prompt, interactive]
related_articles:
  - docs/kb/packages/templates.md
  - docs/kb/packages/config.md
  - docs/kb/packages/switch.md
  - docs/kb/packages/interactive.md
  - docs/kb/packages/prompt.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# cmd/init — Project Scaffolding Subcommand

## Overview

The `doug init` subcommand is implemented across four files in `cmd/`:

| File | Responsibility |
|------|----------------|
| `cmd/init.go` | Cobra command wiring, `doInitProject` core, utility functions (`dougYAMLContent`, `injectBuildSystemPermissions`, project identity helpers) |
| `cmd/init_workflow.go` | Top-level orchestration (`runInitWorkflow`), agent selection, build system prompt, config prompts; all interactive prompts go through `internal/interactive.Prompter` |
| `cmd/init_install.go` | Install plan model: `buildInstallPlan`, `routeTemplateFile`, `executeInstallPlan`, `entryKind`, `installEntry` |
| `cmd/init_merge.go` | Merge algorithms: `mergeGitignore`, `mergeAgents`, `mergeJSONSettings`, `mergeCodexConfigTOML`, and supporting helpers |

`doug init` scaffolds a new doug project by:

1. Generating `.doug/doug.yaml`, `.doug/tasks.yaml`, `.doug/project-state.yaml`, `.doug/PRD.md`, and `CHANGELOG.md` from inline content
2. Building an install plan from embedded `init/` templates and executing it against the project directory
3. Prompting for agent selection (TTY) or defaulting to `claude` (non-TTY / `--agents` flag)
4. Prompting for key config values — `max_retries`, `max_iterations`, `kb_enabled` — on a TTY, or using defaults in non-interactive mode

### Entry Point Chain

```
runInit (cmd/init.go)
  └─ runInitWorkflow (cmd/init_workflow.go)   ← resolves agents, build system, config interactively
       └─ doInitProject (cmd/init.go)          ← generates files and executes install plan
            └─ copyInitTemplates (cmd/init.go)
                 └─ buildInstallPlan + executeInstallPlan (cmd/init_install.go)
```

`initProject(dir, force, buildSystem, selectedAgents, noGitInit)` is a backward-compatible wrapper retained for tests. It calls `doInitProject` with `io.Discard` output and default config values (`maxRetries=3`, `maxIterations=10`, `kbEnabled=true`).

---

## Guard Check

Before writing any files, `doInitProject` checks whether the project is already initialized:

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

**`selectAgentsInteractive(p interactive.Prompter) []string`** — uses `p.SelectOne` to pick the primary agent and `p.Confirm` to optionally include additional agents. Defaults to `["claude"]` on error. All interaction goes through the shared `interactive.Prompter` abstraction — no direct `io.Writer`/`io.Reader` access. See [internal/interactive](interactive.md).

---

## Build System Detection

Build system precedence (three steps, first non-empty value wins):

1. `--build-system` flag (validated; must be `go`, `npm`, `pnpm`, or `static`)
2. `config.DetectBuildSystem(dir)` — reads marker files (`go.mod`, `pnpm-workspace.yaml`, `package.json`, `index.html`)
3. Interactive prompt on a TTY when `--build-system` was not provided (auto-detected value shown as default; falls back to `"go"` when nothing detected)
4. `"go"` — final fallback (non-TTY, no marker files, no flag)

**`selectBuildSystemInteractive(p interactive.Prompter, detected string) string`** uses `p.SelectOne` to present the `go`, `npm`, `pnpm`, `static` options. The auto-detected value (if any) is passed as the default index; falls back to `"go"` when `detected` is empty or not in the options list.

The resolved `bs` value is passed to `copyInitTemplates` for permission injection and written into `build_system:` in `doug.yaml`. See [internal/config](config.md).

---

## Config Prompts

After agent and build system selection, `runInitWorkflow` prompts for three `doug.yaml` config values when running on a TTY. When the terminal is not interactive (CI, piped input), defaults are used silently.

| Prompt | Default | `doug.yaml` field |
|--------|---------|-------------------|
| `max_retries` | `3` | `max_retries` |
| `max_iterations` | `10` | `max_iterations` |
| `kb_enabled` | `true` | `kb_enabled` |

**`promptConfigInt(p interactive.Prompter, label string, defaultVal int) int`** — calls `p.Text` to read an integer value; returns `defaultVal` on empty input, parse error, or negative value.

`kb_enabled` uses `p.Confirm(label, defaultYes)` directly — no wrapper function.

The resolved values are passed to `doInitProject` and written into `doug.yaml`. Unlike agent selection and build system, there are no flags to override these config values in non-interactive mode; the defaults apply.

---

 **`doug init` does not create project files.** It never generates `go.mod`, `go.sum`, `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml`, a `Makefile`, or any source code. The human (or a coding agent) is responsible for initializing the actual project (`go mod init`, `npm init`, etc.) before or after running `doug init`. The `build_system` field only tells the orchestrator which toolchain commands to run — it does not scaffold the project itself.

---

## Generated Files

| File | Content source | Notes |
|------|----------------|-------|
| `.doug/doug.yaml` | `dougYAMLContent(bs, primaryAgent, maxRetries, maxIterations, kbEnabled)` | Four explicit agent command fields; no commented-out alternatives |
| `.doug/tasks.yaml` | `tasksYAMLContent()` | One example epic, two tasks, all required fields |
| `.doug/project-state.yaml` | `projectStateContent()` → `"{}\n"` | Empty YAML; `BootstrapFromTasks` populates on first run |
| `.doug/PRD.md` | `prdContent()` | Blank template with section headers |
| `.gitignore` | `init/.gitignore` merged into any existing root `.gitignore` | Guarantees `.doug/` is ignored without clobbering existing project ignore rules |
| `CHANGELOG.md` | `changelogContent()` | Keep a Changelog format; `[Unreleased]` section; **never overwritten** even with `--force` |

All are written with `state.AtomicWrite` (write to `.tmp` then `os.Rename`). `CHANGELOG.md` is skipped entirely if it already exists, regardless of `--force`.

### Agent command fields in doug.yaml

`dougYAMLContent` generates four explicit agent command fields for the selected provider — no commented-out alternatives for other providers:

```yaml
run_agent_command: '...'       # Command used for doug run and post-epic KB synthesis
plan_agent_command: '...'      # Command used for interactive doug plan sessions
scaffold_agent_command: '...'  # Command used for doug scaffold
research_agent_command: '...'  # Command used for doug research
```

Each field carries the provider-specific invocation style. For example, the `claude` provider uses `claude -p "..."` for `run_agent_command` (headless) but `claude "..."` (interactive) for `plan_agent_command` and `research_agent_command`. See [cmd/switch](switch.md) for the `agentRegistry` that defines these per-provider commands.

Single-quoting is required because the value contains `[DOUG_TASK_ID: ` (colon-space), which YAML interprets as a key-value separator in plain scalars. Single-quoted scalars allow embedded double-quotes and colons without escaping.

`max_retries`, `max_iterations`, and `kb_enabled` are written from the values resolved during init (interactive choices or defaults).

---

## Install Plan Model

`copyInitTemplates` delegates template installation to a two-phase plan/execute model:

```go
func copyInitTemplates(w io.Writer, dir string, force bool, selectedAgents []string, buildSystem string) error {
    entries, err := buildInstallPlan(dir, agentSelected, buildSystem)
    // ...
    return executeInstallPlan(w, dir, entries, force)
}
```

### `entryKind` — merge strategy enum

| Value | Behaviour |
|-------|-----------|
| `entryKindCopy` | Write template bytes verbatim; respects `--force` |
| `entryKindMergeJSON` | Deep-merge template into existing JSON; `--force` writes directly |
| `entryKindMergeGitignore` | Append missing non-comment lines; `--force` ignored — always merges |
| `entryKindMergeAgentsMD` | Append or update managed doug block in `AGENTS.md`; always merges |
| `entryKindMergeCodexTOML` | Inject managed defaults into `.codex/config.toml`; `--force` writes directly |

### `installEntry`

Each plan entry carries pre-read template bytes and all routing metadata so execution only touches the destination filesystem:

```go
type installEntry struct {
    DstPath    string    // absolute destination
    DisplayRel string    // display path shown in terminal output
    Kind       entryKind // merge strategy
    Data       []byte    // pre-processed template bytes
    projectID  string    // populated for entryKindMergeAgentsMD only
    projectName string   // populated for entryKindMergeAgentsMD only
}
```

### `buildInstallPlan`

`buildInstallPlan(dir string, agentSelected map[string]bool, buildSystem string) ([]installEntry, error)` walks the embedded `init/` FS and calls `routeTemplateFile` for each entry. AGENTS.md project metadata is resolved at plan-build time (reading the existing file if present) so execution only touches the destination.

### `routeTemplateFile` — routing rules

| Template path pattern | Destination | Kind |
|----------------------|-------------|------|
| `.claude/**` | `{dir}/.claude/**` (if claude selected) | `MergeJSON` for `.json`, `MergeCodexTOML` for `.toml`, else `Copy` |
| `.codex/**` | `{dir}/.codex/**` (if codex selected) | same dispatch |
| `.gemini/**` | `{dir}/.gemini/**` (if gemini selected) | same dispatch |
| `.pi/**` | `{dir}/.pi/**` (always) | `MergeJSON` for `.json`, else `Copy` |
| `skills/**` | `{dir}/{provider}/skills/{rel}` for each selected provider | `Copy` |
| `skills-config.yaml` | — (removed; not present in embedded FS) | — |
| `.gitignore` | `{dir}/.gitignore` | `MergeGitignore` |
| `AGENTS.md` | `{dir}/AGENTS.md` | `MergeAgentsMD` |
| `CLAUDE.md` | `{dir}/CLAUDE.md` | `Copy` |
| `skills-config.yaml` | — (removed; not present in embedded FS) | — |
| `*_TEMPLATE.md` | `{dir}/.doug/logs/{filename}` | `Copy` |
| anything else | — | warning + skip |

Unknown template files log a warning and are silently skipped. Add a routing case in `routeTemplateFile` for any new file added to `internal/templates/init/`.

---

## Merge Algorithms (`cmd/init_merge.go`)

All merge functions are extracted to `cmd/init_merge.go` and are independently testable.

### `mergeGitignore(existing, template string) string`

Appends non-comment, non-blank lines from template that are not already present in existing. Preserves all existing content. If existing is empty, returns the template as-is.

### `mergeAgents(existing, dougSection, projectID, projectName string) string`

- Empty existing: returns `dougSection`
- Marker absent: appends `dougSection` after existing content
- Marker present: calls `ensureMetadataInBlock` to inject project metadata if missing; leaves the rest of the block unchanged

### `mergeJSONSettings(existing, template []byte) ([]byte, error)`

Deep-merges managed template into existing JSON. Nested objects are merged recursively (`deepMergeJSON`); string arrays are union-merged (`mergeStringArrays`); all other values from the template overwrite existing. Returns re-serialised JSON with trailing newline.

### `mergeCodexConfigTOML(existing string) string`

Injects managed root-level keys (`approval_policy`, `sandbox_mode`, `web_search`) and `[sandbox_workspace_write]` section defaults into an existing `.codex/config.toml`. Existing values are overwritten to match managed defaults; missing keys are appended.

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

## init/ Template Inventory

Files embedded in `internal/templates/init/`:

| File | Destination in new project |
|------|---------------------------|
| `CLAUDE.md` | `{dir}/CLAUDE.md` |
| `AGENTS.md` | `{dir}/AGENTS.md` with a delimited `Doug-Specific Instructions` section |
| `skills-config.yaml` | — (removed; not present in embedded FS; skill selection is via `policy.tasks[type].skill` in `doug.yaml`) |
| `skills/implement-feature/SKILL.md` | `{dir}/.claude/skills/implement-feature/SKILL.md`, `{dir}/.codex/skills/implement-feature/SKILL.md`, and/or `{dir}/.gemini/skills/implement-feature/SKILL.md` depending on selected agents |
| `skills/implement-bugfix/SKILL.md` | `{dir}/.claude/skills/implement-bugfix/SKILL.md`, `{dir}/.codex/skills/implement-bugfix/SKILL.md`, and/or `{dir}/.gemini/skills/implement-bugfix/SKILL.md` depending on selected agents |
| `skills/implement-documentation/SKILL.md` | `{dir}/.claude/skills/implement-documentation/SKILL.md`, `{dir}/.codex/skills/implement-documentation/SKILL.md`, and/or `{dir}/.gemini/skills/implement-documentation/SKILL.md` depending on selected agents |
| `skills/manual-review/SKILL.md` | `{dir}/.claude/skills/manual-review/SKILL.md`, `{dir}/.codex/skills/manual-review/SKILL.md`, and/or `{dir}/.gemini/skills/manual-review/SKILL.md` depending on selected agents |
| `skills/plan/**` | `{dir}/.claude/skills/plan/**`, `{dir}/.codex/skills/plan/**`, and/or `{dir}/.gemini/skills/plan/**` depending on selected agents |
| `skills/scaffold/SKILL.md` | `{dir}/.claude/skills/scaffold/SKILL.md`, `{dir}/.codex/skills/scaffold/SKILL.md`, and/or `{dir}/.gemini/skills/scaffold/SKILL.md` depending on selected agents |
| `skills/research/SKILL.md` | `{dir}/.claude/skills/research/SKILL.md`, `{dir}/.codex/skills/research/SKILL.md`, and/or `{dir}/.gemini/skills/research/SKILL.md` depending on selected agents |
| `.claude/settings.json` | `{dir}/.claude/settings.json` (selected agents only) |
| `.codex/config.toml` | `{dir}/.codex/config.toml` (selected agents only) |
| `.gemini/settings.json` | `{dir}/.gemini/settings.json` (selected agents only) |
| `.gemini/policies/doug-default.json` | `{dir}/.gemini/policies/doug-default.json` (selected agents only) |
| `.pi/extensions/handoff.ts` | `{dir}/.pi/extensions/handoff.ts` (always) |
| `.gitignore` | `{dir}/.gitignore` |
| `SESSION_RESULTS_TEMPLATE.md` | `{dir}/.doug/logs/SESSION_RESULTS_TEMPLATE.md` |
| `BUG_REPORT_TEMPLATE.md` | `{dir}/.doug/logs/BUG_REPORT_TEMPLATE.md` |
| `FAILURE_REPORT_TEMPLATE.md` | `{dir}/.doug/logs/FAILURE_REPORT_TEMPLATE.md` |

### `AGENTS.md` template

`internal/templates/init/AGENTS.md` contains two HTML comment annotations:
- `<!-- Generated by doug init — project metadata below is managed automatically -->` above the `DOUG_PROJECT_ID`/`DOUG_PROJECT_NAME` lines, marking them as auto-managed
- `<!-- Edit the rules below to reflect your repository's operating conventions -->` above the instructional content, signalling which part is user-editable

The bug report path is made explicit: `.doug/logs/BUG_REPORT_TEMPLATE.md`. The generated `AGENTS.md` now also distinguishes the blocking-only live handoff file `.doug/ACTIVE_BUG.md` from the canonical durable archive under `.doug/logs/bugs/{epic}/`, so agents do not treat every bug as a runtime interruption.

---

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| `--force` | `false` | Skip guard check; overwrite all existing files |
| `--build-system` | `""` | Override auto-detection and prompt: `go`, `npm`, `pnpm`, or `static` |
| `--agents` | `""` | Comma-separated agent names (e.g. `claude,gemini`) |
| `--no-git-init` | `false` | Skip running `git init` after scaffolding |

---

## Key Decisions

**`runInit` → `runInitWorkflow` → `doInitProject` separation**: `runInit` is a four-line Cobra handler. `runInitWorkflow` owns all interactive resolution (agent selection, build system, config prompts) and takes an `initWorkflowOptions` struct — including an optional injectable `prompter interactive.Prompter` — so it is fully testable without touching `os.Stdin`/`os.Stdout`. `doInitProject` owns all file I/O given pre-resolved values.

**`initWorkflowOptions` struct**: packages all flag values plus the optional `prompter` field for test injection. When `prompter` is nil, `runInitWorkflow` constructs one via `interactive.NewWithIO(w, br, isTTY)`. Tests inject a stub implementing `interactive.Prompter` without touching real I/O.

**`initProject` as backward-compat wrapper**: Keeps existing call sites in tests working. Calls `doInitProject` with `io.Discard` and hardcoded defaults. New tests should prefer calling `runInitWorkflow` for integration coverage or `doInitProject` for direct control.

**Install plan model (plan/execute separation)**: `buildInstallPlan` reads and pre-processes all template bytes and emits a slice of `installEntry` values with explicit merge strategies. `executeInstallPlan` applies them in order. This means routing logic is independently testable without a real filesystem.

**Prompt helpers use `interactive.Prompter`**: All interactive prompts in `cmd/init_workflow.go` go through the `interactive.Prompter` interface. Tests inject a stub implementing `interactive.Prompter` (or use `interactive.NewWithIO(..., isTTY=false)` for the fallback path) instead of raw `io.Writer`/`io.Reader`. This eliminates global `os.Stdin`/`os.Stdout` dependencies and provides a single seam for TTY vs. non-TTY behavior. See [internal/interactive](interactive.md).

**`dougYAMLContent` generates four explicit command fields — no commented alternatives**: Generated `doug.yaml` contains `run_agent_command`, `plan_agent_command`, `scaffold_agent_command`, and `research_agent_command` for the selected provider only. Removing commented-out alternative provider blocks avoids confusion about which line is active and keeps the file clean for selected-agent installs. Use `doug switch {agent}` to change providers later.

**Guard on `.doug/project-state.yaml` only**: This is the canonical state file. Other files (`doug.yaml`, `.doug/PRD.md`) are user-editable config — they get a warning + skip rather than a hard error.

**`state.AtomicWrite` for all generated files**: Write to `.tmp` then `os.Rename`. Consistent with the project-wide atomic write pattern even for new files, and prevents any partial-write state if init is interrupted.

**`--force` skips guard entirely**: With `--force`, `doInitProject` does not check for `.doug/project-state.yaml` at all.

**Build system prompt always fires on TTY when `--build-system` is absent**: The prompt fires for any agent combination when `--build-system` is not provided. The auto-detected value (if any) is shown as the highlighted default.

**Config prompts are TTY-only, no flags**: `max_retries`, `max_iterations`, and `kb_enabled` are prompted interactively but cannot be overridden via flags. Non-interactive runs always use the defaults (`3`, `10`, `true`). Edit `doug.yaml` after init to change them.

**Per-provider skill directories**: Skill files are copied only for the agents selected during `doug init`, and each selected provider gets its own local directory (`.claude/skills/`, `.codex/skills/`, `.gemini/skills/`). Files under `init/skills/**` preserve their relative subtree paths, so a skill can include `references/` or other supporting files. Provider settings files are also scaffolded only for selected agents.

**`.claude/settings.json` template is base-only**: The embedded template contains only non-build-system permissions (Read, Write, Edit, Glob, Grep, git commands, make, etc.). Build-system-specific Bash permissions (`go build *`, `npm ci`, etc.) are injected at runtime by `injectBuildSystemPermissions` so the file is scoped to the actual project toolchain.

**`.gitignore` is merged, not skipped**: `doug init` always ensures the root `.gitignore` contains `.doug/`. If a `.gitignore` already exists, its contents are preserved and the missing `doug` ignore entry is appended idempotently.

**`CHANGELOG.md` is never overwritten**: Uses `os.IsNotExist` to guard creation — permission errors or other stat failures do not silently skip it. `--force` does not override this guard; the changelog is user-maintained.

**`PRD.md` lives in `.doug/`**: All orchestrator-owned files are consolidated under `.doug/`. The `ACTIVE_TASK.md` briefing header includes an explicit `**PRD File**: {dougDir}/PRD.md` line so agents always have the correct path.

**`AGENTS.md` owns doug policy, launch prompts own transient task routing, skills stay generic**: `doug init` appends a clearly delimited doug-specific section to `AGENTS.md` covering progressive disclosure, reporting rules, and the agent-facing file contract. That section is intentionally conditional: `.doug/ACTIVE_TASK.md` is authoritative only for doug-managed runs, so manual sessions are not globally redirected just because the file exists. The per-run `*_agent_command` prompts are where doug tells the launched agent to use `.doug/ACTIVE_TASK.md` for run, plan, scaffold, and research sessions. Skill files remain task workflows rather than repeating repo policy.

**CLAUDE.md is scaffolded as `@AGENTS.md`**: `CLAUDE.md` is scaffolded as a single-line include (`@AGENTS.md`) so any agent reading `CLAUDE.md` picks up the repository's `AGENTS.md` instructions.

**`git init` runs by default**: After all scaffolding completes, `doInitProject` runs `git init` on the target directory unless `.git/` already exists (silent skip) or `--no-git-init` is passed.

---

## Project Identity Metadata

During `doug init`, the managed AGENTS.md block is populated with two repo-level identity fields at the top of the block:

```
DOUG_PROJECT_ID: my-project-a1b2c3
DOUG_PROJECT_NAME: My Project
```

These fields are consumed by `doug-stats` to aggregate session statistics across providers (Claude Code, Codex, Gemini) for the same local project.

### Generation rules

| Field | Generation | Preservation |
|-------|------------|--------------|
| `DOUG_PROJECT_ID` | `slugify(dirName)` + `-` + 6 random hex chars | Never regenerated once written |
| `DOUG_PROJECT_NAME` | Title-cased from dir name | Preserved if present; derived again only on fresh init |

**`DOUG_PROJECT_ID` is the canonical repo identity**. It is generated once and must not be replaced on re-init (with or without `--force`). The value stored in `AGENTS.md` is the single source of truth.

### Implementation helpers

| Function | Responsibility |
|----------|----------------|
| `slugify(s string) string` | Lowercase, alphanumeric + hyphens; collapses consecutive separators |
| `generateProjectID(dirName string) string` | `<slug>-<6hexchars>` using `crypto/rand` |
| `generateProjectName(dirName string) string` | Title-case words split on `-`, `_`, space |
| `extractManagedBlockField(content, fieldName string) string` | Reads `KEY: value` from inside the managed block |
| `ensureMetadataInBlock(content, id, name string) string` | Injects metadata after START marker if absent; no-op if present |

### Storage

Project metadata lives **only** inside the `<!-- DOUG-SPECIFIC-INSTRUCTIONS:START/END -->` block in `AGENTS.md`. It is never written to `ACTIVE_TASK.md`, task files, or runtime state.

---

## Edge Cases & Gotchas

**`--force` with `copyInitTemplates`**: The `force` flag is threaded through to `executeInstallPlan`. `entryKindCopy` and `entryKindMergeJSON`/`entryKindMergeCodexTOML` honour `--force` by writing the template directly. `entryKindMergeAgentsMD` and `entryKindMergeGitignore` always merge regardless of `--force`.

**Unknown `init/` files are warned and skipped**: If a new file is added to `internal/templates/init/` without a matching case in `routeTemplateFile`, it logs a warning and continues. Add a routing case for any new file type.

**`doug.yaml` not in the guard list**: `doInitProject` checks only `.doug/project-state.yaml` for the guard. If `doug.yaml` exists without that file, init proceeds — the existing `doug.yaml` gets a warning and is skipped (or overwritten with `--force`).

---

## Related Topics

- [internal/templates](templates.md) — embedded `init/` and `runtime/` FSes
- [internal/config](config.md) — `DetectBuildSystem` used by `--build-system` detection
- [cmd/switch](switch.md) — `agentRegistry` definition and provider command formats
- [Go Infrastructure](../infrastructure/go.md) — project structure and cmd/ conventions
