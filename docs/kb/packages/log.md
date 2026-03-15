---
title: internal/log — Colored Terminal Output & Logger Interface
updated: 2026-03-15
category: Packages
tags: [log, ansi, terminal, output, fatal, logger, interface]
related_articles:
  - docs/kb/infrastructure/go.md
---

# internal/log — Colored Terminal Output & Logger Interface

## Purpose

`internal/log` provides the `Logger` interface and a concrete `StderrLogger` implementation used throughout the orchestrator. Also retains package-level functions (now writing to stderr) for callers that do not hold a `Logger` instance. No external dependencies — ANSI codes only.

## Key Facts

- Output format: `[LEVEL] message\n` on **stderr** (changed from stdout in EPIC-12)
- `Fatal` logs `[ERROR]` then calls `OsExit(1)` — it does NOT panic
- `OsExit` is an exported package-level `var` so tests can inject a no-op without subprocess overhead
- `Section` prints a blank line, separator, title, separator, blank line — matches `log_section` in `lib/logging.sh`
- `Logger` is the interface threaded through `Orchestrator` and all handlers; package-level functions are kept as a fallback

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
log.New()     // *StderrLogger — writes colored output to os.Stderr
log.Discard() // Logger — silently discards all output; useful in tests
```

`New()` is called once in `orchestrator.New()` and stored on the `Orchestrator` struct. The same `Logger` instance is passed through `LoopContext.Logger` to all handlers.

`Discard()` returns a `discardLogger` that is a no-op for all methods except `Fatal`, which still calls `OsExit(1)`.

## Package-Level Functions (legacy callers)

```go
log.Info("starting iteration")       // white [INFO]
log.Success("task complete")         // green [SUCCESS]
log.Warning("retrying task")         // yellow [WARNING]
log.Error("build failed")            // red [ERROR]
log.Fatal("unrecoverable error")     // red [ERROR] + os.Exit(1)
log.Section("EPIC-2-001")            // cyan box-draw separator + title
```

These write to `os.Stderr`. Prefer `ctx.Logger.*` in handler and orchestrator code; use package-level functions only in `cmd/` or one-off callers that do not hold a `Logger`.

## ANSI Colors

| Function  | Color  | Code          |
|-----------|--------|---------------|
| `Info`    | White  | `\033[1;37m`  |
| `Success` | Green  | `\033[0;32m`  |
| `Warning` | Yellow | `\033[1;33m`  |
| `Error`   | Red    | `\033[0;31m`  |
| `Fatal`   | Red    | `\033[0;31m`  |
| `Section` | Cyan   | `\033[0;36m`  |

Note: `Info` uses bright white (`1;37m`), not blue.

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
- **Output is stderr, not stdout** — changed in EPIC-12. Do not update tests expecting stdout.

## Related

- [Go Infrastructure](../infrastructure/go.md) — project conventions
- [internal/orchestrator](orchestrator.md) — `Orchestrator` struct holds a `Logger` instance
- [internal/handlers](handlers.md) — all handlers receive `Logger` via `LoopContext.Logger`
