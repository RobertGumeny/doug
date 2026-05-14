package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/config"
)

// Compile-time assertion: DefaultBackend must implement Backend.
var _ Backend = DefaultBackend{}
var _ Backend = PiAdapter{}

func TestNewBackend(t *testing.T) {
	t.Run("returns DefaultBackend for empty execution mode", func(t *testing.T) {
		b := NewBackend(config.ResolvedExecution{})
		if _, ok := b.(DefaultBackend); !ok {
			t.Fatalf("got %T, want DefaultBackend", b)
		}
	})
	t.Run("returns DefaultBackend for subprocess execution mode", func(t *testing.T) {
		b := NewBackend(config.ResolvedExecution{ExecutionMode: "subprocess"})
		if _, ok := b.(DefaultBackend); !ok {
			t.Fatalf("got %T, want DefaultBackend", b)
		}
	})
	t.Run("returns PiAdapter for rpc execution mode", func(t *testing.T) {
		b := NewBackend(config.ResolvedExecution{ExecutionMode: "rpc"})
		if _, ok := b.(PiAdapter); !ok {
			t.Fatalf("got %T, want PiAdapter", b)
		}
	})
}

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
		var cancelled atomic.Bool
		var timeoutCalled atomic.Bool

		b := DefaultBackend{}
		resp, err := b.Run(ctx, RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Lifecycle: LifecycleHooks{
				Timeout: func(time.Duration) {
					timeoutCalled.Store(true)
				},
				Cancellation: func(_ time.Duration, cause error) {
					if errors.Is(cause, context.Canceled) {
						cancelled.Store(true)
					}
				},
			},
			Output: io.Discard,
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
		if !cancelled.Load() {
			t.Fatal("expected cancellation hook to run")
		}
		if timeoutCalled.Load() {
			t.Fatal("timeout hook should not run for manual cancellation")
		}
	})
}

func TestPiAdapter_Run(t *testing.T) {
	t.Run("delegates Doug-native request through private Pi launch spec", func(t *testing.T) {
		var got piLaunchSpec
		var timeoutCalled atomic.Bool
		var cancellationCalled atomic.Bool
		adapter := PiAdapter{
			launcher: piLauncherFunc(func(_ context.Context, spec piLaunchSpec) (RunResponse, error) {
				got = spec
				spec.Lifecycle.Timeout(time.Second)
				spec.Lifecycle.Cancellation(time.Second, context.Canceled)
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
				ID:         "EPIC-23-001",
				Type:       "feature",
				Attempt:    2,
				MaxRetries: 3,
				EpicID:     "EPIC-23",
				EpicName:   "Pi Adapter",
			},
			Brief: CanonicalBrief{
				Path:      filepath.Join("briefs", "ACTIVE_TASK.md"),
				Format:    BriefFormatMarkdown,
				Authority: ArtifactAuthorityDoug,
			},
			ContextLoadOrder: []ContextInput{
				{Kind: ContextInputProjectInstructions, Path: "AGENTS.md", Required: false, Authority: ArtifactAuthorityProject},
				{Kind: ContextInputCanonicalBrief, Path: ".doug/ACTIVE_TASK.md", Required: true, Authority: ArtifactAuthorityDoug},
			},
			Artifacts: ArtifactSurfaces{
				Read: []ArtifactSurface{
					{Path: reqPath("workspace"), Purpose: ArtifactPurposeProjectWorkspace, Authority: ArtifactAuthorityProject, AgentFacing: true},
					{Path: reqPath("brief"), Purpose: ArtifactPurposeCanonicalBrief, Authority: ArtifactAuthorityDoug, AgentFacing: true},
				},
				Write: []ArtifactSurface{
					{Path: reqPath("workspace"), Purpose: ArtifactPurposeProjectWorkspace, Authority: ArtifactAuthorityProject, AgentFacing: true},
					{Path: reqPath("failure"), Purpose: ArtifactPurposeFailureHandoff, Authority: ArtifactAuthorityDoug, AgentFacing: false},
				},
			},
			Routing: RoutingInputs{
				Workflow:  "run",
				SkillName: "implement-feature",
			},
			Policy: PolicyInputs{
				SessionPolicy: "one_task_one_session",
			},
			Restrictions: RestrictionHooks{
				Read: RestrictionHook{
					Mode:  RestrictionModeInherit,
					Paths: []string{"AGENTS.md", ".doug/PRD.md"},
				},
				Write: RestrictionHook{
					Mode:  RestrictionModeAllowList,
					Paths: []string{".", ".doug/ACTIVE_TASK.md"},
				},
			},
			Lifecycle: LifecycleHooks{
				Timeout: func(time.Duration) {
					timeoutCalled.Store(true)
				},
				Cancellation: func(time.Duration, error) {
					cancellationCalled.Store(true)
				},
			},
		}

		resp, err := adapter.Run(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.WorkingDir != req.ProjectRoot {
			t.Fatalf("working dir = %q, want %q", got.WorkingDir, req.ProjectRoot)
		}
		if got.Request.Phase != string(req.Phase) {
			t.Fatalf("phase = %q, want %q", got.Request.Phase, req.Phase)
		}
		if got.Request.Execution.Mode != string(piExecutionModeOneShot) {
			t.Fatalf("execution mode = %q, want %q", got.Request.Execution.Mode, piExecutionModeOneShot)
		}
		if got.Request.Execution.Command != req.Command {
			t.Fatalf("command = %q, want %q", got.Request.Execution.Command, req.Command)
		}
		wantDir := filepath.Join(req.ProjectRoot, ".doug", "logs", piSessionRootDir, "EPIC-23", "EPIC-23-001", "attempt-2")
		if got.Request.Session.Mode != "retain" {
			t.Fatalf("session mode = %q, want retain", got.Request.Session.Mode)
		}
		if got.Request.Session.Directory != wantDir {
			t.Fatalf("session dir = %q, want %q", got.Request.Session.Directory, wantDir)
		}
		if got.Request.Task != (piRPCTask{
			ID:         req.Task.ID,
			Type:       req.Task.Type,
			Attempt:    req.Task.Attempt,
			MaxRetries: req.Task.MaxRetries,
			EpicID:     req.Task.EpicID,
			EpicName:   req.Task.EpicName,
		}) {
			t.Fatalf("task = %+v", got.Request.Task)
		}
		if got.Request.Brief != (piRPCBrief{
			Path:      req.Brief.Path,
			Format:    string(req.Brief.Format),
			Authority: string(req.Brief.Authority),
		}) {
			t.Fatalf("brief = %+v", got.Request.Brief)
		}
		wantContext := []piRPCContextInput{
			{Kind: string(ContextInputProjectInstructions), Path: "AGENTS.md", Required: false, Authority: string(ArtifactAuthorityProject)},
			{Kind: string(ContextInputCanonicalBrief), Path: ".doug/ACTIVE_TASK.md", Required: true, Authority: string(ArtifactAuthorityDoug)},
		}
		if !reflect.DeepEqual(got.Request.Context, wantContext) {
			t.Fatalf("context = %+v, want %+v", got.Request.Context, wantContext)
		}
		wantArtifacts := piRPCArtifacts{
			Read: []piRPCArtifactSurface{
				{Path: reqPath("workspace"), Purpose: string(ArtifactPurposeProjectWorkspace), Authority: string(ArtifactAuthorityProject), AgentFacing: true},
				{Path: reqPath("brief"), Purpose: string(ArtifactPurposeCanonicalBrief), Authority: string(ArtifactAuthorityDoug), AgentFacing: true},
			},
			Write: []piRPCArtifactSurface{
				{Path: reqPath("workspace"), Purpose: string(ArtifactPurposeProjectWorkspace), Authority: string(ArtifactAuthorityProject), AgentFacing: true},
				{Path: reqPath("failure"), Purpose: string(ArtifactPurposeFailureHandoff), Authority: string(ArtifactAuthorityDoug), AgentFacing: false},
			},
		}
		if !reflect.DeepEqual(got.Request.Artifacts, wantArtifacts) {
			t.Fatalf("artifacts = %+v, want %+v", got.Request.Artifacts, wantArtifacts)
		}
		if got.Request.Routing != (piRPCRouting{Workflow: "run", SkillName: "implement-feature"}) {
			t.Fatalf("routing = %+v", got.Request.Routing)
		}
		if got.Request.Policy != (piRPCPolicy{SessionPolicy: "one_task_one_session"}) {
			t.Fatalf("policy = %+v", got.Request.Policy)
		}
		wantRestrictions := piRPCRestrictions{
			Read: piRPCRestrictionHook{
				Mode:  string(RestrictionModeInherit),
				Paths: []string{"AGENTS.md", ".doug/PRD.md"},
			},
			Write: piRPCRestrictionHook{
				Mode:  string(RestrictionModeAllowList),
				Paths: []string{".", ".doug/ACTIVE_TASK.md"},
			},
		}
		if !reflect.DeepEqual(got.Request.Restrictions, wantRestrictions) {
			t.Fatalf("restrictions = %+v, want %+v", got.Request.Restrictions, wantRestrictions)
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
		if resp.SessionID != "pi-session-123" {
			t.Fatalf("session id = %q, want pi-session-123", resp.SessionID)
		}
		if !timeoutCalled.Load() {
			t.Fatal("expected timeout hook to be forwarded to launcher")
		}
		if !cancellationCalled.Load() {
			t.Fatal("expected cancellation hook to be forwarded to launcher")
		}
	})

	t.Run("missing launcher rejects before Pi launch", func(t *testing.T) {
		adapter := PiAdapter{launcher: nil}
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

func TestPiCLILauncher_Run(t *testing.T) {
	rawBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	newTestLauncher := func(mode string) piCLILauncher {
		return piCLILauncher{
			command:  rawBin,
			baseArgs: []string{"-test.run=^$"},
			newCommand: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				cmd := exec.CommandContext(ctx, name, args...)
				cmd.Env = append(os.Environ(), "TEST_PI_RPC_MODE="+mode)
				return cmd
			},
		}
	}

	t.Run("starts pi rpc with Doug-managed working and session directories", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")
		var stderr bytes.Buffer

		resp, err := newTestLauncher("startup_only").Run(context.Background(), piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Session: piRPCSession{Directory: sessionDir},
			},
			Output: &stderr,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
		if resp.SessionID != "pi-session-123" {
			t.Fatalf("session id = %q, want pi-session-123", resp.SessionID)
		}
		if !reflect.DeepEqual(resp.AvailableSessionIDs, []string{"pi-session-123"}) {
			t.Fatalf("available session ids = %v, want [pi-session-123]", resp.AvailableSessionIDs)
		}
		if resp.ExitCode == nil || *resp.ExitCode != 0 {
			t.Fatalf("exit code = %v, want 0", resp.ExitCode)
		}
		if _, statErr := os.Stat(sessionDir); statErr != nil {
			t.Fatalf("expected session dir to exist: %v", statErr)
		}
		if !bytes.Contains(stderr.Bytes(), []byte(`pi rpc stdout: {"command":"get_state"`)) {
			t.Fatalf("expected mirrored pi rpc stdout in output, got %q", stderr.String())
		}
	})

	t.Run("supervises prompt completion through rpc events", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")
		var output bytes.Buffer

		resp, err := newTestLauncher("prompt_success").Run(context.Background(), piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Execution: piRPCExecution{Command: "solve the task"},
				Session:   piRPCSession{Directory: sessionDir},
			},
			Output: &output,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
		if resp.SessionID != "pi-session-123" {
			t.Fatalf("session id = %q, want pi-session-123", resp.SessionID)
		}
		wantIDs := []string{"pi-session-123", "pi-session-456"}
		if !reflect.DeepEqual(resp.AvailableSessionIDs, wantIDs) {
			t.Fatalf("available session ids = %v, want %v", resp.AvailableSessionIDs, wantIDs)
		}
		if !bytes.Contains(output.Bytes(), []byte(`pi rpc stdout: {"data":{"sessionId":"pi-session-456"}`)) {
			t.Fatalf("expected mirrored agent_end event in output, got %q", output.String())
		}
	})

	t.Run("startup response failures are surfaced as rejected runs", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")

		resp, err := newTestLauncher("startup_error").Run(context.Background(), piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Session: piRPCSession{Directory: sessionDir},
			},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if resp.Status != RunStatusRejected {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusRejected)
		}
	})

	t.Run("prompt failures are surfaced as rejected runs", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")

		resp, err := newTestLauncher("prompt_error").Run(context.Background(), piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Execution: piRPCExecution{Command: "solve the task"},
				Session:   piRPCSession{Directory: sessionDir},
			},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if resp.Status != RunStatusRejected {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusRejected)
		}
	})

	t.Run("manual cancellation reports cancelled and fires only cancellation hook", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)

		var timeoutCalled atomic.Bool
		var cancellationCalled atomic.Bool
		resp, err := newTestLauncher("prompt_hang").Run(ctx, piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Execution: piRPCExecution{Command: "solve the task"},
				Session:   piRPCSession{Directory: sessionDir},
			},
			Lifecycle: LifecycleHooks{
				Timeout: func(time.Duration) {
					timeoutCalled.Store(true)
				},
				Cancellation: func(_ time.Duration, cause error) {
					if errors.Is(cause, context.Canceled) {
						cancellationCalled.Store(true)
					}
				},
			},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context canceled", err)
		}
		if resp.Status != RunStatusCancelled {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCancelled)
		}
		if !cancellationCalled.Load() {
			t.Fatal("expected cancellation hook to run")
		}
		if timeoutCalled.Load() {
			t.Fatal("timeout hook should not run for manual cancellation")
		}
	})

	t.Run("transmits write restrictions to Pi prompt payload", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")

		resp, err := newTestLauncher("prompt_with_restrictions").Run(context.Background(), piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Execution: piRPCExecution{Command: "solve the task"},
				Session:   piRPCSession{Directory: sessionDir},
				Restrictions: piRPCRestrictions{
					Read: piRPCRestrictionHook{
						Mode:  string(RestrictionModeInherit),
						Paths: []string{"AGENTS.md"},
					},
					Write: piRPCRestrictionHook{
						Mode:  string(RestrictionModeAllowList),
						Paths: []string{".", ".doug/ACTIVE_TASK.md"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != RunStatusCompleted {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCompleted)
		}
	})

	t.Run("deadline expiry reports cancelled and fires timeout plus cancellation hooks", func(t *testing.T) {
		projectRoot := t.TempDir()
		sessionDir := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-23", "EPIC-23-003", "attempt-1")
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		var timeoutCalled atomic.Bool
		var cancellationCalled atomic.Bool
		resp, err := newTestLauncher("prompt_hang").Run(ctx, piLaunchSpec{
			WorkingDir: projectRoot,
			Request: piRPCRequest{
				Execution: piRPCExecution{Command: "solve the task"},
				Session:   piRPCSession{Directory: sessionDir},
			},
			Lifecycle: LifecycleHooks{
				Timeout: func(time.Duration) {
					timeoutCalled.Store(true)
				},
				Cancellation: func(_ time.Duration, cause error) {
					if errors.Is(cause, context.DeadlineExceeded) {
						cancellationCalled.Store(true)
					}
				},
			},
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context deadline exceeded", err)
		}
		if resp.Status != RunStatusCancelled {
			t.Fatalf("status = %q, want %q", resp.Status, RunStatusCancelled)
		}
		if !timeoutCalled.Load() {
			t.Fatal("expected timeout hook to run")
		}
		if !cancellationCalled.Load() {
			t.Fatal("expected cancellation hook to run for deadline expiry")
		}
	})
}

func TestBuildPiPromptPayload(t *testing.T) {
	t.Run("omits restrictions when none are configured", func(t *testing.T) {
		payload := buildPiPromptPayload("req-1", "do the task", piRPCRestrictions{})
		if _, ok := payload["restrictions"]; ok {
			t.Fatal("expected no restrictions field when restrictions are empty")
		}
		if payload["id"] != "req-1" || payload["type"] != "prompt" || payload["message"] != "do the task" {
			t.Fatalf("unexpected base fields: %v", payload)
		}
	})

	t.Run("includes restrictions when write mode is configured", func(t *testing.T) {
		r := piRPCRestrictions{
			Write: piRPCRestrictionHook{Mode: string(RestrictionModeAllowList), Paths: []string{"/workspace"}},
		}
		payload := buildPiPromptPayload("req-2", "do the task", r)
		got, ok := payload["restrictions"]
		if !ok {
			t.Fatal("expected restrictions field to be present")
		}
		if !reflect.DeepEqual(got, r) {
			t.Fatalf("restrictions = %+v, want %+v", got, r)
		}
	})

	t.Run("includes restrictions when read paths are configured", func(t *testing.T) {
		r := piRPCRestrictions{
			Read: piRPCRestrictionHook{Mode: string(RestrictionModeInherit), Paths: []string{"AGENTS.md"}},
		}
		payload := buildPiPromptPayload("req-3", "do the task", r)
		if _, ok := payload["restrictions"]; !ok {
			t.Fatal("expected restrictions field to be present when read paths are set")
		}
	})
}

type piLauncherFunc func(ctx context.Context, spec piLaunchSpec) (RunResponse, error)

func (f piLauncherFunc) Run(ctx context.Context, spec piLaunchSpec) (RunResponse, error) {
	return f(ctx, spec)
}

func reqPath(name string) string {
	return filepath.Join("/tmp", name)
}
