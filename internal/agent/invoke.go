package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// splitShellArgs tokenizes s like a POSIX shell, respecting single and double
// quotes and backslash escapes outside quotes. No variable expansion or
// globbing is performed. This allows agent commands in doug.yaml such as:
//
//	claude -p "Refer to CLAUDE.md for instructions"
//
// to be parsed correctly instead of being fragmented by whitespace splitting.
func splitShellArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inSingle := false
	inDouble := false

	// 3-clause loop: the index i must be manipulable inside the loop body so
	// that backslash-escape sequences (e.g. \" inside double quotes) can
	// consume the next character via i++ without an extra layer of state.
	// A range loop over the string would not allow that look-ahead.
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case inSingle:
			if ch == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(ch)
			}
		case inDouble:
			if ch == '\\' && i+1 < len(s) {
				next := s[i+1]
				// Characters escapable inside double quotes per POSIX
				if next == '"' || next == '\\' || next == '$' || next == '`' || next == '\n' {
					cur.WriteByte(next)
					i++
				} else {
					cur.WriteByte(ch)
				}
			} else if ch == '"' {
				inDouble = false
			} else {
				cur.WriteByte(ch)
			}
		case ch == '\\':
			if i+1 < len(s) {
				cur.WriteByte(s[i+1])
				i++
			}
		case ch == '\'':
			inSingle = true
		case ch == '"':
			inDouble = true
		case ch == ' ' || ch == '\t':
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}

	if inSingle {
		return nil, fmt.Errorf("unterminated single quote in agent command")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote in agent command")
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}

	return args, nil
}

// RunAgent invokes the agent using agentCommand parsed with shell-style
// tokenization (respects quoted strings) into executable + args (no shell
// wrapping). The call blocks until the agent exits.
//
// ctx controls the lifetime of the agent subprocess. Cancelling ctx causes
// RunAgent to kill the subprocess and return promptly.
//
// output receives the agent's combined stdout and stderr. If nil, stdin/stdout/
// stderr are all connected to the parent terminal so the agent can run
// interactively. Pass a file or io.Discard to capture or suppress terminal
// output — useful for agents like Codex that unconditionally stream to the
// terminal even in non-interactive mode. Captured runs remain headless and do
// not inherit stdin.
//
// If heartbeatInterval is > 0 and heartbeatFn is non-nil, heartbeatFn is called
// periodically with elapsed runtime while the agent process is running.
//
// Returns the wall-clock duration and any error. A non-zero exit code from
// the agent is returned as an error containing the exit code.
func RunAgent(
	ctx context.Context,
	agentCommand, projectRoot string,
	heartbeatInterval time.Duration,
	heartbeatFn func(elapsed time.Duration),
	output io.Writer,
) (time.Duration, error) {
	trimmed := strings.TrimSpace(agentCommand)
	if trimmed == "" {
		return 0, fmt.Errorf("agentCommand must not be empty or whitespace")
	}

	parts, err := splitShellArgs(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse agent command: %w", err)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = projectRoot
	if output != nil {
		cmd.Stdout = output
		cmd.Stderr = output
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start agent %q: %w", parts[0], err)
	}

	// done is closed after cmd.Wait returns, allowing goroutines below to exit
	// cleanly without leaking.
	done := make(chan struct{})

	// Kill the subprocess if the context is cancelled before it finishes.
	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		case <-done:
		}
	}()

	if heartbeatInterval > 0 && heartbeatFn != nil {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-ticker.C:
					heartbeatFn(time.Since(start))
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			}
		}()
	}

	waitErr := cmd.Wait()
	close(done)
	duration := time.Since(start)

	if ctx.Err() != nil {
		return duration, ctx.Err()
	}

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return duration, fmt.Errorf("agent exited with code %d", exitErr.ExitCode())
		}
		return duration, fmt.Errorf("agent command failed: %w", waitErr)
	}

	return duration, nil
}
