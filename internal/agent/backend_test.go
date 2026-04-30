package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Compile-time assertion: DefaultBackend must implement Backend.
var _ Backend = DefaultBackend{}
var _ Backend = PiAdapter{}

func TestDefaultBackend_Run(t *testing.T) {
	rawBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	testBin := filepath.ToSlash(rawBin)

	t.Run("delegates to RunAgent and returns positive duration", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "0")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)
		b := DefaultBackend{}
		resp, err := b.Run(context.Background(), RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
		if resp.Duration <= 0 {
			t.Errorf("expected positive duration, got %v", resp.Duration)
		}
		if resp.ExitCode == nil || *resp.ExitCode != 0 {
			t.Fatalf("exit code = %v, want 0", resp.ExitCode)
		}
		if resp.SessionID != "" {
			t.Fatalf("session id = %q, want empty", resp.SessionID)
		}
		if len(resp.RestrictionViolations) != 0 {
			t.Fatalf("restriction violations = %+v, want none", resp.RestrictionViolations)
		}
	})

	t.Run("non-zero exit code propagates as error", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "1")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)
		b := DefaultBackend{}
		resp, err := b.Run(context.Background(), RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err == nil {
			t.Fatal("expected error for non-zero exit code, got nil")
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
		if resp.ExitCode == nil || *resp.ExitCode != 1 {
			t.Fatalf("exit code = %v, want 1", resp.ExitCode)
		}
	})

	t.Run("empty command returns validation error", func(t *testing.T) {
		b := DefaultBackend{}
		resp, err := b.Run(context.Background(), RunRequest{
			Command:     "",
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err == nil {
			t.Fatal("expected validation error for empty command, got nil")
		}
		if resp.Status != RunStatusRejected {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusRejected)
		}
		if resp.ExitCode != nil {
			t.Fatalf("exit code = %v, want nil", resp.ExitCode)
		}
	})

	t.Run("context cancellation propagates", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_SLEEP_MS", "5000")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		b := DefaultBackend{}
		resp, err := b.Run(ctx, RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err == nil {
			t.Fatal("expected error from cancelled context, got nil")
		}
		if resp.Status != RunStatusCancelled {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCancelled)
		}
		if resp.ExitCode != nil {
			t.Fatalf("exit code = %v, want nil", resp.ExitCode)
		}
	})
}

func TestPiAdapter_Run(t *testing.T) {
	t.Run("delegates Doug-native request through private Pi launch spec", func(t *testing.T) {
		var got piLaunchSpec
		adapter := PiAdapter{
			launcher: piLauncherFunc(func(_ context.Context, spec piLaunchSpec) (RunResponse, error) {
				got = spec
				code := 0
				return RunResponse{
					Status:    RunStatusCompleted,
					Duration:  2,
					ExitCode:  &code,
					SessionID: "pi-session-123",
				}, nil
			}),
		}

		req := RunRequest{
			Phase:       RunPhaseRuntime,
			Command:     "unused-by-adapter-boundary",
			ProjectRoot: t.TempDir(),
			Task: TaskContext{
				ID:      "EPIC-23-001",
				Attempt: 2,
				EpicID:  "EPIC-23",
			},
		}

		resp, err := adapter.Run(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Request.Phase != req.Phase {
			t.Fatalf("phase = %q, want %q", got.Request.Phase, req.Phase)
		}
		if got.Request.Command != req.Command {
			t.Fatalf("command = %q, want %q", got.Request.Command, req.Command)
		}
		if got.Request.ProjectRoot != req.ProjectRoot {
			t.Fatalf("project root = %q, want %q", got.Request.ProjectRoot, req.ProjectRoot)
		}
		if got.Request.Task != req.Task {
			t.Fatalf("task = %+v, want %+v", got.Request.Task, req.Task)
		}
		wantDir := filepath.Join(req.ProjectRoot, ".doug", "logs", piSessionRootDir, "EPIC-23", "EPIC-23-001", "attempt-2")
		if got.SessionDir != wantDir {
			t.Fatalf("session dir = %q, want %q", got.SessionDir, wantDir)
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
		if resp.SessionID != "pi-session-123" {
			t.Fatalf("session id = %q, want pi-session-123", resp.SessionID)
		}
	})

	t.Run("missing launcher rejects before Pi launch", func(t *testing.T) {
		adapter := PiAdapter{}
		req := RunRequest{
			Phase:       RunPhasePlanning,
			ProjectRoot: t.TempDir(),
			Task: TaskContext{
				ID: "PLAN",
			},
		}

		resp, err := adapter.Run(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if resp.Status != RunStatusRejected {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusRejected)
		}
		if resp.SessionID != "" {
			t.Fatalf("session id = %q, want empty", resp.SessionID)
		}
	})
}

type piLauncherFunc func(ctx context.Context, spec piLaunchSpec) (RunResponse, error)

func (f piLauncherFunc) Run(ctx context.Context, spec piLaunchSpec) (RunResponse, error) {
	return f(ctx, spec)
}
