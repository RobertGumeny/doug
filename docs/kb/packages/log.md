---
title: internal/log — Styled Terminal Output & Logger Interface
updated: 2026-06-27
category: Packages
tags: [log, lipgloss, terminal, output, fatal, logger, interface]
related_articles:
  - docs/kb/packages/style.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# internal/log — Styled Terminal Output & Logger Interface

## Purpose

`internal/log` provides the `Logger` interface and a concrete `StderrLogger` implementation used throughout the orchestrator. Package-level functions remain for callers that do not hold a `Logger` instance. Presentation is routed through the shared Lipgloss palette in `internal/style` while preserving the public logging API.

## Key Facts

- Output format remains `[LEVEL] message\n` on **stderr**.
- `Fatal` logs `[ERROR]` then calls `OsExit(1)` — it does NOT panic.
- `OsExit` is an exported package-level `var` so tests can inject a no-op without subprocess overhead.
- `Section` prints a blank line, separator, title, separator, blank line — matches `log_section` in `lib/logging.sh`.
- `Logger` is the interface threaded through `Orchestrator` and all handlers; package-level functions are kept as a fallback.
- All writes go through a package-local `writef` helper that intentionally discards write errors; logger output is best-effort, not a control-flow boundary.
- Lipgloss/termenv handle terminal capability detection, so non-TTY and `NO_COLOR` output renders as plain, stable text without tests depending on raw ANSI sequences.

## Logger Interface

```go
type Logger interface {
    Info(msg string)
    Success(msg string)
    Warning(msg string)
    Error(msg string)
    Fatal(msg string)
    Section(title string)
}
```

### Constructors

```go
log.New()     // *StderrLogger — writes styled output to os.Stderr
log.Discard() // Logger — silently discards all output; useful in tests
```

`New()` is called once in `orchestrator.New()` and stored on the `Orchestrator` struct. The same `Logger` instance is passed through `LoopContext.Logger` to all handlers.

`Discard()` returns a `discardLogger` that is a no-op for all methods except `Fatal`, which still calls `OsExit(1)`.

## Package-Level Functions (legacy callers)

```go
log.Info("starting iteration")
log.Success("task complete")
log.Warning("retrying task")
log.Error("build failed")
log.Fatal("unrecoverable error") // [ERROR] + os.Exit(1)
log.Section("EPIC-2-001")
```

These write to `os.Stderr`. Prefer `ctx.Logger.*` in handler and orchestrator code; use package-level functions only in `cmd/` or one-off callers that do not hold a `Logger`.

## Testing Fatal

Replace `OsExit` before calling `Fatal` in tests:

```go
var exitCode int
log.OsExit = func(code int) { exitCode = code }
defer func() { log.OsExit = os.Exit }()

log.Fatal("bad state")
// exitCode == 1
```

## Common Pitfalls

- **Use `ctx.Logger.*` in handlers/orchestrator, not package-level `log.*`** — the Logger interface is the injection point for future TUI backends; package-level functions bypass it.
- **Do not call `log.Fatal` in library code** — it calls `os.Exit` and bypasses deferred cleanup. Use it only at the top of the main loop where a clean exit is acceptable.
- **Section separator is 46 `━` characters** — do not change the length or character; it must match the Bash visual style.
- **Output is stderr, not stdout** — do not update tests expecting stdout.
- **Do not assert hard-coded ANSI color sequences** — assert labels/plain output or use ANSI-stripping helpers when tests target rendering behavior.
- **Logger output is best-effort** — follow the package-local `writef` helper pattern from [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md); do not add per-call error branches for `fmt.Fprint*` here.

## Related

- [internal/style](style.md) — shared Lipgloss palette used by log rendering
- [Go Infrastructure](../infrastructure/go.md) — project conventions
- [internal/orchestrator](orchestrator.md) — `Orchestrator` struct holds a `Logger` instance
- [internal/handlers](handlers.md) — all handlers receive `Logger` via `LoopContext.Logger`
