// Package orchestrator contains the core orchestration logic for the doug
// binary: bootstrapping state from tasks, managing task pointers, and
// validating state consistency.
package orchestrator

import (
	"time"

	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/types"
)

// LoopContext carries all per-iteration state for the orchestration main loop.
// It is initialised once per iteration by the run command and passed to handler
// functions (HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete).
//
// Fields match the explicit list in the EPIC-5 task description:
//
//	TaskID, TaskType, Attempts, CurrentEpic, SessionResult,
//	Config, BuildSystem, ProjectRoot, TaskStartTime
//
// Additional fields (State, Tasks, and file paths) are included so that
// handler functions receive everything they need without additional parameters.
type LoopContext struct {
	// Per-iteration identity
	TaskID    string
	TaskType  types.TaskType
	Attempts  int

	// Snapshot of current_epic at iteration start (for display/logging)
	CurrentEpic types.EpicState

	// Agent output parsed from the session file
	SessionResult *types.SessionResult

	// Orchestrator configuration (from doug.yaml + CLI flag overrides)
	Config *config.OrchestratorConfig

	// Build system for the project (Go or npm)
	BuildSystem build.BuildSystem

	// Absolute path to the project root directory
	ProjectRoot string

	// Wall-clock start time for this task iteration
	TaskStartTime time.Time

	// Mutable shared state — mutated in memory and persisted by handlers
	State *types.ProjectState
	Tasks *types.Tasks

	// File system paths used by handlers
	StatePath     string // path to project-state.yaml
	TasksPath     string // path to tasks.yaml
	LogsDir       string // path to logs/ directory
	ChangelogPath string // path to CHANGELOG.md
}
