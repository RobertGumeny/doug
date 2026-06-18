---
title: Drain Subprocess Pipes Before Wait
updated: 2026-06-17
category: Patterns
tags: [exec, pipe, deadlock, stdout, cmd-wait, pi, concurrency]
related_articles:
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/packages/agent.md
---

# Drain Subprocess Pipes Before Wait

## Overview

When Doug reads a subprocess's stdout through an `os.Pipe` (e.g. the Pi RPC
launcher in `internal/agent/pi_adapter.go`), the reader **must consume the pipe
to EOF before `cmd.Wait()` returns** — even on error paths where Doug has
already decided to give up on the output.

This is a hard rule. Violating it produces a silent deadlock that looks exactly
like an agent "hanging" — no error, no exit, just two processes waiting on each
other forever. It was the root cause of the EPIC-44-001 hanging test
(`TestPiCLILauncher_Run/stdout_scanner_errors_report_transport_failure`).

## Why The Deadlock Happens

An OS pipe has a small fixed buffer (~64 KB). The contract is non-negotiable:

- **The writer (Pi) cannot finish a write until someone reads the pipe.** If the
  buffer is full, `write()` blocks.
- **`cmd.Wait()` cannot return until the child exits.** If the child is blocked
  on a write, it cannot exit.

So if Doug's reader stops early — while Pi still has bytes buffered — Pi blocks
on its write, `cmd.Wait()` blocks on Pi, and neither side can make progress:

```
Pi:  write(stdout) ── blocked, pipe full, nobody reading
Doug: cmd.Wait()   ── blocked, waiting for Pi to exit
                      └── deadlock
```

The trigger is any reader exit path that returns before EOF:

- an oversized line that exceeds the `bufio.Scanner` token limit (`ErrTooLong`)
- a JSON decode error on a malformed line
- a failure mirroring output to the log writer
- any future early `return` added to the read loop

## Implementation

Drain on **every** exit path with a single `defer`, not inline at one call site:

```go
func readPiJSONL(r io.Reader, out chan<- piRPCEnvelope, errs chan<- error, mirror io.Writer) {
	defer close(out)
	// Always drain whatever Pi still has buffered on stdout before returning.
	// If we stop reading early (oversized line, decode error, mirror failure)
	// while Pi is mid-write, the OS pipe fills, Pi blocks on the write, and our
	// cmd.Wait() blocks on Pi — a deadlock. Discarding the remainder lets Pi
	// finish writing and exit so cmd.Wait() can return.
	defer func() { _, _ = io.Copy(io.Discard, r) }()

	scanner := bufio.NewScanner(r)
	// ... read loop with multiple early-return error paths ...
}
```

The `defer` is the point: it covers paths that don't exist yet. An inline
`io.Copy` in just one branch (e.g. only after `scanner.Err()`) leaves every
other early return able to re-introduce the deadlock, and the next person to add
a `return` to the loop won't know the rule.

## Key Decisions

**Why discard instead of process the remainder?** Once Doug has hit an error and
decided to abandon the stream, the remaining bytes are noise (often megabytes of
partial streamed deltas). The drain exists purely to unblock the writer, not to
recover data — `io.Discard` is the correct sink.

**Why not just kill the child?** Doug still wants `cmd.Wait()` to reap the
process and surface its exit code (used to classify
`RunStatusTransportFailure`). Draining lets the child exit cleanly so the exit
code is meaningful, rather than masking it behind a signal.

## Edge Cases & Gotchas

**This is separate from the scanner buffer limit.** `readPiJSONL` caps a single
JSONL line at 8 MB (`scanner.Buffer(..., 8*1024*1024)`). Raising or lowering
that limit does not remove the deadlock risk — it only changes which inputs trip
it. The drain is required regardless.

**Regression guard.** Because the failure mode is a hang (not a wrong value),
test it with a goroutine + timeout so a regression fails loudly instead of
wedging the whole suite:

```go
done := make(chan struct{})
go func() { defer close(done); resp, err = launcher.Run(ctx, spec) }()
select {
case <-done:
case <-time.After(10 * time.Second):
    t.Fatal("Run deadlocked: reader did not drain Pi stdout before cmd.Wait()")
}
```

See `decode_error_with_buffered_stdout_does_not_deadlock` and
`stdout_scanner_errors_report_transport_failure` in
`internal/agent/backend_test.go`, backed by subprocess modes in
`internal/agent/testmain_test.go`.

## Related Topics

- [Exec Command Pattern](pattern-exec-command.md) — broader subprocess invocation rules, exit-code extraction
- [internal/agent](../packages/agent.md) — the Pi RPC launcher and `RunStatus` classification
