package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunAgent invokes the agent using agentCommand split by whitespace into
// executable + args (no shell wrapping). Stdout and Stderr are piped to the
// parent process in real time. The call blocks until the agent exits.
//
// Returns the wall-clock duration and any error. A non-zero exit code from
// the agent is returned as an error containing the exit code.
func RunAgent(agentCommand, projectRoot string) (time.Duration, error) {
	trimmed := strings.TrimSpace(agentCommand)
	if trimmed == "" {
		return 0, fmt.Errorf("agentCommand must not be empty or whitespace")
	}

	parts := strings.Fields(trimmed)
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
