# Doug Testing Strategy

## Overview
Doug prioritizes behavioral correctness over line coverage. Every test should catch a real regression that would impact the tool's reliability in an automated loop.

## Test Levels

### 1. Unit Tests
- Focused on individual packages and logic (parsing, state mutation, logic transitions).
- Use `t.TempDir()` for filesystem operations to ensure isolation.
- Mocking is used sparingly, primarily for the build system and external toolchains (where needed).
- **Standards:**
  - Prefer table-driven tests for complex logic or multiple scenarios.
  - Use `t.Helper()` for shared setup and utility functions.
  - Shared utilities live in `internal/testutil`.
  - Avoid fragile absolute path string comparisons. Raw string comparison is appropriate only when the behavior under test is a string contract: path construction, CLI argument construction, rendered config/text, IDs, modes, statuses, or relative artifact names. When the behavior under test is filesystem identity, especially after a path has crossed an OS boundary (`os.Getwd`, subprocess cwd, `filepath.Abs`, symlinked temp roots, external tools), compare identity with `os.Stat` + `os.SameFile` or compare a canonicalized form (`filepath.EvalSymlinks`) rather than raw strings.

### 2. Integration Tests
- Verify the interaction between multiple packages (e.g., orchestrator and handlers).
- These tests are opt-in: default `make test` runs the fast suite only, while `make test-integration` runs the end-to-end smoke coverage.
- Both targets use explicit `go test` timeouts so a stuck subprocess turns into a failure with stack traces instead of an indefinite hang.
- The smoke tests in `integration/smoke_test.go` verify the full loop with a mock agent across four paths:
  - `TestSmokeFullLoop` — SUCCESS happy path
  - `TestBugFixAndResume` — BUG → bugfix → resume
  - `TestFailureRetryBlocked` — FAILURE → retry → BLOCKED
  - `TestBuildFailAfterSuccess` — build failure after agent SUCCESS

## Shared Test Utilities

The `internal/testutil` package provides shared helpers used across test packages:

- `WriteFile(t, path, content)` — creates a file (and parent directories) with the given content; calls `t.Fatalf` on failure.

Used by: `internal/agent`, `internal/build`, `internal/config`, `internal/handlers`.

## Package-Specific Status

| Package | Status | Notes |
|---------|--------|-------|
| `internal/interactive` | Strong | Bubble Tea models tested directly via `Update`; fallback path covered via `NewWithIO(isTTY=false)`; `IsInteractive` TTY-skip pattern used for environment-dependent tests. |
| `internal/agent` | Strong | Uses `internal/testutil.WriteFile`; full lifecycle coverage. |
| `internal/build` | Strong | npm and pnpm `Build()` methods covered; static no-ops intentionally untested. |
| `internal/config` | Strong | Uses `internal/testutil.WriteFile`; `DetectBuildSystem` fully covered. |
| `internal/handlers` | Strong | Table-driven; state-save failure paths tested for `HandleBug` and `HandleFailure`; changelog wiring tested in `HandleSuccess`. |
| `internal/orchestrator` | Strong | Dependency checks use descriptive names reflecting actual assertions. |
| `internal/state` | Strong | — |
| `internal/types` | Strong | — |
| `internal/plan` | Strong | `planBriefBlock` alignment-checkpoint phrases and `InitialPlanDocument` draft-vs-confirmed seed phrases covered by narrow regression tests; `RefreshPlanDocument` multiline intent and reported-bug context rendering covered. |
| `cmd` | Strong | Agent registry, init workflow, upgrade flow (inspect/report/apply across all drift kinds and representative stale-workspace scenarios), and planning setup (ACTIVE_TASK.md brief content, PLAN.md seed content, alignment-summary and source-of-truth phrases) fully covered. |

## Strengths
- **Isolation:** Universal use of `t.TempDir()` prevents cross-test contamination.
- **Realism:** Most tests use real git repositories and filesystem structures rather than extensive mocking.
- **Fast default loop:** Integration smoke tests are separated from the default pass so day-to-day epic completion checks stay quick.
- **Clarity:** Test names are descriptive and map to specific behaviors.
- **Shared utilities:** `internal/testutil` eliminates duplicate `writeFile` helpers across packages.

## Out-of-Scope
- End-to-end tests that invoke a real agent binary against a live API.
- Performance or load testing.
- Tests for `StaticBuildSystem` no-op methods (low risk, low value).
- 100% line coverage as a target.
