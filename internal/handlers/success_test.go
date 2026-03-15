package handlers_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Mock build system
// ---------------------------------------------------------------------------

type mockBuildSystem struct {
	installErr   error
	buildErr     error
	testErr      error
	initialized  bool
	installCalls int
}

func (m *mockBuildSystem) Install() error {
	m.installCalls++
	if m.installErr != nil {
		return m.installErr
	}
	m.initialized = true
	return nil
}
func (m *mockBuildSystem) Build() error        { return m.buildErr }
func (m *mockBuildSystem) Test() error         { return m.testErr }
func (m *mockBuildSystem) IsInitialized() bool { return m.initialized }

// ---------------------------------------------------------------------------
// Git repo helper
// ---------------------------------------------------------------------------

// setupGitRepo initialises a minimal git repository in a temp directory with
// one initial commit, so that git reset --hard HEAD and git commit work.
func setupGitRepo(t *testing.T) string {
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

	// Write initial tracked files so that reset --hard HEAD has a clean base.
	writeFile(t, filepath.Join(dir, ".doug", "tasks.yaml"), "epic:\n  id: EPIC-5\n  tasks: []\n")
	writeFile(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n## [Unreleased]\n\n### Added\n\n### Fixed\n\n### Changed\n")

	// Create .doug/ directory for orchestrator state (untracked — not committed).
	if err := os.MkdirAll(filepath.Join(dir, ".doug"), 0o755); err != nil {
		t.Fatalf("mkdirall .doug: %v", err)
	}
	writeFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")

	runGit("add", "-A")
	runGit("commit", "-m", "initial")

	return dir
}

// writeFile is a test helper that writes content to path, creating parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdirall %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// State / tasks helpers
// ---------------------------------------------------------------------------

func makeFeatureState() *types.ProjectState {
	return &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:         "EPIC-5",
			Name:       "Handlers",
			BranchName: "feature/EPIC-5",
			StartedAt:  "2026-02-24T00:00:00Z",
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeFeature,
			ID:       "EPIC-5-001",
			Attempts: 1,
		},
		NextTask: types.TaskPointer{
			Type: types.TaskTypeFeature,
			ID:   "EPIC-5-002",
		},
	}
}

func makeDocsState() *types.ProjectState {
	return &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:         "EPIC-5",
			Name:       "Handlers",
			BranchName: "feature/EPIC-5",
			StartedAt:  "2026-02-24T00:00:00Z",
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeDocumentation,
			ID:       "KB_UPDATE",
			Attempts: 1,
		},
	}
}

func makeTwoTaskTasks(firstStatus, secondStatus types.Status) *types.Tasks {
	return &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-5",
			Name: "Handlers",
			Tasks: []types.Task{
				{ID: "EPIC-5-001", Type: types.TaskTypeFeature, Status: firstStatus, UserDefined: true},
				{ID: "EPIC-5-002", Type: types.TaskTypeFeature, Status: secondStatus, UserDefined: true},
			},
		},
	}
}

func makeSingleTaskDone() *types.Tasks {
	return &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-5",
			Name: "Handlers",
			Tasks: []types.Task{
				{ID: "EPIC-5-001", Type: types.TaskTypeFeature, Status: types.StatusDone, UserDefined: true},
			},
		},
	}
}

func baseCtx(dir string, bs *mockBuildSystem, st *types.ProjectState, ts *types.Tasks) *orchestrator.LoopContext {
	dougDir := filepath.Join(dir, ".doug")
	return &orchestrator.LoopContext{
		TaskID:        "EPIC-5-001",
		TaskType:      types.TaskTypeFeature,
		Attempts:      1,
		CurrentEpic:   st.CurrentEpic,
		Config:        &config.OrchestratorConfig{MaxRetries: 5, KBEnabled: true},
		BuildSystem:   bs,
		ProjectRoot:   dir,
		TaskStartTime: time.Now(),
		State:         st,
		Tasks:         ts,
		StatePath:     filepath.Join(dougDir, "project-state.yaml"),
		TasksPath:     filepath.Join(dougDir, "tasks.yaml"),
		DougDir:       dougDir,
		LogsDir:       filepath.Join(dougDir, "logs"),
		ChangelogPath: filepath.Join(dir, "CHANGELOG.md"),
		Logger:        log.Discard(),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandleSuccess_BuildFails_ReturnsBuildFailure(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{buildErr: fmt.Errorf("compilation error")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	initialAttempts := ctx.Attempts // 1
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.BuildFailure {
		t.Errorf("expected BuildFailure, got %v", result.Kind)
	}
	// Project must be PAUSED.
	if st.Status != types.ProjectStatusPaused {
		t.Errorf("expected project status PAUSED, got %q", st.Status)
	}
	// Attempt counter must be decremented (not consumed on BUILD_FAILURE).
	if st.ActiveTask.Attempts != initialAttempts-1 {
		t.Errorf("expected attempts %d after build failure, got %d", initialAttempts-1, st.ActiveTask.Attempts)
	}
}

func TestHandleSuccess_TestsFail_FirstTime_ReturnsRetry(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{testErr: fmt.Errorf("test failure: TestFoo")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	initialAttempts := ctx.State.ActiveTask.Attempts // 1
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First test failure must return Retry, not BuildFailure.
	if result.Kind != handlers.Retry {
		t.Errorf("expected Retry on first test failure, got %v", result.Kind)
	}
	// Project must NOT be paused.
	if st.Status == types.ProjectStatusPaused {
		t.Errorf("expected project not paused on first test failure, got PAUSED")
	}
	// Retry counter must NOT be decremented (increments normally).
	if st.ActiveTask.Attempts != initialAttempts {
		t.Errorf("expected attempts %d unchanged after first test failure, got %d", initialAttempts, st.ActiveTask.Attempts)
	}
	// Consecutive counter must be incremented.
	if st.ActiveTask.ConsecutiveTestFailures != 1 {
		t.Errorf("expected ConsecutiveTestFailures=1, got %d", st.ActiveTask.ConsecutiveTestFailures)
	}
	// Test failure output must be stored.
	if st.ActiveTask.TestFailureOutput == "" {
		t.Error("expected TestFailureOutput to be set after first test failure")
	}
}

func TestHandleSuccess_TestsFail_SecondConsecutive_ReturnsBuildFailure(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{testErr: fmt.Errorf("test failure: TestFoo")}
	st := makeFeatureState()
	// Simulate that a previous test failure already occurred.
	st.ActiveTask.ConsecutiveTestFailures = 1
	st.ActiveTask.TestFailureOutput = "previous failure output"
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Second consecutive failure must pause the project.
	if result.Kind != handlers.BuildFailure {
		t.Errorf("expected BuildFailure on second consecutive test failure, got %v", result.Kind)
	}
	if st.Status != types.ProjectStatusPaused {
		t.Errorf("expected project status PAUSED, got %q", st.Status)
	}
}

func TestHandleSuccess_DepsInstallFails_ReturnsBuildFailure(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{installErr: fmt.Errorf("go mod download: network error")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{
		Outcome:           types.OutcomeSuccess,
		DependenciesAdded: []string{"github.com/some/dep"},
	}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.BuildFailure {
		t.Errorf("expected BuildFailure, got %v", result.Kind)
	}
	if st.Status != types.ProjectStatusPaused {
		t.Errorf("expected project status PAUSED, got %q", st.Status)
	}
}

func TestHandleSuccess_UninitializedBuildSystem_InstallsBeforeVerification(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{initialized: false}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}
	if bs.installCalls != 1 {
		t.Errorf("expected install to run once for uninitialized build system, got %d", bs.installCalls)
	}
}

func TestHandleSuccess_UninitializedBuildSystem_InstallFails_ReturnsBuildFailure(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{
		initialized: false,
		installErr:  fmt.Errorf("pnpm install: failed"),
	}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.BuildFailure {
		t.Errorf("expected BuildFailure, got %v", result.Kind)
	}
	if bs.installCalls != 1 {
		t.Errorf("expected install to run once for uninitialized build system, got %d", bs.installCalls)
	}
	if st.Status != types.ProjectStatusPaused {
		t.Errorf("expected project status PAUSED, got %q", st.Status)
	}
}

func TestHandleSuccess_FeatureTask_MoreTasksRemain_ReturnsContinue(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	// Two tasks: first IN_PROGRESS, second TODO — KB not needed yet
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{
		Outcome:        types.OutcomeSuccess,
		ChangelogEntry: "Added LoopContext and HandleSuccess",
	}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}

	// Task should now be DONE
	found := false
	for _, task := range ts.Epic.Tasks {
		if task.ID == "EPIC-5-001" {
			found = true
			if task.Status != types.StatusDone {
				t.Errorf("task status: got %q, want %q", task.Status, types.StatusDone)
			}
		}
	}
	if !found {
		t.Error("task EPIC-5-001 not found in tasks list")
	}

	// State should have advanced to the next task
	if st.ActiveTask.ID != "EPIC-5-002" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "EPIC-5-002")
	}
}

func TestHandleSuccess_LastFeatureTask_KBEnabled_InjectsKBUpdate(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	// Single task (already DONE after HandleSuccess marks it), kb_enabled=true
	ts := &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-5",
			Name: "Handlers",
			Tasks: []types.Task{
				{ID: "EPIC-5-001", Type: types.TaskTypeFeature, Status: types.StatusInProgress, UserDefined: true},
			},
		},
	}
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}

	// Active task should now be KB_UPDATE documentation task
	if st.ActiveTask.ID != "KB_UPDATE" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "KB_UPDATE")
	}
	if st.ActiveTask.Type != types.TaskTypeDocumentation {
		t.Errorf("ActiveTask.Type: got %q, want %q", st.ActiveTask.Type, types.TaskTypeDocumentation)
	}
	// NextTask should be empty
	if st.NextTask.ID != "" {
		t.Errorf("NextTask.ID should be empty after KB injection, got %q", st.NextTask.ID)
	}
}

func TestHandleSuccess_LastFeatureTask_KBDisabled_ReturnsContinue(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	ts := &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-5",
			Name: "Handlers",
			Tasks: []types.Task{
				{ID: "EPIC-5-001", Type: types.TaskTypeFeature, Status: types.StatusInProgress, UserDefined: true},
			},
		},
	}
	ctx := baseCtx(dir, bs, st, ts)
	ctx.Config.KBEnabled = false
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}

	// When KB disabled, active task should NOT be KB_UPDATE
	if st.ActiveTask.ID == "KB_UPDATE" {
		t.Error("KB_UPDATE should not be injected when kb_enabled=false")
	}
}

func TestHandleSuccess_DocumentationTask_ReturnsEpicComplete(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeDocsState()
	// All feature tasks done — documentation task is synthetic, no tasks.yaml entry
	ts := makeSingleTaskDone()
	ctx := baseCtx(dir, bs, st, ts)
	ctx.TaskID = "KB_UPDATE"
	ctx.TaskType = types.TaskTypeDocumentation
	ctx.CurrentEpic = st.CurrentEpic
	agentResult := &types.SessionResult{
		Outcome:        types.OutcomeSuccess,
		ChangelogEntry: "Synthesized knowledge base",
	}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.EpicComplete {
		t.Errorf("expected EpicComplete, got %v", result.Kind)
	}

	// completed_at should be set
	if st.CurrentEpic.CompletedAt == nil {
		t.Error("CurrentEpic.CompletedAt should be set after docs task success")
	}
	if *st.CurrentEpic.CompletedAt == "" {
		t.Error("CurrentEpic.CompletedAt should not be empty")
	}
}

func TestHandleSuccess_CommitFails_ReturnsRetry(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)

	// Make the project root point to a non-git dir to simulate commit failure.
	// We copy ctx but override ProjectRoot to a plain directory.
	badDir := t.TempDir()
	// Write state and tasks files to badDir so SaveProjectState/SaveTasks succeed.
	writeFile(t, filepath.Join(badDir, "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")
	writeFile(t, filepath.Join(badDir, ".doug", "tasks.yaml"), "epic:\n  id: EPIC-5\n  tasks: []\n")
	writeFile(t, filepath.Join(badDir, "CHANGELOG.md"), "# Changelog\n\n## [Unreleased]\n\n### Added\n\n### Fixed\n\n### Changed\n")

	ctx.ProjectRoot = badDir
	ctx.StatePath = filepath.Join(badDir, "project-state.yaml")
	ctx.TasksPath = filepath.Join(badDir, ".doug", "tasks.yaml")
	ctx.ChangelogPath = filepath.Join(badDir, "CHANGELOG.md")

	// badDir is not a git repo, so git commit will fail
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}
	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Retry {
		t.Errorf("expected Retry on git commit failure, got %v", result.Kind)
	}
}

func TestHandleSuccess_MetricsRecorded(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	initialMetricsCount := len(st.Metrics.Tasks)
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}
	if len(st.Metrics.Tasks) != initialMetricsCount+1 {
		t.Errorf("metrics: got %d tasks, want %d", len(st.Metrics.Tasks), initialMetricsCount+1)
	}
	last := st.Metrics.Tasks[len(st.Metrics.Tasks)-1]
	if last.TaskID != "EPIC-5-001" {
		t.Errorf("metric task_id: got %q, want %q", last.TaskID, "EPIC-5-001")
	}
	if last.Outcome != "SUCCESS" {
		t.Errorf("metric outcome: got %q, want %q", last.Outcome, "SUCCESS")
	}
}

func TestHandleSuccess_CommitSHACaptured(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}
	if len(st.Metrics.Tasks) == 0 {
		t.Fatal("expected at least one metric entry")
	}
	last := st.Metrics.Tasks[len(st.Metrics.Tasks)-1]
	if last.CommitSHA == "" {
		t.Error("expected CommitSHA to be set on last metric entry, got empty string")
	}
	if len(last.CommitSHA) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len=%d)", last.CommitSHA, len(last.CommitSHA))
	}
}

func TestHandleSuccess_TestsPass_AfterPreviousFailure_ResetsCounts(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{} // tests pass this time
	st := makeFeatureState()
	// Simulate a prior test failure that was retried.
	st.ActiveTask.ConsecutiveTestFailures = 1
	st.ActiveTask.TestFailureOutput = "previous failure output"
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue when tests pass, got %v", result.Kind)
	}
	// Consecutive counter and output must be cleared.
	if st.ActiveTask.ConsecutiveTestFailures != 0 {
		t.Errorf("expected ConsecutiveTestFailures=0 after tests pass, got %d", st.ActiveTask.ConsecutiveTestFailures)
	}
	if st.ActiveTask.TestFailureOutput != "" {
		t.Errorf("expected TestFailureOutput cleared after tests pass, got %q", st.ActiveTask.TestFailureOutput)
	}
}

func TestHandleSuccess_BuildFails_StateSaveFails_ReturnsBuildFailureWithError(t *testing.T) {
	// When the state save after a build failure fails, HandleSuccess returns
	// (BuildFailure, non-nil error). We simulate by pointing StatePath to a
	// directory that does not exist so the atomic write fails.
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{buildErr: errors.New("build broken")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	// Point StatePath to a non-existent directory so SaveProjectState fails.
	ctx.StatePath = filepath.Join(dir, "nonexistent", "project-state.yaml")
	agentResult := &types.SessionResult{Outcome: types.OutcomeSuccess}

	result, err := handlers.HandleSuccess(ctx, agentResult, 0)

	if result.Kind != handlers.BuildFailure {
		t.Errorf("expected BuildFailure, got %v", result.Kind)
	}
	if err == nil {
		t.Error("expected non-nil error when state save fails, got nil")
	}
}
