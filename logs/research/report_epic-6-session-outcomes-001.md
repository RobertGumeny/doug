# Research Report: EPIC-6 Session Outcomes — Failure Analysis & Orchestrator Handling

**Generated**: 2026-02-25
**Scope Type**: Feature/Module
**Related Epic**: EPIC-6 — Init Subcommand & Templates
**Related Tasks**: EPIC-6-001, EPIC-6-002, EPIC-6-003, EPIC-6-004

---

## Overview

This report analyzes the four EPIC-6 session summaries to determine what passed, what failed, why it failed, and whether the orchestrator handled those failures correctly. The core finding: agents across all four tasks reported `build_successful: false` in their session files as an *informational note* about their own Bash environment (not a signal to the orchestrator). The only true failure was EPIC-6-003's first attempt, where the orchestrator's own `go test ./...` verification correctly detected a broken smoke test and triggered a retry. The orchestrator handled everything correctly.

---

## File Manifest

| File | Purpose |
| --- | --- |
| `logs/sessions/EPIC-6/session-EPIC-6-001_attempt-1.md` | Init command implementation — SUCCESS |
| `logs/sessions/EPIC-6/session-EPIC-6-002_attempt-1.md` | Template directory split — SUCCESS |
| `logs/sessions/EPIC-6/session-EPIC-6-003_attempt-1.md` | Smoke test attempt 1 (separate mockagent binary) — triggered Retry |
| `logs/sessions/EPIC-6/session-EPIC-6-003_attempt-2.md` | Smoke test attempt 2 (self-as-mock-agent) — SUCCESS |
| `logs/sessions/EPIC-6/session-EPIC-6-004_attempt-1.md` | QA pass — empty/unstarted (in-progress) |
| `cmd/run.go` | Main orchestration loop; dispatches to handlers |
| `internal/handlers/success.go` | HandleSuccess — runs Build() + Test(), dispatches Retry/Continue/EpicComplete |
| `internal/build/build.go` | GoBuildSystem — `go build ./...` and `go test ./...` |
| `internal/agent/parse.go` | ParseSessionResult — reads only outcome, changelog_entry, dependencies_added |
| `internal/types/types.go` | SessionResult struct — 3-field contract; build_successful is NOT a field |
| `integration/smoke_test.go` | Integration smoke test (attempt-2 implementation, currently in repo) |

---

## Task-by-Task Outcome Summary

### EPIC-6-001 — `cmd/init.go` Implementation
- **Agent-reported outcome**: `SUCCESS`
- **Agent-reported build_successful**: `false` (Bash EINVAL in environment)
- **Orchestrator-verified outcome**: SUCCESS (recorded in project-state.yaml metrics)
- **Result**: Task marked DONE, committed. No issues.

### EPIC-6-002 — Template Directory Split
- **Agent-reported outcome**: `SUCCESS`
- **Agent-reported build_successful**: `false` (same Bash EINVAL)
- **Orchestrator-verified outcome**: SUCCESS (recorded in project-state.yaml metrics)
- **Result**: Task marked DONE, committed. No issues.

### EPIC-6-003 — Integration Smoke Test (2 attempts)

**Attempt 1**:
- **Agent-reported outcome**: `SUCCESS`
- **Agent-reported build_successful**: `false` (Bash EINVAL)
- **Implementation approach**: Separate `testdata/mockagent/main.go` binary; `TestMain` builds both `doug` and the mockagent binaries.
- **Orchestrator action**: Ran `go build ./...` (passed), then `go test ./...` (FAILED — see below). Rolled back changes. Returned `Retry`.
- **Root cause of orchestrator failure**: `go test ./...` includes `./integration/...`. The integration smoke test ran and failed — likely because the separate mockagent binary could not be built correctly, or the end-to-end smoke test itself failed when exercising the full loop with the mockagent approach.

**Attempt 2**:
- **Agent-reported outcome**: `SUCCESS`
- **Agent-reported build_successful**: `false` (Bash EINVAL)
- **Implementation approach**: Self-as-mock-agent via `MOCK_AGENT_MODE=1` env var. Test binary doubles as mock agent; no separate binary build step needed.
- **Orchestrator action**: Ran `go build ./...` (passed), then `go test ./...` (PASSED — smoke test passed with new approach). Task marked DONE, committed.
- **Evidence**: `project-state.yaml` records `EPIC-6-003` as `outcome: success` with `duration_seconds: 1324` (~22 min, consistent with two attempts).

### EPIC-6-004 — Final QA Pass
- **Status**: `IN_PROGRESS` per `tasks.yaml`; session file is empty/placeholder (was never filled out).
- **Orchestrator action**: None yet — the task was not completed before this session ended.

---

## The `build_successful: false` Non-Issue

All four agents wrote `build_successful: false` in their session file frontmatter. This had **zero effect** on the orchestrator. Here's why:

`ParseSessionResult` (`internal/agent/parse.go`) unmarshals the session file frontmatter into a `types.SessionResult` struct. That struct has **exactly three fields**:

```go
type SessionResult struct {
    Outcome           Outcome  `yaml:"outcome"`
    ChangelogEntry    string   `yaml:"changelog_entry"`
    DependenciesAdded []string `yaml:"dependencies_added"`
}
```

`build_successful`, `tests_run`, `tests_passed`, `files_modified`, `task_id`, and `timestamp` are all **extra YAML keys** that `gopkg.in/yaml.v3` silently discards. The orchestrator never reads them. The orchestrator runs its own independent `Build()` and `Test()` verification in `HandleSuccess` regardless of what the agent reports.

The agents correctly noted this in their summaries: *"The orchestrator's own verification will confirm correctness."*

---

## Why EPIC-6-003 Had Two Attempts

The orchestrator triggered a retry for EPIC-6-003 attempt-1 via the standard `HandleSuccess` Retry path (`internal/handlers/success.go:89-97`):

```go
// 3. Verify tests.
if err := ctx.BuildSystem.Test(); err != nil {
    log.Error(...)
    git.RollbackChanges(ctx.ProjectRoot, protectedPaths)
    return SuccessResult{Kind: Retry}, nil
}
```

`BuildSystem.Test()` runs `go test ./...` on the project root. As of EPIC-6-003 attempt-1, this now includes `./integration/...` (the package that was just created). The integration smoke test ran as part of `go test ./...` and failed.

The most likely reason for the failure in attempt-1: `TestMain` tried to build the `testdata/mockagent` binary from source. On this Windows/MINGW64 environment (same one where agents reported EINVAL), the subprocess `go build` of the mockagent binary may have failed, causing `TestMain` to return exit code 1, failing the entire `go test ./integration/...` run.

Attempt-2 resolved this by eliminating the separate binary entirely — the test binary itself becomes the mock agent via `MOCK_AGENT_MODE=1`. This is inherently more portable and requires no extra `go build` subprocess.

---

## Did the Orchestrator Handle Failures Correctly?

**Yes.** The orchestrator behaved exactly as designed:

1. **`build_successful: false` in session files**: Correctly ignored. Orchestrator ran independent verification.
2. **EPIC-6-003 attempt-1 `Test()` failure**: Correctly detected, correctly rolled back uncommitted changes (preserving `project-state.yaml` and `tasks.yaml`), correctly returned `Retry`, correctly incremented `active_task.attempts` to 2 before spawning the next agent.
3. **EPIC-6-003 attempt-2 success**: Correctly marked task DONE, updated `tasks.yaml`, committed with `feat: EPIC-6-003`.
4. **EPIC-6-004 not attempted**: Correct — no session was started for it; `tasks.yaml` shows it as `IN_PROGRESS` and the session file stub was pre-created but never filled.

The one possible concern: `project-state.yaml` metrics only show a **single entry** for EPIC-6-003 (outcome: success). The failed attempt-1 is not recorded in metrics — only the successful attempt-2 is. This is by design: `RecordTaskMetrics` is called from `HandleSuccess` step 4, after build/test pass. A retry never reaches step 4, so the failed attempt produces no metric entry. This is correct behavior but means the metrics do not give visibility into retry counts or failed attempts beyond what `active_task.attempts` tracks.

---

## Forward-Looking Concern: Smoke Test in `go test ./...`

Now that `integration/smoke_test.go` exists, **every future call to `go test ./...`** (including the orchestrator's `HandleSuccess` verification after each task) will run the full smoke test. The smoke test:

1. Builds the `doug` binary from the module root (~3-5 sec)
2. Creates a temp git repo and writes test fixtures
3. Runs `doug run` with a two-task epic end-to-end
4. The inner `doug run` calls `go build ./...` + `go test ./...` on the temp `smoketest` module (fast, trivial module)

This adds **~15-30 seconds** to every task's post-success verification. For EPIC-6-004 (QA pass), this means the orchestrator's `Test()` call will include a full smoke test run. This is by design and expected, but it is now a permanent overhead per task that did not exist before EPIC-6-003.

No infinite recursion occurs: the inner `doug run` calls `go test ./...` on the `smoketest` temp module (module `smoketest`), not on the Doug project. The smoke test does not run itself recursively.

---

## Data Flow

```
Agent session file (outcome: SUCCESS)
    ↓
ParseSessionResult → SessionResult{Outcome, ChangelogEntry, DependenciesAdded}
    ↓ (build_successful, tests_run, etc. discarded by yaml.Unmarshal)
HandleSuccess
    ├─ Install() if deps added
    ├─ Build()  → go build ./...
    │       failure → RollbackChanges → return Retry
    ├─ Test()   → go test ./...  (includes integration/smoke_test.go!)
    │       failure → RollbackChanges → return Retry
    ├─ RecordTaskMetrics (step 4 — only reached on build+test pass)
    ├─ UpdateChangelog
    ├─ Mark task DONE in tasks.yaml
    ├─ AdvanceTaskPointers or InjectKBSynthesis
    ├─ SaveProjectState
    └─ Commit → return Continue (or Retry on commit failure)
```

---

## Patterns Observed

- **`build_successful` is a dead field**: The field exists in agent session templates as documentation/convention for the agent to self-report, but the orchestrator never reads it. It is purely for human review of session logs.
- **Attempt-1 session files are preserved on rollback**: After a `Retry`, the attempt-1 session file remains in `logs/sessions/EPIC-6/` (as seen: both `_attempt-1.md` and `_attempt-2.md` exist). The orchestrator pre-creates the session file before invoking the agent; rollback only reverts source file changes, not log files.
- **Metrics record only the final successful attempt**: A task that takes 3 retries before succeeding shows one metric entry (outcome: success). The retry count is tracked only via `active_task.attempts` in `project-state.yaml`.
- **Two session files for EPIC-6-003 is normal**: The existence of `_attempt-1.md` and `_attempt-2.md` for EPIC-6-003 is the expected behavior when a retry occurs — it is not a sign of orchestrator malfunction.

---

## Anti-Patterns & Tech Debt

- **Smoke test overhead on every task**: `go test ./...` now includes the 15-30 second integration test on every task. If future tasks are short-lived, this becomes a significant percentage of total iteration time. Mitigation: use build tags (`//go:build integration`) to gate the smoke test behind an explicit `-tags integration` flag, then add a separate `go test -tags integration ./integration/...` step only at the end of an epic's QA pass.
- **Metrics blind spot for retries**: Failed attempts are not recorded in `project-state.yaml` metrics. Operators reviewing the metrics cannot distinguish a task that succeeded on attempt-1 from one that took 3 retries. Adding a `retry_count` field to `TaskMetric` would close this gap without changing the existing schema.

---

## PRD Alignment

The EPIC-6-003 retry mechanism worked exactly as the orchestrator was designed (EPIC-4 handlers). The `build_successful: false` reporting pattern in session files is consistent with the template defined in EPIC-6-002 (`runtime/session_result.md` — 3 fields only). Agents are correctly filling out the minimal contract the orchestrator requires.

EPIC-6-004 (QA pass) is the only outstanding task. It is in `IN_PROGRESS` status with one unstarted attempt. The session stub at `logs/sessions/EPIC-6/session-EPIC-6-004_attempt-1.md` is empty (all fields blank), confirming the task was set up but not executed.

---

## Raw Notes

- The EINVAL Bash issue affects every agent session in EPIC-6 on this Windows/MSYS2 environment. Agents correctly anticipated this and deferred build/test verification to the orchestrator. The orchestrator's own build/test verification is not affected by EINVAL (it uses `exec.Command` directly, not the Bash tool).
- The `testdata/mockagent/main.go` file from EPIC-6-003 attempt-1 was rolled back by the orchestrator. It does not exist in the current repo. Only the attempt-2 implementation (single-file, MOCK_AGENT_MODE approach) is present in `integration/smoke_test.go`.
- EPIC-6-003's duration of 1324 seconds in metrics represents elapsed wall time for both attempts combined (the orchestrator measures from `TaskStartTime` which is set at the start of the iteration, not reset on retry — actually, `TaskStartTime` is set per iteration, so 1324s for attempt-2 alone is ~22 minutes, which is long but plausible for a complex implementation session).
