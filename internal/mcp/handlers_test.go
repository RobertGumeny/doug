package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/lifecycle"
	"github.com/robertgumeny/doug/internal/runlock"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func writeMCPFixtures(t *testing.T, root string, projectState, tasks string) lifecycle.Paths {
	t.Helper()
	paths := lifecycle.DefaultPaths(root)
	testutil.WriteFile(t, paths.StatePath, projectState)
	testutil.WriteFile(t, paths.TasksPath, tasks)
	return paths
}

func mcpProjectState(activeID string, attempts int) string {
	if activeID == "" {
		return `current_epic:
    id: EPIC-MCP
    name: MCP Epic
    branch_name: feature/EPIC-MCP
    started_at: "2026-01-01T00:00:00Z"
    completed_at: null
active_task:
    type: feature
    id: ""
next_task:
    type: feature
    id: TASK-1
metrics:
    total_tasks_completed: 0
    total_duration_seconds: 0
    tasks: []
`
	}
	return `current_epic:
    id: EPIC-MCP
    name: MCP Epic
    branch_name: feature/EPIC-MCP
    started_at: "2026-01-01T00:00:00Z"
    completed_at: null
active_task:
    type: feature
    id: ` + activeID + `
    attempts: ` + string(rune('0'+attempts)) + `
next_task:
    type: documentation
    id: TASK-2
metrics:
    total_tasks_completed: 0
    total_duration_seconds: 0
    tasks: []
`
}

func mcpTasks(first, second types.Status) string {
	return `epic:
    id: EPIC-MCP
    name: MCP Epic
    tasks:
        - id: TASK-1
          type: feature
          status: ` + string(first) + `
          description: Build MCP status
          acceptance_criteria:
            - Status works
        - id: TASK-2
          type: documentation
          status: ` + string(second) + `
          description: Document MCP usage
          acceptance_criteria:
            - Docs work
`
}

func TestGetStatusIncludesLifecycleStateWithoutMutation(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("TASK-1", 2), mcpTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "# Active\n")
	beforeState := mustRead(t, paths.StatePath)
	beforeTasks := mustRead(t, paths.TasksPath)

	resp, err := ToolHandler{ProjectRoot: root}.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if resp.CurrentEpic != "EPIC-MCP" || resp.LifecyclePhase != string(lifecycle.StatusActiveTask) {
		t.Fatalf("unexpected status response: %#v", resp)
	}
	if resp.ActiveAssignment == nil || resp.ActiveAssignment.ID != "TASK-1" || resp.AttemptCount != 2 {
		t.Fatalf("active assignment/attempt missing: %#v", resp)
	}
	if resp.BriefPath != filepath.Join(paths.DougDir, "ACTIVE_TASK.md") {
		t.Fatalf("BriefPath = %q", resp.BriefPath)
	}
	if resp.Blocked || resp.Completed {
		t.Fatalf("blocked/completed = %v/%v, want false/false", resp.Blocked, resp.Completed)
	}
	if !contains(resp.AllowedNextActions, ToolReportTaskComplete) || !contains(resp.AllowedNextActions, ToolReportTaskBlocked) {
		t.Fatalf("allowed actions missing report tools: %#v", resp.AllowedNextActions)
	}
	if got := mustRead(t, paths.StatePath); got != beforeState {
		t.Fatal("GetStatus mutated project-state.yaml")
	}
	if got := mustRead(t, paths.TasksPath); got != beforeTasks {
		t.Fatal("GetStatus mutated tasks.yaml")
	}
}

func TestGetNextTaskFailsFastWhenRunLockHeld(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	lock, err := runlock.TryAcquire(paths.DougDir, "test driver")
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	defer func() { _ = lock.Close() }()

	_, err = ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}}.GetNextTask()
	if err == nil || !strings.Contains(err.Error(), "run lock is held") {
		t.Fatalf("GetNextTask err = %v, want lock-held error", err)
	}
	if _, statErr := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); !os.IsNotExist(statErr) {
		t.Fatalf("GetNextTask should not write ACTIVE_TASK.md while lock is held; stat err=%v", statErr)
	}
}

func TestGetStatusDoesNotClaimWorkWhileRunLockHeld(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	beforeState := mustRead(t, paths.StatePath)
	beforeTasks := mustRead(t, paths.TasksPath)
	lock, err := runlock.TryAcquire(paths.DougDir, "test driver")
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	defer func() { _ = lock.Close() }()

	resp, err := ToolHandler{ProjectRoot: root}.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if resp.LifecyclePhase != string(lifecycle.StatusNoActiveTask) || !contains(resp.AllowedNextActions, ToolGetNextTask) {
		t.Fatalf("unexpected status while locked: %#v", resp)
	}
	if got := mustRead(t, paths.StatePath); got != beforeState {
		t.Fatal("GetStatus mutated project-state.yaml while lock was held")
	}
	if got := mustRead(t, paths.TasksPath); got != beforeTasks {
		t.Fatal("GetStatus mutated tasks.yaml while lock was held")
	}
	if _, statErr := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); !os.IsNotExist(statErr) {
		t.Fatalf("GetStatus should not claim/write ACTIVE_TASK.md; stat err=%v", statErr)
	}
}

func TestGetNextTaskWritesAndReturnsCanonicalBriefWithContextGuidance(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))

	resp, err := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}}.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask: %v", err)
	}
	if !resp.Claimed || resp.ActiveAssignment == nil || resp.ActiveAssignment.ID != "TASK-1" {
		t.Fatalf("did not claim TASK-1: %#v", resp)
	}
	brief := mustRead(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"))
	if resp.Brief != brief || !strings.Contains(resp.Brief, "Build MCP status") || !strings.Contains(resp.Brief, "## Result") {
		t.Fatalf("response did not include canonical brief: %q", resp.Brief)
	}
	if !strings.Contains(resp.DispatcherGuidance, "fresh worker context") || !strings.Contains(resp.DispatcherGuidance, "thin dispatcher") {
		t.Fatalf("missing dispatcher/worker guidance: %q", resp.DispatcherGuidance)
	}
}

func TestReportTaskCompleteParsesResultAndUsesVerifiedSuccessPathBeforeAdvance(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}, BuildSystem: &build.StaticBuildSystem{}}
	if _, err := h.GetNextTask(); err != nil {
		t.Fatalf("GetNextTask: %v", err)
	}
	writeOutcome(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "SUCCESS")
	verified := false
	h.HandleSuccess = func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error) {
		if result.Outcome != types.OutcomeSuccess {
			t.Fatalf("outcome = %q, want SUCCESS", result.Outcome)
		}
		verified = true
		_, err := lifecycle.CompleteVerifiedTask(lifecycle.Options{Paths: lifecycle.DefaultPaths(root)}, ctx.TaskID)
		return handlers.SuccessResult{Kind: handlers.Continue}, err
	}

	resp, err := h.ReportTaskComplete("TASK-1")
	if err != nil {
		t.Fatalf("ReportTaskComplete: %v", err)
	}
	if !verified {
		t.Fatal("verified success path was not invoked")
	}
	if resp.Outcome != "SUCCESS" {
		t.Fatalf("Outcome = %q", resp.Outcome)
	}
	loaded, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loaded.ActiveTask.ID != "TASK-2" {
		t.Fatalf("active task after completion = %#v, want TASK-2", loaded.ActiveTask)
	}
}

func TestReportTaskBlockedParsesFailureAndRecordsBlockedState(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 1}}
	if _, err := h.GetNextTask(); err != nil {
		t.Fatalf("GetNextTask: %v", err)
	}
	writeOutcome(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "FAILURE")

	resp, err := h.ReportTaskBlocked("TASK-1")
	if err != nil {
		t.Fatalf("ReportTaskBlocked: %v", err)
	}
	if resp.Outcome != "FAILURE" || !resp.Blocked {
		t.Fatalf("expected blocked failure response, got %#v", resp)
	}
	loaded, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if got := loaded.Epic.Tasks[0].Status; got != types.StatusBlocked {
		t.Fatalf("TASK-1 status = %q, want BLOCKED", got)
	}
}

func TestBacklogDrainedGetNextTaskAssignsPostEpicLifecycleWork(t *testing.T) {
	root := t.TempDir()
	completed := `current_epic:
    id: EPIC-MCP
    name: MCP Epic
    branch_name: feature/EPIC-MCP
    started_at: "2026-01-01T00:00:00Z"
    completed_at: "2026-01-02T00:00:00Z"
active_task:
    type: ""
    id: ""
next_task:
    type: ""
    id: ""
metrics:
    total_tasks_completed: 0
    total_duration_seconds: 0
    tasks: []
`
	paths := writeMCPFixtures(t, root, completed, mcpTasks(types.StatusDone, types.StatusDone))
	resp, err := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 1, ReviewEnabled: true}}.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask: %v", err)
	}
	if !resp.Claimed || resp.ActiveAssignment == nil || resp.ActiveAssignment.ID != "POST_EPIC_REVIEW" {
		t.Fatalf("expected post-epic review assignment, got %#v", resp)
	}
	if !strings.Contains(resp.Brief, "## Result") || !strings.Contains(resp.Brief, "post-epic review") {
		t.Fatalf("post-epic lifecycle brief missing contract: %q", resp.Brief)
	}
	if _, err := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); err != nil {
		t.Fatalf("ACTIVE_TASK.md not written: %v", err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeOutcome(t *testing.T, path, outcome string) {
	t.Helper()
	data := mustRead(t, path)
	data = strings.Replace(data, `outcome: ""`, `outcome: "`+outcome+`"`, 1)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
