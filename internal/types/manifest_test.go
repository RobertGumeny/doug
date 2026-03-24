package types_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/manifest.yaml"

	testutil.WriteFile(t, path, `
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

	manifest, err := types.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if manifest.SchemaVersion != types.ManifestSchemaVersionV1 {
		t.Fatalf("SchemaVersion: got %d, want %d", manifest.SchemaVersion, types.ManifestSchemaVersionV1)
	}
	if manifest.Project.Name != "Acme App" {
		t.Errorf("Project.Name: got %q, want %q", manifest.Project.Name, "Acme App")
	}
	if manifest.Project.Mode != "greenfield" {
		t.Errorf("Project.Mode: got %q, want %q", manifest.Project.Mode, "greenfield")
	}
	if manifest.Scaffold.PackageManager != "pnpm" {
		t.Errorf("Scaffold.PackageManager: got %q, want %q", manifest.Scaffold.PackageManager, "pnpm")
	}
	if got, want := len(manifest.Dependencies.Runtime), 2; got != want {
		t.Errorf("Dependencies.Runtime len: got %d, want %d", got, want)
	}
	if got, want := len(manifest.Dependencies.Development), 2; got != want {
		t.Errorf("Dependencies.Development len: got %d, want %d", got, want)
	}
	if got, want := len(manifest.Constraints), 1; got != want {
		t.Errorf("Constraints len: got %d, want %d", got, want)
	}
}

func TestLoadManifest_MissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/manifest.yaml"

	testutil.WriteFile(t, path, `
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

	_, err := types.LoadManifest(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `missing required field "project.mode"`) {
		t.Fatalf("expected missing field error for project.mode, got: %v", err)
	}
}

func TestLoadManifest_UnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/manifest.yaml"

	testutil.WriteFile(t, path, `
schema_version: 2
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
  development:
    - "typescript"
constraints:
  - "Deploy on Vercel"
`)

	_, err := types.LoadManifest(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported schema_version 2") {
		t.Fatalf("expected unsupported schema_version error, got: %v", err)
	}
}
