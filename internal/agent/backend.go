package agent

import (
	"context"
	"errors"
	"io"
	"os/exec"
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

// RunPhase identifies the Doug workflow phase being executed.
type RunPhase string

const (
	RunPhaseRuntime    RunPhase = "runtime"
	RunPhasePlanning   RunPhase = "planning"
	RunPhaseScaffold   RunPhase = "scaffold"
	RunPhasePostEpicKB RunPhase = "post_epic_kb"
)

// BriefFormat identifies the on-disk format of a canonical briefing artifact.
type BriefFormat string

const (
	BriefFormatMarkdown BriefFormat = "markdown"
)

// ContextInputKind classifies an additional context artifact for the backend.
type ContextInputKind string

const (
	ContextInputProjectInstructions ContextInputKind = "project_instructions"
	ContextInputProductContext      ContextInputKind = "product_context"
	ContextInputCanonicalBrief      ContextInputKind = "canonical_brief"
	ContextInputWorkingArtifact     ContextInputKind = "working_artifact"
)

// RestrictionMode describes how a future backend should interpret a hook.
type RestrictionMode string

const (
	RestrictionModeInherit   RestrictionMode = "inherit"
	RestrictionModeAllowList RestrictionMode = "allow_list"
)

// TaskContext identifies the Doug task being executed.
type TaskContext struct {
	ID         string
	Type       string
	Attempt    int
	MaxRetries int
	EpicID     string
	EpicName   string
}

// CanonicalBrief points to the Doug-owned briefing artifact for this run.
type CanonicalBrief struct {
	Path      string
	Format    BriefFormat
	Authority string
}

// ContextInput describes an ordered context artifact the backend may load.
type ContextInput struct {
	Kind     ContextInputKind
	Path     string
	Required bool
}

// RoutingInputs provide Doug-owned routing signals for backend selection.
type RoutingInputs struct {
	Workflow  string
	SkillName string
}

// PolicyInputs carries Doug-owned policy placeholders without encoding any
// backend-specific transport contract.
type PolicyInputs struct {
	SessionPolicy string
}

// RestrictionHook reserves a backend-facing hook point for read or write
// restrictions. Future backends may translate this into provider-native policy.
type RestrictionHook struct {
	Mode  RestrictionMode
	Paths []string
}

// RestrictionHooks groups read/write restriction hooks.
type RestrictionHooks struct {
	Read  RestrictionHook
	Write RestrictionHook
}

// RunStatus reports backend/runtime transport state only. It intentionally
// does not encode Doug workflow outcomes such as SUCCESS or BUG, which remain
// authoritative in ACTIVE_TASK.md and are parsed separately by the orchestrator.
type RunStatus string

const (
	RunStatusCompleted RunStatus = "completed"
	RunStatusRejected  RunStatus = "rejected"
	RunStatusCancelled RunStatus = "cancelled"
)

// RestrictionViolation reports a backend-level read/write restriction breach.
// The current DefaultBackend does not enforce restrictions, so production runs
// return an empty list until a future backend translates these hooks.
type RestrictionViolation struct {
	Kind   string
	Path   string
	Detail string
}

// RunRequest holds all inputs for a single agent invocation.
type RunRequest struct {
	// Phase identifies the Doug workflow path that produced this request.
	Phase RunPhase

	// Task identifies the Doug task context for the run.
	Task TaskContext

	// Brief identifies the Doug-owned canonical briefing artifact for this run.
	Brief CanonicalBrief

	// ContextLoadOrder lists any backend-loadable context artifacts in stable
	// order so future backends can preserve prompt-cache-friendly sequencing.
	ContextLoadOrder []ContextInput

	// Routing provides Doug-native routing inputs such as workflow and skill.
	Routing RoutingInputs

	// Policy provides Doug-owned session policy placeholders.
	Policy PolicyInputs

	// Restrictions reserves backend hook points for read/write controls.
	Restrictions RestrictionHooks

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
	// Status reports the runtime/transport state of the backend invocation
	// without implying any Doug workflow outcome semantics.
	Status RunStatus

	// Duration is the wall-clock time the agent process ran.
	Duration time.Duration

	// ExitCode captures the subprocess exit code when one exists. It is nil
	// when the backend rejects the request before launch or no subprocess ran.
	ExitCode *int

	// SessionID reserves a backend-owned runtime identifier such as a provider
	// session or run ID. DefaultBackend leaves this empty.
	SessionID string

	// RestrictionViolations reports backend-enforced policy breaches.
	RestrictionViolations []RestrictionViolation
}

// DefaultBackend is the production Backend. It wraps RunAgent and preserves
// all existing call-site behavior with no changes.
type DefaultBackend struct{}

// Run implements Backend by delegating to RunAgent.
func (DefaultBackend) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	d, err := RunAgent(ctx, req.Command, req.ProjectRoot, req.HeartbeatInterval, req.HeartbeatFn, req.Output)
	resp := RunResponse{
		Status:   RunStatusCompleted,
		Duration: d,
	}
	if err == nil {
		code := 0
		resp.ExitCode = &code
		return resp, nil
	}

	exitCode := extractExitCode(err)
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		resp.Status = RunStatusCancelled
	case exitCode >= 0:
		code := exitCode
		resp.ExitCode = &code
	default:
		resp.Status = RunStatusRejected
	}

	return resp, err
}

func extractExitCode(err error) int {
	if err == nil {
		return -1
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return parseExitCode(err.Error())
}

func parseExitCode(msg string) int {
	const prefix = "agent exited with code "
	if len(msg) <= len(prefix) || msg[:len(prefix)] != prefix {
		return -1
	}

	code := 0
	for i := len(prefix); i < len(msg); i++ {
		ch := msg[i]
		if ch < '0' || ch > '9' {
			return -1
		}
		code = code*10 + int(ch-'0')
	}
	return code
}
