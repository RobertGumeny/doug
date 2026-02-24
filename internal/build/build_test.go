package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/build"
)

// writeFile is a test helper that creates a file in dir with the given contents.
func writeFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

// --- IsInitialized ---

func TestGoBuildSystemIsInitialized_FalseWhenGoSumMissing(t *testing.T) {
	dir := t.TempDir()
	g := build.NewGoBuildSystem(dir)
	if g.IsInitialized() {
		t.Error("expected IsInitialized to return false when go.sum does not exist")
	}
}

func TestGoBuildSystemIsInitialized_TrueWhenGoSumExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.sum", "")
	g := build.NewGoBuildSystem(dir)
	if !g.IsInitialized() {
		t.Error("expected IsInitialized to return true when go.sum exists")
	}
}

func TestGoBuildSystemIsInitialized_FalseWhenOnlyGoModExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	g := build.NewGoBuildSystem(dir)
	if g.IsInitialized() {
		t.Error("expected IsInitialized to return false when only go.mod exists (no go.sum)")
	}
}

// --- Build ---

func TestGoBuildSystemBuildFailureIncludesOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {\n\tUNDEFINEDSYMBOL\n}\n")

	g := build.NewGoBuildSystem(dir)
	err := g.Build()
	if err == nil {
		t.Fatal("expected Build to return an error for code with syntax errors")
	}
	// Error should include compiler output (go reports undefined identifiers).
	if !strings.Contains(err.Error(), "UNDEFINEDSYMBOL") && !strings.Contains(err.Error(), "undefined") {
		t.Errorf("expected error to contain compiler output, got: %v", err)
	}
}

func TestGoBuildSystemBuildSucceeds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	g := build.NewGoBuildSystem(dir)
	if err := g.Build(); err != nil {
		t.Errorf("expected Build to succeed for valid Go code, got: %v", err)
	}
}

// --- Test ---

func TestGoBuildSystemTestFailureIncludesOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go",
		"package main\n\nimport \"testing\"\n\nfunc TestAlwaysFails(t *testing.T) {\n\tt.Fatal(\"intentional failure\")\n}\n",
	)

	g := build.NewGoBuildSystem(dir)
	err := g.Test()
	if err == nil {
		t.Fatal("expected Test to return an error for a failing test")
	}
	if !strings.Contains(err.Error(), "intentional failure") && !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("expected error to contain test output, got: %v", err)
	}
}

func TestGoBuildSystemTestSucceeds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "main_test.go",
		"package main\n\nimport \"testing\"\n\nfunc TestPasses(t *testing.T) {}\n",
	)

	g := build.NewGoBuildSystem(dir)
	if err := g.Test(); err != nil {
		t.Errorf("expected Test to succeed for passing tests, got: %v", err)
	}
}

// --- Install ---

func TestGoBuildSystemInstallFailsWithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	g := build.NewGoBuildSystem(dir)
	err := g.Install()
	if err == nil {
		t.Error("expected Install to fail in a directory with no go.mod")
	}
}
