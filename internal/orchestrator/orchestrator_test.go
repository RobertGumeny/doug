package orchestrator

import (
	"context"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
)

// backendFunc adapts a plain function to the agent.Backend interface for use
// in tests that need to inject a controllable backend.
type backendFunc func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error)

func (f backendFunc) Run(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
	return f(ctx, req)
}

func TestExecBackend_SelectsDefaultBackendForEmptyMode(t *testing.T) {
	o := &Orchestrator{}
	b := o.execBackend(config.ResolvedExecution{})
	if _, ok := b.(agent.DefaultBackend); !ok {
		t.Fatalf("expected DefaultBackend for empty mode, got %T", b)
	}
}

func TestExecBackend_SelectsDefaultBackendForSubprocessMode(t *testing.T) {
	o := &Orchestrator{}
	b := o.execBackend(config.ResolvedExecution{ExecutionMode: "subprocess"})
	if _, ok := b.(agent.DefaultBackend); !ok {
		t.Fatalf("expected DefaultBackend for subprocess mode, got %T", b)
	}
}

func TestExecBackend_SelectsPiAdapterForRPCMode(t *testing.T) {
	o := &Orchestrator{}
	b := o.execBackend(config.ResolvedExecution{ExecutionMode: "rpc"})
	if _, ok := b.(agent.PiAdapter); !ok {
		t.Fatalf("expected PiAdapter for rpc mode, got %T", b)
	}
}

func TestExecBackend_ReturnsInjectedBackendOverPolicy(t *testing.T) {
	var stub backendFunc = func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		return agent.RunResponse{}, nil
	}
	o := &Orchestrator{backend: stub}
	b := o.execBackend(config.ResolvedExecution{ExecutionMode: "rpc"})
	if _, ok := b.(agent.DefaultBackend); ok {
		t.Fatal("expected injected stub backend, got DefaultBackend")
	}
	if _, ok := b.(agent.PiAdapter); ok {
		t.Fatal("expected injected stub backend, got PiAdapter — policy must not override injection")
	}
	if b == nil {
		t.Fatal("execBackend returned nil")
	}
}
