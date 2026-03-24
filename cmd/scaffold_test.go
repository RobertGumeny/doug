package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
)

func TestRootHelp_IncludesScaffoldCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})
	defer rootCmd.SetArgs(nil)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute(): %v", err)
	}

	if !strings.Contains(buf.String(), "scaffold") {
		t.Fatalf("expected help output to include scaffold command; got:\n%s", buf.String())
	}
}

func TestScaffoldProject_MissingProjectState(t *testing.T) {
	dir := t.TempDir()

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ".doug/project-state.yaml") {
		t.Fatalf("expected error to mention project-state.yaml, got: %v", err)
	}
	if !strings.Contains(err.Error(), "run doug init first") {
		t.Fatalf("expected actionable init guidance, got: %v", err)
	}
}

func TestScaffoldProject_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ".doug/plan/manifest.yaml") {
		t.Fatalf("expected error to mention manifest path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "before running doug scaffold") {
		t.Fatalf("expected actionable manifest guidance, got: %v", err)
	}
}

func TestScaffoldProject_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "manifest.yaml"), `
schema_version: 1
project:
  name: "Acme App"
scaffold:
  language: "typescript"
  runtime: "node"
  framework: "nextjs"
  package_manager: "pnpm"
  build_system: "npm-scripts"
dependencies:
  runtime:
    - "next"
  development:
    - "typescript"
constraints:
  - "Deploy on Vercel"
`)

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `missing required field "project.mode"`) {
		t.Fatalf("expected manifest validation error, got: %v", err)
	}
}

func TestScaffoldProject_ValidManifest(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "manifest.yaml"), `
schema_version: 1
project:
  name: "Acme App"
  mode: "greenfield"
scaffold:
  language: "typescript"
  runtime: "node"
  framework: "nextjs"
  package_manager: "pnpm"
  build_system: "npm-scripts"
dependencies:
  runtime:
    - "next"
    - "react"
  development:
    - "typescript"
    - "eslint"
constraints:
  - "Deploy on Vercel"
`)

	if err := scaffoldProject(dir); err != nil {
		t.Fatalf("scaffoldProject(valid manifest): %v", err)
	}
}
