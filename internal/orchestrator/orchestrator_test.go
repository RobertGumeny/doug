package orchestrator

import (
	"context"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
)

// backendFunc adapts a plain function to the agent.Backend interface for use
// in tests that need to inject a controllable backend.
type backendFunc func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error)

func (f backendFunc) Run(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
	return f(ctx, req)
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
