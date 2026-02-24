---
title: internal/build — BuildSystem Interface & Implementations
updated: 2026-02-24
category: Packages
tags: [build, go, npm, interface, exec]
related_articles:
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/infrastructure/go.md
---

# internal/build — BuildSystem Interface & Implementations

## Purpose

`internal/build` defines the `BuildSystem` interface and provides `GoBuildSystem` and `NpmBuildSystem` implementations. The orchestrator uses this package to verify builds and run tests after each agent task.

## Key Facts

- All commands use `exec.Command` with an explicit args slice — no `sh -c` or `eval`
- `Build()` and `Test()` errors include the last 50 lines of command output
- `IsInitialized()` determines whether `Install()` needs to run (missing dependencies)
- `NewBuildSystem` is the entry point — callers never construct implementations directly

## Interface

```go
type BuildSystem interface {
    Install() error        // download/install dependencies
    Build() error          // compile
    Test() error           // run test suite
    IsInitialized() bool   // true if dependencies already installed
}
```

## Factory

```go
bs, err := build.NewBuildSystem("go", projectRoot)   // returns *GoBuildSystem
bs, err := build.NewBuildSystem("npm", projectRoot)  // returns *NpmBuildSystem
bs, err := build.NewBuildSystem("python", projectRoot) // returns error
```

Unknown types return a descriptive error. The `build_system` config value (`"go"` or `"npm"`) is passed directly to this factory.

## GoBuildSystem

| Method | Command | IsInitialized check |
|--------|---------|---------------------|
| `Install` | `go mod download` | — |
| `Build` | `go build ./...` | — |
| `Test` | `go test ./...` | — |
| `IsInitialized` | — | `go.sum` exists |

`IsInitialized()` checks for `go.sum` (not `go.mod`). A project with `go.mod` but no `go.sum` has not had `go mod tidy` run yet and is not ready.

## NpmBuildSystem

| Method | Command | IsInitialized check |
|--------|---------|---------------------|
| `Install` | `npm install` | — |
| `Build` | `npm run build` | — |
| `Test` | `npm run test` (conditional) | — |
| `IsInitialized` | — | `node_modules/` dir exists |

`IsInitialized()` returns false if `node_modules` is a file rather than a directory.

### NpmBuildSystem.Test() Skip Conditions

`Test()` returns `nil` (skip, not failure) when:
1. `package.json` is missing or malformed
2. `package.json` has no `scripts.test` key
3. Command output contains the `NO_TESTS_CONFIGURED` sentinel string

The sentinel check runs before the error check — it is honoured even when `npm run test` exits non-zero.

## Error Format

Build and Test errors include the last 50 lines of output:

```go
// Error message structure on failure:
// <exec.ExitError>
// <last 50 lines of CombinedOutput>
```

Log the full error string to surface compiler output or test failure details.

## Common Pitfalls

- **Never call `go mod tidy` via `BuildSystem`** — the orchestrator only calls `Install()` (`go mod download`). If you add a new import in source code, run `go mod tidy` yourself before writing your session result.
- **`NpmBuildSystem.IsInitialized()` requires a directory** — a plain file named `node_modules` returns false.
- **GoBuildSystem.IsInitialized() checks `go.sum`, not `go.mod`** — a fresh project with only `go.mod` is not considered initialized.

## Related

- [Exec Command Pattern](../patterns/pattern-exec-command.md) — how subprocess invocation works
- [Go Infrastructure](../infrastructure/go.md) — `go mod download` vs `go mod tidy` distinction
