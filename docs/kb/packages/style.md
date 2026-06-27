---
title: internal/style — Shared Lipgloss Palette
updated: 2026-06-27
category: Packages
tags: [style, lipgloss, terminal, palette]
related_articles:
  - docs/kb/packages/log.md
  - docs/kb/infrastructure/go.md
---

# internal/style — Shared Lipgloss Palette

## Purpose

`internal/style` defines Doug's shared Lipgloss style palette for terminal output. It centralizes reusable styles for log badges, section headers, hints, selected rows, and status text.

## Key Facts

- `NewPalette(w io.Writer)` builds styles from a Lipgloss renderer bound to the target writer.
- Lipgloss/termenv own terminal capability detection, including non-TTY and `NO_COLOR` behavior.
- Non-TTY and no-color output should render as stable plain text; tests should assert rendered strings, not hard-coded ANSI escape sequences.
- The package is an internal implementation detail and does not change Doug's public command or logging APIs.

## Related

- [internal/log](log.md) — consumes the shared palette for log badges and section headers
