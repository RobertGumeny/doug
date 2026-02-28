package agent

import (
	"fmt"
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
// wrapping). Stdout and Stderr are piped to the parent process in real time.
// The call blocks until the agent exits.
//
// Returns the wall-clock duration and any error. A non-zero exit code from
// the agent is returned as an error containing the exit code.
func RunAgent(agentCommand, projectRoot string) (time.Duration, error) {
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start agent %q: %w", parts[0], err)
	}

	waitErr := cmd.Wait()
	duration := time.Since(start)

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return duration, fmt.Errorf("agent exited with code %d", exitErr.ExitCode())
		}
		return duration, fmt.Errorf("agent command failed: %w", waitErr)
	}

	return duration, nil
}
