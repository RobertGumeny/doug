package orchestrator

import (
	"path/filepath"
	"testing"
)

func TestNewPaths_IncludesManifestPath(t *testing.T) {
	root := "/tmp/project"

	paths := NewPaths(root)

	want := filepath.Join(root, ".doug", "plan", "manifest.yaml")
	if paths.ManifestPath != want {
		t.Fatalf("ManifestPath: got %q, want %q", paths.ManifestPath, want)
	}
}
