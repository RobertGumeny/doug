package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// PiInteractiveLaunchRequest describes a true terminal-interactive Pi launch.
// Unlike PiAdapter/RPC runs, this starts Pi in its normal visible CLI mode and
// attaches Pi directly to Doug's current stdin/stdout/stderr.
type PiInteractiveLaunchRequest struct {
	// ProjectRoot is the working directory for the Pi process.
	ProjectRoot string

	// SessionDir is the retained Pi session directory. If empty, it is derived
	// from ProjectRoot, Phase, and Task using PiInteractiveSessionDir.
	SessionDir string

	// Phase and Task identify the Doug workflow context used for default session
	// directory construction.
	Phase RunPhase
	Task  TaskContext

	// Prompt is an optional initial instruction passed to Pi as a positional
	// argument. Leave empty to open a normal interactive Pi session without a
	// bootstrap prompt.
	Prompt string

	// Lifecycle exposes optional cancellation/timeout callbacks.
	Lifecycle LifecycleHooks
}

// PiInteractiveLauncher launches Pi in normal terminal-interactive mode. It is
// intentionally separate from PiAdapter, which is the JSON-RPC execution path.
type PiInteractiveLauncher struct {
	command    string
	baseArgs   []string
	newCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewPiInteractiveLauncher returns the reusable production launcher for visible
// terminal-interactive Pi sessions.
func NewPiInteractiveLauncher() PiInteractiveLauncher {
	return PiInteractiveLauncher{command: "pi"}
}

// PiInteractiveSessionDir returns Doug's retained Pi session directory for a
// true-interactive launch. It matches the RPC Pi session layout so sessions are
// discoverable under one Doug-owned log tree.
func PiInteractiveSessionDir(projectRoot string, phase RunPhase, task TaskContext) string {
	return piSessionDir(RunRequest{ProjectRoot: projectRoot, Phase: phase, Task: task})
}

func buildPiInteractiveArgs(baseArgs []string, sessionDir, prompt string) []string {
	args := append([]string{}, baseArgs...)
	args = append(args, "--session-dir", sessionDir)
	if prompt != "" {
		args = append(args, prompt)
	}
	return args
}

// Run starts Pi in normal CLI mode, attached to the current terminal. It blocks
// until Pi exits and returns transport metadata only; Doug workflow outcomes, if
// any, still belong in Doug-owned artifacts such as ACTIVE_TASK.md.
func (l PiInteractiveLauncher) Run(ctx context.Context, req PiInteractiveLaunchRequest) (RunResponse, error) {
	if req.ProjectRoot == "" {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("pi interactive project root is required")
	}

	sessionDir := req.SessionDir
	if sessionDir == "" {
		sessionDir = PiInteractiveSessionDir(req.ProjectRoot, req.Phase, req.Task)
	}
	if sessionDir == "" {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("pi interactive session directory is required")
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("create pi interactive session directory: %w", err)
	}

	command := l.command
	if command == "" {
		command = "pi"
	}
	newCommand := l.newCommand
	if newCommand == nil {
		newCommand = exec.CommandContext
	}

	args := buildPiInteractiveArgs(l.baseArgs, sessionDir, req.Prompt)
	cmd := newCommand(ctx, command, args...)
	cmd.Dir = req.ProjectRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return RunResponse{Status: RunStatusRejected}, fmt.Errorf("start pi interactive process: %w", err)
	}

	waitErr := cmd.Wait()
	duration := time.Since(start)
	resp := RunResponse{Status: RunStatusCompleted, Duration: duration}

	if ctx.Err() != nil {
		resp.Status = RunStatusCancelled
		fireLifecycleHooks(req.Lifecycle, duration, ctx.Err())
		return resp, ctx.Err()
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			resp.ExitCode = &code
			return resp, fmt.Errorf("pi interactive exited with code %d", code)
		}
		resp.Status = RunStatusRejected
		return resp, fmt.Errorf("wait for pi interactive process: %w", waitErr)
	}

	code := 0
	resp.ExitCode = &code
	return resp, nil
}
