package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

// backendFunc adapts a function to the agent.Backend interface.
type backendFunc func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error)

func (f backendFunc) Run(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
	return f(ctx, req)
}

func TestScaffoldProject_MissingProjectState(t *testing.T) {
	dir := t.TempDir()

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ".doug/project-state.yaml") {
		t.Fatalf("expected error to mention project-state.yaml, got: %v", err)
	}
}

func TestScaffoldProject_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ".doug/plan/manifest.yaml") {
		t.Fatalf("expected error to mention manifest path, got: %v", err)
	}
}

func TestScaffoldProject_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	writeManifest(t, dir)
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "manifest.yaml"), `
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

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `missing required field "project.mode"`) {
		t.Fatalf("expected manifest validation error, got: %v", err)
	}
}

func TestScaffoldProject_SuccessDispatchesOnceWithoutStateWrites(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "scaffold_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
	writeManifest(t, dir)

	restore := stubScaffoldDeps()
	defer restore()

	var runCalls int
	var successCalls int
	var failureCalls int

	scaffoldCheckDeps = func(cfg *config.OrchestratorConfig) error { return nil }
	scaffoldNewBuild = func(buildSystemType, projectRoot string) (build.BuildSystem, error) {
		if buildSystemType != "pnpm" {
			t.Fatalf("build system = %q, want %q", buildSystemType, "pnpm")
		}
		return &stubBuildSystem{}, nil
	}
	scaffoldRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		runCalls++
		activeTaskPath := filepath.Join(req.ProjectRoot, ".doug", "ACTIVE_TASK.md")
		if req.Phase != agent.RunPhaseScaffold {
			t.Fatalf("phase = %q, want %q", req.Phase, agent.RunPhaseScaffold)
		}
		if req.Task.ID != "SCAFFOLD" || req.Task.Type != string(types.TaskTypeScaffold) {
			t.Fatalf("unexpected task context: %+v", req.Task)
		}
		if req.Task.Attempt != 1 || req.Task.MaxRetries != 1 {
			t.Fatalf("unexpected task attempt context: %+v", req.Task)
		}
		if req.Brief.Path != activeTaskPath || req.Brief.Format != agent.BriefFormatMarkdown || req.Brief.Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected brief: %+v", req.Brief)
		}
		manifestPath := filepath.Join(req.ProjectRoot, ".doug", "plan", "manifest.yaml")
		if len(req.ContextLoadOrder) != 4 {
			t.Fatalf("contextLoadOrder length = %d, want 4", len(req.ContextLoadOrder))
		}
		if req.ContextLoadOrder[2].Kind != agent.ContextInputCanonicalBrief || req.ContextLoadOrder[2].Path != activeTaskPath || !req.ContextLoadOrder[2].Required || req.ContextLoadOrder[2].Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected canonical brief context: %+v", req.ContextLoadOrder[2])
		}
		if req.ContextLoadOrder[3].Kind != agent.ContextInputWorkingArtifact || req.ContextLoadOrder[3].Path != manifestPath || !req.ContextLoadOrder[3].Required || req.ContextLoadOrder[3].Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected manifest working artifact context: %+v", req.ContextLoadOrder[3])
		}
		if req.Routing.Workflow != "scaffold" || req.Routing.SkillName != "scaffold" {
			t.Fatalf("unexpected routing: %+v", req.Routing)
		}
		if req.Restrictions.Read.Mode != agent.RestrictionModeInherit || req.Restrictions.Write.Mode != agent.RestrictionModeInherit {
			t.Fatalf("unexpected restrictions: %+v", req.Restrictions)
		}
		if len(req.Restrictions.Read.Paths) != 5 || req.Restrictions.Read.Paths[4] != manifestPath {
			t.Fatalf("unexpected read restriction paths: %+v", req.Restrictions.Read.Paths)
		}
		if len(req.Artifacts.Read) != 5 {
			t.Fatalf("read artifact count = %d, want 5", len(req.Artifacts.Read))
		}
		if req.Artifacts.Read[4].Path != manifestPath || req.Artifacts.Read[4].Purpose != agent.ArtifactPurposeWorkingArtifact {
			t.Fatalf("unexpected manifest read artifact: %+v", req.Artifacts.Read[4])
		}
		if len(req.Artifacts.Write) != 4 {
			t.Fatalf("write artifact count = %d, want 4", len(req.Artifacts.Write))
		}
		if req.Artifacts.Write[0].Path != req.ProjectRoot || req.Artifacts.Write[0].Purpose != agent.ArtifactPurposeProjectWorkspace {
			t.Fatalf("unexpected project workspace write artifact: %+v", req.Artifacts.Write[0])
		}
		if !strings.Contains(req.Command, "scaffold") {
			t.Fatalf("expected scaffold skill in command, got %q", req.Command)
		}
		if !strings.Contains(req.Command, "SCAFFOLD") {
			t.Fatalf("expected scaffold task id in command, got %q", req.Command)
		}
		replaceAgentOutcome(t, activeTaskPath, "SUCCESS")
		code := 0
		return agent.RunResponse{
			Status:              agent.RunStatusCompleted,
			Duration:            2 * time.Second,
			ExitCode:            &code,
			SessionID:           "pi-session-123",
			AvailableSessionIDs: []string{"pi-session-123", "pi-session-456"},
		}, nil
	})
	scaffoldHandleSuccess = func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error) {
		successCalls++
		if ctx.Attempts != 1 || ctx.Config.MaxRetries != 1 {
			t.Fatalf("unexpected attempt config: attempts=%d max_retries=%d", ctx.Attempts, ctx.Config.MaxRetries)
		}
		if ctx.StatePath == filepath.Join(dir, ".doug", "project-state.yaml") {
			t.Fatal("expected scaffold success handler to avoid the real project-state.yaml path")
		}
		if ctx.TasksPath == filepath.Join(dir, ".doug", "tasks.yaml") {
			t.Fatal("expected scaffold success handler to avoid the real tasks.yaml path")
		}
		if result.Outcome != types.OutcomeSuccess {
			t.Fatalf("result outcome = %q, want %q", result.Outcome, types.OutcomeSuccess)
		}
		if agentDurationSeconds != 2 {
			t.Fatalf("agentDurationSeconds = %d, want 2", agentDurationSeconds)
		}
		return handlers.SuccessResult{Kind: handlers.Continue}, nil
	}
	scaffoldHandleFailure = func(ctx *types.LoopContext, agentDurationSeconds int) error {
		failureCalls++
		return nil
	}

	if err := scaffoldProject(dir); err != nil {
		t.Fatalf("scaffoldProject(success): %v", err)
	}

	if runCalls != 1 {
		t.Fatalf("runCalls = %d, want 1", runCalls)
	}
	if successCalls != 1 {
		t.Fatalf("successCalls = %d, want 1", successCalls)
	}
	if failureCalls != 0 {
		t.Fatalf("failureCalls = %d, want 0", failureCalls)
	}

	assertActiveTaskContains(t, dir, []string{
		"**Task ID**: SCAFFOLD",
		"**Task Type**: scaffold",
		"## Build System",
		"**System**: pnpm",
		"## Manifest Context",
		"schema_version: 1",
		"package_manager: pnpm",
		"build_system: npm-scripts",
		"constraints:",
		"Deploy on Vercel",
	})
	metadataPath := agent.RunMetadataPath(filepath.Join(dir, ".doug", "logs", "output", "output-SCAFFOLD_attempt-1.log"))
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read scaffold run metadata: %v", err)
	}
	if !strings.Contains(string(metadata), `"pi-session-456"`) {
		t.Fatalf("expected scaffold run metadata to capture session ids, got:\n%s", metadata)
	}
	assertFileEquals(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	assertFileEquals(t, filepath.Join(dir, ".doug", "doug.yaml"), "scaffold_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
}

func TestScaffoldProject_FailureDispatchesOnceAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "scaffold_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
	writeManifest(t, dir)

	restore := stubScaffoldDeps()
	defer restore()

	var runCalls int
	var successCalls int
	var failureCalls int

	scaffoldCheckDeps = func(cfg *config.OrchestratorConfig) error { return nil }
	scaffoldNewBuild = func(buildSystemType, projectRoot string) (build.BuildSystem, error) {
		return &stubBuildSystem{}, nil
	}
	scaffoldRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		runCalls++
		replaceAgentOutcome(t, filepath.Join(req.ProjectRoot, ".doug", "ACTIVE_TASK.md"), "FAILURE")
		return agent.RunResponse{Duration: time.Second}, nil
	})
	scaffoldHandleSuccess = func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error) {
		successCalls++
		return handlers.SuccessResult{Kind: handlers.Continue}, nil
	}
	scaffoldHandleFailure = func(ctx *types.LoopContext, agentDurationSeconds int) error {
		failureCalls++
		if ctx.StatePath == filepath.Join(dir, ".doug", "project-state.yaml") {
			t.Fatal("expected scaffold failure handler to avoid the real project-state.yaml path")
		}
		if ctx.Attempts != 1 || ctx.Config.MaxRetries != 1 {
			t.Fatalf("unexpected attempt config: attempts=%d max_retries=%d", ctx.Attempts, ctx.Config.MaxRetries)
		}
		return fmt.Errorf("forced scaffold failure")
	}

	err := scaffoldProject(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "forced scaffold failure") {
		t.Fatalf("expected failure handler error, got: %v", err)
	}
	if runCalls != 1 {
		t.Fatalf("runCalls = %d, want 1", runCalls)
	}
	if successCalls != 0 {
		t.Fatalf("successCalls = %d, want 0", successCalls)
	}
	if failureCalls != 1 {
		t.Fatalf("failureCalls = %d, want 1", failureCalls)
	}

	assertFileEquals(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	assertFileEquals(t, filepath.Join(dir, ".doug", "doug.yaml"), "scaffold_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
}

func TestScaffoldProject_PolicyWriteScopesUpgradeContractRestrictions(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), `
scaffold_agent_command: mock-agent {{skill_name}} {{task_id}}
policy:
  tasks:
    scaffold:
      write_scopes:
        - custom/output
`)
	writeManifest(t, dir)

	restore := stubScaffoldDeps()
	defer restore()

	scaffoldCheckDeps = func(cfg *config.OrchestratorConfig) error { return nil }
	scaffoldNewBuild = func(buildSystemType, projectRoot string) (build.BuildSystem, error) {
		return &stubBuildSystem{}, nil
	}
	scaffoldRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.Restrictions.Write.Mode != agent.RestrictionModeAllowList {
			t.Fatalf("write restriction mode = %q, want %q", req.Restrictions.Write.Mode, agent.RestrictionModeAllowList)
		}
		foundScope := false
		for _, p := range req.Restrictions.Write.Paths {
			if strings.Contains(p, "custom/output") {
				foundScope = true
				break
			}
		}
		if !foundScope {
			t.Fatalf("custom/output not found in write restriction paths: %v", req.Restrictions.Write.Paths)
		}
		replaceAgentOutcome(t, filepath.Join(req.ProjectRoot, ".doug", "ACTIVE_TASK.md"), "SUCCESS")
		return agent.RunResponse{}, nil
	})
	scaffoldHandleSuccess = func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error) {
		return handlers.SuccessResult{Kind: handlers.Continue}, nil
	}
	scaffoldHandleFailure = func(ctx *types.LoopContext, agentDurationSeconds int) error {
		return nil
	}

	if err := scaffoldProject(dir); err != nil {
		t.Fatalf("scaffoldProject: %v", err)
	}

	assertActiveTaskContains(t, dir, []string{"## Write Scope Constraints", "custom/output"})
}

func TestBuildScaffoldTask(t *testing.T) {
	task, err := buildScaffoldTask(&types.Manifest{
		SchemaVersion: 1,
		Project: types.ManifestProject{
			Name: "Acme App",
			Mode: "greenfield",
		},
		Scaffold: types.ManifestScaffold{
			Language:       "typescript",
			Runtime:        "node",
			Framework:      "nextjs",
			PackageManager: "pnpm",
			BuildSystem:    "npm-scripts",
		},
	})
	if err != nil {
		t.Fatalf("buildScaffoldTask: %v", err)
	}

	if task.ID != "SCAFFOLD" {
		t.Fatalf("task.ID = %q, want %q", task.ID, "SCAFFOLD")
	}
	if task.Type != types.TaskTypeScaffold {
		t.Fatalf("task.Type = %q, want %q", task.Type, types.TaskTypeScaffold)
	}
	if !task.Type.IsSynthetic() {
		t.Fatal("expected scaffold task type to be synthetic")
	}
	if len(task.AcceptanceCriteria) == 0 {
		t.Fatal("expected scaffold task acceptance criteria")
	}
}

func TestResolveScaffoldBuildSystem(t *testing.T) {
	tests := []struct {
		name     string
		manifest *types.Manifest
		want     string
	}{
		{
			name: "supported build system passes through",
			manifest: &types.Manifest{
				Scaffold: types.ManifestScaffold{
					BuildSystem: "go",
				},
			},
			want: "go",
		},
		{
			name: "package manager maps npm scripts manifest to pnpm verifier",
			manifest: &types.Manifest{
				Scaffold: types.ManifestScaffold{
					Runtime:        "node",
					PackageManager: "pnpm",
					BuildSystem:    "npm-scripts",
				},
			},
			want: "pnpm",
		},
		{
			name: "node runtime falls back to npm",
			manifest: &types.Manifest{
				Scaffold: types.ManifestScaffold{
					Runtime:     "node",
					BuildSystem: "unknown",
				},
			},
			want: "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveScaffoldBuildSystem(tt.manifest); got != tt.want {
				t.Fatalf("resolveScaffoldBuildSystem() = %q, want %q", got, tt.want)
			}
		})
	}
}

type stubBuildSystem struct{}

func (s *stubBuildSystem) Install() error      { return nil }
func (s *stubBuildSystem) Build() error        { return nil }
func (s *stubBuildSystem) Test() error         { return nil }
func (s *stubBuildSystem) IsInitialized() bool { return true }

func stubScaffoldDeps() func() {
	oldLoadConfig := scaffoldLoadConfig
	oldCheckDeps := scaffoldCheckDeps
	oldNewBuild := scaffoldNewBuild
	oldRunAgent := scaffoldRunAgent
	oldParseResult := scaffoldParseResult
	oldHandleSuccess := scaffoldHandleSuccess
	oldHandleFailure := scaffoldHandleFailure

	return func() {
		scaffoldLoadConfig = oldLoadConfig
		scaffoldCheckDeps = oldCheckDeps
		scaffoldNewBuild = oldNewBuild
		scaffoldRunAgent = oldRunAgent
		scaffoldParseResult = oldParseResult
		scaffoldHandleSuccess = oldHandleSuccess
		scaffoldHandleFailure = oldHandleFailure
	}
}

func writeManifest(t *testing.T, dir string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "manifest.yaml"), `
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
}

func replaceAgentOutcome(t *testing.T, activeTaskPath, outcome string) {
	t.Helper()
	data, err := os.ReadFile(activeTaskPath)
	if err != nil {
		t.Fatalf("read ACTIVE_TASK.md: %v", err)
	}
	content := strings.Replace(string(data), `outcome: ""`, fmt.Sprintf(`outcome: "%s"`, outcome), 1)
	if err := os.WriteFile(activeTaskPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write ACTIVE_TASK.md: %v", err)
	}
}

func assertActiveTaskContains(t *testing.T, dir string, wants []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".doug", "ACTIVE_TASK.md"))
	if err != nil {
		t.Fatalf("read ACTIVE_TASK.md: %v", err)
	}
	content := string(data)
	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in ACTIVE_TASK.md, got:\n%s", want, content)
		}
	}
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s changed unexpectedly:\n%s", path, string(data))
	}
}
