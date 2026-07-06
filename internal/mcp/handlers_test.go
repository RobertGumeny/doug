package mcp

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")
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

func TestDiagnoseLifecycleResponseIncludesFindingsManualReviewAndActions(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("TASK-1", 1), mcpTasks(types.StatusTODO, types.StatusTODO))
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	testutil.WriteFile(t, briefPath, "**Task ID**: TASK-2\n")
	beforeState := mustRead(t, paths.StatePath)
	beforeTasks := mustRead(t, paths.TasksPath)
	beforeBrief := mustRead(t, briefPath)

	resp, err := ToolHandler{ProjectRoot: root}.DiagnoseLifecycle()
	if err != nil {
		t.Fatalf("DiagnoseLifecycle: %v", err)
	}
	if resp.LifecyclePhase != string(lifecycle.StatusNoActiveTask) || resp.ActiveAssignment == nil || resp.ActiveAssignment.ID != "TASK-1" {
		t.Fatalf("unexpected diagnostic status: %#v", resp)
	}
	if !contains(resp.AllowedNextActions, lifecycle.DiagnosticActionManualReview) || !contains(resp.AllowedNextActions, lifecycle.DiagnosticActionGetStatus) {
		t.Fatalf("allowed diagnostic actions = %#v, want get_status and manual_review", resp.AllowedNextActions)
	}
	if !responseFinding(resp.Findings, "AMBIGUOUS_ACTIVE_BRIEF_DRIFT", "error", true) {
		t.Fatalf("findings = %#v, want ambiguous active brief drift with severity/error and manual review", resp.Findings)
	}
	if got := mustRead(t, paths.StatePath); got != beforeState {
		t.Fatal("DiagnoseLifecycle mutated project-state.yaml")
	}
	if got := mustRead(t, paths.TasksPath); got != beforeTasks {
		t.Fatal("DiagnoseLifecycle mutated tasks.yaml")
	}
	if got := mustRead(t, briefPath); got != beforeBrief {
		t.Fatal("DiagnoseLifecycle mutated ACTIVE_TASK.md")
	}
}

func TestAllowedNextActionsUseBackwardCompatibleStringGrammar(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("TASK-1", 1), mcpTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")

	statusResp, err := ToolHandler{ProjectRoot: root}.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	assertActionGrammar(t, statusResp.AllowedNextActions)
	if !contains(statusResp.AllowedNextActions, ToolReportTaskComplete) || !contains(statusResp.AllowedNextActions, ToolReportTaskBlocked) {
		t.Fatalf("status allowed actions = %#v, want report tool names", statusResp.AllowedNextActions)
	}

	if err := os.Remove(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); err != nil {
		t.Fatalf("remove active brief: %v", err)
	}
	diagnosticsResp, err := ToolHandler{ProjectRoot: root}.DiagnoseLifecycle()
	if err != nil {
		t.Fatalf("DiagnoseLifecycle: %v", err)
	}
	assertActionGrammar(t, diagnosticsResp.AllowedNextActions)
	if !contains(diagnosticsResp.AllowedNextActions, lifecycle.DiagnosticActionReconcileRepair) {
		t.Fatalf("diagnostic allowed actions = %#v, want repair action", diagnosticsResp.AllowedNextActions)
	}
}

func TestToolDefinitionsCoverEveryToolWithMetadataAndSchemas(t *testing.T) {
	definitions := ToolDefinitions()
	names := ToolNames()
	if len(definitions) != len(names) {
		t.Fatalf("ToolDefinitions length = %d, want %d", len(definitions), len(names))
	}
	for i, definition := range definitions {
		if definition.Name != names[i] {
			t.Fatalf("definition[%d].Name = %q, want %q", i, definition.Name, names[i])
		}
		if strings.TrimSpace(definition.Description) == "" {
			t.Fatalf("definition %q missing description", definition.Name)
		}
		if got := definition.InputSchema["type"]; got != "object" {
			t.Fatalf("definition %q schema type = %#v, want object", definition.Name, got)
		}
		if _, ok := definition.InputSchema["properties"]; !ok {
			t.Fatalf("definition %q missing schema properties", definition.Name)
		}
	}
}

func TestReconcileLifecycleRequiresExplicitRepairMode(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(mcpProjectState("TASK-1", 1), "    id: TASK-2", "    id: \"\"", 1)
	paths := writeMCPFixtures(t, root, projectState, mcpTasks(types.StatusTODO, types.StatusTODO))
	stateBefore := mustRead(t, paths.StatePath)

	_, err := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}}.ReconcileLifecycle("")
	if err == nil || !strings.Contains(err.Error(), "unsupported reconcile mode") {
		t.Fatalf("ReconcileLifecycle without explicit repair mode err = %v, want unsupported mode", err)
	}
	if got := mustRead(t, paths.StatePath); got != stateBefore {
		t.Fatal("ReconcileLifecycle without repair mode mutated project-state.yaml")
	}
	if _, statErr := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); !os.IsNotExist(statErr) {
		t.Fatalf("ReconcileLifecycle without repair mode should not write ACTIVE_TASK.md; stat err=%v", statErr)
	}
}

func TestReconcileLifecycleRepairResponseListsChangedFilesAndFields(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(mcpProjectState("TASK-1", 1), "    id: TASK-2", "    id: \"\"", 1)
	paths := writeMCPFixtures(t, root, projectState, mcpTasks(types.StatusTODO, types.StatusTODO))
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")

	resp, err := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}}.ReconcileLifecycle("repair")
	if err != nil {
		t.Fatalf("ReconcileLifecycle: %v", err)
	}
	if !resp.Repaired || resp.ManualReview {
		t.Fatalf("unexpected reconcile response: %#v", resp)
	}
	if !responseChangedFile(resp.ChangedFiles, paths.StatePath) || !responseChangedFile(resp.ChangedFiles, briefPath) {
		t.Fatalf("ChangedFiles = %#v, want state and brief", resp.ChangedFiles)
	}
	if !responseChangedField(resp.ChangedFields, "next_task") || !responseChangedField(resp.ChangedFields, "active_brief.task_id") {
		t.Fatalf("ChangedFields = %#v, want next_task and active_brief.task_id", resp.ChangedFields)
	}
	if !strings.Contains(mustRead(t, briefPath), "**Task ID**: TASK-1") {
		t.Fatalf("brief not repaired:\n%s", mustRead(t, briefPath))
	}
}

func TestReconcileLifecycleUnsupportedDriftReturnsManualReviewWithoutMutation(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("TASK-1", 1), mcpTasks(types.StatusTODO, types.StatusTODO))
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	testutil.WriteFile(t, briefPath, "**Task ID**: TASK-2\n")
	stateBefore := mustRead(t, paths.StatePath)
	briefBefore := mustRead(t, briefPath)

	resp, err := ToolHandler{ProjectRoot: root}.ReconcileLifecycle("repair")
	if err != nil {
		t.Fatalf("ReconcileLifecycle: %v", err)
	}
	if !resp.ManualReview || resp.Repaired || len(resp.ChangedFiles) != 0 || len(resp.ChangedFields) != 0 {
		t.Fatalf("unexpected manual-review response: %#v", resp)
	}
	if got := mustRead(t, paths.StatePath); got != stateBefore {
		t.Fatal("state mutated for unsupported drift")
	}
	if got := mustRead(t, briefPath); got != briefBefore {
		t.Fatal("brief mutated for unsupported drift")
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

func TestDiagnosticsAndRepairRespectRunLockPolicy(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	beforeState := mustRead(t, paths.StatePath)
	beforeTasks := mustRead(t, paths.TasksPath)
	lock, err := runlock.TryAcquire(paths.DougDir, "test driver")
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	defer func() { _ = lock.Close() }()

	statusResp, err := ToolHandler{ProjectRoot: root}.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if statusResp.LifecyclePhase != string(lifecycle.StatusNoActiveTask) || !contains(statusResp.AllowedNextActions, ToolGetNextTask) {
		t.Fatalf("unexpected status while locked: %#v", statusResp)
	}
	diagnosticsResp, err := ToolHandler{ProjectRoot: root}.DiagnoseLifecycle()
	if err != nil {
		t.Fatalf("DiagnoseLifecycle: %v", err)
	}
	if diagnosticsResp.LifecyclePhase != string(lifecycle.StatusNoActiveTask) || !contains(diagnosticsResp.AllowedNextActions, lifecycle.DiagnosticActionGetNextTask) {
		t.Fatalf("unexpected diagnostics while locked: %#v", diagnosticsResp)
	}
	_, err = ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}}.ReconcileLifecycle("repair")
	if err == nil || !strings.Contains(err.Error(), "run lock is held") {
		t.Fatalf("ReconcileLifecycle err = %v, want lock-held error", err)
	}
	if got := mustRead(t, paths.StatePath); got != beforeState {
		t.Fatal("diagnostics/status/locked repair mutated project-state.yaml")
	}
	if got := mustRead(t, paths.TasksPath); got != beforeTasks {
		t.Fatal("diagnostics/status/locked repair mutated tasks.yaml")
	}
	if _, statErr := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); !os.IsNotExist(statErr) {
		t.Fatalf("read-only diagnostics should not claim/write ACTIVE_TASK.md; stat err=%v", statErr)
	}
}

func TestGetNextTaskWritesAndReturnsCanonicalBriefWithContextGuidance(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "PRD.md"), "PRD FULL PAYLOAD SENTINEL")
	testutil.WriteFile(t, filepath.Join(root, "docs", "kb", "README.md"), "KB FULL PAYLOAD SENTINEL")
	testutil.WriteFile(t, filepath.Join(root, "CHANGELOG.md"), "CHANGELOG FULL PAYLOAD SENTINEL")

	resp, err := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}}.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask: %v", err)
	}
	if !resp.Claimed || resp.ActiveAssignment == nil || resp.ActiveAssignment.ID != "TASK-1" {
		t.Fatalf("did not claim TASK-1: %#v", resp)
	}
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	brief := mustRead(t, briefPath)
	if resp.AssignmentBriefPath != briefPath || resp.BriefPath != briefPath {
		t.Fatalf("brief path fields = %q/%q, want %q", resp.AssignmentBriefPath, resp.BriefPath, briefPath)
	}
	if resp.Brief != brief || !strings.Contains(resp.Brief, "Build MCP status") || !strings.Contains(resp.Brief, "## Agent Result") {
		t.Fatalf("response did not include canonical brief: %q", resp.Brief)
	}
	if !strings.Contains(resp.DispatcherInstruction, ".doug/ACTIVE_TASK.md") || !strings.Contains(resp.DispatcherInstruction, "fresh worker") {
		t.Fatalf("missing concise dispatcher instruction: %q", resp.DispatcherInstruction)
	}
	if !strings.Contains(resp.DispatcherGuidance, "fresh worker context") || !strings.Contains(resp.DispatcherGuidance, "thin dispatcher") {
		t.Fatalf("missing dispatcher/worker guidance: %q", resp.DispatcherGuidance)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if len(resp.Brief) > 5000 {
		t.Fatalf("interactive brief is too large (%d bytes); context should be referenced by path", len(resp.Brief))
	}
	for _, ref := range []string{".doug/PRD.md", "docs/kb/README.md", "Build System"} {
		if !strings.Contains(resp.Brief, ref) {
			t.Fatalf("brief missing context pointer %q:\n%s", ref, resp.Brief)
		}
	}
	for _, sentinel := range []string{"PRD FULL PAYLOAD SENTINEL", "KB FULL PAYLOAD SENTINEL", "CHANGELOG FULL PAYLOAD SENTINEL"} {
		if strings.Contains(string(encoded), sentinel) || strings.Contains(resp.Brief, sentinel) {
			t.Fatalf("get_next_task inlined context payload %q: %s", sentinel, encoded)
		}
	}
}

func TestInteractiveClaimAfterCompleteWritesFreshAssignmentBrief(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("", 0), mcpTasks(types.StatusTODO, types.StatusTODO))
	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}, BuildSystem: &build.StaticBuildSystem{}}
	first, err := h.GetNextTask()
	if err != nil {
		t.Fatalf("first GetNextTask: %v", err)
	}
	if !first.Claimed || first.ActiveAssignment == nil || first.ActiveAssignment.ID != "TASK-1" {
		t.Fatalf("first claim = %#v, want TASK-1", first)
	}
	writeOutcome(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "SUCCESS")
	h.HandleSuccess = func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error) {
		_, err := lifecycle.CompleteVerifiedTask(lifecycle.Options{Paths: lifecycle.DefaultPaths(root)}, ctx.TaskID)
		return handlers.SuccessResult{Kind: handlers.Continue}, err
	}
	if _, err := h.ReportTaskComplete("TASK-1"); err != nil {
		t.Fatalf("ReportTaskComplete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); !os.IsNotExist(err) {
		t.Fatalf("completed handoff should clear stale ACTIVE_TASK.md before next claim; stat err=%v", err)
	}

	second, err := h.GetNextTask()
	if err != nil {
		t.Fatalf("second GetNextTask: %v", err)
	}
	if !second.Claimed || second.AlreadyActive || second.ActiveAssignment == nil || second.ActiveAssignment.ID != "TASK-2" {
		t.Fatalf("second claim = %#v, want fresh TASK-2", second)
	}
	brief := mustRead(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"))
	if !strings.Contains(brief, "**Task ID**: TASK-2") || strings.Contains(brief, "**Task ID**: TASK-1") {
		t.Fatalf("fresh assignment brief not written after completion:\n%s", brief)
	}
}

func TestGetStatusReconnectRediscoversInterruptedActiveAssignmentWithoutClaimingNewTask(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("TASK-1", 1), mcpTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")
	beforeState := mustRead(t, paths.StatePath)
	beforeBrief := mustRead(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"))

	resp, err := ToolHandler{ProjectRoot: root}.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus reconnect: %v", err)
	}
	if resp.LifecyclePhase != string(lifecycle.StatusActiveTask) || resp.ActiveAssignment == nil || resp.ActiveAssignment.ID != "TASK-1" {
		t.Fatalf("did not rediscover active assignment: %#v", resp)
	}
	if contains(resp.AllowedNextActions, ToolGetNextTask) || !contains(resp.AllowedNextActions, ToolReportTaskComplete) {
		t.Fatalf("reconnect allowed actions = %#v, want report actions without claiming", resp.AllowedNextActions)
	}
	if got := mustRead(t, paths.StatePath); got != beforeState {
		t.Fatal("GetStatus reconnect claimed or mutated state")
	}
	if got := mustRead(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); got != beforeBrief {
		t.Fatal("GetStatus reconnect rewrote active brief")
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
	if resp.SuccessResultKind != "continue" || !strings.Contains(resp.Message, "lifecycle advanced") {
		t.Fatalf("completion response did not distinguish normal advance: %#v", resp)
	}
	if !strings.Contains(resp.TerminalGuidance, "stop") || !strings.Contains(resp.TerminalGuidance, "renew context") {
		t.Fatalf("TerminalGuidance = %q, want stop/renew context guidance", resp.TerminalGuidance)
	}
	loaded, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loaded.ActiveTask.ID != "TASK-2" {
		t.Fatalf("active task after completion = %#v, want TASK-2", loaded.ActiveTask)
	}
}

func TestReportTaskCompleteTerminalSuccessFinalizesEpicAndArchivesRuntime(t *testing.T) {
	root := setupMCPGitRepo(t)
	paths := writeMCPFixtures(t, root, `current_epic:
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
`, `epic:
    id: EPIC-MCP
    name: MCP Epic
    tasks:
        - id: TASK-1
          type: feature
          status: TODO
          description: Finish MCP epic
          acceptance_criteria:
            - Epic finalizes
`)
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "PRD.md"), "# MCP PRD\n")
	gitAddCommit(t, root, "initial runtime")

	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}, BuildSystem: &build.StaticBuildSystem{}}
	if _, err := h.GetNextTask(); err != nil {
		t.Fatalf("GetNextTask: %v", err)
	}
	writeOutcome(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "SUCCESS")

	resp, err := h.ReportTaskComplete("TASK-1")
	if err != nil {
		t.Fatalf("ReportTaskComplete terminal: %v", err)
	}
	if resp.Outcome != "SUCCESS" || resp.SuccessResultKind != "epic_complete" || !strings.Contains(resp.Message, "epic finalized") {
		t.Fatalf("terminal completion response did not distinguish epic completion: %#v", resp)
	}
	if !resp.Completed || resp.ActiveAssignment != nil || len(resp.AllowedNextActions) != 0 {
		t.Fatalf("final status = %#v, want completed with no active assignment/actions", resp.StatusResponse)
	}
	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.ActiveTask.ID != "" || loadedState.NextTask.ID != "" {
		t.Fatalf("runtime pointers not cleared after finalization: active=%#v next=%#v", loadedState.ActiveTask, loadedState.NextTask)
	}
	loadedTasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if got := loadedTasks.Epic.Tasks[0].Status; got != types.StatusDone {
		t.Fatalf("TASK-1 status = %q, want DONE", got)
	}
	archiveDir := filepath.Join(paths.DougDir, "logs", "epics", "EPIC-MCP")
	for _, name := range []string{"PRD.md", "tasks.yaml", "project-state.yaml", "ACTIVE_TASK.md", "archived_at.txt"} {
		if _, err := os.Stat(filepath.Join(archiveDir, name)); err != nil {
			t.Fatalf("runtime snapshot missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); !os.IsNotExist(err) {
		t.Fatalf("live ACTIVE_TASK.md should be cleared after finalization; stat err=%v", err)
	}
}

func TestGetNextTaskAfterInteractiveTerminalCompletionClaimsPostEpicReview(t *testing.T) {
	root := setupMCPGitRepo(t)
	paths := writeMCPFixtures(t, root, `current_epic:
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
`, `epic:
    id: EPIC-MCP
    name: MCP Epic
    tasks:
        - id: TASK-1
          type: feature
          status: TODO
          description: Finish MCP epic
          acceptance_criteria:
            - Epic finalizes
`)
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "PRD.md"), "# MCP PRD\n")
	gitAddCommit(t, root, "initial runtime")

	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3, ReviewEnabled: true}, BuildSystem: &build.StaticBuildSystem{}}
	if _, err := h.GetNextTask(); err != nil {
		t.Fatalf("GetNextTask runtime: %v", err)
	}
	writeOutcome(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "SUCCESS")
	completeResp, err := h.ReportTaskComplete("TASK-1")
	if err != nil {
		t.Fatalf("ReportTaskComplete terminal: %v", err)
	}
	if completeResp.SuccessResultKind != "epic_complete" || completeResp.ActiveAssignment != nil {
		t.Fatalf("terminal completion did not finalize interactive epic: %#v", completeResp)
	}

	postResp, err := h.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask post-epic: %v", err)
	}
	if !postResp.Claimed || postResp.ActiveAssignment == nil || postResp.ActiveAssignment.ID != "POST_EPIC_REVIEW" {
		t.Fatalf("expected interactive finalization to allow post-epic review claim, got %#v", postResp)
	}
	if !strings.Contains(postResp.Brief, "post-epic review") {
		t.Fatalf("post-epic review brief missing review contract: %q", postResp.Brief)
	}
}

func TestGetNextTaskAfterInteractiveTerminalCompletionClaimsPostEpicKBWithoutHeadlessFinalize(t *testing.T) {
	root := setupMCPGitRepo(t)
	paths := writeMCPFixtures(t, root, `current_epic:
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
`, `epic:
    id: EPIC-MCP
    name: MCP Epic
    tasks:
        - id: TASK-1
          type: feature
          status: TODO
          description: Finish MCP epic
          acceptance_criteria:
            - Epic finalizes
`)
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "PRD.md"), "# MCP PRD\n")
	gitAddCommit(t, root, "initial runtime")

	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3, ReviewEnabled: false, KBEnabled: true}, BuildSystem: &build.StaticBuildSystem{}}
	if _, err := h.GetNextTask(); err != nil {
		t.Fatalf("GetNextTask runtime: %v", err)
	}
	writeOutcome(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "SUCCESS")
	completeResp, err := h.ReportTaskComplete("TASK-1")
	if err != nil {
		t.Fatalf("ReportTaskComplete terminal: %v", err)
	}
	if completeResp.SuccessResultKind != "epic_complete" {
		t.Fatalf("terminal completion did not use MCP finalization path: %#v", completeResp)
	}

	postResp, err := h.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask post-epic KB: %v", err)
	}
	if !postResp.Claimed || postResp.ActiveAssignment == nil || postResp.ActiveAssignment.ID != "POST_EPIC_KB" {
		t.Fatalf("expected post-epic KB claim after interactive completion without headless finalize, got %#v", postResp)
	}
	if !strings.Contains(postResp.Brief, "KB and changelog") {
		t.Fatalf("post-epic KB brief missing synthesis contract: %q", postResp.Brief)
	}
}

func TestReportTaskCompleteEpicCompleteOutcomeStillUsesSuccessResultKind(t *testing.T) {
	root := t.TempDir()
	paths := writeMCPFixtures(t, root, mcpProjectState("TASK-1", 1), mcpTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n\n## Agent Result\n---\noutcome: \"EPIC_COMPLETE\"\nchangelog_entry: \"\"\ndependencies_added: []\nbugs: []\n---\n")
	h := ToolHandler{ProjectRoot: root, Config: &config.OrchestratorConfig{BuildSystem: "static", MaxRetries: 3}, BuildSystem: &build.StaticBuildSystem{}}
	h.HandleSuccess = func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error) {
		if result.Outcome != types.OutcomeEpicComplete {
			t.Fatalf("outcome = %q, want EPIC_COMPLETE", result.Outcome)
		}
		_, err := lifecycle.CompleteVerifiedTask(lifecycle.Options{Paths: lifecycle.DefaultPaths(root)}, ctx.TaskID)
		return handlers.SuccessResult{Kind: handlers.Continue}, err
	}

	resp, err := h.ReportTaskComplete("TASK-1")
	if err != nil {
		t.Fatalf("ReportTaskComplete: %v", err)
	}
	if resp.Outcome != "EPIC_COMPLETE" || resp.SuccessResultKind != "continue" {
		t.Fatalf("response should preserve reported outcome and success kind separately: %#v", resp)
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
	if !strings.Contains(resp.TerminalGuidance, "stop") || !strings.Contains(resp.TerminalGuidance, "renew context") {
		t.Fatalf("TerminalGuidance = %q, want stop/renew context guidance", resp.TerminalGuidance)
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
	if !strings.Contains(resp.Brief, "## Agent Result") || !strings.Contains(resp.Brief, "post-epic review") {
		t.Fatalf("post-epic lifecycle brief missing contract: %q", resp.Brief)
	}
	if _, err := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); err != nil {
		t.Fatalf("ACTIVE_TASK.md not written: %v", err)
	}
}

func setupMCPGitRepo(t *testing.T) string {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test Agent")
	return root
}

func gitAddCommit(t *testing.T, root, message string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", message)
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
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

func assertActionGrammar(t *testing.T, actions []string) {
	t.Helper()
	grammar := regexp.MustCompile(`^[a-z_]+(\([a-z_]+=[a-z_]+\))?$`)
	for _, action := range actions {
		if !grammar.MatchString(action) {
			t.Fatalf("allowed_next_actions entry %q does not match backward-compatible action grammar", action)
		}
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

func responseChangedFile(files []ChangedFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func responseChangedField(fields []ChangedField, fieldName string) bool {
	for _, field := range fields {
		if field.Field == fieldName {
			return true
		}
	}
	return false
}

func responseFinding(findings []DiagnosticFinding, code, severity string, requiresManualReview bool) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.Severity == severity && finding.RequiresManualReview == requiresManualReview {
			return true
		}
	}
	return false
}
