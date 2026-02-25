---
title: Go Infrastructure & Best Practices
updated: 2026-02-24
category: Infrastructure
tags: [go, golang, build, testing, distribution, goreleaser]
related_articles:
  - docs/kb/dependencies/go-1-26.md
  - docs/kb/packages/types.md
  - docs/kb/packages/state.md
  - docs/kb/packages/config.md
  - docs/kb/packages/log.md
  - docs/kb/packages/build.md
  - docs/kb/packages/git.md
  - docs/kb/packages/orchestrator.md
  - docs/kb/packages/metrics.md
  - docs/kb/packages/changelog.md
  - docs/kb/packages/agent.md
  - docs/kb/packages/handlers.md
---

# Go Infrastructure & Best Practices

## Overview

Doug is built with Go 1.26, the current stable release as of project start. The binary is distributed via GoReleaser for Linux, macOS, and Windows. All contributors should be on 1.26 or newer.

```bash
go version   # should output go1.26.x or higher
```

The `go.mod` minimum version is pinned to `1.26`. Do not lower it.

## Module Path

```
github.com/robertgumeny/doug
```

Replace `robertgumeny` if forked. All internal imports use this path.

## Project Structure

```
doug/
├── cmd/            # Cobra subcommands only — no business logic here
├── internal/
│   ├── types/      # All shared structs and typed constants (EPIC-1-002)
│   ├── state/      # LoadProjectState, SaveProjectState, LoadTasks, SaveTasks (EPIC-1-003)
│   ├── config/     # OrchestratorConfig, LoadConfig, DetectBuildSystem (EPIC-1-004)
│   ├── log/        # Info, Success, Warning, Error, Fatal, Section — ANSI colors (EPIC-2-001)
│   ├── build/      # BuildSystem interface, GoBuildSystem, NpmBuildSystem (EPIC-2-002/003)
│   ├── git/        # EnsureEpicBranch, RollbackChanges, Commit (EPIC-2-004)
│   ├── orchestrator/ # BootstrapFromTasks, task pointer management, validation (EPIC-3-001/002/003)
│   ├── metrics/    # RecordTaskMetrics, UpdateMetricTotals, PrintEpicSummary (EPIC-3-004)
│   ├── changelog/  # UpdateChangelog — idempotent CHANGELOG.md update (EPIC-3-004)
│   ├── agent/      # CreateSessionFile, WriteActiveTask, RunAgent, ParseSessionResult (EPIC-4)
│   ├── templates/  # Embedded session_result.md template via //go:embed (EPIC-4-001)
│   └── handlers/   # HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete (EPIC-5)
├── integration/    # End-to-end tests with real git repos and mock agents
├── main.go         # One line: cmd.Execute()
```

**Rule**: `cmd/` wires things together. All logic lives in `internal/`. If a function in `cmd/` is doing more than calling into `internal/`, it belongs in a package.

## Dependencies

Current approved dependencies:

| Package                  | Purpose                                   |
| ------------------------ | ----------------------------------------- |
| `github.com/spf13/cobra` | CLI framework (`run`, `init` subcommands) |
| `gopkg.in/yaml.v3`       | YAML marshal/unmarshal for state files    |

Everything else should be stdlib. In particular:

**No go-git** — all git operations use `exec.Command("git", ...)` with an explicit args slice.

**No logging library** — custom `internal/log` package using ANSI codes and stdlib only.

**No alternative YAML libraries** — do not introduce `goccy/go-yaml` or `sigs.k8s.io/yaml`.

When adding a new dependency, run `go mod tidy` before writing your session result. The orchestrator's install step runs `go mod download`, which only downloads modules already listed in `go.mod` — it does not resolve new imports from source. You must run `go mod tidy` yourself.

## Key Decisions

**`exec.Command` over shell eval**: Never use `sh -c` or string concatenation to build shell commands. Always pass an explicit args slice. This is a hard rule — it applies to git, build commands, and agent invocation.

**Atomic file writes**: All state file writes go to a `.tmp` file first, then `os.Rename` to the final path. This prevents partial writes from corrupting `project-state.yaml` or `tasks.yaml` if the process is killed mid-write.

**Single `SaveState()` call per iteration**: Load state structs once, mutate in memory, write once. Never multiple sequential mutations to the same file.

**Three failure tiers**: Unambiguous self-correction is silent (Tier 1), recoverable-with-risk emits a warning (Tier 2), ambiguous or git-state-touching failures exit loudly with a clear message (Tier 3). Before any self-correction, ask: could this same condition re-trigger next iteration? If yes, Tier 3.

## Implementation

**Exec commands:**

```go
// Good
cmd := exec.Command("git", "commit", "-m", message)
cmd.Dir = projectRoot

// Bad — shell injection risk, not cross-platform
cmd := exec.Command("sh", "-c", "git commit -m "+message)
```

**Atomic file write:**

```go
tmp := path + ".tmp"
if err := os.WriteFile(tmp, data, 0644); err != nil {
    return err
}
return os.Rename(tmp, path)
```

**Error wrapping:**

```go
// Good — enough context for the caller to log without re-wrapping
return fmt.Errorf("loading project state from %s: %w", path, err)

// Too vague
return fmt.Errorf("failed to load file: %w", err)
```

**Failure tier mapping:**

```go
// Tier 1: handle internally, return nothing
func fixAttemptCounter(state *types.ProjectState) {
    state.ActiveTask.Attempts--
}

// Tier 2: return a warning result, not an error
type ValidationResult struct { AutoCorrected bool; Description string }

// Tier 3: return a non-nil error; main loop calls log.Fatal
return fmt.Errorf("nested bug detected during bugfix task %s — manual intervention required", taskID)
```

**Table-driven tests:**

```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    {"valid SUCCESS", "SUCCESS", "SUCCESS", false},
    {"empty outcome", "", "", true},
    {"unknown outcome", "DONE", "", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

**Integration test skip:**

```go
if testing.Short() {
    t.Skip("skipping integration test")
}
```

## Go 1.26 Features Relevant to Doug

**Green Tea GC (now default)**: Reduces GC overhead by 10–40% for allocation-heavy programs. Doug's YAML struct allocations and file I/O benefit from this automatically. To disable if you see a regression: `GOEXPERIMENT=nogreenteagc` at build time.

**`new()` accepts expressions**: Useful for optional pointer fields in structs. `new(someExpression)` allocates a pointer to the result. Use it where it reduces boilerplate on `ProjectState` optional fields.

**`go fix` is now a modernizer**: Rewritten on the same analysis framework as `go vet`. Run `go fix ./...` periodically — fixers are behavior-preserving and update idioms automatically.

**Stack-allocated slice backing stores**: The compiler stack-allocates slice backing stores in more cases. Short-lived slices in the hot loop (session parsing, task iteration) are cheaper with no code changes needed.

**Faster small allocations**: Size-specialized malloc reduces allocations under 512 bytes by up to 30%. Free win for struct-heavy orchestrator code.

## Build & Distribution

GoReleaser produces release binaries for:

| OS      | Architectures |
| ------- | ------------- |
| Linux   | amd64, arm64  |
| macOS   | amd64, arm64  |
| Windows | amd64         |

| Command            | Effect                                  |
| ------------------ | --------------------------------------- |
| `make build`       | `go build -o doug .`                    |
| `make test`        | `go test ./...`                         |
| `make lint`        | `go vet ./...`                          |
| `make release-dry` | `goreleaser release --snapshot --clean` |

CI runs `go test ./...` and `go vet ./...` on `ubuntu-latest` and `macos-latest` on every push and PR. Do not merge with a failing CI run.

## Edge Cases & Gotchas

**`go.sum` and `IsInitialized()`**: `GoBuildSystem.IsInitialized()` checks for `go.sum` (not `go.mod`). A project with `go.mod` but no `go.sum` has not had `go mod tidy` run and is not ready for `go mod download`. Ensure `go.sum` is committed before starting tasks that depend on installed dependencies.

**Cross-platform paths**: Use `filepath.Join` everywhere — never string concatenation. Use `os.Executable()` or pass `projectRoot` explicitly as a parameter. Never use `os.Getwd()` as a proxy for project root; it breaks when the binary is invoked from a different directory.

**Line endings**: When parsing agent-written files (session results, `ACTIVE_TASK.md`), handle both `\r\n` and `\n`. Agents running on Windows will produce CRLF.

**`go mod download` vs `go mod tidy`**: The orchestrator runs `go mod download` after a task that sets `dependencies_added`. This only fetches modules already in `go.mod`. If you added a new import in source code, you must run `go mod tidy` yourself before writing your session result, or the subsequent build verification will fail and the task will be retried.

## Useful Commands

```bash
# Modernize code to current idioms
go fix ./...

# Check for issues
go vet ./...

# Tidy after adding a new import
go mod tidy

# Build for a specific platform
GOOS=windows GOARCH=amd64 go build -o doug.exe .

# Run only unit tests (skip integration)
go test -short ./...

# Run everything including integration
go test ./...
```

## Related Topics

- [Go 1.26 Dependency](../dependencies/go-1-26.md) — version pinning and upgrade notes
- [internal/types](../packages/types.md) — structs and typed constants
- [internal/state](../packages/state.md) — state file I/O and typed errors
- [internal/config](../packages/config.md) — config loading and build system detection
- [internal/log](../packages/log.md) — colored terminal output functions
- [internal/build](../packages/build.md) — BuildSystem interface, GoBuildSystem, NpmBuildSystem
- [internal/git](../packages/git.md) — EnsureEpicBranch, RollbackChanges, Commit
- [internal/orchestrator](../packages/orchestrator.md) — bootstrap, task pointers, validation
- [internal/metrics](../packages/metrics.md) — RecordTaskMetrics, PrintEpicSummary
- [internal/changelog](../packages/changelog.md) — idempotent CHANGELOG.md update
- [internal/agent](../packages/agent.md) — CreateSessionFile, WriteActiveTask, RunAgent, ParseSessionResult
- [internal/handlers](../packages/handlers.md) — HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete; run loop integration
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — write-to-temp-then-rename pattern
- [Exec Command Pattern](../patterns/pattern-exec-command.md) — safe subprocess invocation
