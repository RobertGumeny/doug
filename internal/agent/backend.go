package agent

import (
	"context"
	"io"
	"time"
)

// Backend is the execution seam through which all agent invocations pass.
//
// All four call sites route agent execution through this interface:
//
//  1. internal/orchestrator/run.go — Orchestrator.Run main loop
//     Uses cfg.RunAgentCommand; heartbeat enabled; output goes to a per-task log file.
//
//  2. internal/orchestrator/post_epic_kb.go — runPostEpicKB
//     Uses cfg.RunAgentCommand; heartbeat enabled; output goes to the post-epic-KB log file.
//
//  3. cmd/scaffold.go — scaffoldProjectContext (via package-level scaffoldRunAgent var)
//     Uses cfg.ScaffoldAgentCommand; heartbeat enabled; output goes to a scaffold log file.
//
//  4. cmd/plan.go — planProjectContext (via package-level planRunAgent var)
//     Uses cfg.PlanAgentCommand; no heartbeat; output is nil (interactive terminal).
//
// Contract shared by all call sites:
//   - A non-zero exit code from the agent is non-fatal: callers log a warning
//     and continue to read the session result from ACTIVE_TASK.md.
//   - Context cancellation causes the subprocess to be killed; Run returns
//     ctx.Err() in that case.
//   - An empty or whitespace-only Command is a validation error returned before
//     the subprocess is started.
//
// The default production implementation is DefaultBackend, which delegates to
// RunAgent with no behavior change. Introduce alternative implementations
// (e.g. for the Pi contract or for testing) without touching any call site.
type Backend interface {
	Run(ctx context.Context, req RunRequest) (RunResponse, error)
}

// RunRequest holds all inputs for a single agent invocation.
type RunRequest struct {
	// Command is the fully resolved agent command string. All placeholders
	// ({{skill_name}}, {{task_id}}) must be substituted by the caller before
	// constructing the request. The string is tokenized by POSIX shell rules
	// internally — no sh -c wrapping is applied.
	Command string

	// ProjectRoot is the working directory for the agent subprocess.
	ProjectRoot string

	// HeartbeatInterval, when > 0 and HeartbeatFn is non-nil, triggers
	// periodic elapsed-time callbacks while the agent process is running.
	HeartbeatInterval time.Duration

	// HeartbeatFn receives the elapsed duration at each heartbeat tick.
	// Ignored when HeartbeatInterval is 0 or HeartbeatFn is nil.
	HeartbeatFn func(elapsed time.Duration)

	// Output receives the agent's combined stdout and stderr. When nil the
	// subprocess inherits the parent's stdin/stdout/stderr (interactive mode,
	// used by cmd/plan.go). Pass a file or io.Discard to capture or suppress
	// output in non-interactive runs.
	Output io.Writer
}

// RunResponse holds the outputs from a completed agent invocation.
type RunResponse struct {
	// Duration is the wall-clock time the agent process ran.
	Duration time.Duration
}

// DefaultBackend is the production Backend. It wraps RunAgent and preserves
// all existing call-site behavior with no changes.
type DefaultBackend struct{}

// Run implements Backend by delegating to RunAgent.
func (DefaultBackend) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	d, err := RunAgent(ctx, req.Command, req.ProjectRoot, req.HeartbeatInterval, req.HeartbeatFn, req.Output)
	return RunResponse{Duration: d}, err
}
