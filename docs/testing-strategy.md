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

### 2. Integration Tests
- Verify the interaction between multiple packages (e.g., orchestrator and handlers).
- The "smoke test" in `integration/smoke_test.go` verifies the full loop with a mock agent.

## Package-Specific Status

| Package | Status | Key Improvements Needed |
|---------|--------|-------------------------|
| `internal/agent` | Strong | Consolidate inline helpers. |
| `internal/build` | Fair | Add tests for npm/pnpm Build() methods. |
| `internal/config` | Strong | - |
| `internal/handlers` | Strong | Refactor to table-driven tests; add failure path tests. |
| `internal/orchestrator` | Strong | Fix misleading dependency checks. |
| `internal/state` | Strong | - |
| `internal/types` | Strong | - |
| `cmd` | Good | Add more CLI-level tests. |

## Strengths
- **Isolation:** Universal use of `t.TempDir()` prevents cross-test contamination.
- **Realism:** Most tests use real git repositories and filesystem structures rather than extensive mocking.
- **Clarity:** Test names are descriptive and map to specific behaviors.

## Misleading or Low-Value Tests
- **`TestCheckDependencies_GitAlwaysRequired`**: Does not actually assert that git is required; it only checks that the check passes when git is present.
- **Static Build System No-ops**: Testing methods that are intentionally empty provides low value.

## Prioritized Gaps
1. **Handler Failure Paths:** `HandleBug` and `HandleFailure` need tests for when state-saving fails.
2. **Build System Implementation:** Npm and Pnpm build systems lack testing for their `Build()` implementation.
3. **Integration Scenarios:** The smoke test only covers the SUCCESS path; it needs to cover BUG, FAILURE, and build-fail-after-SUCCESS paths.
4. **Shared Utilities:** Duplicate filesystem helpers (`writeFile`) across packages lead to maintenance overhead.
