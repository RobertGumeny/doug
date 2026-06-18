package build_test

import (
	"github.com/robertgumeny/doug/internal/testutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/build"
)

// writeFile is a test helper that creates a file in dir with the given contents.
func writeFile(t *testing.T, dir, name, contents string) {
	testutil.WriteFile(t, filepath.Join(dir, name), contents)
}

// --- IsInitialized ---

func TestGoBuildSystemIsInitialized_FalseWhenGoModMissing(t *testing.T) {
	dir := t.TempDir()
	g := build.NewGoBuildSystem(dir)
	if g.IsInitialized() {
		t.Error("expected IsInitialized to return false when go.mod does not exist")
	}
}

func TestGoBuildSystemIsInitialized_TrueWhenGoModExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.26\n")
	g := build.NewGoBuildSystem(dir)
	if !g.IsInitialized() {
		t.Error("expected IsInitialized to return true when go.mod exists")
	}
}

func TestGoBuildSystemIsInitialized_TrueWhenOnlyGoModExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.26\n")
	g := build.NewGoBuildSystem(dir)
	if !g.IsInitialized() {
		t.Error("expected IsInitialized to return true when only go.mod exists (no go.sum)")
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
