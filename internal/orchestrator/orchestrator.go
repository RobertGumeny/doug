package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
)

// Orchestrator owns the configuration, path layout, logger, and build system
// for a single doug run. Its Run method (introduced in a later task) will
// contain the full orchestration loop currently in cmd/run.go.
type Orchestrator struct {
	cfg               *config.OrchestratorConfig
	paths             Paths
	logger            log.Logger
	buildSystem       build.BuildSystem
	backend           agent.Backend
	infraRetrySleeper func(context.Context, time.Duration) error
}

// New constructs an Orchestrator, resolving the build system from cfg and paths.
// Returns an error if the build system identifier in cfg is not recognised.
func New(cfg *config.OrchestratorConfig, paths Paths) (*Orchestrator, error) {
	logger := log.New()
	modulePath := filepath.Join(paths.ProjectRoot, cfg.ModuleRoot)
	warnIfMissingModuleGoMod(cfg, modulePath, logger)

	buildSys, err := build.NewBuildSystem(cfg.BuildSystem, modulePath)
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

func warnIfMissingModuleGoMod(cfg *config.OrchestratorConfig, modulePath string, logger log.Logger) {
	if cfg.ModuleRoot == "" {
		return
	}

	goModPath := filepath.Join(modulePath, "go.mod")
	info, err := os.Stat(goModPath)
	if err == nil && !info.IsDir() {
		return
	}

	logger.Warning(fmt.Sprintf("module_root %q resolves to %s, but no go.mod was found there; continuing", cfg.ModuleRoot, modulePath))
}

// execBackend returns the injected test backend when set, otherwise returns
// the production Pi backend via agent.NewBackend.
func (o *Orchestrator) execBackend() agent.Backend {
	if o.backend != nil {
		return o.backend
	}
	return agent.NewBackend()
}
