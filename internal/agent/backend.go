package agent

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"time"

	"github.com/robertgumeny/doug/internal/config"
)

// Backend is the execution seam for supervised subprocess/RPC agent invocations.
//
// In Pi-configured projects (interaction_mode: interactive or rpc), NewBackend
// returns a PiAdapter — the required Doug-to-agent execution boundary. Doug never
// launches agent subprocesses directly in these modes; Pi owns model selection,
// tool enforcement, and agent process lifecycle. In non-Pi projects, NewBackend
// returns DefaultBackend, which invokes the agent subprocess directly.
//
// Supervised subprocess/RPC call sites route agent execution through this interface:
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
//  4. cmd/research.go — researchProjectContext (via package-level researchRunAgent var)
//     Uses cfg.ResearchAgentCommand; no heartbeat; output is nil (interactive terminal);
//     write-scoped to .doug/logs/research/ via ResearchContract.
//
// Contract shared by all call sites:
//   - A non-zero exit code from the agent is non-fatal: callers log a warning
//     and continue to read the session result from ACTIVE_TASK.md.
//   - Context cancellation causes the subprocess to be killed; Run returns
//     ctx.Err() in that case.
//   - An empty or whitespace-only Command is a validation error returned before
//     the subprocess is started.
//   - RunResponse carries only runtime/transport facts. Workflow outcomes
//     (SUCCESS, FAILURE, BUG, EPIC_COMPLETE) are authoritative only in
//     ACTIVE_TASK.md and are parsed separately by the orchestrator.
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
	RunPhaseResearch   RunPhase = "research"
)

// BriefFormat identifies the on-disk format of a canonical briefing artifact.
type BriefFormat string

const (
	BriefFormatMarkdown BriefFormat = "markdown"
)

// ArtifactAuthority identifies which system owns a run artifact contractually.
type ArtifactAuthority string

const (
	ArtifactAuthorityProject ArtifactAuthority = "project"
	ArtifactAuthorityDoug    ArtifactAuthority = "doug"
	ArtifactAuthorityPi      ArtifactAuthority = "pi"
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
	Authority ArtifactAuthority
}

// ContextInput describes an ordered context artifact the backend may load.
type ContextInput struct {
	Kind      ContextInputKind
	Path      string
	Required  bool
	Authority ArtifactAuthority
}

// ArtifactPurpose classifies an artifact surface exposed to a backend.
type ArtifactPurpose string

const (
	ArtifactPurposeProjectInstructions ArtifactPurpose = "project_instructions"
	ArtifactPurposeProductContext      ArtifactPurpose = "product_context"
	ArtifactPurposeCanonicalBrief      ArtifactPurpose = "canonical_brief"
	ArtifactPurposeWorkingArtifact     ArtifactPurpose = "working_artifact"
	ArtifactPurposeProjectWorkspace    ArtifactPurpose = "project_workspace"
	ArtifactPurposeBugHandoff          ArtifactPurpose = "bug_handoff"
	ArtifactPurposeFailureHandoff      ArtifactPurpose = "failure_handoff"
	ArtifactPurposeKnowledgeBase       ArtifactPurpose = "knowledge_base"
	ArtifactPurposeRuntimeArchive      ArtifactPurpose = "runtime_archive"
	ArtifactPurposeSessionArchive      ArtifactPurpose = "session_archive"
)

// ArtifactSurface describes one read or write path surface exposed to a backend.
type ArtifactSurface struct {
	Path        string
	Purpose     ArtifactPurpose
	Authority   ArtifactAuthority
	AgentFacing bool
}

// ArtifactSurfaces enumerates the intended read and write path surfaces for a run.
// Doug-owned control and lifecycle artifacts are non-agent-facing by default; only
// surfaces listed under Write are intended writable surfaces for that run.
type ArtifactSurfaces struct {
	Read  []ArtifactSurface
	Write []ArtifactSurface
}

// RoutingInputs provide Doug-owned routing signals for backend selection.
type RoutingInputs struct {
	Workflow        string
	SkillName       string
	InteractionMode string // resolved interaction mode (e.g. "subprocess", "rpc"); empty means backend default
}

// PolicyInputs carries Doug-owned policy inputs resolved before backend
// invocation so the backend does not need to invent policy.
type PolicyInputs struct {
	SessionPolicy   string // resolved routing profile for session policy
	ToolPolicy      string // resolved tool-access policy identifier
	SessionDefaults string // resolved session defaults identifier
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

// LifecycleHooks exposes optional timeout/cancellation callbacks so callers
// can observe backend lifecycle interrupts without changing workflow outcome
// authority or transport behavior.
type LifecycleHooks struct {
	Timeout      func(elapsed time.Duration)
	Cancellation func(elapsed time.Duration, cause error)
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
// DefaultBackend (subprocess compat path) does not enforce restrictions; PiAdapter
// (rpc mode) translates restriction hooks into Pi-native policy enforcement.
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

	// Artifacts exposes the intended read-path hook points and writable
	// surfaces for the run. Future backends may translate this into provider-
	// native policy while keeping Doug-owned control artifacts non-agent-facing
	// by default.
	Artifacts ArtifactSurfaces

	// Routing provides Doug-native routing inputs such as workflow and skill.
	Routing RoutingInputs

	// Policy provides Doug-owned session policy placeholders.
	Policy PolicyInputs

	// Restrictions reserves backend hook points for read/write controls.
	Restrictions RestrictionHooks

	// Lifecycle exposes optional timeout/cancellation callbacks for callers
	// that need to observe backend interruption paths.
	Lifecycle LifecycleHooks

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

	// AvailableSessionIDs reports any backend-visible session identifiers
	// observed during the run. This is runtime-only observability data and does
	// not affect Doug workflow semantics.
	AvailableSessionIDs []string

	// RestrictionViolations reports backend-enforced policy breaches.
	RestrictionViolations []RestrictionViolation
}

// NewBackend returns the Backend selected by the resolved execution policy.
//
// config.InteractionModeInteractive ("interactive") or config.InteractionModeRPC
// ("rpc") → PiAdapter, the required execution boundary for Pi-configured
// projects. Pi owns model selection, tool enforcement, and agent process lifecycle.
//
// config.InteractionModeSubprocess ("subprocess") or "" → DefaultBackend, the
// compatibility path for non-Pi agents (claude, codex, gemini). An empty
// interaction_mode is accepted as a backward-compatible alias for "subprocess"
// when no policy is set in doug.yaml. New projects using non-Pi agents should
// set interaction_mode: subprocess explicitly.
//
// Unknown modes are rejected by PrepareExecution before NewBackend is called.
// The default case here is therefore only reached in tests or direct construction.
func NewBackend(exec config.ResolvedExecution) Backend {
	switch exec.InteractionMode {
	case config.InteractionModeInteractive, config.InteractionModeRPC:
		return NewPiAdapter()
	default:
		// "subprocess" or "" — compatibility path for direct subprocess agents.
		return DefaultBackend{}
	}
}

// DefaultBackend is the compatibility path for non-Pi agents (claude, codex, gemini).
// It launches agents as direct subprocesses and does not enforce write restrictions
// or model selection — those remain agent-owned in this mode.
//
// Entry conditions: interaction_mode is config.InteractionModeSubprocess ("subprocess")
// or unset ("") in doug.yaml. Pi-configured projects use PiAdapter instead
// (interaction_mode: interactive or rpc).
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
		fireLifecycleHooks(req.Lifecycle, d, err)
	case exitCode >= 0:
		code := exitCode
		resp.ExitCode = &code
	default:
		resp.Status = RunStatusRejected
	}

	return resp, err
}

func fireLifecycleHooks(hooks LifecycleHooks, elapsed time.Duration, cause error) {
	if errors.Is(cause, context.DeadlineExceeded) && hooks.Timeout != nil {
		hooks.Timeout(elapsed)
	}
	if (errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded)) && hooks.Cancellation != nil {
		hooks.Cancellation(elapsed, cause)
	}
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
