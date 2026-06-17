package types

import (
	"time"

	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
)

// LoopContext carries all per-iteration state for the orchestration main loop.
// It is initialised once per iteration and passed to handler functions
// (HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete).
type LoopContext struct {
	// Per-iteration identity
	TaskID   string
	TaskType TaskType
	Attempts int

	// Snapshot of current_epic at iteration start (for display/logging)
	CurrentEpic EpicState

	// Orchestrator configuration (from doug.yaml + CLI flag overrides)
	Config *config.OrchestratorConfig

	// Build system for the project (Go or npm)
	BuildSystem build.BuildSystem

	// Absolute path to the project root directory
	ProjectRoot string

	// Wall-clock start time for this task iteration
	TaskStartTime time.Time

	// Mutable shared state — mutated in memory and persisted by handlers
	State *ProjectState
	Tasks *Tasks

	// File system paths used by handlers
	StatePath     string // path to .doug/project-state.yaml
	TasksPath     string // path to tasks.yaml
	DougDir       string // path to .doug/ directory (ACTIVE_TASK.md, ACTIVE_BUG.md, ACTIVE_FAILURE.md)
	LogsDir       string // path to .doug/logs/ directory (session/bug/failure archives)
	ChangelogPath string // path to CHANGELOG.md

	// Provider observability captured from the backend run for this iteration.
	ProviderWaitMs   int64
	ProviderFailures []ProviderFailure

	// Logger is the structured output writer for this loop iteration.
	Logger log.Logger
}
