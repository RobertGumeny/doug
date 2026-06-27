---
title: internal/interactive — Reusable Interactive Command UX
updated: 2026-05-11
category: Packages
tags: [interactive, prompt, bubbletea, tty, cli, input, composer]
related_articles:
  - docs/kb/packages/prompt.md
  - docs/kb/patterns/pattern-best-effort-writes.md
  - docs/kb/packages/init.md
---

# internal/interactive — Reusable Interactive Command UX

## Overview

`internal/interactive` is the shared interactive command UX foundation for Doug CLI commands. It owns the terminal interaction abstraction so command packages can drive prompts without importing Bubble Tea types directly.

When a real terminal is available, prompts are backed by Bubble Tea. When running in CI, pipes, or tests, a plain line-reader fallback is used that returns default values without touching the terminal.

---

## API

### Prompter interface

```go
type Prompter interface {
    SelectOne(question string, options []string, defaultIdx int) (int, string, error)
    Confirm(question string, defaultYes bool) (bool, error)
    Text(question string, defaultVal string) (string, error)
    Compose(header string, defaultVal string) (string, error)
}
```

**SelectOne** — Presents a Bubbles `list`-backed cursor-navigable list. Navigate with ↑/↓ or j/k, confirm with Enter or Space. Returns `(index, value, error)`; error only when `options` is empty. Filtering/search UI is disabled so this remains a simple non-filtering list.

**Confirm** — Presents a `[Y/n]` / `[y/N]` yes/no prompt. Press y/Y, n/N, or Enter (accepts default). Ctrl+C cancels to the default.

**Text** — Presents a Bubbles `textinput`-backed single-line input. Enter submits; Ctrl+C cancels to `defaultVal`; empty input returns `defaultVal`. When `defaultVal != ""` it is displayed inline as `question [defaultVal]: `.

**Compose** — Presents a Bubbles `textarea`-backed multi-line text entry prompt with `header` as the instructions banner. Long lines wrap using textarea behavior while preserving the submitted text. Enter submits the full buffer; Shift+Enter inserts a newline; Ctrl+J is also supported as a newline fallback for terminals that cannot reliably distinguish Shift+Enter; Ctrl+C cancels and returns `defaultVal`. Ctrl+D is also accepted as a submit shortcut for compatibility. Returns `defaultVal` when no text is entered.

---

### IsInteractive

```go
func IsInteractive() bool
```

Reports whether the current process is attached to an interactive terminal. When `false`, `New()` returns the plain fallback prompter and all prompt methods return their default values without reading from the terminal.

Call this once per command invocation when the command must warn the user or bail out before the first prompt.

---

### Constructors

```go
func New() Prompter
```

Returns a `Prompter` appropriate for the current environment. Uses the Bubble Tea path when `os.Stdin` is a terminal; otherwise the plain fallback.

```go
func NewWithIO(w io.Writer, r io.Reader, isTTY bool) Prompter
```

Returns a `Prompter` that reads from `r` and writes to `w`. When `isTTY` is `false`, the fallback is always used. **Use this constructor in tests.**

---

## Design Notes

**Bubble Tea behind an interface** — Commands import `interactive.Prompter`, not any Bubble Tea type. This keeps Bubble Tea as an implementation detail and prevents it from leaking into command-layer types.

**Non-interactive fallback** — `NewWithIO(..., isTTY=false)` or `New()` when stdin is not a TTY returns a `fallbackPrompter` that delegates to `internal/prompt` with `isTTY=false`. All methods return the default value without writing to the terminal.

**Bubbles-backed prompt internals** — TTY prompts use Bubbles components (`list`, `textinput`, and `textarea`) behind the package-local Bubble Tea models. Tests cover the package-visible models directly so command semantics stay stable while Bubbles owns cursor movement, editing, and wrapping.

**Compose testability** — `Compose` on the fallback prompter returns `defaultVal` immediately. Tests construct the prompter with `NewWithIO` and `isTTY=false` to exercise command logic without a real terminal or a running Bubble Tea program. The `composeModel` is also tested directly as a unit, including Enter-submit, Shift+Enter/Ctrl+J newline insertion, Ctrl+D submit, Ctrl+C default/cancel behavior, wrapping, and inline key hints.

**No fatal paths** — All methods return the default value on cancellation (Ctrl+C) or empty input. No prompt failure causes a fatal command exit.

---

## Relationship to internal/prompt

`internal/prompt` is the lower-level, `io.Writer`/`io.Reader`-injected helper layer. `internal/interactive` wraps it for command use: the fallback prompter delegates to `internal/prompt`; the Bubble Tea prompter runs its own terminal models. Command packages should use `internal/interactive`, not `internal/prompt` directly.

---

## Related

- [internal/prompt](prompt.md) — lower-level prompt helpers (`SelectOne`, `Confirm`, `Text`, `IsTTY`)
- [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md)
- [cmd/init](init.md) — all interactive prompts (agent selection, build system, config values) use this abstraction
