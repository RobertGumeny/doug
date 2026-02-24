package handlers_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// EpicComplete handler helpers
// ---------------------------------------------------------------------------

func epicCtx(dir string, st *types.ProjectState) *orchestrator.LoopContext {
	return &orchestrator.LoopContext{
		TaskID:        "KB_UPDATE",
		TaskType:      types.TaskTypeDocumentation,
		Attempts:      1,
		CurrentEpic:   st.CurrentEpic,
		SessionResult: &types.SessionResult{Outcome: types.OutcomeEpicComplete},
		Config:        &config.OrchestratorConfig{MaxRetries: 5},
		BuildSystem:   &mockBuildSystem{},
		ProjectRoot:   dir,
		TaskStartTime: time.Now(),
		State:         st,
		Tasks:         makeSingleTaskDone(),
		StatePath:     filepath.Join(dir, "project-state.yaml"),
		TasksPath:     filepath.Join(dir, "tasks.yaml"),
		LogsDir:       filepath.Join(dir, "logs"),
		ChangelogPath: filepath.Join(dir, "CHANGELOG.md"),
	}
}

func makeEpicCompleteState() *types.ProjectState {
	now := "2026-02-24T23:59:00Z"
	return &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:          "EPIC-5",
			Name:        "Handlers & Main Loop",
			BranchName:  "feature/EPIC-5",
			StartedAt:   "2026-02-24T00:00:00Z",
			CompletedAt: &now,
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeDocumentation,
			ID:       "KB_UPDATE",
			Attempts: 1,
		},
		KBEnabled: true,
	}
}

// ---------------------------------------------------------------------------
// Tests: HandleEpicComplete
// ---------------------------------------------------------------------------

func TestHandleEpicComplete_Success_ReturnsNil(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeEpicCompleteState()
	ctx := epicCtx(dir, st)

	// Write a new file so there is something to commit.
	writeFile(t, filepath.Join(dir, "docs", "kb", "article.md"), "# KB Article\n")

	err := handlers.HandleEpicComplete(ctx)

	if err != nil {
		t.Errorf("expected nil error on success, got: %v", err)
	}
}

func TestHandleEpicComplete_NothingToCommit_ReturnsNil(t *testing.T) {
	// When the working tree is already clean (all changes committed by prior
	// handlers), ErrNothingToCommit must be treated as non-fatal.
	dir := setupGitRepo(t)
	st := makeEpicCompleteState()
	ctx := epicCtx(dir, st)

	// Do NOT write any new files — working tree is already clean after setupGitRepo.

	err := handlers.HandleEpicComplete(ctx)

	if err != nil {
		t.Errorf("expected nil error when nothing to commit, got: %v", err)
	}
}

func TestHandleEpicComplete_CommitFails_ReturnsError(t *testing.T) {
	// Point ProjectRoot to a non-git directory so git commit fails with a real error.
	badDir := t.TempDir()
	writeFile(t, filepath.Join(badDir, "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")

	st := makeEpicCompleteState()
	ctx := epicCtx(badDir, st)

	err := handlers.HandleEpicComplete(ctx)

	if err == nil {
		t.Fatal("expected non-nil error when git commit fails (non-git dir)")
	}
}

func TestHandleEpicComplete_CommitFails_ErrorIsNotNothingToCommit(t *testing.T) {
	// Verify that the error returned is a real commit error, not ErrNothingToCommit.
	badDir := t.TempDir()
	writeFile(t, filepath.Join(badDir, "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")

	st := makeEpicCompleteState()
	ctx := epicCtx(badDir, st)

	err := handlers.HandleEpicComplete(ctx)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if errors.Is(err, git.ErrNothingToCommit) {
		t.Error("should not return ErrNothingToCommit — expected a real commit failure")
	}
}

func TestHandleEpicComplete_CommitFails_ErrorContainsEpicID(t *testing.T) {
	badDir := t.TempDir()
	writeFile(t, filepath.Join(badDir, "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")

	st := makeEpicCompleteState()
	ctx := epicCtx(badDir, st)

	err := handlers.HandleEpicComplete(ctx)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "EPIC-5") {
		t.Errorf("error should contain epic ID %q, got: %q", "EPIC-5", err.Error())
	}
}

func TestHandleEpicComplete_MetricsTablePrinted(t *testing.T) {
	// Smoke test: verify HandleEpicComplete does not panic when printing metrics.
	dir := setupGitRepo(t)
	st := makeEpicCompleteState()
	st.Metrics.Tasks = []types.TaskMetric{
		{TaskID: "EPIC-5-001", Outcome: "success", DurationSeconds: 120, CompletedAt: "2026-02-24T00:01:00Z"},
		{TaskID: "EPIC-5-002", Outcome: "success", DurationSeconds: 90, CompletedAt: "2026-02-24T00:02:00Z"},
	}
	st.Metrics.TotalTasksCompleted = 2
	st.Metrics.TotalDurationSeconds = 210

	ctx := epicCtx(dir, st)

	// Should not panic.
	err := handlers.HandleEpicComplete(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
