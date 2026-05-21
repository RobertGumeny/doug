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
//   - "pi" when any phase has interaction_mode: rpc (Pi is the required execution boundary)
//   - "git"
//   - The language toolchain: "go" when cfg.BuildSystem is "go" (default),
//     "npm" when cfg.BuildSystem is "npm", or "pnpm" when cfg.BuildSystem is "pnpm"
//
// Returns a descriptive error listing every missing binary; nil if all are
// present.
func CheckDependencies(cfg *config.OrchestratorConfig) error {
	required := []string{"git"}
	if cfg.Policy.RequiresRPC() {
		required = append([]string{"pi"}, required...)
	}

	switch cfg.BuildSystem {
	case "npm":
		required = append(required, "npm")
	case "pnpm":
		required = append(required, "pnpm")
	case "static":
		// no toolchain required
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
func EnsureProjectReady(buildSys build.BuildSystem, buildSystemName string, l log.Logger) error {
	if !buildSys.IsInitialized() {
		l.Warning(fmt.Sprintf("project is not initialized (build system: %s) — "+
			"skipping pre-flight build/test checks", buildSystemName))
		return nil
	}

	l.Info("running pre-flight build check")
	if err := buildSys.Build(); err != nil {
		return fmt.Errorf("pre-flight build failed (last 50 lines of output above):\n%w", err)
	}
	l.Success("pre-flight build passed")

	l.Info("running pre-flight test check")
	if err := buildSys.Test(); err != nil {
		return fmt.Errorf("pre-flight tests failed (last 50 lines of output above):\n%w", err)
	}
	l.Success("pre-flight tests passed")

	return nil
}
