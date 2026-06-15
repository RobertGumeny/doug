package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/testutil"
)

// backendFunc adapts a plain function to the agent.Backend interface for use
// in tests that need to inject a controllable backend.
type backendFunc func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error)

func (f backendFunc) Run(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
	return f(ctx, req)
}

func TestNew_DefaultModuleRootUsesProjectRoot(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, filepath.Join(root, "go.mod"), "module example.com/root\ngo 1.26\n")

	o, err := New(&config.OrchestratorConfig{BuildSystem: "go"}, NewPaths(root))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if !o.buildSystem.IsInitialized() {
		t.Fatal("expected empty module_root to keep build system rooted at project root")
	}
}

func TestNew_ModuleRootAnchorsBuildSystemInSubdirectory(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "engine")
	testutil.WriteFile(t, filepath.Join(moduleDir, "go.mod"), "module example.com/engine\ngo 1.26\n")

	o, err := New(&config.OrchestratorConfig{BuildSystem: "go", ModuleRoot: "engine"}, NewPaths(root))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if !o.buildSystem.IsInitialized() {
		t.Fatal("expected build system rooted at module_root subdirectory to find go.mod")
	}
}

func TestExecBackend_SelectsPiAdapter(t *testing.T) {
	// NewBackend always returns PiAdapter; execBackend delegates to it when no
	// test backend is injected.
	o := &Orchestrator{}
	b := o.execBackend()
	if _, ok := b.(agent.PiAdapter); !ok {
		t.Fatalf("expected PiAdapter, got %T", b)
	}
}

func TestExecBackend_ReturnsInjectedBackend(t *testing.T) {
	var stub backendFunc = func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		return agent.RunResponse{}, nil
	}
	o := &Orchestrator{backend: stub}
	b := o.execBackend()
	if _, ok := b.(agent.PiAdapter); ok {
		t.Fatal("expected injected stub backend, got PiAdapter")
	}
	if b == nil {
		t.Fatal("execBackend returned nil")
	}
}
