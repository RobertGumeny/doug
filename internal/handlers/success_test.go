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
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Mock build system
// ---------------------------------------------------------------------------

type mockBuildSystem struct {
	installErr  error
	buildErr    error
	testErr     error
	initialized bool
}

func (m *mockBuildSystem) Install() error      { return m.installErr }
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
	writeFile(t, filepath.Join(dir, "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")
	writeFile(t, filepath.Join(dir, "tasks.yaml"), "epic:\n  id: EPIC-5\n  tasks: []\n")
	writeFile(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n## [Unreleased]\n\n### Added\n\n### Fixed\n\n### Changed\n")

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
		KBEnabled: true,
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
		KBEnabled: true,
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
	return &orchestrator.LoopContext{
		TaskID:        "EPIC-5-001",
		TaskType:      types.TaskTypeFeature,
		Attempts:      1,
		CurrentEpic:   st.CurrentEpic,
		SessionResult: &types.SessionResult{Outcome: types.OutcomeSuccess},
		Config:        &config.OrchestratorConfig{MaxRetries: 5},
		BuildSystem:   bs,
		ProjectRoot:   dir,
		TaskStartTime: time.Now(),
		State:         st,
		Tasks:         ts,
		StatePath:     filepath.Join(dir, "project-state.yaml"),
		TasksPath:     filepath.Join(dir, "tasks.yaml"),
		LogsDir:       filepath.Join(dir, "logs"),
		ChangelogPath: filepath.Join(dir, "CHANGELOG.md"),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandleSuccess_BuildFails_ReturnsRetry(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{buildErr: fmt.Errorf("compilation error")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)

	result, err := handlers.HandleSuccess(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Retry {
		t.Errorf("expected Retry, got %v", result.Kind)
	}
}

func TestHandleSuccess_TestsFail_ReturnsRetry(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{testErr: fmt.Errorf("test failure: TestFoo")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)

	result, err := handlers.HandleSuccess(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Retry {
		t.Errorf("expected Retry, got %v", result.Kind)
	}
}

func TestHandleSuccess_DepsInstallFails_ReturnsRetry(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{installErr: fmt.Errorf("go mod download: network error")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	ctx.SessionResult = &types.SessionResult{
		Outcome:           types.OutcomeSuccess,
		DependenciesAdded: []string{"github.com/some/dep"},
	}

	result, err := handlers.HandleSuccess(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Retry {
		t.Errorf("expected Retry, got %v", result.Kind)
	}
}

func TestHandleSuccess_FeatureTask_MoreTasksRemain_ReturnsContinue(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{}
	st := makeFeatureState()
	// Two tasks: first IN_PROGRESS, second TODO — KB not needed yet
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	ctx.SessionResult = &types.SessionResult{
		Outcome:        types.OutcomeSuccess,
		ChangelogEntry: "Added LoopContext and HandleSuccess",
	}

	result, err := handlers.HandleSuccess(ctx)

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

	result, err := handlers.HandleSuccess(ctx)

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
	st.KBEnabled = false
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

	result, err := handlers.HandleSuccess(ctx)

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
	ctx.SessionResult = &types.SessionResult{
		Outcome:        types.OutcomeSuccess,
		ChangelogEntry: "Synthesized knowledge base",
	}

	result, err := handlers.HandleSuccess(ctx)

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
	writeFile(t, filepath.Join(badDir, "tasks.yaml"), "epic:\n  id: EPIC-5\n  tasks: []\n")
	writeFile(t, filepath.Join(badDir, "CHANGELOG.md"), "# Changelog\n\n## [Unreleased]\n\n### Added\n\n### Fixed\n\n### Changed\n")

	ctx.ProjectRoot = badDir
	ctx.StatePath = filepath.Join(badDir, "project-state.yaml")
	ctx.TasksPath = filepath.Join(badDir, "tasks.yaml")
	ctx.ChangelogPath = filepath.Join(badDir, "CHANGELOG.md")

	// badDir is not a git repo, so git commit will fail
	result, err := handlers.HandleSuccess(ctx)

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

	result, err := handlers.HandleSuccess(ctx)

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
	if last.Outcome != "success" {
		t.Errorf("metric outcome: got %q, want %q", last.Outcome, "success")
	}
}

func TestHandleSuccess_BuildFails_RollbackError_ReturnsRetryWithError(t *testing.T) {
	// When rollback itself fails, HandleSuccess returns (Retry, non-nil error).
	// We simulate this by making the ProjectRoot a non-git dir so rollback fails.
	badDir := t.TempDir()
	bs := &mockBuildSystem{buildErr: errors.New("build broken")}
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)

	ctx := &orchestrator.LoopContext{
		TaskID:        "EPIC-5-001",
		TaskType:      types.TaskTypeFeature,
		Attempts:      1,
		CurrentEpic:   st.CurrentEpic,
		SessionResult: &types.SessionResult{Outcome: types.OutcomeSuccess},
		Config:        &config.OrchestratorConfig{MaxRetries: 5},
		BuildSystem:   bs,
		ProjectRoot:   badDir, // not a git repo → rollback will fail
		TaskStartTime: time.Now(),
		State:         st,
		Tasks:         ts,
		StatePath:     filepath.Join(badDir, "project-state.yaml"),
		TasksPath:     filepath.Join(badDir, "tasks.yaml"),
		LogsDir:       filepath.Join(badDir, "logs"),
		ChangelogPath: filepath.Join(badDir, "CHANGELOG.md"),
	}

	result, err := handlers.HandleSuccess(ctx)

	// Should still return Retry (with a non-nil error describing the rollback failure)
	if result.Kind != handlers.Retry {
		t.Errorf("expected Retry, got %v", result.Kind)
	}
	if err == nil {
		t.Error("expected non-nil error when rollback fails, got nil")
	}
}
