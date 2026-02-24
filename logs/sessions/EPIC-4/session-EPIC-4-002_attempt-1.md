---
task_id: "EPIC-4-002"
outcome: "SUCCESS"
timestamp: "2026-02-24T23:20:00Z"
changelog_entry: "Added WriteActiveTask and GetSkillForTaskType to the agent layer for writing ACTIVE_TASK.md with skill instructions and bug context"
files_modified:
  - internal/agent/activetask.go
  - internal/agent/activetask_test.go
tests_run: 13
tests_passed: 13
build_successful: true
duration_seconds: 480
estimated_tokens: 50000
---

## Implementation Summary

Implemented `internal/agent/activetask.go` with two public functions:
- `GetSkillForTaskType(taskType, configPath string) (string, error)` — resolves skill instructions for a task type by reading `skills-config.yaml` then loading the corresponding `SKILL.md` file, with hardcoded name and content fallbacks.
- `WriteActiveTask(config ActiveTaskConfig) error` — writes `logs/ACTIVE_TASK.md` with task metadata, skill instructions, and (for bugfix tasks) the bug context from `logs/ACTIVE_BUG.md`.

## Files Changed

- `internal/agent/activetask.go` — Full implementation with `ActiveTaskConfig`, `skillsConfigFile`, `hardcodedSkillNames`, `hardcodedSkillContent`, `GetSkillForTaskType`, `WriteActiveTask`, `resolveSkillName`, and `readBugContext`.
- `internal/agent/activetask_test.go` — 13 table-style subtests covering all acceptance criteria.

## Key Decisions

- `resolveSkillName` is a private helper separating config-reading from file-reading, making the two fallback tiers testable independently.
- `MkdirAll` is called on `LogsDir` before writing `ACTIVE_TASK.md` so the function works even when the logs directory doesn't yet exist (consistent with `CreateSessionFile`).
- The four hardcoded skill names mirror the Bash `get_skill_for_task_type` fallback exactly (`implement-feature`, `implement-bugfix`, `implement-documentation`, `manual-review`).
- For bugfix tasks with a missing `ACTIVE_BUG.md`, a `log.Warning` is emitted and the Bug Context section is omitted rather than returning an error — matches the acceptance criterion "omitted gracefully with a warning log".

## Test Coverage

- ✅ Reads skill content from SKILL.md via skills-config.yaml
- ✅ Falls back to hardcoded skill names when skills-config.yaml is missing
- ✅ Returns hardcoded fallback content when SKILL.md file is missing
- ✅ Returns error for unknown task type not in config and with no hardcoded default
- ✅ Returns error for unknown type when config is missing
- ✅ All four known task types work with hardcoded fallback
- ✅ ACTIVE_TASK.md written to logs dir with correct content
- ✅ Existing ACTIVE_TASK.md is overwritten (never archived)
- ✅ Bugfix task includes Bug Context section from ACTIVE_BUG.md
- ✅ Bugfix task omits Bug Context section when ACTIVE_BUG.md is missing
- ✅ Feature task does not include Bug Context section
- ✅ Documentation task type is preserved correctly as "documentation"
- ✅ Creates logs directory if it does not exist
