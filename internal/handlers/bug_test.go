package handlers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Bug handler helpers
// ---------------------------------------------------------------------------

// bugCtx builds a LoopContext suitable for HandleBug tests.
func bugCtx(dir string, taskID string, taskType types.TaskType, st *types.ProjectState, ts *types.Tasks) *orchestrator.LoopContext {
	return &orchestrator.LoopContext{
		TaskID:        taskID,
		TaskType:      taskType,
		Attempts:      1,
		CurrentEpic:   st.CurrentEpic,
		SessionResult: &types.SessionResult{Outcome: types.OutcomeBug},
		Config:        &config.OrchestratorConfig{MaxRetries: 5},
		BuildSystem:   &mockBuildSystem{},
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

// makeBugfixState returns a ProjectState with a bugfix active task.
func makeBugfixState() *types.ProjectState {
	return &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:        "EPIC-5",
			StartedAt: "2026-02-24T00:00:00Z",
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeBugfix,
			ID:       "BUG-EPIC-5-001",
			Attempts: 1,
		},
		KBEnabled: true,
	}
}

// ---------------------------------------------------------------------------
// Tests: nested bug detection
// ---------------------------------------------------------------------------

func TestHandleBug_NestedBug_ReturnsFatalError(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeBugfixState()
	ts := makeInProgressTasks("BUG-EPIC-5-001")

	ctx := bugCtx(dir, "BUG-EPIC-5-001", types.TaskTypeBugfix, st, ts)

	err := handlers.HandleBug(ctx)

	if err == nil {
		t.Fatal("expected non-nil error for nested bug, got nil")
	}
}

func TestHandleBug_NestedBug_ErrorContainsDiagnosticInfo(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeBugfixState()
	ts := makeInProgressTasks("BUG-EPIC-5-002")

	ctx := bugCtx(dir, "BUG-EPIC-5-002", types.TaskTypeBugfix, st, ts)

	err := handlers.HandleBug(ctx)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BUG-EPIC-5-002") {
		t.Errorf("error should contain task ID %q, got: %q", "BUG-EPIC-5-002", msg)
	}
	if !strings.Contains(msg, "nested") {
		t.Errorf("error should contain %q, got: %q", "nested", msg)
	}
}

// ---------------------------------------------------------------------------
// Tests: bug ID generation and state mutation
// ---------------------------------------------------------------------------

func TestHandleBug_BugID_IsPrefixedWithBUG(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.ActiveTask.ID != "BUG-EPIC-5-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "BUG-EPIC-5-001")
	}
}

func TestHandleBug_ActiveTask_TypeIsBugfix(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	_ = handlers.HandleBug(ctx)

	if st.ActiveTask.Type != types.TaskTypeBugfix {
		t.Errorf("ActiveTask.Type: got %q, want %q", st.ActiveTask.Type, types.TaskTypeBugfix)
	}
}

func TestHandleBug_NextTask_IsInterruptedTask(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-003")

	ctx := bugCtx(dir, "EPIC-5-003", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.NextTask.ID != "EPIC-5-003" {
		t.Errorf("NextTask.ID: got %q, want %q", st.NextTask.ID, "EPIC-5-003")
	}
}

func TestHandleBug_UserDefinedTask_NextTaskTypeFromTasksYAML(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	// tasks.yaml has EPIC-5-003 with type "feature"
	ts := makeInProgressTasks("EPIC-5-003")

	ctx := bugCtx(dir, "EPIC-5-003", types.TaskTypeFeature, st, ts)

	_ = handlers.HandleBug(ctx)

	if st.NextTask.Type != types.TaskTypeFeature {
		t.Errorf("NextTask.Type: got %q, want %q", st.NextTask.Type, types.TaskTypeFeature)
	}
}

func TestHandleBug_SyntheticTask_NextTaskTypeFromCtx(t *testing.T) {
	// CI-5 fix: synthetic tasks (documentation) are not in tasks.yaml;
	// their type must be preserved from ctx.TaskType directly.
	dir := setupGitRepo(t)
	st := makeDocsState()
	ts := makeSingleTaskDone()

	ctx := bugCtx(dir, "KB_UPDATE", types.TaskTypeDocumentation, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.NextTask.Type != types.TaskTypeDocumentation {
		t.Errorf("NextTask.Type: got %q, want %q", st.NextTask.Type, types.TaskTypeDocumentation)
	}
	if st.NextTask.ID != "KB_UPDATE" {
		t.Errorf("NextTask.ID: got %q, want %q", st.NextTask.ID, "KB_UPDATE")
	}
}

// ---------------------------------------------------------------------------
// Tests: ACTIVE_BUG.md archive
// ---------------------------------------------------------------------------

func TestHandleBug_MissingActiveBug_BugStillScheduled(t *testing.T) {
	// If ACTIVE_BUG.md is absent, the bug must still be scheduled (archive is skipped).
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	// logs/ACTIVE_BUG.md is NOT created

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Fatalf("expected nil error when ACTIVE_BUG.md is missing, got: %v", err)
	}
	if st.ActiveTask.Type != types.TaskTypeBugfix {
		t.Errorf("ActiveTask.Type: got %q, want bugfix", st.ActiveTask.Type)
	}
	if st.ActiveTask.ID != "BUG-EPIC-5-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "BUG-EPIC-5-001")
	}
}

func TestHandleBug_MissingActiveBug_ArchiveDirNotCreated(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	_ = handlers.HandleBug(ctx)

	archiveDir := filepath.Join(dir, "logs", "bugs", "EPIC-5")
	if _, statErr := os.Stat(archiveDir); statErr == nil {
		t.Error("archive directory should not be created when ACTIVE_BUG.md is missing")
	}
}

func TestHandleBug_ArchivesBugReportToCorrectPath(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-003")

	logsDir := filepath.Join(dir, "logs")
	writeFile(t, filepath.Join(logsDir, "ACTIVE_BUG.md"), "# Bug\n\nDetailed bug report content.")

	ctx := bugCtx(dir, "EPIC-5-003", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// logs/bugs/{epic}/bug-{taskID}.md (taskID, not bugID, in the filename)
	expectedArchive := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-003.md")
	data, readErr := os.ReadFile(expectedArchive)
	if readErr != nil {
		t.Fatalf("archived file not found at %s: %v", expectedArchive, readErr)
	}
	if !strings.Contains(string(data), "Detailed bug report content.") {
		t.Errorf("archived content does not match source: %q", string(data))
	}
}

func TestHandleBug_ArchiveReadsFromFlatPath_NotSubdirectory(t *testing.T) {
	// CI-1 fix: must read from logs/ACTIVE_BUG.md, NOT logs/bugs/ACTIVE_BUG.md.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	logsDir := filepath.Join(dir, "logs")
	// Write to the FLAT path (correct location).
	writeFile(t, filepath.Join(logsDir, "ACTIVE_BUG.md"), "# Bug\n\nCorrect flat path.")
	// Do NOT write to logs/bugs/ACTIVE_BUG.md.

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	archivePath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	data, readErr := os.ReadFile(archivePath)
	if readErr != nil {
		t.Fatalf("expected archive at %s: %v", archivePath, readErr)
	}
	if !strings.Contains(string(data), "Correct flat path.") {
		t.Errorf("unexpected archive content: %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// Tests: metrics
// ---------------------------------------------------------------------------

func TestHandleBug_MetricsRecorded(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	initialCount := len(st.Metrics.Tasks)

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	_ = handlers.HandleBug(ctx)

	if len(st.Metrics.Tasks) != initialCount+1 {
		t.Errorf("metrics: got %d tasks, want %d", len(st.Metrics.Tasks), initialCount+1)
	}
	last := st.Metrics.Tasks[len(st.Metrics.Tasks)-1]
	if last.TaskID != "EPIC-5-001" {
		t.Errorf("metric task_id: got %q, want %q", last.TaskID, "EPIC-5-001")
	}
	if last.Outcome != "bug" {
		t.Errorf("metric outcome: got %q, want %q", last.Outcome, "bug")
	}
}

// ---------------------------------------------------------------------------
// Tests: returns nil (non-fatal) for normal bug scheduling
// ---------------------------------------------------------------------------

func TestHandleBug_FeatureTask_ReturnsNil(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-002")

	ctx := bugCtx(dir, "EPIC-5-002", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx)

	if err != nil {
		t.Errorf("expected nil error for normal bug scheduling, got: %v", err)
	}
}
