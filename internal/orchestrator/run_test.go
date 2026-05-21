package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
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

func prependFakePATHBinaries(t *testing.T, names ...string) {
	t.Helper()
	shimDir := t.TempDir()
	for _, name := range names {
		testutil.WriteFile(t, filepath.Join(shimDir, name), "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(filepath.Join(shimDir, name), 0o755); err != nil {
			t.Fatalf("chmod fake binary %s: %v", name, err)
		}
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
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

// TestRun_PolicyWriteScopesUpgradeContractRestrictions verifies that write scopes
// configured in doug.yaml policy are applied to the RunRequest restriction contract,
// upgrading the write restriction mode from Inherit to AllowList.
func TestRun_PolicyWriteScopesUpgradeContractRestrictions(t *testing.T) {
	const epicID = "EPIC-SCOPE"
	const taskID = "EPIC-SCOPE-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.Restrictions.Write.Mode != agent.RestrictionModeAllowList {
			return agent.RunResponse{}, fmt.Errorf("write restriction mode = %q, want AllowList", req.Restrictions.Write.Mode)
		}
		foundScope := false
		for _, p := range req.Restrictions.Write.Paths {
			if strings.Contains(p, "custom/output") {
				foundScope = true
				break
			}
		}
		if !foundScope {
			return agent.RunResponse{}, fmt.Errorf("custom/output not in write restriction paths: %v", req.Restrictions.Write.Paths)
		}

		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		return agent.RunResponse{}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:   "go",
			MaxRetries:    3,
			MaxIterations: 5,
			KBEnabled:     false,
			Policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {
						WriteScopes: []string{"custom/output"},
					},
				},
			},
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRun_UsesPiRPCAndParsesActiveTaskOutcome verifies the production runtime
// path launches Pi in RPC mode, sends the Doug prompt as a Pi message instead of
// executing it as a binary, and reads the workflow outcome from ACTIVE_TASK.md.
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

// TestRun_PropagatesInteractionModeToRoutingWhenRPC verifies that when the policy
// configures interaction_mode: rpc for the feature task type, the resolved mode
// propagates to req.Routing.InteractionMode in the RunRequest sent to the backend.
func TestRun_PropagatesInteractionModeToRoutingWhenRPC(t *testing.T) {
	prependFakePATHBinaries(t, "pi")

	const epicID = "EPIC-EXEC"
	const taskID = "EPIC-EXEC-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.Routing.InteractionMode != "rpc" {
			return agent.RunResponse{}, fmt.Errorf("interaction mode = %q, want rpc", req.Routing.InteractionMode)
		}

		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		return agent.RunResponse{}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:   "go",
			MaxRetries:    3,
			MaxIterations: 5,
			KBEnabled:     false,
			Policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {InteractionMode: "rpc"},
				},
			},
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRun_PropagatesDefaultInteractionModeToRouting verifies that when no
// interaction_mode is configured, runtime resolves to its built-in rpc default.
func TestRun_PropagatesDefaultInteractionModeToRouting(t *testing.T) {
	const epicID = "EPIC-SUB"
	const taskID = "EPIC-SUB-001"
	dir := setupRunRepo(t, epicID)
	paths := NewPaths(dir)
	writeRunState(t, dir, epicID, taskID)

	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.Routing.InteractionMode != "rpc" {
			return agent.RunResponse{}, fmt.Errorf("interaction mode = %q, want default rpc", req.Routing.InteractionMode)
		}

		data, err := os.ReadFile(req.Brief.Path)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "EPIC_COMPLETE"`, 1)
		if err := os.WriteFile(req.Brief.Path, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		return agent.RunResponse{}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			BuildSystem:   "go",
			MaxRetries:    3,
			MaxIterations: 5,
			KBEnabled:     false,
		},
		paths:       paths,
		logger:      log.Discard(),
		buildSystem: &runLoopBuildSystem{},
		backend:     stub,
	}

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
