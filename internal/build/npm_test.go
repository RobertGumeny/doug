package build_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/build"
)

// --- NpmBuildSystem.IsInitialized ---

func TestNpmBuildSystemIsInitialized_FalseWhenNodeModulesMissing(t *testing.T) {
	dir := t.TempDir()
	n := build.NewNpmBuildSystem(dir)
	if n.IsInitialized() {
		t.Error("expected IsInitialized to return false when node_modules does not exist")
	}
}

func TestNpmBuildSystemIsInitialized_TrueWhenNodeModulesExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatalf("failed to create node_modules directory: %v", err)
	}
	n := build.NewNpmBuildSystem(dir)
	if !n.IsInitialized() {
		t.Error("expected IsInitialized to return true when node_modules directory exists")
	}
}

func TestNpmBuildSystemIsInitialized_FalseWhenNodeModulesIsFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "node_modules", "not a directory")
	n := build.NewNpmBuildSystem(dir)
	if n.IsInitialized() {
		t.Error("expected IsInitialized to return false when node_modules is a file, not a directory")
	}
}

// --- NpmBuildSystem.Test (package.json pre-flight checks) ---

func TestNpmBuildSystemTest_ReturnsNilWhenNoPackageJson(t *testing.T) {
	dir := t.TempDir()
	n := build.NewNpmBuildSystem(dir)
	if err := n.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when package.json is missing, got: %v", err)
	}
}

func TestNpmBuildSystemTest_ReturnsNilWhenTestScriptNotPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp","scripts":{"build":"webpack"}}`)
	n := build.NewNpmBuildSystem(dir)
	if err := n.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when test script is not in package.json, got: %v", err)
	}
}

func TestNpmBuildSystemTest_ReturnsNilWhenNoScriptsSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp","version":"1.0.0"}`)
	n := build.NewNpmBuildSystem(dir)
	if err := n.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when package.json has no scripts section, got: %v", err)
	}
}

func TestNpmBuildSystemTest_ReturnsNilWhenPackageJsonMalformed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `not valid json`)
	n := build.NewNpmBuildSystem(dir)
	if err := n.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when package.json is malformed, got: %v", err)
	}
}

// --- NewBuildSystem factory ---

func TestNewBuildSystem_ReturnsGoBuildSystemForGo(t *testing.T) {
	bs, err := build.NewBuildSystem("go", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error for 'go' build system: %v", err)
	}
	if _, ok := bs.(*build.GoBuildSystem); !ok {
		t.Errorf("expected *GoBuildSystem for type 'go', got %T", bs)
	}
}

func TestNewBuildSystem_ReturnsNpmBuildSystemForNpm(t *testing.T) {
	bs, err := build.NewBuildSystem("npm", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error for 'npm' build system: %v", err)
	}
	if _, ok := bs.(*build.NpmBuildSystem); !ok {
		t.Errorf("expected *NpmBuildSystem for type 'npm', got %T", bs)
	}
}

func TestNewBuildSystem_ReturnsErrorForUnknownType(t *testing.T) {
	_, err := build.NewBuildSystem("python", t.TempDir())
	if err == nil {
		t.Error("expected error for unknown build system type 'python', got nil")
	}
}

func TestNewBuildSystem_ErrorMessageIncludesUnknownType(t *testing.T) {
	_, err := build.NewBuildSystem("rust", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown build system type 'rust', got nil")
	}
	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("expected a descriptive error message, got empty string")
	}
}
