---
title: doug upgrade — Workspace Upgrade Workflow
updated: 2026-07-24
category: Features
tags: [upgrade, migration, managed-surfaces, skills, claude]
related_articles:
  - docs/kb/packages/init.md
  - docs/kb/packages/templates.md
  - docs/kb/features/execution-model.md
---

# doug upgrade — Workspace Upgrade Workflow

## Overview

`doug upgrade` inspects a workspace, reports proposed changes, and reconciles Doug-managed setup. It is deliberately conservative: it installs the canonical built-in skills without moving, overwriting, or deleting user-owned skill content.

```text
doug upgrade [--dry-run] [--force]
```

`--dry-run` is read-only. Use it first to review every install, legacy migration, retained conflict, bridge change, and warning. `--force` permits removal of retired `.codex/` and `.gemini/` artifacts; it never bypasses the clean-Git-tree requirement for skill or Claude-bridge changes.

## Current Skill Layout

Doug owns exactly these six namespaced skills:

- `doug-implement-feature`
- `doug-implement-bugfix`
- `doug-implement-documentation`
- `doug-scaffold`
- `doug-plan`
- `doug-research`

Their canonical home is `.agents/skills/doug-*/`. Built-in skills are deterministic derivatives of the embedded templates. Pi extensions and settings remain under `.pi/`; the skill move does not change Pi's existing project-trust requirement for local skills.

Claude is a supported managed surface, not a retired artifact. Doug normally exposes the canonical skills through this exact relative bridge:

```text
.claude/skills -> ../.agents/skills
```

A wrong or broken bridge is refused rather than retargeted. When a real, non-empty `.claude/skills/` directory must be preserved, Doug leaves its entries untouched, copies only the six `doug-*` skills, writes `.claude/skills/.doug-managed-skills.json`, and warns that the bridge uses managed copies. Only a valid manifest with the exact six-name inventory proves those fallback copies are Doug-owned and may be refreshed.

## Ownership Model

| Surface | Ownership and upgrade behavior |
|---|---|
| `.agents/skills/doug-*/**` | Doug-managed deterministic derivatives; missing or outdated files are reinstalled. |
| `.claude/skills` | Supported managed bridge. A symlink is preferred; a populated directory receives manifest-recorded Doug copies only. |
| `.pi/extensions/**` | Pi-native managed extension/settings surface; retained under `.pi/`. |
| Legacy unnamespaced skill tree | See [Legacy migration](#legacy-migration); it is never the canonical home. |
| `.doug/ACTIVE_TASK.md`, state, logs | Doug-managed runtime state; not inspected by upgrade. |
| `AGENTS.md`, `.gitignore`, `CHANGELOG.md`, PRD and task inputs | User-authored or merge-only; not overwritten by upgrade. |

Managed-root checks compare path components, so similarly named paths such as `.agents-old` and `.pirate` are never treated as Doug surfaces.

## Upgrade Behavior

Inspection runs in this order: retired artifacts, retired execution config, legacy skills, canonical managed surfaces, then the Claude bridge. The report labels each proposed action as Install, Remove, Retain, Warning, Reconcile, or Manual action.

- Missing or outdated canonical skill files and Pi extension files are reinstalled from embedded templates.
- Retired execution config fields (`policy`, `interaction_mode`, `execution_mode`, and `*_agent_command`) are stripped from `.doug/doug.yaml`.
- `.codex/` and `.gemini/` are retired artifacts. They are removed only with `--force`.
- Changes to skills or the Claude bridge require no staged or unstaged changes to tracked Git files. Untracked and ignored files do not block the operation. Commit or stash first; `--force` cannot override this gate.
- Upgrade only changes the working tree. It does not run `git add` or `git rm`; review and commit the results, or roll them back with Git.

A current workspace's second `doug upgrade --dry-run` reports no skill or bridge drift.

## Legacy Migration

Previous releases installed unnamespaced built-in skills below `.pi/skills/`. Upgrade removes that legacy tree only when it exactly matches the frozen inventory of the final unnamespaced templates: every expected relative path must be a regular file with its recorded SHA-256, with no missing, extra, modified, or non-regular entries.

If that proof succeeds, upgrade installs the namespaced `.agents/skills/doug-*/` set and removes the fingerprint-matched legacy tree. If the proof fails—including for versions older than that final template set—Doug preserves the tree and emits a warning. This is the **stale-duplicate** outcome: the user removes the old tree manually when ready, while namespaced `doug-*` identities prevent a Pi name collision in the meantime. User-owned or uncertain legacy entries are never moved, overwritten, or deleted.

## Recommended Per-Repository Migration

1. Commit or stash tracked changes if the upgrade will change skills or the Claude bridge.
2. Run `doug upgrade --dry-run` and review every action, especially Retain and Warning items.
3. Run `doug upgrade` (add `--force` only when intentionally removing retired `.codex/` or `.gemini/` directories).
4. Review the working-tree changes and commit them. For a preserved stale duplicate, remove the old legacy tree manually only after confirming it is no longer needed.
5. Run `doug upgrade --dry-run` again. A successful migration has no canonical-skill or Claude-bridge drift.

## Related Topics

- [cmd/init](../packages/init.md) — canonical skill installation and Claude bridge creation
- [internal/templates](../packages/templates.md) — embedded template inventory
- [Interaction Model And Pi Policy Ownership](execution-model.md) — Pi-only execution and extension boundary
