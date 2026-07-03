// Package lifecycle provides the shared internal task lifecycle core used by
// interactive control surfaces without exposing a public Doug API.
package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// StatusKind describes Doug's current assignment lifecycle state.
type StatusKind string

const (
	StatusNoActiveTask StatusKind = "NO_ACTIVE_TASK"
	StatusActiveTask   StatusKind = "ACTIVE_TASK"
	StatusComplete     StatusKind = "COMPLETE"
)

// Paths identifies the lifecycle files owned by Doug.
type Paths struct {
	ProjectRoot string
	DougDir     string
	StatePath   string
	TasksPath   string
}

// DefaultPaths returns the conventional .doug lifecycle paths for projectRoot.
func DefaultPaths(projectRoot string) Paths {
	dougDir := filepath.Join(projectRoot, ".doug")
	return Paths{
		ProjectRoot: projectRoot,
		DougDir:     dougDir,
		StatePath:   filepath.Join(dougDir, "project-state.yaml"),
		TasksPath:   filepath.Join(dougDir, "tasks.yaml"),
	}
}

// Options configures status discovery and assignment claiming.
type Options struct {
	Paths       Paths
	MaxRetries  int
	BuildSystem string
	Logger      log.Logger
}

// Status reports the current lifecycle state without mutating any files.
type Status struct {
	Kind           StatusKind
	EpicID         string
	ActiveTask     types.TaskPointer
	NextTask       types.TaskPointer
	ActiveTaskPath string
	AllTasksDone   bool
}

// ClaimResult reports the result of a mutating assignment claim.
type ClaimResult struct {
	Status
	AlreadyActive bool
	Claimed       bool
	Attempt       int
}

// DiscoverStatus loads Doug state and tasks and reports whether an assignment
// is currently active, available to claim, or complete. It never mutates state.
func DiscoverStatus(opts Options) (Status, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return Status{}, err
	}
	return discover(paths, projectState, tasks), nil
}

// ClaimNext claims the current TODO assignment for an interactive worker. The
// claim writes .doug/ACTIVE_TASK.md and persists the incremented attempt counter
// but deliberately does not write IN_PROGRESS into tasks.yaml, preserving the
// existing headless lifecycle semantics.
func ClaimNext(opts Options) (ClaimResult, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return ClaimResult{}, err
	}

	current := discover(paths, projectState, tasks)
	if current.Kind == StatusActiveTask {
		return ClaimResult{Status: current, AlreadyActive: true, Attempt: current.ActiveTask.Attempts}, nil
	}
	if current.Kind == StatusComplete {
		return ClaimResult{Status: current}, nil
	}

	task, ok := firstTODO(tasks)
	if !ok {
		current.Kind = StatusComplete
		current.AllTasksDone = true
		return ClaimResult{Status: current}, nil
	}

	projectState.ActiveTask = types.TaskPointer{Type: task.Type, ID: task.ID, Attempts: projectState.ActiveTask.Attempts}
	if projectState.ActiveTask.Attempts < 0 {
		projectState.ActiveTask.Attempts = 0
	}
	projectState.ActiveTask.Attempts++
	projectState.NextTask = nextTODOAfter(tasks, task.ID)

	if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
		return ClaimResult{}, fmt.Errorf("save project state after claim: %w", err)
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.Discard()
	}
	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:             task.ID,
		TaskType:           task.Type,
		DougDir:            paths.DougDir,
		ProjectRoot:        paths.ProjectRoot,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Attempts:           projectState.ActiveTask.Attempts,
		MaxRetries:         opts.MaxRetries,
		BuildSystem:        opts.BuildSystem,
	}, logger); err != nil {
		return ClaimResult{}, fmt.Errorf("write active task: %w", err)
	}

	claimed := discover(paths, projectState, tasks)
	return ClaimResult{Status: claimed, Claimed: true, Attempt: projectState.ActiveTask.Attempts}, nil
}

func normalizePaths(paths Paths) Paths {
	if paths.ProjectRoot == "" {
		paths.ProjectRoot = "."
	}
	if paths.DougDir == "" {
		paths.DougDir = filepath.Join(paths.ProjectRoot, ".doug")
	}
	if paths.StatePath == "" {
		paths.StatePath = filepath.Join(paths.DougDir, "project-state.yaml")
	}
	if paths.TasksPath == "" {
		paths.TasksPath = filepath.Join(paths.DougDir, "tasks.yaml")
	}
	return paths
}

func load(paths Paths) (*types.ProjectState, *types.Tasks, error) {
	projectState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, nil, fmt.Errorf("project state not found at %s", paths.StatePath)
		}
		return nil, nil, fmt.Errorf("load project state: %w", err)
	}
	tasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, nil, fmt.Errorf("tasks file not found at %s", paths.TasksPath)
		}
		return nil, nil, fmt.Errorf("load tasks: %w", err)
	}
	return projectState, tasks, nil
}

func discover(paths Paths, projectState *types.ProjectState, tasks *types.Tasks) Status {
	activePath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	st := Status{
		EpicID:         projectState.CurrentEpic.ID,
		ActiveTask:     projectState.ActiveTask,
		NextTask:       projectState.NextTask,
		ActiveTaskPath: activePath,
		AllTasksDone:   types.AreAllUserTasksComplete(tasks),
	}
	if st.AllTasksDone {
		st.Kind = StatusComplete
		return st
	}
	if projectState.ActiveTask.ID != "" && fileExists(activePath) {
		st.Kind = StatusActiveTask
		return st
	}
	st.Kind = StatusNoActiveTask
	return st
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstTODO(tasks *types.Tasks) (types.Task, bool) {
	for _, task := range tasks.Epic.Tasks {
		if task.Status == types.StatusTODO {
			return task, true
		}
	}
	return types.Task{}, false
}

func nextTODOAfter(tasks *types.Tasks, activeID string) types.TaskPointer {
	foundActive := false
	for _, task := range tasks.Epic.Tasks {
		if foundActive && task.Status == types.StatusTODO {
			return types.TaskPointer{Type: task.Type, ID: task.ID}
		}
		if task.ID == activeID {
			foundActive = true
		}
	}
	return types.TaskPointer{}
}
