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
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// EpicComplete handler helpers
// ---------------------------------------------------------------------------

func epicCtx(dir string, st *types.ProjectState) *orchestrator.LoopContext {
	dougDir := filepath.Join(dir, ".doug")
	return &orchestrator.LoopContext{
		TaskID:        "KB_UPDATE",
		TaskType:      types.TaskTypeDocumentation,
		Attempts:      1,
		CurrentEpic:   st.CurrentEpic,
		Config:        &config.OrchestratorConfig{MaxRetries: 5},
		BuildSystem:   &mockBuildSystem{},
		ProjectRoot:   dir,
		TaskStartTime: time.Now(),
		State:         st,
		Tasks:         makeSingleTaskDone(),
		StatePath:     filepath.Join(dougDir, "project-state.yaml"),
		TasksPath:     filepath.Join(dougDir, "tasks.yaml"),
		DougDir:       dougDir,
		LogsDir:       filepath.Join(dougDir, "logs"),
		ChangelogPath: filepath.Join(dir, "CHANGELOG.md"),
		Logger:        log.Discard(),
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
	}
}

// ---------------------------------------------------------------------------
// Tests: HandleEpicComplete
// ---------------------------------------------------------------------------

func TestHandleEpicComplete_ReturnsNilOnSuccess(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{
			name: "with new file to commit",
			setup: func(t *testing.T, dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "docs", "kb", "article.md"), "# KB Article\n")
			},
		},
		{
			// When the working tree is already clean (all changes committed by prior
			// handlers), ErrNothingToCommit must be treated as non-fatal.
			name:  "nothing to commit",
			setup: func(t *testing.T, dir string) {},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeEpicCompleteState()
			ctx := epicCtx(dir, st)
			tc.setup(t, dir)

			err := handlers.HandleEpicComplete(ctx)

			if err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestHandleEpicComplete_CommitFails(t *testing.T) {
	// Point ProjectRoot to a non-git directory so git commit fails with a real error.
	tests := []struct {
		name  string
		check func(t *testing.T, err error)
	}{
		{
			name: "returns non-nil error",
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("expected non-nil error when git commit fails (non-git dir)")
				}
			},
		},
		{
			name: "error is not ErrNothingToCommit",
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("expected non-nil error")
				}
				if errors.Is(err, git.ErrNothingToCommit) {
					t.Error("should not return ErrNothingToCommit — expected a real commit failure")
				}
			},
		},
		{
			name: "error contains epic ID",
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("expected non-nil error")
				}
				if !strings.Contains(err.Error(), "EPIC-5") {
					t.Errorf("error should contain epic ID %q, got: %q", "EPIC-5", err.Error())
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			badDir := t.TempDir()
			testutil.WriteFile(t, filepath.Join(badDir, "project-state.yaml"), "current_epic:\n  id: EPIC-5\n")

			st := makeEpicCompleteState()
			ctx := epicCtx(badDir, st)

			err := handlers.HandleEpicComplete(ctx)

			tc.check(t, err)
		})
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

func TestHandleEpicComplete_SetsCompletedAtWhenMissing(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeEpicCompleteState()
	st.CurrentEpic.CompletedAt = nil
	ctx := epicCtx(dir, st)

	// Write a new file so final commit can succeed.
	testutil.WriteFile(t, filepath.Join(dir, "docs", "kb", "rollover.md"), "# rollover\n")

	err := handlers.HandleEpicComplete(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.CurrentEpic.CompletedAt == nil || *st.CurrentEpic.CompletedAt == "" {
		t.Fatal("expected completed_at to be populated when missing")
	}
}
