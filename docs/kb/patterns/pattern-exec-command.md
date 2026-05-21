---
title: Exec Command Pattern
updated: 2026-02-23
category: Patterns
tags: [exec, git, shell, cross-platform, security]
related_articles:
  - docs/kb/infrastructure/go.md
---

# Exec Command Pattern

## Overview

All external process invocations in doug use `exec.Command` with an explicit args slice. No `sh -c`, no string concatenation into a shell command, no `eval`. This is a hard rule that applies to git operations, build system commands, and agent invocation.

## Implementation

```go
// Good — explicit args slice
cmd := exec.Command("git", "commit", "-m", message)
cmd.Dir = projectRoot
output, err := cmd.CombinedOutput()

// Bad — shell injection risk, breaks on Windows
cmd := exec.Command("sh", "-c", "git commit -m "+message)

// Bad — same problem with fmt.Sprintf
cmd := exec.Command("sh", "-c", fmt.Sprintf("git commit -m %q", message))
```

## Capturing Output

For commands where you need output on failure (build errors, test failures):

```go
func runAndCapture(projectRoot string, args ...string) ([]byte, error) {
    cmd := exec.Command(args[0], args[1:]...)
    cmd.Dir = projectRoot
    out, err := cmd.CombinedOutput()
    if err != nil {
        // Return last 50 lines of output with the error
        lines := strings.Split(string(out), "\n")
        if len(lines) > 50 {
            lines = lines[len(lines)-50:]
        }
        return out, fmt.Errorf("%w\n%s", err, strings.Join(lines, "\n"))
    }
    return out, nil
}
```

## Agent Output Routing

Agent stdout/stderr must not be buffered in memory, but they also must not unconditionally stream to the terminal — some agents (e.g. `codex exec`) force-stream output even in non-interactive mode, which pollutes the orchestrator display.

The `RunAgent` signature accepts an `output io.Writer`. In `doug run` the orchestrator always provides a log file; `nil` falls back to `os.Stdout`/`os.Stderr` for callers that explicitly want pass-through.

```go
cmd := exec.Command(args[0], args[1:]...)
cmd.Dir = projectRoot
if output != nil {
    cmd.Stdout = output   // redirect to file — silences terminal
    cmd.Stderr = output
} else {
    cmd.Stdout = os.Stdout   // pass-through (original behaviour)
    cmd.Stderr = os.Stderr
}

start := time.Now()
if err := cmd.Start(); err != nil {
    return 0, fmt.Errorf("starting agent: %w", err)
}
// …wait / heartbeat loop…
```

Output logs land at `.doug/logs/output/{epic}/output-{taskID}_attempt-{N}.log`, separate from the session archive under `.doug/logs/sessions/` so the KB synthesis scan is not affected.

Never use `CombinedOutput()` or `Output()` for the agent command — these buffer all output in memory until the process exits.

## Parsing the Resolved Command String

Doug ultimately passes a resolved command string into `exec.Command` on the direct subprocess path (for example when `interaction_mode` resolves to `subprocess`). Split that string into executable + args before passing to `exec.Command`:

```go
parts := strings.Fields(resolvedCommand)
if len(parts) == 0 {
    return fmt.Errorf("agent command is empty")
}
cmd := exec.Command(parts[0], parts[1:]...)
```

`strings.Fields` splits on any whitespace and handles multiple spaces correctly. Do not use `strings.Split(s, " ")` — it produces empty strings on multiple consecutive spaces.

## Key Decisions

**Why not `sh -c`?** Two reasons: shell injection risk if any variable content reaches the command string, and `sh` is not available on Windows without WSL or Git Bash. `exec.Command` with an explicit args slice works identically on all platforms.

**Why `cmd.Dir` instead of `os.Chdir`?** `os.Chdir` changes the working directory for the entire process. `cmd.Dir` scopes the change to the subprocess. Since doug may eventually run tasks concurrently, `os.Chdir` would be unsafe.

**Why not `os/exec` with a shell wrapper for convenience?** The explicit args slice is more verbose but the tradeoff is correctness and cross-platform safety. It is the right default for a tool that runs on Windows.

## Edge Cases & Gotchas

**Exit code vs error**: A non-zero exit code from `cmd.Run()` or `cmd.Wait()` returns an `*exec.ExitError`. Use `errors.As` to extract the exit code if needed:

```go
var exitErr *exec.ExitError
if errors.As(err, &exitErr) {
    code := exitErr.ExitCode()
}
```

**`git commit` with nothing to commit**: Returns exit code 1 with the message "nothing to commit." This is not a fatal error in doug — treat it as a no-op. Check for this specifically rather than treating all non-zero exits from `git commit` as failures.

**PATH on macOS**: GUI-launched processes (e.g. an IDE running doug) may have a minimal PATH that excludes `/usr/local/bin` or `/opt/homebrew/bin`. If `CheckDependencies` reports a binary missing that you know is installed, this is likely the cause. Run doug from a terminal.

## Related Topics

See [Go Infrastructure & Best Practices](../infrastructure/go.md) for the broader exec conventions.
