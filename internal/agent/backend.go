package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/robertgumeny/doug/internal/config"
)

// Backend is the execution seam for supervised Pi agent invocations.
//
// Production Doug agent workflows route through PiAdapter. Doug never launches
// arbitrary provider executables such as claude, codex, gemini, or configured
// agent command templates directly; Pi owns provider/model selection, tool
// enforcement, and agent process lifecycle.
//
// RunResponse carries only runtime/transport facts. Workflow outcomes (SUCCESS,
// FAILURE, BUG, EPIC_COMPLETE) are authoritative only in ACTIVE_TASK.md and are
// parsed separately by the orchestrator.
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
	InteractionMode string // resolved interaction mode (e.g. "interactive", "rpc")
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
// PiAdapter translates restriction hooks into Pi-native policy enforcement.
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

	// Command is the fully resolved Doug-owned workflow prompt. All placeholders
	// ({{skill_name}}, {{task_id}}) must be substituted by the caller before
	// constructing the request.
	Command string

	// ProjectRoot is the working directory for the Pi invocation.
	ProjectRoot string

	// HeartbeatInterval, when > 0 and HeartbeatFn is non-nil, triggers
	// periodic elapsed-time callbacks while the agent process is running.
	HeartbeatInterval time.Duration

	// HeartbeatFn receives the elapsed duration at each heartbeat tick.
	// Ignored when HeartbeatInterval is 0 or HeartbeatFn is nil.
	HeartbeatFn func(elapsed time.Duration)

	// Output receives mirrored Pi RPC output when supported. Pass a file or
	// io.Discard to capture or suppress output in non-interactive runs.
	Output io.Writer
}

// RunResponse holds the outputs from a completed agent invocation.
type RunResponse struct {
	// Status reports the runtime/transport state of the backend invocation
	// without implying any Doug workflow outcome semantics.
	Status RunStatus

	// Duration is the wall-clock time the agent process ran.
	Duration time.Duration

	// ExitCode captures the Pi process exit code when one exists. It is nil
	// when the backend rejects the request before launch or no process ran.
	ExitCode *int

	// SessionID reserves a backend-owned runtime identifier such as a Pi session
	// or run ID.
	SessionID string

	// AvailableSessionIDs reports any backend-visible session identifiers
	// observed during the run. This is runtime-only observability data and does
	// not affect Doug workflow semantics.
	AvailableSessionIDs []string

	// RestrictionViolations reports backend-enforced policy breaches.
	RestrictionViolations []RestrictionViolation
}

// NewBackend returns Doug's production agent backend.
//
// Pi is the exclusive agent execution boundary. The resolved execution policy is
// still accepted so call sites keep passing the complete preparation result, but
// backend selection no longer falls back to direct provider child processes.
func NewBackend(_ config.ResolvedExecution) Backend {
	return NewPiAdapter()
}

func fireLifecycleHooks(hooks LifecycleHooks, elapsed time.Duration, cause error) {
	if errors.Is(cause, context.DeadlineExceeded) && hooks.Timeout != nil {
		hooks.Timeout(elapsed)
	}
	if (errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded)) && hooks.Cancellation != nil {
		hooks.Cancellation(elapsed, cause)
	}
}
