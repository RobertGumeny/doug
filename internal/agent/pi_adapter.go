package agent

import (
	"context"
	"fmt"
	"path/filepath"
)

const piSessionRootDir = "pi-sessions"

// PiAdapter is a Doug-owned Backend implementation for future Pi RPC runs.
// Command handlers continue to speak only in terms of RunRequest/RunResponse;
// Pi-specific request preparation remains private to internal/agent.
type PiAdapter struct {
	launcher piLauncher
}

type piLauncher interface {
	Run(ctx context.Context, spec piLaunchSpec) (RunResponse, error)
}

type piLaunchSpec struct {
	Request    RunRequest
	SessionDir string
}

// NewPiAdapter constructs a Pi-backed backend boundary. The launcher is kept
// private so future Pi protocol work can evolve without changing call sites.
func NewPiAdapter() PiAdapter {
	return PiAdapter{launcher: rejectingPiLauncher{}}
}

// Run implements Backend by translating the Doug-native request into a
// Pi-owned launch spec, then delegating the Pi-specific work to a private
// launcher.
func (a PiAdapter) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	launcher := a.launcher
	if launcher == nil {
		launcher = rejectingPiLauncher{}
	}

	return launcher.Run(ctx, piLaunchSpec{
		Request:    req,
		SessionDir: piSessionDir(req),
	})
}

func piSessionDir(req RunRequest) string {
	epicID := req.Task.EpicID
	if epicID == "" {
		epicID = phaseSessionComponent(req.Phase)
	}

	taskID := req.Task.ID
	if taskID == "" {
		taskID = "task"
	}

	attemptDir := "attempt-0"
	if req.Task.Attempt > 0 {
		attemptDir = fmt.Sprintf("attempt-%d", req.Task.Attempt)
	}

	return filepath.Join(req.ProjectRoot, ".doug", "logs", piSessionRootDir, epicID, taskID, attemptDir)
}

type rejectingPiLauncher struct{}

func (rejectingPiLauncher) Run(_ context.Context, spec piLaunchSpec) (RunResponse, error) {
	return RunResponse{
		Status: RunStatusRejected,
	}, fmt.Errorf("pi adapter launcher is not configured")
}

func phaseSessionComponent(p RunPhase) string {
	if p == "" {
		return "runtime"
	}
	return string(p)
}
