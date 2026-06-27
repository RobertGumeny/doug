---
title: doug upgrade — Workspace Upgrade Workflow
updated: 2026-05-21
category: Features
tags: [upgrade, workspace, drift, inspection, managed-surfaces, pi-era]
related_articles:
  - docs/kb/packages/init.md
  - docs/kb/packages/agent.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
  - docs/kb/features/planning-lifecycle.md
  - docs/kb/packages/templates.md
  - docs/kb/packages/config.md
---

# doug upgrade — Workspace Upgrade Workflow

## Overview

`doug upgrade` inspects an existing `.doug/` workspace for drift against the current Pi-era contract, reports stale surfaces grouped by kind, and applies deterministic regeneration or migration steps.

The upgrade flow has three explicit stages:

| Stage | What it does |
|-------|-------------|
| **Inspect** | Scans the workspace for all known drift kinds (see below) |
| **Report** | Prints a grouped summary of every drift item and the action each requires |
| **Apply** | Executes actions: removes retired artifacts (with `--force`), reinstalls outdated managed surfaces, and prints actionable guidance for config drift |

Pass `--dry-run` to run Inspect and Report only. No filesystem changes are made in dry-run mode.

---

## Command Interface

```
doug upgrade [--dry-run] [--force]
```

| Flag | Effect |
|------|--------|
| `--dry-run` | Run Inspect + Report only; print what would change without applying |
| `--force` | Remove retired artifacts without confirmation prompts |

`doug upgrade` must be run from the project root (the directory that contains `.doug/`). If `.doug/` is absent the command exits with an error directing the user to run `doug init`.

---

## Surface Ownership Model

The upgrade flow is organized around four surface ownership classes. The ownership class is the authoritative basis for deciding which drift kinds apply and what action is safe during upgrade. All `.doug/` and project-root surfaces belong to one of these classes; the class assignment is determined up front, not resolved ad hoc per file.

### Ownership Classes

| Class | Description |
|-------|-------------|
| **Doug-managed** | Transient briefing and state files that Doug writes and controls freely. Safe to regenerate or overwrite at any time. Includes active runtime state and all log archives. |
| **Merge-aware managed** | Structured surfaces that Doug owns but must not overwrite wholesale. Changes require field-level merge or explicit operator action. |
| **User-authored** | Operator-owned inputs that Doug never rewrites. Root runtime inputs and backlog payload content belong here; Doug only copies them during promotion, never in place. |
| **Deterministic derivative** | Outputs regenerated from authoritative source data (embedded templates or handoff payloads). Should be regenerated rather than hand-maintained. |

### Representative File Assignments

| Path | Ownership Class |
|------|----------------|
| `.doug/ACTIVE_TASK.md` | Doug-managed |
| `.doug/project-state.yaml` | Doug-managed |
| `.doug/logs/epics/{epic}/` | Doug-managed |
| `.doug/intake/bugs/{epic}/` | Doug-managed |
| `.doug/intake/research/` | Doug-managed |
| `.doug/doug.yaml` | Merge-aware managed |
| `.doug/plan/PLAN.md` | Merge-aware managed |
| `.doug/PRD.md` | User-authored |
| `.doug/tasks.yaml` | User-authored |
| `.doug/plan/epics/<EPIC-ID>/PRD.md` | User-authored |
| `.doug/plan/epics/<EPIC-ID>/tasks.yaml` | User-authored |
| `CHANGELOG.md` | User-authored |
| `AGENTS.md` | User-authored (merge-only init surface) |
| `.gitignore` | User-authored (merge-only init surface) |
| `.doug/plan/epics/<EPIC-ID>/metadata.yaml` | Deterministic derivative |
| `.doug/plan/manifest.yaml` | Deterministic derivative |
| `.pi/skills/**` | Deterministic derivative |
| `.pi/extensions/handoff.ts` | Deterministic derivative |

### Upgrade Actions by Ownership Class

`doug upgrade` maps ownership classes to drift detection and action:

| Class | Drift kinds | Upgrade action |
|-------|-------------|----------------|
| Retired artifact | `retired_artifact` | Remove (with `--force`) |
| Merge-aware managed | `missing_config` | Guidance only (no auto-edit) |
| Deterministic derivative (embedded templates) | `missing_managed`, `outdated_managed` | Reinstall from embedded template |
| Doug-managed | (not inspected) | No action — orchestrator controls these |
| User-authored | (not inspected) | No action — operator controls these |

Doug-managed transient files and logs are runtime state that the orchestrator writes directly; `doug upgrade` does not inspect or modify them. User-authored surfaces are the operator's durable inputs and are excluded from drift detection by design.

---

## Drift Kinds

### `retired_artifact`

Paths that existed in the pre-Pi scaffold but are no longer part of the workspace contract.

| Path | Reason |
|------|--------|
| `.claude/` | Pre-Pi provider directory; skills now live in `.pi/skills/` |
| `.codex/` | Pre-Pi provider directory; skills now live in `.pi/skills/` |
| `.gemini/` | Pre-Pi provider directory; skills now live in `.pi/skills/` |

Detection: `os.Stat` presence check against each retired path.
Action: `os.RemoveAll` when `--force` is set; otherwise a warning with guidance to re-run with `--force`.

### `missing_config`

Retired execution config fields found at the top level of `.doug/doug.yaml`. Doug source code now owns execution routing, prompt construction, and Pi mode selection by phase; none of these fields are read from config.

| Field pattern | Retired meaning |
|---------------|-----------------|
| `policy:` | Execution routing hierarchy (interaction modes, routing profiles, tool policy) |
| `interaction_mode:` | Pi interaction mode override |
| `execution_mode:` | Stale execution mode selector |
| `*_agent_command:` | Stale per-phase agent command override |

Detection: raw `yaml.Node` parse of `.doug/doug.yaml`; top-level mapping keys are checked against the retired field list and `*_agent_command` suffix pattern. One drift item per file is emitted listing all retired keys found.
Action: `actionStripConfig` — automatic. `stripRetiredExecutionConfig` rewrites the file with all retired top-level keys removed. No `--force` required; stripping known-bad fields is always safe.

### `missing_managed`

A deterministic-derivative surface (embedded template) that should exist but is entirely absent.

Detection: `buildInstallPlan` produces the expected set of `entryKindCopy` entries under `.pi/`. Each entry's destination is checked with `os.ReadFile`; `ErrNotExist` → `missing_managed`.
Action: `actionReinstall` — triggers a full managed-surface reinstall during Apply.

### `outdated_managed`

A deterministic-derivative surface (embedded template) that exists but differs from the current embedded template.

Detection: same as `missing_managed`, but the file is present; its bytes are compared byte-for-byte against the pre-read template bytes from `buildInstallPlan`. Any difference → `outdated_managed`.
Action: `actionReinstall` — triggers a full managed-surface reinstall during Apply.

---

## Apply Phase

The Apply phase processes each drift item's action in order:

1. **`actionRemove`** — call `os.RemoveAll(AbsPath)`. Executed only when `--force` is set. Without `--force`, a `[WARNING]` is printed instead.
2. **`actionReinstall`** — set a reinstall flag. After all items are processed, call `copyInitTemplates(w, projectRoot, force=true)` once. This reinstalls all `.pi/skills/**` and `.pi/extensions/` surfaces from embedded templates using `entryKindCopy` with `force=true`, which overwrites existing files.
3. **`actionStripConfig`** — call `stripRetiredExecutionConfig(AbsPath)`. Reads the config file, removes all retired top-level keys (`policy`, `interaction_mode`, `execution_mode`, and any `*_agent_command` suffix) using a `yaml.Node` round-trip, and writes the cleaned YAML back. Does not require `--force`.
4. **`actionPatch`** — print a `Manual action required` line to stdout with the config path and description. No automatic YAML edit is applied. (Retained in code; not currently used by any drift kind.)

The single `copyInitTemplates` call is sufficient for all `actionReinstall` items because the install plan covers every managed `.pi/` surface. Running it once with `force=true` is idempotent regardless of how many individual files were flagged.

---

## What Is Not Inspected

`doug upgrade` does not inspect or modify surfaces in the **Doug-managed**, **User-authored**, or **User-authored (merge-only)** ownership classes. Specifically:

- **`.doug/ACTIVE_TASK.md`** — Doug-managed transient briefing state; orchestrator owns the full lifecycle
- **`.doug/project-state.yaml`** — Doug-managed live runtime state
- **`.doug/logs/`** — Doug-managed session, bug, failure, output, and archive logs
- **`.doug/PRD.md`** — user-authored root runtime input
- **`.doug/tasks.yaml`** — user-authored root runtime input
- **`.doug/plan/epics/<EPIC-ID>/PRD.md`** and **`tasks.yaml`** — user-authored backlog payload; immutable after handoff
- **`.doug/plan/epics/<EPIC-ID>/metadata.yaml`** — deterministic derivative; written by `doug handoff` and updated by the runtime completion handler, not by upgrade
- **`.doug/plan/manifest.yaml`** — deterministic derivative; generated by `doug handoff` from greenfield scaffold data
- **`CHANGELOG.md`** — user-authored; never overwritten by any Doug command
- **`AGENTS.md`** — user-authored merge-only surface; `doug init` merges the managed block, upgrade does not re-merge it
- **`.gitignore`** — user-authored merge-only surface; same policy as `AGENTS.md`

Merge-only surfaces (`.gitignore`, `AGENTS.md`) are idempotently handled by `doug init`. Running `doug init` on an already-initialized project with `--force` is the supported path for refreshing these surfaces.

Deterministic derivative backlog files (`metadata.yaml`, `plan/manifest.yaml`) are not reinstalled by upgrade because they carry live lifecycle state (`PLANNED`, `ACTIVE`, `COMPLETED`) and greenfield scaffold data that must not be discarded. Their authoritative source is `doug handoff` and the runtime completion handler, not an embedded template.

---

## Validation Strategy for Consuming Repositories

Running `doug upgrade` in a consuming repository follows a three-step validation path. The automated steps and the remaining manual steps are explicitly separated below.

### Step 1 — Dry-run inspection (no changes)

```
doug upgrade --dry-run
```

Review the report output and confirm that detected drift items are expected. A freshly initialized workspace should show no drift. A pre-Pi workspace should surface retired artifacts (`.claude/`, `.codex/`, `.gemini/`), retired execution config fields (`policy:`, `execution_mode:`, `interaction_mode:`, and `*_agent_command:` in `.doug/doug.yaml`), and missing or outdated `.pi/` managed surfaces.

### Step 2 — Apply automated changes

```
doug upgrade --force
```

This applies all automated actions:

| Action | What it does |
|--------|-------------|
| Remove retired artifacts | `os.RemoveAll` for `.claude/`, `.codex/`, `.gemini/` (requires `--force`) |
| Reinstall managed surfaces | Overwrites all `.pi/skills/**` and `.pi/extensions/handoff.ts` from embedded templates |
| Strip retired config | Removes `policy:`, `interaction_mode:`, `execution_mode:`, and `*_agent_command:` from `.doug/doug.yaml` (no `--force` required) |

There are no manual config steps for execution config drift. `actionStripConfig` is automatic and does not require operator intervention.

### Post-upgrade verification

`doug upgrade --dry-run` should print no drift items when the workspace is fully current. The workspace is ready for Pi-era operation when:

- No retired artifacts are present (`.claude/`, `.codex/`, `.gemini/`)
- All `.pi/skills/**` and `.pi/extensions/handoff.ts` match the current embedded templates
- `.doug/doug.yaml` contains no retired execution config fields (`policy:`, `interaction_mode:`, `execution_mode:`, `*_agent_command:`)

---

## Implementation

Implemented in two files:

| File | Responsibility |
|------|----------------|
| `cmd/upgrade.go` | Command definition, flag binding, `runUpgrade` orchestration, `reportDrift`, `applyUpgrade`, `filterDriftItems`, data types (`driftKind`, `upgradeAction`, `driftItem`) |
| `cmd/upgrade_inspect.go` | `inspectWorkspace`, `inspectRetiredArtifacts`, `inspectConfigDrift`, `inspectManagedSurfaces`, `stripRetiredExecutionConfig`, `removeRetiredExecutionFields`, `isRetiredExecutionField` |

### Key decisions

**Three named stages, not a monolithic function**: `runUpgrade` calls `inspectWorkspace`, then `reportDrift`, then `applyUpgrade` as separate calls so the stage boundary is explicit in code and in terminal output (`log.Section` separators).

**`yaml.Node` instead of a typed snapshot struct**: The previous approach used a `configSnapshot` pointer struct to detect absent fields. EPIC-41-004 replaced this with a direct `yaml.Node` parse — the raw mapping keys are iterated and checked against the retired field list. This avoids maintaining a shadow struct and naturally handles the `*_agent_command` suffix pattern via a string suffix check.

**`buildInstallPlan` reuse for managed surface detection**: The inspection stage calls `buildInstallPlan(projectRoot)` to get both the expected destination paths and the pre-read template bytes. This ensures detection and reinstall use identical data without a second template read.

**Single `copyInitTemplates` call in Apply**: All `actionReinstall` items collapse into one `copyInitTemplates(w, projectRoot, true)` call. The plan covers every managed `.pi/` surface; calling it once is idempotent and avoids partial-reinstall races.

**`--force` gates artifact removal only**: The `--force` flag controls whether retired file-system artifacts (`.claude/`, `.codex/`, `.gemini/`) are deleted. Managed surface reinstall and config stripping do not require `--force` — overwriting Doug-managed template files and removing known-bad config keys are always safe operations.

---

## Related Topics

- [cmd/init](../packages/init.md) — install plan model, `buildInstallPlan`, `copyInitTemplates`, merge strategies
- [internal/templates](../packages/templates.md) — embedded init template inventory and `Init embed.FS`
- [Interaction Model And Pi Policy Ownership](execution-model.md) — Pi-only execution contract and phase-based interaction-mode defaults
- [Planning And Execution Lifecycle Contract](planning-lifecycle.md) — surface ownership model for `.doug/plan/`
- [internal/config](../packages/config.md) — reduced `.doug/doug.yaml` schema and config loading rules
- [internal/agent](../packages/agent.md) — `PrepareExecution`, `PiAdapter`, and the Pi invocation boundary
- [Doug-to-Pi Runtime Contract](pi-runtime-contract.md) — Doug/Pi execution ownership split
