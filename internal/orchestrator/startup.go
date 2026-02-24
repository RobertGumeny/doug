package orchestrator

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
)

// CheckDependencies verifies that all binaries required by the orchestrator
// are available on PATH:
//   - The agent command (e.g., "claude") from cfg.AgentCommand
//   - "git"
//   - The language toolchain: "go" when cfg.BuildSystem is "go" (default),
//     or "npm" when cfg.BuildSystem is "npm"
//
// Returns a descriptive error listing every missing binary; nil if all are
// present.
func CheckDependencies(cfg *config.OrchestratorConfig) error {
	required := []string{cfg.AgentCommand, "git"}

	switch cfg.BuildSystem {
	case "npm":
		required = append(required, "npm")
	default:
		required = append(required, "go")
	}

	var missing []string
	for _, bin := range required {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required binaries on PATH: %s",
			strings.Join(missing, ", "))
	}
	return nil
}

// EnsureProjectReady runs a pre-flight build and test to verify the project is
// in a clean, compilable state before the orchestration loop begins.
//
// If buildSys.IsInitialized() returns false (e.g., go.sum or node_modules is
// absent), the pre-flight checks are skipped and a warning is emitted. This
// handles fresh checkouts or projects where dependencies have not been
// installed yet.
//
// Any build or test failure returns an error that already includes the last 50
// lines of output (embedded by the BuildSystem implementations). The caller
// must treat this as a fatal-level error and exit.
func EnsureProjectReady(buildSys build.BuildSystem, cfg *config.OrchestratorConfig) error {
	if !buildSys.IsInitialized() {
		log.Warning(fmt.Sprintf("project is not initialized (build system: %s) — "+
			"skipping pre-flight build/test checks", cfg.BuildSystem))
		return nil
	}

	log.Info("running pre-flight build check")
	if err := buildSys.Build(); err != nil {
		return fmt.Errorf("pre-flight build failed (last 50 lines of output above):\n%w", err)
	}
	log.Success("pre-flight build passed")

	log.Info("running pre-flight test check")
	if err := buildSys.Test(); err != nil {
		return fmt.Errorf("pre-flight tests failed (last 50 lines of output above):\n%w", err)
	}
	log.Success("pre-flight tests passed")

	return nil
}
