package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
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

	activeTaskPath := filepath.Join(dir, ".doug", "ACTIVE_TASK.md")
	data, err := os.ReadFile(activeTaskPath)
	if err != nil {
		t.Fatalf("read ACTIVE_TASK.md: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"**Task ID**: SCAFFOLD",
		"**Task Type**: scaffold",
		"## Manifest Context",
		"schema_version: 1",
		"package_manager: pnpm",
		"constraints:",
		"Deploy on Vercel",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in ACTIVE_TASK.md, got:\n%s", want, content)
		}
	}

	projectStateData, err := os.ReadFile(filepath.Join(dir, ".doug", "project-state.yaml"))
	if err != nil {
		t.Fatalf("read project-state.yaml: %v", err)
	}
	if string(projectStateData) != "{}\n" {
		t.Fatalf("expected project-state.yaml to remain unchanged, got:\n%s", string(projectStateData))
	}
}

func TestBuildScaffoldTask(t *testing.T) {
	task, err := buildScaffoldTask(&types.Manifest{
		SchemaVersion: 1,
		Project: types.ManifestProject{
			Name: "Acme App",
			Mode: "greenfield",
		},
		Scaffold: types.ManifestScaffold{
			Language:       "typescript",
			Runtime:        "node",
			Framework:      "nextjs",
			PackageManager: "pnpm",
			BuildSystem:    "npm-scripts",
		},
	})
	if err != nil {
		t.Fatalf("buildScaffoldTask: %v", err)
	}

	if task.ID != "SCAFFOLD" {
		t.Fatalf("task.ID = %q, want %q", task.ID, "SCAFFOLD")
	}
	if task.Type != types.TaskTypeScaffold {
		t.Fatalf("task.Type = %q, want %q", task.Type, types.TaskTypeScaffold)
	}
	if !task.Type.IsSynthetic() {
		t.Fatal("expected scaffold task type to be synthetic")
	}
	if len(task.AcceptanceCriteria) == 0 {
		t.Fatal("expected scaffold task acceptance criteria")
	}
}
