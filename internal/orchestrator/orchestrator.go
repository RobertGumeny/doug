package orchestrator

import (
	"fmt"
	"path/filepath"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
)

// Orchestrator owns the configuration, path layout, logger, and build system
// for a single doug run. Its Run method (introduced in a later task) will
// contain the full orchestration loop currently in cmd/run.go.
type Orchestrator struct {
	cfg         *config.OrchestratorConfig
	paths       Paths
	logger      log.Logger
	buildSystem build.BuildSystem
	backend     agent.Backend
}

// New constructs an Orchestrator, resolving the build system from cfg and paths.
// Returns an error if the build system identifier in cfg is not recognised.
func New(cfg *config.OrchestratorConfig, paths Paths) (*Orchestrator, error) {
	buildSys, err := build.NewBuildSystem(cfg.BuildSystem, filepath.Join(paths.ProjectRoot, cfg.ModuleRoot))
	if err != nil {
		return nil, fmt.Errorf("build system: %w", err)
	}
	return &Orchestrator{
		cfg:         cfg,
		paths:       paths,
		logger:      log.New(),
		buildSystem: buildSys,
	}, nil
}

// execBackend returns the injected test backend when set, otherwise returns
// the production Pi backend via agent.NewBackend.
func (o *Orchestrator) execBackend() agent.Backend {
	if o.backend != nil {
		return o.backend
	}
	return agent.NewBackend()
}
