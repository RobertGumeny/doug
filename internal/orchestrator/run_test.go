package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/testutil"
)

// runLoopBuildSystem is a stub that always reports initialized and succeeds.
type runLoopBuildSystem struct{}

func (s *runLoopBuildSystem) Install() error      { return nil }
func (s *runLoopBuildSystem) Build() error        { return nil }
func (s *runLoopBuildSystem) Test() error         { return nil }
func (s *runLoopBuildSystem) Lint() error         { return nil }
func (s *runLoopBuildSystem) IsInitialized() bool { return true }

// setupRunRepo creates a minimal git repository for run loop tests.
// The repo is on feature/<epicID> and has .doug/ in .gitignore so that
// git.Commit returns ErrNothingToCommit (non-fatal) during epic finalization.
func setupRunRepo(t *testing.T, epicID string) string {
	t.Helper()
	dir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test Agent")
	testutil.WriteFile(t, filepath.Join(dir, ".gitignore"), ".doug/\n")
	testutil.WriteFile(t, filepath.Join(dir, "README.md"), "# test\n")
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	runGit("checkout", "-b", "feature/"+epicID)

	return dir
}

// writeRunState creates the .doug/ files required by Orchestrator.Run:
// PRD.md (required by plan.FinalizeEpicCompletion), tasks.yaml, and
// project-state.yaml. The active task is a single feature task IN_PROGRESS.
func writeRunState(t *testing.T, dir, epicID, taskID string) {
	t.Helper()
	paths := NewPaths(dir)
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "PRD.md"), "# PRD\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n"+
		"  id: "+epicID+"\n"+
		"  name: Test Run Epic\n"+
		"  tasks:\n"+
		"    - id: "+taskID+"\n"+
		"      type: feature\n"+
		"      status: IN_PROGRESS\n"+
		"      description: Test feature task\n"+
		"      acceptance_criteria:\n"+
		"        - Deliver the feature\n")
	testutil.WriteFile(t, paths.StatePath, "current_epic:\n"+
		"  id: "+epicID+"\n"+
		"  name: Test Run Epic\n"+
		"  branch_name: feature/"+epicID+"\n"+
		"  started_at: \"2026-01-01T00:00:00Z\"\n"+
		"active_task:\n"+
		"  type: feature\n"+
		"  id: "+taskID+"\n"+
		"  attempts: 0\n")
}

func writeCompletedRunState(t *testing.T, dir, epicID, taskID string) {
	t.Helper()
	paths := NewPaths(dir)
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "PRD.md"), "# PRD\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n"+
		"  id: "+epicID+"\n"+
		"  name: Test Run Epic\n"+
		"  tasks:\n"+
		"    - id: "+taskID+"\n"+
		"      type: feature\n"+
		"      status: DONE\n"+
		"      description: Test feature task\n"+
		"      acceptance_criteria:\n"+
		"        - Deliver the feature\n")
	testutil.WriteFile(t, paths.StatePath, "current_epic:\n"+
		"  id: "+epicID+"\n"+
		"  name: Test Run Epic\n"+
		"  branch_name: feature/"+epicID+"\n"+
		"  started_at: \"2026-01-01T00:00:00Z\"\n"+
		"  completed_at: \"2026-01-02T00:00:00Z\"\n"+
		"active_task:\n"+
		"  type: feature\n"+
		"  id: "+taskID+"\n"+
		"  attempts: 1\n")
}

// writeBugfixRunState creates .doug/ files where active_task is a Doug-scheduled
// synthetic bugfix (BUG-<taskID>) and the interrupted feature task waits as a
// backlog task. withPayload controls whether the carried bug payload fields are
// present on active_task, exercising the run-loop dispatch guard.
func writeBugfixRunState(t *testing.T, dir, epicID, interruptedTaskID string, withPayload bool) (bugTaskID string) {
	t.Helper()
	paths := NewPaths(dir)
	bugTaskID = "BUG-" + interruptedTaskID
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "PRD.md"), "# PRD\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n"+
		"  id: "+epicID+"\n"+
		"  name: Test Run Epic\n"+
		"  tasks:\n"+
		"    - id: "+interruptedTaskID+"\n"+
		"      type: feature\n"+
		"      status: IN_PROGRESS\n"+
		"      description: Interrupted feature task\n"+
		"      acceptance_criteria:\n"+
		"        - Deliver the feature\n")
	stateYAML := "current_epic:\n" +
		"  id: " + epicID + "\n" +
		"  name: Test Run Epic\n" +
		"  branch_name: feature/" + epicID + "\n" +
		"  started_at: \"2026-01-01T00:00:00Z\"\n" +
		"active_task:\n" +
		"  type: bugfix\n" +
		"  id: " + bugTaskID + "\n" +
		"  attempts: 0\n"
	if withPayload {
		stateYAML += "  bug_id: " + bugTaskID + "\n" +
			"  bug_severity: high\n" +
			"  bug_source_task: " + interruptedTaskID + "\n" +
			"  bug_body: \"Null pointer in handler\"\n" +
			"  bug_archive_path: .doug/intake/bugs/" + epicID + "/bug-" + interruptedTaskID + ".md\n"
	}
	stateYAML += "next_task:\n" +
		"  type: feature\n" +
		"  id: " + interruptedTaskID + "\n"
	testutil.WriteFile(t, paths.StatePath, stateYAML)
	return bugTaskID
}

func TestRun_FinalizationPathsRunReviewThenPostEpicKBThroughSharedHelper(t *testing.T) {
	tests := []struct {
		name       string
		epID       string
		prepare    func(t *testing.T, dir, epicID, taskID string)
		outcome    string
		wantPhases []agent.RunPhase
	}{
		{
			name:       "resume finalization",
			epID:       "EPIC-FIN-RESUME",
			prepare:    writeCompletedRunState,
			wantPhases: []agent.RunPhase{agent.RunPhasePostEpicReview, agent.RunPhasePostEpicKB},
		},
		{
			name:       "terminal success",
			epID:       "EPIC-FIN-SUCCESS",
			prepare:    writeRunState,
			outcome:    "SUCCESS",
			wantPhases: []agent.RunPhase{agent.RunPhaseRuntime, agent.RunPhasePostEpicReview, agent.RunPhasePostEpicKB},
		},
		{
			name:       "explicit epic complete",
			epID:       "EPIC-FIN-EXPLICIT",
			prepare:    writeRunState,
			outcome:    "EPIC_COMPLETE",
			wantPhases: []agent.RunPhase{agent.RunPhaseRuntime, agent.RunPhasePostEpicReview, agent.RunPhasePostEpicKB},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID := tt.epID + "-001"
			dir := setupRunRepo(t, tt.epID)
			paths := NewPaths(dir)
			tt.prepare(t, dir, tt.epID, taskID)

			var phases []agent.RunPhase
			stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
				phases = append(phases, req.Phase)
				data, err := os.ReadFile(req.Brief.Path)
				if err != nil {
					return agent.RunResponse{}, err
				}
				outcome := tt.outcome
				if req.Phase == agent.RunPhasePostEpicReview || req.Phase == agent.RunPhasePostEpicKB {
					outcome = "SUCCESS"
				} else if outcome == "SUCCESS" {
					if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n\nterminal success change\n"), 0o644); err != nil {
						return agent.RunResponse{}, err
					}
				}
				updated := strings.Replace(string(data), `outcome: ""`, `outcome: "`+outcome+`"`, 1)
				if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
					return agent.RunResponse{}, err
				}
				code := 0
				return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
			})

			o := &Orchestrator{
				cfg: &config.OrchestratorConfig{
					BuildSystem:           "static",
					MaxRetries:            3,
					MaxIterations:         5,
					AgentHeartbeatSeconds: 0,
					KBEnabled:             true,
					ReviewEnabled:         true,
				},
				paths:       paths,
				logger:      log.Discard(),
				buildSystem: &runLoopBuildSystem{},
				backend:     stub,
			}

			if err := o.Run(context.Background()); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if fmt.Sprint(phases) != fmt.Sprint(tt.wantPhases) {
				t.Fatalf("phases = %v, want %v", phases, tt.wantPhases)
			}
		})
	}
}

func TestRun_PostEpicReviewFailureIsWarningOnlyAndPreservesFinalizedState(t *testing.T) {
	const epicID = "EPIC-FIN-REVIEW-WARN"
	const taskID = "EPIC-FIN-REVIEW-WARN-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	var phases []agent.RunPhase
	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		phases = append(phases, req.Phase)
		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, err
		}
		if req.Phase == agent.RunPhasePostEpicReview {
			return agent.RunResponse{Status: agent.RunStatusCompleted}, fmt.Errorf("review provider unavailable")
		}
		outcome := "SUCCESS"
		if req.Phase == agent.RunPhaseRuntime {
			outcome = "EPIC_COMPLETE"
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "`+outcome+`"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, err
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})
	logger := &recordingLogger{}
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "static",
			MaxRetries:            3,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             true,
			ReviewEnabled:         true,
		},
		paths:       paths,
		logger:      logger,
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	wantPhases := []agent.RunPhase{agent.RunPhaseRuntime, agent.RunPhasePostEpicReview, agent.RunPhasePostEpicKB}
	if fmt.Sprint(phases) != fmt.Sprint(wantPhases) {
		t.Fatalf("phases = %v, want %v", phases, wantPhases)
	}
	if !loggerContains(logger.warnings, "advisory post-epic review did not complete") || !loggerContains(logger.warnings, "inspect the completed epic more carefully") || !loggerContains(logger.warnings, "doug review "+epicID) {
		t.Fatalf("expected actionable advisory review warning, got %+v", logger.warnings)
	}
	projectState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if projectState.CurrentEpic.CompletedAt == nil || *projectState.CurrentEpic.CompletedAt == "" {
		t.Fatalf("completed_at was not preserved: %+v", projectState.CurrentEpic)
	}
	if projectState.ActiveTask.ID != "" || projectState.NextTask.ID != "" {
		t.Fatalf("runtime pointers were not finalized: active=%+v next=%+v", projectState.ActiveTask, projectState.NextTask)
	}
}

// TestRun_SyntheticBugfixWithPayloadDispatches verifies that a Doug-scheduled
// synthetic bugfix (BUG-<taskID> ID + carried bug payload) passes the dispatch
// guard and reaches the agent backend.
func TestRun_SyntheticBugfixWithPayloadDispatches(t *testing.T) {
	const epicID = "EPIC-BUGFIX"
	const interruptedTaskID = "EPIC-BUGFIX-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	bugTaskID := writeBugfixRunState(t, dir, epicID, interruptedTaskID, true)

	var backendCalled bool
	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		backendCalled = true
		if req.Task.ID != bugTaskID || req.Task.Type != "bugfix" {
			return agent.RunResponse{}, fmt.Errorf("unexpected task context: %+v", req.Task)
		}
		if req.Routing.SkillName != "implement-bugfix" {
			return agent.RunResponse{}, fmt.Errorf("routing skill = %q, want implement-bugfix", req.Routing.SkillName)
		}
		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, err
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, err
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "go",
			MaxRetries:            3,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !backendCalled {
		t.Fatal("expected synthetic bugfix with payload to dispatch to the backend")
	}
}

// TestRun_SyntheticBugfixWithoutPayloadRejectedAtGuard verifies that a bugfix
// active task carrying the synthetic BUG-<taskID> ID but no bug payload is
// rejected by the run-loop dispatch guard before the agent backend is reached.
func TestRun_SyntheticBugfixWithoutPayloadRejectedAtGuard(t *testing.T) {
	const epicID = "EPIC-BUGFIX-NP"
	const interruptedTaskID = "EPIC-BUGFIX-NP-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeBugfixRunState(t, dir, epicID, interruptedTaskID, false)

	var backendCalled bool
	stub := backendFunc(func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		backendCalled = true
		return agent.RunResponse{Status: agent.RunStatusCompleted}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "go",
			MaxRetries:            3,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run to reject bugfix with no payload at the dispatch guard")
	}
	if !strings.Contains(err.Error(), "bug payload") {
		t.Fatalf("unexpected guard error: %v", err)
	}
	if backendCalled {
		t.Fatal("backend must not be dispatched when the bug payload is missing")
	}
}

func prependFakePiRPC(t *testing.T) (argvPath, promptPath string) {
	t.Helper()
	shimDir := t.TempDir()
	argvPath = filepath.Join(shimDir, "pi.argv")
	promptPath = filepath.Join(shimDir, "pi.prompt.json")
	piPath := filepath.Join(shimDir, "pi")
	testutil.WriteFile(t, piPath, `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_PI_ARGV"
while IFS= read -r line; do
	case "$line" in
		*'"type":"get_state"'*)
			printf '{"type":"response","id":"doug-startup","success":true,"data":{"sessionId":"fake-pi-session"}}\n'
			;;
		*'"type":"prompt"'*)
			printf '%s\n' "$line" > "$FAKE_PI_PROMPT"
			perl -0pi -e 's/outcome: ""/outcome: "EPIC_COMPLETE"/' .doug/ACTIVE_TASK.md
			printf '{"type":"response","id":"doug-prompt","success":true}\n'
			printf '{"type":"agent_end","id":"doug-prompt","data":{"outcome":"FAILURE"}}\n'
			;;
		*'"type":"get_session_stats"'*)
			printf '{"type":"response","id":"doug-session-stats","success":true,"data":{"sessionId":"fake-pi-session","tokens":{"input":10,"output":5,"cacheRead":2,"cacheWrite":1},"cost":0.0042}}\n'
			;;
	esac
done
exit 0
`)
	if err := os.Chmod(piPath, 0o755); err != nil {
		t.Fatalf("chmod fake pi: %v", err)
	}
	t.Setenv("FAKE_PI_ARGV", argvPath)
	t.Setenv("FAKE_PI_PROMPT", promptPath)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argvPath, promptPath
}

func hasContextInput(inputs []agent.ContextInput, want agent.ContextInput) bool {
	for _, input := range inputs {
		if input.Kind == want.Kind && input.Path == want.Path && input.Required == want.Required && input.Authority == want.Authority {
			return true
		}
	}
	return false
}

// TestRun_RoutesAgentExecutionThroughBackendSeam verifies that Orchestrator.Run
// invokes the agent through the Backend interface with the correct RunRequest
// fields: phase, task context, brief, context load order, routing (workflow and
// skill name), restrictions, and initial Pi prompt. The stub backend writes
// EPIC_COMPLETE so the run completes in one iteration without real agent execution.
func TestRun_RoutesAgentExecutionThroughBackendSeam(t *testing.T) {
	const epicID = "EPIC-RUN"
	const taskID = "EPIC-RUN-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	activeTaskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")

	var backendCalled bool
	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		backendCalled = true

		if req.Phase != agent.RunPhaseRuntime {
			return agent.RunResponse{}, fmt.Errorf("phase = %q, want %q", req.Phase, agent.RunPhaseRuntime)
		}
		if req.Task.ID != taskID || req.Task.Type != "feature" {
			return agent.RunResponse{}, fmt.Errorf("unexpected task context: %+v", req.Task)
		}
		if req.Task.Attempt != 1 || req.Task.MaxRetries != 3 {
			return agent.RunResponse{}, fmt.Errorf("unexpected task attempt context: %+v", req.Task)
		}
		if req.Task.EpicID != epicID {
			return agent.RunResponse{}, fmt.Errorf("task.EpicID = %q, want %q", req.Task.EpicID, epicID)
		}
		if req.Brief.Path != activeTaskPath || req.Brief.Format != agent.BriefFormatMarkdown || req.Brief.Authority != agent.ArtifactAuthorityDoug {
			return agent.RunResponse{}, fmt.Errorf("unexpected brief: %+v", req.Brief)
		}
		if !hasContextInput(req.ContextLoadOrder, agent.ContextInput{
			Kind:      agent.ContextInputCanonicalBrief,
			Path:      activeTaskPath,
			Required:  true,
			Authority: agent.ArtifactAuthorityDoug,
		}) {
			return agent.RunResponse{}, fmt.Errorf("missing canonical brief context entry in %+v", req.ContextLoadOrder)
		}
		if req.Routing.Workflow != "run" || req.Routing.SkillName != "implement-feature" {
			return agent.RunResponse{}, fmt.Errorf("unexpected routing: %+v", req.Routing)
		}
		if req.Restrictions.Read.Mode != agent.RestrictionModeInherit {
			return agent.RunResponse{}, fmt.Errorf("read restriction mode = %q, want Inherit", req.Restrictions.Read.Mode)
		}
		if req.Restrictions.Write.Mode != agent.RestrictionModeInherit {
			return agent.RunResponse{}, fmt.Errorf("write restriction mode = %q, want Inherit (no write scopes configured)", req.Restrictions.Write.Mode)
		}
		if !strings.Contains(req.InitialPrompt, "implement-feature") {
			return agent.RunResponse{}, fmt.Errorf("expected skill name in prompt, got %q", req.InitialPrompt)
		}
		if !strings.Contains(req.InitialPrompt, taskID) {
			return agent.RunResponse{}, fmt.Errorf("expected task ID in prompt, got %q", req.InitialPrompt)
		}
		markerPath := filepath.Join(paths.LogsDir, "pi-sessions", epicID, taskID, "attempt-1", "attempt-start.json")
		markerData, err := os.ReadFile(markerPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: attempt-start marker must exist before backend invocation: %w", err)
		}
		marker := string(markerData)
		for _, want := range []string{`"task_id": "` + taskID + `"`, `"attempt": 1`, `"started_at": `} {
			if !strings.Contains(marker, want) {
				return agent.RunResponse{}, fmt.Errorf("stub: attempt-start marker missing %q: %s", want, marker)
			}
		}

		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}

		code := 0
		return agent.RunResponse{
			Status:   agent.RunStatusCompleted,
			ExitCode: &code,
		}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "go",
			MaxRetries:            3,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !backendCalled {
		t.Fatal("expected injected backend to be called, but it was not — seam may be bypassed")
	}
}

// TestRun_UsesPiRPCAndParsesActiveTaskOutcome verifies the production runtime
// path launches Pi in RPC mode, sends the Doug prompt as a Pi message instead of
// executing it as a binary, and reads the workflow outcome from ACTIVE_TASK.md.
func TestRun_LogsFirstResponseAndNoResponseWarning(t *testing.T) {
	const epicID = "EPIC-UX"
	const taskID = "EPIC-UX-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.HeartbeatInterval != time.Second {
			return agent.RunResponse{}, fmt.Errorf("HeartbeatInterval = %s, want 1s", req.HeartbeatInterval)
		}
		req.HeartbeatFn(2*time.Second, "(no activity)")
		req.HeartbeatFn(3*time.Second, "bash internal/agent/pi_adapter.go")
		req.FirstResponseFn(4 * time.Second)
		req.HeartbeatFn(5*time.Second, "generating...")

		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}

		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	logger := &recordingLogger{}
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:                   "go",
			MaxRetries:                    3,
			MaxIterations:                 5,
			AgentHeartbeatSeconds:         1,
			FirstResponseThresholdSeconds: 2,
			KBEnabled:                     false,
		},
		paths:       paths,
		logger:      logger,
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if count := countExact(logger.warnings, "⚠ no provider response yet (+2s)"); count != 1 {
		t.Fatalf("no-response warning count = %d, want 1; warnings=%v", count, logger.warnings)
	}
	if !containsString(logger.infos, "► first response (+4s)") {
		t.Fatalf("missing first-response callout in infos: %v", logger.infos)
	}
	if !containsString(logger.infos, "[EPIC-UX-001] +3s — bash internal/agent/pi_adapter.go") {
		t.Fatalf("missing heartbeat activity line in infos: %v", logger.infos)
	}
	if !containsString(logger.sections, "[EPIC-UX-001] attempt 1/3 — Test feature task") {
		t.Fatalf("missing attempt header with task description in sections: %v", logger.sections)
	}
}

func TestRun_TTYLiveStatusSuppressesHeartbeatLogs(t *testing.T) {
	const epicID = "EPIC-TTY"
	const taskID = "EPIC-TTY-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	var statusOut bytes.Buffer
	oldWriter, oldIsTTY := liveStatusWriter, liveStatusIsTTY
	liveStatusWriter = &statusOut
	liveStatusIsTTY = func() bool { return true }
	t.Cleanup(func() {
		liveStatusWriter = oldWriter
		liveStatusIsTTY = oldIsTTY
	})

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		req.HeartbeatFn(500*time.Millisecond, "before delay")
		req.HeartbeatFn(time.Second, "tool\nunsafe \x1b[31mred\x1b[0m")

		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, err
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, err
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	logger := &recordingLogger{}
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:                   "go",
			MaxRetries:                    3,
			MaxIterations:                 5,
			AgentHeartbeatSeconds:         1,
			FirstResponseThresholdSeconds: 0,
			KBEnabled:                     false,
		},
		paths:       paths,
		logger:      logger,
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if containsString(logger.infos, "[EPIC-TTY-001] +1s — tool unsafe red") {
		t.Fatalf("TTY run emitted per-heartbeat log line: %v", logger.infos)
	}
	got := statusOut.String()
	for _, want := range []string{"EPIC-TTY-001", "+1s", "tool unsafe red", "Ctrl-C to interrupt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "\n") || strings.Contains(got, "\x1b[31m") {
		t.Fatalf("status output contains unsanitized multiline or color escape: %q", got)
	}
}

func TestRun_LogsAgentEndSummaryBeforeOutcome(t *testing.T) {
	const epicID = "EPIC-SUMMARY"
	const taskID = "EPIC-SUMMARY-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}

		code := 0
		return agent.RunResponse{
			Status:           agent.RunStatusCompleted,
			Duration:         65*time.Second + 400*time.Millisecond,
			ExitCode:         &code,
			FirstResponseMs:  2300,
			ToolCallCount:    7,
			ProviderFailures: 2,
		}, nil
	})

	logger := &recordingLogger{}
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "go",
			MaxRetries:            3,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             false,
		},
		paths:       paths,
		logger:      logger,
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	summaryIndex := indexOfString(logger.infos, "agent finished in 1m 5s — first response +2s, 7 tool calls, 2 provider failures")
	outcomeIndex := indexOfString(logger.infos, "outcome: EPIC_COMPLETE")
	if summaryIndex == -1 {
		t.Fatalf("missing agent summary in infos: %v", logger.infos)
	}
	if outcomeIndex == -1 {
		t.Fatalf("missing outcome line in infos: %v", logger.infos)
	}
	if summaryIndex > outcomeIndex {
		t.Fatalf("summary should be logged before outcome; infos=%v", logger.infos)
	}
}

func TestFormatAttemptHeader(t *testing.T) {
	got := formatAttemptHeader("EPIC-45-005", 2, 3, "Include task description in the per-attempt section header.")
	want := "[EPIC-45-005] attempt 2/3 — Include task description in the per-attempt section header."
	if got != want {
		t.Fatalf("formatAttemptHeader() = %q, want %q", got, want)
	}
}

func TestFormatAttemptHeader_TruncatesDescriptionTo80Characters(t *testing.T) {
	description := strings.Repeat("a", 81)
	got := formatAttemptHeader("EPIC-45-005", 1, 3, description)
	want := fmt.Sprintf("[EPIC-45-005] attempt 1/3 — %s", strings.Repeat("a", 77)+"...")
	if got != want {
		t.Fatalf("formatAttemptHeader() = %q, want %q", got, want)
	}
	gotDescription := strings.TrimPrefix(got, "[EPIC-45-005] attempt 1/3 — ")
	if len([]rune(gotDescription)) != 80 {
		t.Fatalf("truncated description length = %d, want 80", len([]rune(gotDescription)))
	}
}

func TestFormatAgentEndSummary(t *testing.T) {
	got := formatAgentEndSummary(agent.RunResponse{
		Duration:         125*time.Second + 600*time.Millisecond,
		FirstResponseMs:  1499,
		ToolCallCount:    3,
		ProviderFailures: 4,
	})
	want := "agent finished in 2m 6s — first response +1s, 3 tool calls, 4 provider failures"
	if got != want {
		t.Fatalf("formatAgentEndSummary() = %q, want %q", got, want)
	}
}

func indexOfString(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

func countExact(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRun_UsesPiRPCAndParsesActiveTaskOutcome(t *testing.T) {
	argvPath, promptPath := prependFakePiRPC(t)

	const epicID = "EPIC-PI"
	const taskID = "EPIC-PI-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:   "static",
			MaxRetries:    3,
			MaxIterations: 5,
			KBEnabled:     false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	argvData, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatalf("read fake pi argv: %v", err)
	}
	argv := string(argvData)
	if !strings.Contains(argv, "--mode\nrpc") || !strings.Contains(argv, "--session-dir") {
		t.Fatalf("expected runtime to launch pi in RPC mode, got argv:\n%s", argv)
	}

	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read fake pi prompt payload: %v", err)
	}
	promptPayload := string(promptData)
	if !strings.Contains(promptPayload, "[DOUG_TASK_ID: "+taskID+"]") {
		t.Fatalf("expected Doug prompt to be sent as Pi RPC message, got payload:\n%s", promptPayload)
	}

	archivePath := filepath.Join(paths.LogsDir, "sessions", epicID, fmt.Sprintf("session-%s_attempt-1.md", taskID))
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archived ACTIVE_TASK.md: %v", err)
	}
	archive := string(archiveData)
	if !strings.Contains(archive, `outcome: "EPIC_COMPLETE"`) {
		t.Fatalf("expected runtime to parse outcome from ACTIVE_TASK.md archive, got:\n%s", archive)
	}
	if strings.Contains(archive, `outcome: "FAILURE"`) {
		t.Fatalf("runtime must not use Pi event payload as Doug workflow outcome, got:\n%s", archive)
	}
}

func TestRun_RetriesTransportFailureWithoutConsumingTaskAttempt(t *testing.T) {
	const epicID = "EPIC-INFRA"
	const taskID = "EPIC-INFRA-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	activeTaskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	var calls int
	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		calls++
		if req.Task.Attempt != 1 {
			return agent.RunResponse{}, fmt.Errorf("attempt = %d, want 1", req.Task.Attempt)
		}
		if calls == 1 {
			return agent.RunResponse{Status: agent.RunStatusTransportFailure}, fmt.Errorf("provider unavailable")
		}
		data, err := os.ReadFile(activeTaskPath)
		if err != nil {
			return agent.RunResponse{}, err
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(activeTaskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, err
		}
		return agent.RunResponse{Status: agent.RunStatusCompleted}, nil
	})

	var slept []time.Duration
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "go",
			MaxRetries:            3,
			MaxInfraRetries:       3,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
		infraRetrySleeper: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 2 {
		t.Fatalf("backend calls = %d, want 2", calls)
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("backoff sleeps = %v, want [1s]", slept)
	}

	recordPath := filepath.Join(paths.LogsDir, "failures", epicID, fmt.Sprintf("infra-failure-%s-attempt-1.md", taskID))
	recordData, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read infra failure record: %v", err)
	}
	recordText := string(recordData)
	for _, want := range []string{
		`task_id: "` + taskID + `"`,
		`attempt: 1`,
		`class: "transport_failure"`,
		`backend_status: "transport_failure"`,
		`error: "provider unavailable"`,
		`exit_code: ""`,
		`output_log: "` + filepath.Join(paths.LogsDir, "output", epicID, fmt.Sprintf("output-%s_attempt-1.log", taskID)) + `"`,
	} {
		if !strings.Contains(recordText, want) {
			t.Fatalf("infra failure record missing %q:\n%s", want, recordText)
		}
	}
}

func TestRun_TransportFailureCapWritesDurableFailureAndHalts(t *testing.T) {
	const epicID = "EPIC-INFRA-CAP"
	const taskID = "EPIC-INFRA-CAP-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	var calls int
	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		calls++
		if req.Task.Attempt != 1 {
			return agent.RunResponse{}, fmt.Errorf("attempt = %d, want 1", req.Task.Attempt)
		}
		return agent.RunResponse{Status: agent.RunStatusTransportFailure}, fmt.Errorf("rpc transport down")
	})

	var slept []time.Duration
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:           "go",
			MaxRetries:            5,
			MaxInfraRetries:       2,
			MaxIterations:         5,
			AgentHeartbeatSeconds: 0,
			KBEnabled:             false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
		infraRetrySleeper: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	}

	err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run to halt with transport failure cap error")
	}
	if !strings.Contains(err.Error(), "transport failed 2/2") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("backend calls = %d, want 2", calls)
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("backoff sleeps = %v, want [1s]", slept)
	}

	stateData, err := os.ReadFile(paths.StatePath)
	if err != nil {
		t.Fatalf("read project state: %v", err)
	}
	stateText := string(stateData)
	if strings.Contains(stateText, "attempts: 1") || strings.Contains(stateText, "attempts: 2") {
		t.Fatalf("expected task attempts restored to zero/omitted, got:\n%s", stateText)
	}
	if !strings.Contains(stateText, "infra_retries: 2") {
		t.Fatalf("expected infra_retries persisted as 2, got:\n%s", stateText)
	}

	failureData, err := os.ReadFile(filepath.Join(paths.DougDir, "ACTIVE_FAILURE.md"))
	if err != nil {
		t.Fatalf("read ACTIVE_FAILURE.md: %v", err)
	}
	failureText := string(failureData)
	if !strings.Contains(failureText, "Infrastructure retries: 2/2") || !strings.Contains(failureText, taskID) {
		t.Fatalf("unexpected failure report:\n%s", failureText)
	}

	for i, class := range []string{"transport_failure", "transport_failure_retry_cap"} {
		recordPath := filepath.Join(paths.LogsDir, "failures", epicID, fmt.Sprintf("infra-failure-%s-attempt-%d.md", taskID, i+1))
		recordData, err := os.ReadFile(recordPath)
		if err != nil {
			t.Fatalf("read infra failure record %d: %v", i+1, err)
		}
		recordText := string(recordData)
		for _, want := range []string{
			`task_id: "` + taskID + `"`,
			fmt.Sprintf("attempt: %d", i+1),
			`class: "` + class + `"`,
			`backend_status: "transport_failure"`,
			`error: "rpc transport down"`,
			`output_log: "` + filepath.Join(paths.LogsDir, "output", epicID, fmt.Sprintf("output-%s_attempt-1.log", taskID)) + `"`,
		} {
			if !strings.Contains(recordText, want) {
				t.Fatalf("infra failure record %d missing %q:\n%s", i+1, want, recordText)
			}
		}
	}
}
