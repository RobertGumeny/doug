---
title: Go Infrastructure & Best Practices
updated: 2026-03-12
category: Infrastructure
tags: [go, golang, build, testing, ci, coverage, distribution, goreleaser]
related_articles:
  - docs/kb/dependencies/go-1-26.md
  - docs/kb/features/oss-beta-readiness.md
  - docs/kb/patterns/pattern-best-effort-writes.md
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
  - docs/kb/packages/init.md
  - docs/kb/packages/templates.md
---

# Go Infrastructure & Best Practices

## Overview

doug is built with Go 1.26, the current stable release as of project start. The binary is distributed via GoReleaser for Linux, macOS, and Windows. All contributors should be on 1.26 or newer.

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
├── cmd/
│   ├── run.go      # run subcommand — main orchestration loop
│   └── init.go     # init subcommand — project scaffolding
├── internal/
│   ├── types/      # All shared structs and typed constants
│   ├── state/      # LoadProjectState, SaveProjectState, LoadTasks, SaveTasks
│   ├── config/     # OrchestratorConfig, LoadConfig, DetectBuildSystem
│   ├── log/        # Info, Success, Warning, Error, Fatal, Section — ANSI colors
│   ├── build/      # BuildSystem interface, GoBuildSystem, NpmBuildSystem
│   ├── git/        # EnsureEpicBranch, RollbackChanges, Commit
│   ├── orchestrator/ # BootstrapFromTasks, task pointer management, validation
│   ├── metrics/    # RecordTaskMetrics, UpdateMetricTotals, PrintEpicSummary
│   ├── changelog/  # UpdateChangelog — idempotent CHANGELOG.md update
│   ├── agent/      # WriteActiveTask, RunAgent, ParseSessionResult, ArchiveActiveTask
│   ├── templates/
│   │   ├── runtime/          # Orchestrator-internal templates (never copied to projects)
│   │   │   └── session_result.md  # 3-field frontmatter template used by WriteActiveTask
│   │   ├── init/             # Files copied to new projects by `doug init`
│   │   │   ├── CLAUDE.md, AGENTS.md
│   │   │   ├── *_TEMPLATE.md files (SESSION_RESULTS, BUG_REPORT, FAILURE_REPORT)
│   │   │   └── skills/       # implement-feature, implement-bugfix, implement-documentation
│   │   └── templates.go      # //go:embed Runtime, Init, SessionResult
│   └── handlers/   # HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete
├── integration/    # Empty package (smoke_test removed); doc.go only
├── main.go         # One line: cmd.Execute()
```

**Rule**: `cmd/` wires things together. All logic lives in `internal/`. If a function in `cmd/` is doing more than calling into `internal/`, it belongs in a package.

## Dependencies

Current approved dependencies:

| Package                              | Purpose                                                   |
| ------------------------------------ | --------------------------------------------------------- |
| `github.com/spf13/cobra`             | CLI framework (`run`, `init` subcommands)                 |
| `gopkg.in/yaml.v3`                   | YAML marshal/unmarshal for state files                    |
| `github.com/charmbracelet/bubbletea` | Terminal UI runtime for `internal/interactive` prompters  |

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

**Best-effort terminal or injected-writer output:**

```go
func writef(w io.Writer, format string, args ...any) {
    _, _ = fmt.Fprintf(w, format, args...)
}

// Good: prompt or summary output that should not change command flow
writef(os.Stdout, "Selection (1-4, or press Enter for go): ")
```

Use this pattern only for informational output where a write error is intentionally non-fatal. Persisted file writes and other correctness-affecting I/O must still return errors.

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

## Go 1.26 Features Relevant to doug

**Green Tea GC (now default)**: Reduces GC overhead by 10–40% for allocation-heavy programs. doug's YAML struct allocations and file I/O benefit from this automatically. To disable if you see a regression: `GOEXPERIMENT=nogreenteagc` at build time.

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

| Command            | Effect                                                                 |
| ------------------ | ---------------------------------------------------------------------- |
| `make build`       | `mkdir -p bin && go build -ldflags "-X github.com/robertgumeny/doug/cmd.version=..." -o bin/doug .` |
| `make test`        | `go test ./...`                                                        |
| `make lint`        | non-mutating `gofmt -l .` check, then `golangci-lint run`, then `go vet ./...` |
| `make release-dry` | `goreleaser release --snapshot --clean`                                |

CI runs on `ubuntu-latest`, `macos-latest`, and `windows-latest`. Ubuntu is the canonical quality gate: it runs `go test -coverprofile=coverage.out ./...`, the formatting check, `golangci-lint`, and `go vet ./...`. On push events it also attempts a Codecov upload via GitHub OIDC, but that upload is intentionally non-blocking so transient Codecov outages do not fail the entire CI job. The other matrix jobs still run `go test ./...` so cross-platform regressions remain visible without generating duplicate coverage reports.

Treat formatting, lint, and vet failures as merge blockers. `make lint` is intentionally aligned to the CI checks so local failures should match the GitHub Actions result closely.

## Edge Cases & Gotchas

**`go.sum` and `IsInitialized()`**: `GoBuildSystem.IsInitialized()` checks for `go.sum` (not `go.mod`). A project with `go.mod` but no `go.sum` has not had `go mod tidy` run and is not ready for `go mod download`. Ensure `go.sum` is committed before starting tasks that depend on installed dependencies.

**`make build` shells out to `git describe` for versioning**: The Makefile falls back to `dev` when git metadata is unavailable, but agent tasks running under a no-git policy may need direct `go build ./...` or `go build -o /tmp/doug .` verification instead of `make build`.

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
- [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md) — when to discard `fmt.Fprint*` errors intentionally
- [internal/changelog](../packages/changelog.md) — idempotent CHANGELOG.md update
- [internal/agent](../packages/agent.md) — WriteActiveTask, RunAgent, ParseSessionResult, ArchiveActiveTask
- [internal/templates](../packages/templates.md) — Runtime/Init embed.FS, SessionResult string, template contents
- [internal/handlers](../packages/handlers.md) — HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete; run loop integration
- [cmd/init](../packages/init.md) — `doug init` subcommand, project scaffolding, copyInitTemplates
- [OSS Beta Repository Readiness](../features/oss-beta-readiness.md) — repository-facing OSS metadata, templates, and badges
- [Atomic File Writes](../patterns/pattern-atomic-file-writes.md) — write-to-temp-then-rename pattern
- [Exec Command Pattern](../patterns/pattern-exec-command.md) — safe subprocess invocation
