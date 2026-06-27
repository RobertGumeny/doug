---
title: internal/dougpath — Doug Storage Path Helpers
updated: 2026-06-27
category: Packages
tags: [paths, storage, intake, logs, epics, forensics]
related_articles:
  - docs/kb/features/planning-lifecycle.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/stats.md
  - docs/kb/packages/plan.md
---

# internal/dougpath — Doug Storage Path Helpers

`internal/dougpath` centralizes Doug-owned durable artifact paths so callers do not reconstruct `.doug/intake/` or `.doug/logs/epics/` layouts ad hoc.

## Path Root

`dougpath.New(projectRoot)` returns a `Paths` value with these roots:

- `ProjectRoot` — repository root passed by the caller
- `DougDir` — `{projectRoot}/.doug`
- `IntakeDir` — `{projectRoot}/.doug/intake`
- `LogsDir` — `{projectRoot}/.doug/logs`
- `EpicsDir` — `{projectRoot}/.doug/logs/epics`

## Planning Intake Paths

Planning intake is push-style input for a future planning run:

- `IntakeBugsDir()` → `.doug/intake/bugs/`
- `IntakeBugDir(epicID)` → `.doug/intake/bugs/{epicID}/`
- `IntakeBugPath(epicID, taskID)` → `.doug/intake/bugs/{epicID}/bug-{taskID}.md`
- `IntakeResearchDir()` → `.doug/intake/research/`
- `IntakeResearchPath(filename)` → `.doug/intake/research/{filename}`

Legacy `.doug/logs/bugs/` and `.doug/logs/research/` inputs may still be read by compatibility loaders, but new writers should target `.doug/intake/`.

## Epic Forensic Paths

Runtime and post-epic evidence is pull-style forensic data under `.doug/logs/epics/{epicID}/`:

- `EpicDir(epicID)` → epic forensic root
- `EpicPRDPath(epicID)`, `EpicTasksPath(epicID)`, `EpicProjectStatePath(epicID)` → finalized runtime snapshot files
- `ReviewArtifactPath(epicID, version)` → `epic-review.md` or `epic-review-vN.md` at the epic forensic root
- `TaskAttemptDir(epicID, taskID, attempt)` → `.doug/logs/epics/{epicID}/{taskID}/attempt-{N}/`
- `SessionArchivePath(...)` → `session.md`
- `StatsPath(...)` → `stats.json`
- `AttemptStartPath(...)` → `attempt-start.json`
- `TransportFailurePath(...)` → `infra-failure.md`

`PiSessionDir(...)` intentionally returns the same task attempt directory. Doug stabilizes the directory, not the Pi transcript filename: Pi-native timestamped JSONL files may remain inside that directory without being renamed. Use `PiNativeSessionPath(...)` only when a specific native filename is already known.

## Migration Boundary

Doug does not automatically migrate historical old-layout artifacts. Commands that need history may keep read-through compatibility, but new production writers should use the helpers for the current storage contract.
