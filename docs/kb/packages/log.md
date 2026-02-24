---
title: internal/log — Colored Terminal Output
updated: 2026-02-24
category: Packages
tags: [log, ansi, terminal, output, fatal]
related_articles:
  - docs/kb/infrastructure/go.md
---

# internal/log — Colored Terminal Output

## Purpose

`internal/log` provides six colored terminal output functions that match the `[LEVEL] message` visual style of the Bash orchestrator. No external dependencies — ANSI codes only.

## Key Facts

- Output format: `[LEVEL] message\n` on stdout (matches Bash orchestrator convention)
- `Fatal` logs `[ERROR]` then calls `OsExit(1)` — it does NOT panic
- `OsExit` is an exported package-level `var` so tests can inject a no-op without subprocess overhead
- `Section` prints a blank line, separator, title, separator, blank line — matches `log_section` in `lib/logging.sh`

## API

```go
log.Info("starting iteration")       // white [INFO]
log.Success("task complete")         // green [SUCCESS]
log.Warning("retrying task")         // yellow [WARNING]
log.Error("build failed")            // red [ERROR]
log.Fatal("unrecoverable error")     // red [ERROR] + os.Exit(1)
log.Section("EPIC-2-001")            // cyan box-draw separator + title
```

## ANSI Colors

| Function  | Color  | Code          |
|-----------|--------|---------------|
| `Info`    | White  | `\033[1;37m`  |
| `Success` | Green  | `\033[0;32m`  |
| `Warning` | Yellow | `\033[1;33m`  |
| `Error`   | Red    | `\033[0;31m`  |
| `Fatal`   | Red    | `\033[0;31m`  |
| `Section` | Cyan   | `\033[0;36m`  |

Note: `Info` uses bright white (`1;37m`), not blue. The Bash orchestrator's `[INFO]` is blue, but the task spec takes precedence here.

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

- **Do not call `log.Fatal` in library code** — it calls `os.Exit` and bypasses deferred cleanup. Use it only in the main orchestration loop where a clean exit is acceptable.
- **Section separator is 46 `━` characters** — do not change the length or character; it must match the Bash visual style.

## Related

- [Go Infrastructure](../infrastructure/go.md) — project conventions
