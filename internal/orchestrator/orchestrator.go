package orchestrator

import (
	"fmt"

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
}

// New constructs an Orchestrator, resolving the build system from cfg and paths.
// Returns an error if the build system identifier in cfg is not recognised.
func New(cfg *config.OrchestratorConfig, paths Paths, logger log.Logger) (*Orchestrator, error) {
	buildSys, err := build.NewBuildSystem(cfg.BuildSystem, paths.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("build system: %w", err)
	}
	return &Orchestrator{
		cfg:         cfg,
		paths:       paths,
		logger:      logger,
		buildSystem: buildSys,
	}, nil
}
