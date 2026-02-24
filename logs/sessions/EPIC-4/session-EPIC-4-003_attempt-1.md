---
task_id: "EPIC-4-003"
outcome: "SUCCESS"
timestamp: "2026-02-24T23:20:00Z"
changelog_entry: "Added RunAgent function to invoke agent commands with live stdout/stderr streaming and duration tracking"
duration_seconds: 240
estimated_tokens: 30000
files_modified:
  - internal/agent/invoke.go
  - internal/agent/invoke_test.go
tests_run: 17
tests_passed: 17
build_successful: true
---

## Implementation Summary

Implemented `RunAgent(agentCommand, projectRoot string) (time.Duration, error)` in `internal/agent/invoke.go`. The function splits the command string by whitespace using `strings.Fields` (no shell wrapping), sets `cmd.Dir = projectRoot`, wires `cmd.Stdout` and `cmd.Stderr` to `os.Stdout`/`os.Stderr` for real-time output, and measures wall-clock duration from before `cmd.Start()` to `cmd.Wait()` completion.

## Files Changed

- `internal/agent/invoke.go` — `RunAgent` implementation with validation, exec setup, duration measurement, and typed error for non-zero exit codes
- `internal/agent/invoke_test.go` — `TestMain` subprocess helper + `TestRunAgent` table-driven tests covering all five acceptance criteria

## Key Decisions

- Used `strings.Fields` for whitespace-split (handles multiple spaces and tabs); `strings.TrimSpace` catches empty/whitespace-only guard before splitting
- Used `*exec.ExitError` type assertion to extract the exact exit code for the error message
- Test strategy: `TestMain` acts as a subprocess controller via `TEST_SUBPROCESS_EXIT` env var; tests invoke the test binary itself (`os.Args[0]`) with `-test.run=^$` to get a controllable agent without external dependencies
- Duration is captured after `cmd.Wait()` returns to include all agent I/O completion

## Test Coverage

- ✅ Empty command returns validation error
- ✅ Whitespace-only command returns validation error
- ✅ Successful execution returns positive duration and no error
- ✅ Non-zero exit code returns error containing the exit code
- ✅ Duration is non-negative for successful run
