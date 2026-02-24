// Package build provides the BuildSystem interface and implementations for supported build systems.
package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildSystem defines the interface for managing project build lifecycle operations.
type BuildSystem interface {
	// Install downloads or installs project dependencies.
	Install() error

	// Build compiles the project.
	Build() error

	// Test runs the project's test suite.
	Test() error

	// IsInitialized reports whether the build system has been initialized for the project.
	IsInitialized() bool
}

// GoBuildSystem implements BuildSystem for Go projects using go toolchain commands.
// All commands use exec.Command with an explicit args slice — no shell eval.
type GoBuildSystem struct {
	projectRoot string
}

// NewGoBuildSystem creates a GoBuildSystem rooted at projectRoot.
func NewGoBuildSystem(projectRoot string) *GoBuildSystem {
	return &GoBuildSystem{projectRoot: projectRoot}
}

// IsInitialized returns true if go.sum exists in the project root.
func (g *GoBuildSystem) IsInitialized() bool {
	_, err := os.Stat(filepath.Join(g.projectRoot, "go.sum"))
	return err == nil
}

// Install runs go mod download to fetch all modules listed in go.mod.
func (g *GoBuildSystem) Install() error {
	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = g.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// Build runs go build ./... and returns an error containing the last 50 lines of output on failure.
func (g *GoBuildSystem) Build() error {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = g.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// Test runs go test ./... and returns an error containing the last 50 lines of output on failure.
func (g *GoBuildSystem) Test() error {
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = g.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return wrapOutput(err, out)
	}
	return nil
}

// wrapOutput returns an error that includes the last 50 lines of command output.
func wrapOutput(err error, output []byte) error {
	lines := strings.Split(string(output), "\n")
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}
	return fmt.Errorf("%w\n%s", err, strings.Join(lines, "\n"))
}
