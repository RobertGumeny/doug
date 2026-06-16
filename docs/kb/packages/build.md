---
title: internal/build — BuildSystem Interface & Implementations
updated: 2026-06-16
category: Packages
tags: [build, go, npm, pnpm, interface, exec, module-root]
related_articles:
  - docs/kb/features/module-root.md
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/infrastructure/go.md
---

# internal/build — BuildSystem Interface & Implementations

## Purpose

`internal/build` defines the `BuildSystem` interface and provides `GoBuildSystem`, `NpmBuildSystem`, and `PnpmBuildSystem` implementations. The orchestrator uses this package to verify builds and run tests after each agent task.

## Key Facts

- All commands use `exec.Command` with an explicit args slice — no `sh -c` or `eval`
- `Build()` and `Test()` errors include the last 50 lines of command output
- `IsInitialized()` determines whether `Install()` needs to run (missing dependencies)
- `NewBuildSystem` is the entry point — callers never construct implementations directly
- The `projectRoot` passed to `NewBuildSystem` is already the resolved build root. For configured subdirectory modules, `internal/orchestrator` passes `filepath.Join(paths.ProjectRoot, cfg.ModuleRoot)`.
- **`internal/build` does not create project files.** It never runs `go mod init`, `npm init`, or creates `go.mod`, `package.json`, etc. Those files must already exist. `GoBuildSystem.IsInitialized()` checks for the module sentinel (`go.mod`); Node build systems check whether dependencies have been installed (`node_modules/`). If it returns false, the orchestrator skips pre-flight checks entirely rather than failing.

## Interface

```go
type BuildSystem interface {
    Install() error        // download/install dependencies
    Build() error          // compile
    Test() error           // run test suite
    Lint() error           // run the default lint check; no-op for "static"
    IsInitialized() bool   // true if dependencies already installed
}
```

`Lint()` is the build-system default lint check. It is only called when `config.LintEnabled` is true and no explicit `LintCommand` is configured. For custom lint commands use `build.RunLint` instead.

## Factory

```go
bs, err := build.NewBuildSystem("go", projectRoot)     // returns *GoBuildSystem
bs, err := build.NewBuildSystem("npm", projectRoot)    // returns *NpmBuildSystem
bs, err := build.NewBuildSystem("pnpm", projectRoot)   // returns *PnpmBuildSystem
bs, err := build.NewBuildSystem("python", projectRoot) // returns error
```

Unknown types return a descriptive error. The `build_system` config value (`"go"`, `"npm"`, `"pnpm"`, or `"static"`) is passed directly to this factory. The root argument should be treated as authoritative; implementations should not reinterpret `.doug/doug.yaml` or apply `module_root` themselves.

## RunLint (package-level function)

```go
func RunLint(projectRoot, command string) error
```

Runs an arbitrary lint command in `projectRoot` using a safe parsed-command path. The command string is split via `strings.Fields` into executable + args — no shell eval, no `sh -c`. Returns an error (with the last 50 lines of output) on non-zero exit.

Called by `handlers.runLint` when `config.LintCommand` is non-empty. For the build-system default, `BuildSystem.Lint()` is called instead.

## GoBuildSystem

| Method | Command | IsInitialized check |
|--------|---------|---------------------|
| `Install` | `go mod download` | — |
| `Build` | `go build ./...` | — |
| `Test` | `go test ./...` | — |
| `Lint` | `go vet ./...` | — |
| `IsInitialized` | — | `go.mod` exists |

`IsInitialized()` checks for `go.mod`; a valid Go module is initialized even when it has no `go.sum` yet.

## NpmBuildSystem

| Method | Command | IsInitialized check |
|--------|---------|---------------------|
| `Install` | `npm install` | — |
| `Build` | `npm run build` | — |
| `Test` | `npm run test` (conditional) | — |
| `Lint` | `npm run lint` (conditional) | — |
| `IsInitialized` | — | `node_modules/` dir exists |

`IsInitialized()` returns false if `node_modules` is a file rather than a directory.

### NpmBuildSystem.Test() Skip Conditions

`Test()` returns `nil` (skip, not failure) when:
1. `package.json` is missing or malformed
2. `package.json` has no `scripts.test` key
3. Command output contains the `NO_TESTS_CONFIGURED` sentinel string

The sentinel check runs before the error check — it is honoured even when `npm run test` exits non-zero.

### NpmBuildSystem.Lint() Skip Condition

`Lint()` returns `nil` (skip, not failure) when `package.json` is missing, malformed, or has no `scripts.lint` key.

## PnpmBuildSystem

| Method | Command | IsInitialized check |
|--------|---------|---------------------|
| `Install` | `pnpm install` | — |
| `Build` | `pnpm run build` | — |
| `Test` | `pnpm run test` (conditional) | — |
| `Lint` | `pnpm run lint` (conditional) | — |
| `IsInitialized` | — | `node_modules/` dir exists |

`PnpmBuildSystem` is a peer to `NpmBuildSystem` — same `IsInitialized` check (`node_modules/` directory), same `Test()` skip logic (reads `package.json`).

### PnpmBuildSystem.Test() Skip Conditions

Identical to `NpmBuildSystem`:
1. `package.json` is missing or malformed
2. `package.json` has no `scripts.test` key
3. Command output contains the `NO_TESTS_CONFIGURED` sentinel string

### PnpmBuildSystem.Lint() Skip Condition

Identical to `NpmBuildSystem.Lint()`: returns `nil` when `package.json` is missing, malformed, or has no `scripts.lint` key.

## StaticBuildSystem

`Lint()` is a no-op for static projects (returns `nil`). `Build()`, `Test()`, and `Install()` are also no-ops.

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
- **`NpmBuildSystem` and `PnpmBuildSystem` `IsInitialized()` require a directory** — a plain file named `node_modules` returns false.
- **GoBuildSystem.IsInitialized() checks `go.mod`** — a fresh valid module with only `go.mod` is considered initialized; `go.sum` is not required.
- **Do not add module-root logic here** — the build root is resolved before `NewBuildSystem`; per-command joins or config reads inside `internal/build` would split the contract.
- **pnpm detection uses `pnpm-workspace.yaml`**, not `package.json` — a pnpm monorepo without the workspace file is auto-detected as `npm`.

## Related

- [Build-System Module Root](../features/module-root.md) — build-root resolution and subdirectory module behavior
- [Exec Command Pattern](../patterns/pattern-exec-command.md) — how subprocess invocation works
- [Go Infrastructure](../infrastructure/go.md) — `go mod download` vs `go mod tidy` distinction
