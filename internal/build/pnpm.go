package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PnpmBuildSystem implements BuildSystem for Node.js projects using pnpm commands.
// All commands use exec.Command with an explicit args slice — no shell eval.
type PnpmBuildSystem struct {
	projectRoot string
}

// NewPnpmBuildSystem creates a PnpmBuildSystem rooted at projectRoot.
func NewPnpmBuildSystem(projectRoot string) *PnpmBuildSystem {
	return &PnpmBuildSystem{projectRoot: projectRoot}
}

// IsInitialized returns true if node_modules/ directory exists in the project root.
func (p *PnpmBuildSystem) IsInitialized() bool {
	info, err := os.Stat(filepath.Join(p.projectRoot, "node_modules"))
	return err == nil && info.IsDir()
}

// Install runs pnpm install to fetch all dependencies.
func (p *PnpmBuildSystem) Install() error {
	cmd := exec.Command("pnpm", "install")
	cmd.Dir = p.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// Build runs pnpm run build and returns an error containing the last 50 lines of output on failure.
func (p *PnpmBuildSystem) Build() error {
	cmd := exec.Command("pnpm", "run", "build")
	cmd.Dir = p.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// Test runs pnpm run test if a test script is configured in package.json.
// Returns nil (skip) if no test script key exists in package.json.
// Returns nil (skip) if the command output contains the NO_TESTS_CONFIGURED sentinel.
func (p *PnpmBuildSystem) Test() error {
	if !p.hasTestScript() {
		return nil
	}

	cmd := exec.Command("pnpm", "run", "test")
	cmd.Dir = p.projectRoot
	out, err := cmd.CombinedOutput()

	if strings.Contains(string(out), "NO_TESTS_CONFIGURED") {
		return nil
	}

	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// hasTestScript reports whether package.json in the project root contains a "test" key under "scripts".
func (p *PnpmBuildSystem) hasTestScript() bool {
	data, err := os.ReadFile(filepath.Join(p.projectRoot, "package.json"))
	if err != nil {
		return false
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	_, ok := pkg.Scripts["test"]
	return ok
}
