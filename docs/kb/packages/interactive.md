---
title: internal/interactive — Reusable Interactive Command UX
updated: 2026-05-06
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

**SelectOne** — Presents a cursor-navigable list. Navigate with ↑/↓ or j/k, confirm with Enter or Space. Returns `(index, value, error)`; error only when `options` is empty.

**Confirm** — Presents a `[Y/n]` / `[y/N]` yes/no prompt. Press y/Y, n/N, or Enter (accepts default).

**Text** — Presents a single-line text input. Enter submits; empty input returns `defaultVal`. When `defaultVal != ""` it is displayed inline as `question [defaultVal]: `.

**Compose** — Presents a multi-line text entry prompt with `header` as the instructions banner. The user types freely, pressing Enter to move to a new line, and submits the full buffer with Ctrl+D. Ctrl+C cancels and returns `defaultVal`. Returns `defaultVal` when no text is entered.

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

**Compose testability** — `Compose` on the fallback prompter returns `defaultVal` immediately. Tests construct the prompter with `NewWithIO` and `isTTY=false` to exercise command logic without a real terminal or a running Bubble Tea program. The `composeModel` is also tested directly as a unit.

**No fatal paths** — All methods return the default value on cancellation (Ctrl+C) or empty input. No prompt failure causes a fatal command exit.

---

## Relationship to internal/prompt

`internal/prompt` is the lower-level, `io.Writer`/`io.Reader`-injected helper layer. `internal/interactive` wraps it for command use: the fallback prompter delegates to `internal/prompt`; the Bubble Tea prompter runs its own terminal models. Command packages should use `internal/interactive`, not `internal/prompt` directly.

---

## Related

- [internal/prompt](prompt.md) — lower-level prompt helpers (`SelectOne`, `Confirm`, `Text`, `IsTTY`)
- [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md)
- [cmd/init](init.md) — first command to adopt this abstraction
