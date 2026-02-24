package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NpmBuildSystem implements BuildSystem for Node.js projects using npm commands.
// All commands use exec.Command with an explicit args slice — no shell eval.
type NpmBuildSystem struct {
	projectRoot string
}

// NewNpmBuildSystem creates an NpmBuildSystem rooted at projectRoot.
func NewNpmBuildSystem(projectRoot string) *NpmBuildSystem {
	return &NpmBuildSystem{projectRoot: projectRoot}
}

// IsInitialized returns true if node_modules/ directory exists in the project root.
func (n *NpmBuildSystem) IsInitialized() bool {
	info, err := os.Stat(filepath.Join(n.projectRoot, "node_modules"))
	return err == nil && info.IsDir()
}

// Install runs npm install to fetch all dependencies.
func (n *NpmBuildSystem) Install() error {
	cmd := exec.Command("npm", "install")
	cmd.Dir = n.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// Build runs npm run build and returns an error containing the last 50 lines of output on failure.
func (n *NpmBuildSystem) Build() error {
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = n.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// Test runs npm run test if a test script is configured in package.json.
// Returns nil (skip) if no test script key exists in package.json.
// Returns nil (skip) if the command output contains the NO_TESTS_CONFIGURED sentinel.
func (n *NpmBuildSystem) Test() error {
	if !n.hasTestScript() {
		return nil
	}

	cmd := exec.Command("npm", "run", "test")
	cmd.Dir = n.projectRoot
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
func (n *NpmBuildSystem) hasTestScript() bool {
	data, err := os.ReadFile(filepath.Join(n.projectRoot, "package.json"))
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

// NewBuildSystem returns a BuildSystem implementation for the given buildSystemType.
// Supported types: "go" and "npm".
// Returns a descriptive error for unknown types.
func NewBuildSystem(buildSystemType, projectRoot string) (BuildSystem, error) {
	switch buildSystemType {
	case "go":
		return NewGoBuildSystem(projectRoot), nil
	case "npm":
		return NewNpmBuildSystem(projectRoot), nil
	default:
		return nil, fmt.Errorf("unknown build system type %q: supported types are \"go\" and \"npm\"", buildSystemType)
	}
}
