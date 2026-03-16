---
title: Best-Effort Terminal & Writer Output
updated: 2026-03-15
category: Patterns
tags: [output, writer, errcheck, stderr, stdout, io]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/log.md
  - docs/kb/packages/metrics.md
  - docs/kb/packages/init.md
  - docs/kb/features/revert.md
---

# Best-Effort Terminal & Writer Output

## Rule

When output is informational only and a write failure must **not** change control flow, use a small helper that explicitly discards the `fmt.Fprintf` / `fmt.Fprintln` error:

```go
func writef(w io.Writer, format string, args ...any) {
    _, _ = fmt.Fprintf(w, format, args...)
}
```

Use this for:

- terminal/status output to `os.Stdout` or `os.Stderr`
- report/summary rendering to an injected `io.Writer`
- prompts and guidance text where the surrounding command already owns the real success/failure result

Do **not** use this for:

- file writes that persist state
- subprocess setup where an I/O failure should abort the operation
- any write whose error affects correctness or recovery

## Why

`doug` treats user-facing terminal output and summary rendering as best-effort. The command has already succeeded or failed for its real reason; a broken output stream should not create a new control-flow path. This keeps the code aligned with the non-fatal patterns used elsewhere in the app and satisfies `errcheck` without pretending the write error is actionable.

## Scope

This pattern is currently used in:

- `internal/log` for structured logger output
- `internal/metrics` for `PrintEpicSummary`
- `cmd/` for interactive prompts, list output, and one-shot stderr printing

Package-local helpers are preferred over a shared utility package. Keep the helper adjacent to the output code unless multiple files in the same package need it.

## Examples

```go
// Good: informational output only
writef(os.Stdout, "Selection (1-4, or press Enter for go): ")

// Good: summary rendering should never fail the handler
writef(w, "  %-22s %d\n", "Total Tasks:", total)

// Bad: persisted state write must propagate errors
if err := state.AtomicWrite(path, data); err != nil {
    return fmt.Errorf("write %s: %w", path, err)
}
```

## Related

- [Go Infrastructure](../infrastructure/go.md) — project-wide rules
- [internal/log](../packages/log.md) — canonical logger example
- [internal/metrics](../packages/metrics.md) — best-effort summary rendering
