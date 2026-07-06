---
title: cmd/init — Project Scaffolding Subcommand
updated: 2026-07-04
category: Packages
tags: [init, scaffold, subcommand, templates, build-system, cobra, changelog, prompt, interactive]
related_articles:
  - docs/kb/packages/templates.md
  - docs/kb/packages/config.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/interactive.md
  - docs/kb/packages/prompt.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-best-effort-writes.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/features/upgrade.md
  - docs/kb/features/cli-discoverability.md
---

# cmd/init — Project Scaffolding Subcommand

## Overview

The `doug init` subcommand is implemented across four files in `cmd/`:

| File | Responsibility |
|------|----------------|
| `cmd/init.go` | Cobra command wiring, `doInitProject` core, utility functions (`dougYAMLContent`, project identity helpers) |
| `cmd/init_workflow.go` | Top-level orchestration (`runInitWorkflow`), build system prompt, config prompts; all interactive prompts go through `internal/interactive.Prompter` |
| `cmd/init_install.go` | Install plan model: `buildInstallPlan`, `routeTemplateFile`, `executeInstallPlan`, `entryKind`, `installEntry` |
| `cmd/init_merge.go` | Merge algorithms: `mergeGitignore`, `mergeAgents`, `mergeJSONSettings`, `mergeCodexConfigTOML`, and supporting helpers |

`doug init` scaffolds a new doug project by:

1. Generating `.doug/doug.yaml`, `.doug/tasks.yaml`, `.doug/project-state.yaml`, `.doug/PRD.md`, and `CHANGELOG.md` from inline content
2. Building an install plan from embedded `init/` templates and executing it against the project directory
3. Prompting for build system and key config values — `max_retries`, `max_iterations`, `kb_enabled` — on a TTY, or using defaults in non-interactive mode

### Entry Point Chain

```
runInit (cmd/init.go)
  └─ runInitWorkflow (cmd/init_workflow.go)   ← resolves build system and config interactively
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

## Pi Scaffolding

`doug init` scaffolds the Pi surfaces Doug expects.

- Skills land at `.pi/skills/` regardless of project type
- `.pi/extensions/handoff.ts` is always scaffolded

---

## Build System Detection

Build system precedence (three steps, first non-empty value wins):

1. `--build-system` flag (validated; must be `go`, `npm`, `pnpm`, or `static`)
2. `config.DetectBuildSystem(dir)` — reads marker files (`go.mod`, `pnpm-workspace.yaml`, `package.json`, `index.html`)
3. Interactive prompt on a TTY when `--build-system` was not provided (auto-detected value shown as default; falls back to `"go"` when nothing detected)
4. `"go"` — final fallback (non-TTY, no marker files, no flag)

**`selectBuildSystemInteractive(p interactive.Prompter, detected string) string`** uses `p.SelectOne` to present the `go`, `npm`, `pnpm`, `static` options. The auto-detected value (if any) is passed as the default index; falls back to `"go"` when `detected` is empty or not in the options list.

The resolved `bs` value is written into `build_system:` in `.doug/doug.yaml`. See [internal/config](config.md).

---

## Config Prompts

After build system selection, `runInitWorkflow` prompts for three `.doug/doug.yaml` config values when running on a TTY. When the terminal is not interactive (CI, piped input), defaults are used silently.

| Prompt | Default | `.doug/doug.yaml` field |
|--------|---------|-------------------|
| `max_retries — max FAILURE outcomes before a task is BLOCKED` | `3` | `max_retries` |
| `max_iterations — max orchestrator loop iterations before Doug stops` | `10` | `max_iterations` |
| `kb_enabled — synthesize knowledge-base updates after feature work` | `true` | `kb_enabled` |

**`promptConfigInt(p interactive.Prompter, label string, defaultVal int) int`** — calls `p.Text` to read an integer value; returns `defaultVal` on empty input, parse error, or negative value.

`kb_enabled` uses `p.Confirm(label, defaultYes)` directly — no wrapper function. Each config prompt includes a one-line explanation in the question text so first-run users know what the setting controls before accepting the default.

The resolved values are passed to `doInitProject` and written into `.doug/doug.yaml`. Unlike build system selection, there are no flags to override these config values in non-interactive mode; the defaults apply.

---

 **`doug init` does not create project files.** It never generates `go.mod`, `go.sum`, `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml`, a `Makefile`, or any source code. The human (or a coding agent) is responsible for initializing the actual project (`go mod init`, `npm init`, etc.) before or after running `doug init`. The `build_system` field only tells the orchestrator which toolchain commands to run — it does not scaffold the project itself.

---

## Generated Files

| File | Content source | Notes |
|------|----------------|-------|
| `.doug/doug.yaml` | `dougYAMLContent(bs, maxRetries, maxIterations, kbEnabled)` | Minimal boring config: build system, discoverable optional `module_root`, retry/iteration limits with visible numeric bounds, KB/review enabled, heartbeat, lint settings. |
| `.doug/tasks.yaml` | `tasksYAMLContent()` | One example epic, two tasks, all required fields |
| `.doug/project-state.yaml` | `projectStateContent()` → `"{}\n"` | Empty YAML; `BootstrapFromTasks` populates on first run |
| `.doug/PRD.md` | `prdContent()` | Blank template with section headers |
| `.doug/README.md` | `init/DOUG_README.md` | Doug workspace primer copied by the install plan; points bug handling toward Doug-managed session/intake flows and external bug investigation through research/planning |
| `.gitignore` | `init/.gitignore` merged into any existing root `.gitignore` | Guarantees `.doug/` is ignored without clobbering existing project ignore rules |
| `CHANGELOG.md` | `changelogContent()` | Keep a Changelog format; `[Unreleased]` section; **never overwritten** even with `--force` |

All are written with `state.AtomicWrite` (write to `.tmp` then `os.Rename`). `CHANGELOG.md` is skipped entirely if it already exists, regardless of `--force`.

After files are written, the init epilogue points users to `.doug/README.md`, the structured `doug plan` → `doug handoff` path, and the manual alternative of editing `.doug/PRD.md` plus `.doug/tasks.yaml` before `doug run`. This is part of Doug's CLI discoverability contract; see [CLI Discoverability And Config Diagnostics](../features/cli-discoverability.md).

### `.doug/doug.yaml` stays focused on project/runtime settings

`dougYAMLContent` writes the build system, optional `module_root`, retry/iteration limits, KB toggle, review toggle, heartbeat cadence, and lint settings. Numeric settings include inline comments with the validation bounds users need when editing the generated file. Doug derives Pi prompts and phase behavior in source during execution.

`max_retries`, `max_iterations`, and `kb_enabled` are written from the values resolved during init (interactive choices or defaults). `review_enabled` is emitted as `true` so completed epics get the advisory non-gating review before KB/changelog polish unless users opt out. `max_infra_retries` is written with the default transport retry cap (`3`). `lint_enabled` is always written as `false` (opt-in; override in `.doug/doug.yaml` after init).

---

## Install Plan Model

`copyInitTemplates` delegates template installation to a two-phase plan/execute model:

```go
func copyInitTemplates(w io.Writer, dir string, force bool) error {
    entries, err := buildInstallPlan(dir)
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

`buildInstallPlan(dir string) ([]installEntry, error)` walks the embedded `init/` FS and calls `routeTemplateFile` for each entry. AGENTS.md project metadata is resolved at plan-build time (reading the existing file if present) so execution only touches the destination.

### `routeTemplateFile` — routing rules

| Template path pattern | Destination | Kind |
|----------------------|-------------|------|
| `.pi/**` | `{dir}/.pi/**` | `MergeJSON` for `.json`, else `Copy` |
| `skills/**` | `{dir}/.pi/skills/{rel}` | `Copy` |
| `.gitignore` | `{dir}/.gitignore` | `MergeGitignore` |
| `AGENTS.md` | `{dir}/AGENTS.md` | `MergeAgentsMD` |
| `CLAUDE.md` | `{dir}/CLAUDE.md` | `Copy` |
| `DOUG_README.md` | `{dir}/.doug/README.md` | `Copy` |
| `BUG_REPORT_TEMPLATE.md` | `{dir}/.doug/intake/bugs/BUG_REPORT_TEMPLATE.md` | `Copy` |
| `*_TEMPLATE.md` | `{dir}/.doug/logs/{filename}` | `Copy` |
| anything else | — | warning + skip |

Only files explicitly embedded in `templates.Init` (see [internal/templates](templates.md)) can be routed.

Unknown template files log a warning and are silently skipped. Add a routing case in `routeTemplateFile` for any new file added to `internal/templates/init/`.

The supported role of `.pi/**` is narrower than the generic routing rule may suggest:

- `.pi/skills/**` provides Pi-local skill scaffolding
- `.pi/extensions/handoff.ts` provides an optional Pi-native handoff helper
- Doug itself does not discover `.pi/extensions/*` at runtime or delegate artifact authority to those files

## Follow-Up Notes

- If future Pi work adds more extension files or extension-driven runtime behavior, document each surface explicitly instead of relying on the `.pi/**` routing rule as product documentation.

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

---

## init/ Template Inventory

Files embedded in `internal/templates/init/`:

| File | Destination in new project |
|------|---------------------------|
| `CLAUDE.md` | `{dir}/CLAUDE.md` |
| `AGENTS.md` | `{dir}/AGENTS.md` with a delimited `Doug-Specific Instructions` section |
| `DOUG_README.md` | `{dir}/.doug/README.md` |
| `skills/implement-feature/SKILL.md` | `{dir}/.pi/skills/implement-feature/SKILL.md` |
| `skills/implement-bugfix/SKILL.md` | `{dir}/.pi/skills/implement-bugfix/SKILL.md` |
| `skills/implement-documentation/SKILL.md` | `{dir}/.pi/skills/implement-documentation/SKILL.md` |
| `skills/plan/**` | `{dir}/.pi/skills/plan/**` |
| `skills/scaffold/SKILL.md` | `{dir}/.pi/skills/scaffold/SKILL.md` |
| `skills/research/SKILL.md` | `{dir}/.pi/skills/research/SKILL.md` |
| `.pi/extensions/handoff.ts` | `{dir}/.pi/extensions/handoff.ts` |
| `.gitignore` | `{dir}/.gitignore` |
| `BUG_REPORT_TEMPLATE.md` | `{dir}/.doug/intake/bugs/BUG_REPORT_TEMPLATE.md` |

### `AGENTS.md` template

`internal/templates/init/AGENTS.md` contains two HTML comment annotations:
- `<!-- Generated by doug init — project metadata below is managed automatically -->` above the `DOUG_PROJECT_ID`/`DOUG_PROJECT_NAME` lines, marking them as auto-managed
- `<!-- Edit the rules below to reflect your repository's operating conventions -->` above the instructional content, signalling which part is user-editable

The bug report path is made explicit for out-of-band durable findings: `.doug/intake/bugs/BUG_REPORT_TEMPLATE.md`. Agents working from `ACTIVE_TASK.md` report blocking and non-blocking bugs in the structured result block; Doug archives non-blocking findings under `.doug/intake/bugs/{epic}/`. No separate active bug handoff or session-result template file is scaffolded or required.

---

## Flags

| Flag | Default | Effect |
|------|---------|--------|
| `--force` | `false` | Skip guard check; overwrite all existing files |
| `--build-system` | `""` | Override auto-detection and prompt: `go`, `npm`, `pnpm`, or `static` |
| `--no-git-init` | `false` | Skip running `git init` after scaffolding |

---

## Key Decisions

**`runInit` → `runInitWorkflow` → `doInitProject` separation**: `runInit` is a four-line Cobra handler. `runInitWorkflow` owns all interactive resolution (build system and config prompts) and takes an `initWorkflowOptions` struct — including an optional injectable `prompter interactive.Prompter` — so it is fully testable without touching `os.Stdin`/`os.Stdout`. `doInitProject` owns all file I/O given pre-resolved values.

**`initWorkflowOptions` struct**: packages `force`, `buildSystem`, `noGitInit`, and an optional injectable `prompter interactive.Prompter`. When `prompter` is nil, `runInitWorkflow` constructs one via `interactive.NewWithIO(w, br, isTTY)`. Tests inject a stub implementing `interactive.Prompter` without touching real I/O.

**`initProject` as backward-compat wrapper**: Keeps existing call sites in tests working. Calls `doInitProject` with `io.Discard` and hardcoded defaults. New tests should prefer calling `runInitWorkflow` for integration coverage or `doInitProject` for direct control.

**Install plan model (plan/execute separation)**: `buildInstallPlan(dir)` reads and pre-processes all template bytes from `templates.Init` and emits a slice of `installEntry` values with explicit merge strategies. `executeInstallPlan` applies them in order. This means routing logic is independently testable without a real filesystem.

**Prompt helpers use `interactive.Prompter`**: All interactive prompts in `cmd/init_workflow.go` go through the `interactive.Prompter` interface. Tests inject a stub implementing `interactive.Prompter` (or use `interactive.NewWithIO(..., isTTY=false)` for the fallback path) instead of raw `io.Writer`/`io.Reader`. This eliminates global `os.Stdin`/`os.Stdout` dependencies and provides a single seam for TTY vs. non-TTY behavior. See [internal/interactive](interactive.md).

**`dougYAMLContent` keeps prompts out of config**: Initial Pi prompts are derived at runtime from `config.BuildInitialPrompt`.

**Init generates minimal boring config**: `dougYAMLContent` emits only core project/runtime settings: `build_system`, `module_root`, `max_retries`, `max_infra_retries`, `max_iterations`, `kb_enabled`, `review_enabled`, `agent_heartbeat_seconds`, `first_response_threshold`, and `lint_enabled`. See [internal/config](config.md) for the supported config schema, [internal/agent](agent.md) for `PiAdapter` and `PrepareExecution`, and [Interaction Model And Pi Policy Ownership](../features/execution-model.md) for the cross-cutting execution contract.

**Guard on `.doug/project-state.yaml` only**: This is the canonical state file. Other files (`.doug/doug.yaml`, `.doug/PRD.md`) are user-editable config — they get a warning + skip rather than a hard error.

**`state.AtomicWrite` for all generated files**: Write to `.tmp` then `os.Rename`. Consistent with the project-wide atomic write pattern even for new files, and prevents any partial-write state if init is interrupted.

**`--force` skips guard entirely**: With `--force`, `doInitProject` does not check for `.doug/project-state.yaml` at all.

**Build system prompt always fires on TTY when `--build-system` is absent**: The prompt fires whenever `--build-system` is not provided. The auto-detected value (if any) is shown as the highlighted default.

**Config prompts are TTY-only, no flags**: `max_retries`, `max_iterations`, and `kb_enabled` are prompted interactively with one-line explanations but cannot be overridden via flags. Non-interactive runs always use the defaults (`3`, `10`, `true`). Edit `.doug/doug.yaml` after init to change them.

**Pi-only skill directory**: Skills are always installed at `.pi/skills/`. Files under `init/skills/**` preserve their relative subtree paths, so a skill can include `references/` or other supporting files.

**`.gitignore` is merged, not skipped**: `doug init` always ensures the root `.gitignore` contains `.doug/`. If a `.gitignore` already exists, its contents are preserved and the missing `doug` ignore entry is appended idempotently.

**`CHANGELOG.md` is never overwritten**: Uses `os.IsNotExist` to guard creation — permission errors or other stat failures do not silently skip it. `--force` does not override this guard; the changelog is user-maintained.

**`PRD.md` lives in `.doug/`**: All orchestrator-owned files are consolidated under `.doug/`. The `ACTIVE_TASK.md` briefing header includes an explicit `**PRD File**: {dougDir}/PRD.md` line so agents always have the correct path.

**`AGENTS.md` owns doug policy, launch prompts own transient task routing, skills stay generic**: `doug init` appends a clearly delimited doug-specific section to `AGENTS.md` covering progressive disclosure, reporting rules, and the agent-facing file contract. That section is intentionally conditional: `.doug/ACTIVE_TASK.md` is authoritative only for doug-managed runs, so manual sessions are not globally redirected just because the file exists. The per-run initial Pi prompts (resolved at runtime via `config.BuildInitialPrompt`) tell the launched agent to use `.doug/ACTIVE_TASK.md` for run, plan, scaffold, and research sessions. Skill files remain task workflows rather than repeating repo policy.

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

**`--force` with `copyInitTemplates`**: The `force` flag is threaded through to `executeInstallPlan`. `entryKindCopy` and `entryKindMergeJSON` honour `--force` by writing the template directly. `entryKindMergeAgentsMD` and `entryKindMergeGitignore` always merge regardless of `--force`.

**Unknown `init/` files are warned and skipped**: If a new file is added to `internal/templates/init/` without a matching case in `routeTemplateFile`, it logs a warning and continues. Add a routing case for any new file type.

**`.doug/doug.yaml` not in the guard list**: `doInitProject` checks only `.doug/project-state.yaml` for the guard. If `.doug/doug.yaml` exists without that file, init proceeds — the existing config gets a warning and is skipped (or overwritten with `--force`).

---

## Related Topics

- [internal/templates](templates.md) — embedded `init/` and `runtime/` FSes
- [internal/config](config.md) — `DetectBuildSystem` used by `--build-system` detection
- [Go Infrastructure](../infrastructure/go.md) — project structure and cmd/ conventions
- [doug upgrade](../features/upgrade.md) — reuses `buildInstallPlan` and `copyInitTemplates` from `cmd/init_install.go` to detect and reinstall outdated managed surfaces
