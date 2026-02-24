---
task_id: "EPIC-2-003"
outcome: "SUCCESS"
timestamp: "2026-02-24T21:00:00Z"
changelog_entry: "Added NpmBuildSystem with package.json test-script guard and NewBuildSystem factory"
duration_seconds: 240
estimated_tokens: 18000
files_modified:
  - internal/build/npm.go
  - internal/build/npm_test.go
tests_run: 19
tests_passed: 19
build_successful: true
---

## Implementation Summary

Implemented `NpmBuildSystem` in `internal/build/npm.go` with full `BuildSystem` interface compliance. Added a `NewBuildSystem` factory function that dispatches to `GoBuildSystem` or `NpmBuildSystem` by type string, returning a descriptive error for unknown types.

## Files Changed

- `internal/build/npm.go` — `NpmBuildSystem` struct, constructor, `IsInitialized`/`Install`/`Build`/`Test` methods, `hasTestScript` helper, and `NewBuildSystem` factory
- `internal/build/npm_test.go` — 11 new unit tests for `NpmBuildSystem` and the factory

## Key Decisions

- `IsInitialized()` uses `os.Stat` + `info.IsDir()` to confirm `node_modules/` exists as a directory (a plain file named `node_modules` returns false)
- `Test()` reads `package.json` via `os.ReadFile` + `json.Unmarshal` into an anonymous struct — no external JSON library needed
- If `package.json` is missing or malformed, `hasTestScript()` returns false and `Test()` returns nil (skip) — consistent with the "no test configured" contract
- The `NO_TESTS_CONFIGURED` sentinel check precedes the error check so it is honoured even when npm exits non-zero
- `NewBuildSystem` lives in `npm.go` as specified; it reuses existing `NewGoBuildSystem` and `NewNpmBuildSystem` constructors
- All commands use `exec.Command` with an explicit args slice — no shell eval

## Test Coverage

- ✅ `IsInitialized` returns false when `node_modules/` is absent
- ✅ `IsInitialized` returns true when `node_modules/` directory exists
- ✅ `IsInitialized` returns false when `node_modules` is a file (not a dir)
- ✅ `Test` returns nil when `package.json` is missing
- ✅ `Test` returns nil when `package.json` has no `test` script key
- ✅ `Test` returns nil when `package.json` has no `scripts` section
- ✅ `Test` returns nil when `package.json` is malformed JSON
- ✅ `NewBuildSystem` returns `*GoBuildSystem` for `"go"`
- ✅ `NewBuildSystem` returns `*NpmBuildSystem` for `"npm"`
- ✅ `NewBuildSystem` returns error for unknown type `"python"`
- ✅ `NewBuildSystem` error message is non-empty for unknown type `"rust"`
