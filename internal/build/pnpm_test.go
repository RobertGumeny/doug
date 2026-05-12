package build_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/build"
)

// --- PnpmBuildSystem.IsInitialized ---

func TestPnpmBuildSystemIsInitialized_FalseWhenNodeModulesMissing(t *testing.T) {
	dir := t.TempDir()
	p := build.NewPnpmBuildSystem(dir)
	if p.IsInitialized() {
		t.Error("expected IsInitialized to return false when node_modules does not exist")
	}
}

func TestPnpmBuildSystemIsInitialized_TrueWhenNodeModulesExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatalf("failed to create node_modules directory: %v", err)
	}
	p := build.NewPnpmBuildSystem(dir)
	if !p.IsInitialized() {
		t.Error("expected IsInitialized to return true when node_modules directory exists")
	}
}

func TestPnpmBuildSystemIsInitialized_FalseWhenNodeModulesIsFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "node_modules", "not a directory")
	p := build.NewPnpmBuildSystem(dir)
	if p.IsInitialized() {
		t.Error("expected IsInitialized to return false when node_modules is a file, not a directory")
	}
}

// --- PnpmBuildSystem.Test (package.json pre-flight checks) ---

func TestPnpmBuildSystemTest_ReturnsNilWhenNoPackageJson(t *testing.T) {
	dir := t.TempDir()
	p := build.NewPnpmBuildSystem(dir)
	if err := p.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when package.json is missing, got: %v", err)
	}
}

func TestPnpmBuildSystemTest_ReturnsNilWhenTestScriptNotPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp","scripts":{"build":"webpack"}}`)
	p := build.NewPnpmBuildSystem(dir)
	if err := p.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when test script is not in package.json, got: %v", err)
	}
}

func TestPnpmBuildSystemTest_ReturnsNilWhenNoScriptsSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp","version":"1.0.0"}`)
	p := build.NewPnpmBuildSystem(dir)
	if err := p.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when package.json has no scripts section, got: %v", err)
	}
}

func TestPnpmBuildSystemTest_ReturnsNilWhenPackageJsonMalformed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `not valid json`)
	p := build.NewPnpmBuildSystem(dir)
	if err := p.Test(); err != nil {
		t.Errorf("expected Test to return nil (skip) when package.json is malformed, got: %v", err)
	}
}

// --- NewBuildSystem factory ---

func TestNewBuildSystem_ReturnsPnpmBuildSystemForPnpm(t *testing.T) {
	bs, err := build.NewBuildSystem("pnpm", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error for 'pnpm' build system: %v", err)
	}
	if _, ok := bs.(*build.PnpmBuildSystem); !ok {
		t.Errorf("expected *PnpmBuildSystem for type 'pnpm', got %T", bs)
	}
}
