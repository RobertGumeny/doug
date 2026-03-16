---
title: internal/prompt — Reusable Interactive Prompt Helpers
updated: 2026-03-16
category: Packages
tags: [prompt, interactive, tty, cli, input]
related_articles:
  - docs/kb/packages/init.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# internal/prompt — Reusable Interactive Prompt Helpers

## Overview

`internal/prompt` provides reusable interactive prompt functions for CLI commands. All functions accept `io.Writer` (for output) and `io.Reader` (for input) so they are testable without a real terminal. The `isTTY bool` parameter gates interactivity: when `false`, the function returns the default value silently, satisfying the non-interactive / flag-provided-value path.

---

## API

### IsTTY

```go
func IsTTY(f *os.File) bool
```

Reports whether `f` (typically `os.Stdin`) is connected to an interactive terminal. Returns `false` if the stat call fails. Call this once per command invocation and thread the result through to each prompt function.

### SelectOne

```go
func SelectOne(w io.Writer, r io.Reader, isTTY bool, question string, options []string, defaultIdx int) (int, string, error)
```

Displays a numbered list of options and reads a single selection. When `isTTY == false`, returns `(defaultIdx, options[defaultIdx], nil)` without printing or reading anything. On empty input, or out-of-range / non-numeric input, the default is returned — no error. Returns a non-nil error only when `options` is empty.

### Confirm

```go
func Confirm(w io.Writer, r io.Reader, isTTY bool, question string, defaultYes bool) (bool, error)
```

Displays a `[Y/n]` or `[y/N]` yes/no question. When `isTTY == false`, returns `defaultYes` without prompting. Accepted affirmative inputs: `y`, `yes`. Accepted negative inputs: `n`, `no`. Empty input and any other value return `defaultYes`.

### Text

```go
func Text(w io.Writer, r io.Reader, isTTY bool, question string, defaultVal string) (string, error)
```

Displays a free-text prompt. When `isTTY == false`, returns `defaultVal` without prompting. Empty input returns `defaultVal`. When `defaultVal != ""`, the prompt displays it inline as `question [defaultVal]: `.

---

## Design Notes

**`io.Writer` / `io.Reader` injection** — All functions write to `w` and read from `r`, not `os.Stdout`/`os.Stdin` directly. Tests pass `bytes.Buffer` / `strings.Reader` for full coverage without a real TTY.

**Caller-supplied `isTTY`** — Functions do not re-check the terminal; `isTTY` is a plain `bool`. The caller is responsible for calling `prompt.IsTTY(os.Stdin)` once and threading the result through.

**No fatal paths** — All functions return the default on read error or unrecognised input. No prompt failure causes a fatal command exit.

**`cmd/init.go` uses its own inline prompt functions** — `promptAgentSelection`, `promptBuildSystemSelection`, `promptIntValue`, and `promptBoolValue` are defined locally in `cmd/init.go` and predate the `internal/prompt` package. `internal/prompt` was added as a reusable foundation for future commands and is tested independently.

---

## Related

- [cmd/init](init.md) — local prompt helpers (`promptAgentSelection`, `promptBuildSystemSelection`, `promptIntValue`, `promptBoolValue`)
- [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md)
