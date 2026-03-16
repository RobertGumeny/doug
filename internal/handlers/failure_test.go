package handlers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Failure handler helpers
// ---------------------------------------------------------------------------

// failureCtx builds a LoopContext suitable for HandleFailure tests.
// attempts is set on the context directly (not via state) so tests control
// the retry-vs-block branch explicitly.
func failureCtx(dir string, attempts int, taskID string, taskType types.TaskType, st *types.ProjectState, ts *types.Tasks) *orchestrator.LoopContext {
	dougDir := filepath.Join(dir, ".doug")
	return &orchestrator.LoopContext{
		TaskID:        taskID,
		TaskType:      taskType,
		Attempts:      attempts,
		CurrentEpic:   st.CurrentEpic,
		Config:        &config.OrchestratorConfig{MaxRetries: 5},
		BuildSystem:   &mockBuildSystem{},
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

// makeInProgressTasks returns a single-task Tasks with IN_PROGRESS status.
func makeInProgressTasks(taskID string) *types.Tasks {
	return &types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-5",
			Name: "Handlers",
			Tasks: []types.Task{
				{ID: taskID, Type: types.TaskTypeFeature, Status: types.StatusInProgress, UserDefined: true},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandleFailure_RetryBoundary(t *testing.T) {
	tests := []struct {
		name      string
		attempts  int
		wantError bool
	}{
		{"below max retries returns nil", 2, false},
		{"at max retries returns error", 5, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeFeatureState()
			ts := makeInProgressTasks("EPIC-5-001")

			// attempts=2 with MaxRetries=5 → below limit; attempts=5 → at limit
			ctx := failureCtx(dir, tc.attempts, "EPIC-5-001", types.TaskTypeFeature, st, ts)

			err := handlers.HandleFailure(ctx, 0)

			if tc.wantError && err == nil {
				t.Error("expected non-nil error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestHandleFailure_AtMaxRetries_ErrorContainsTaskIDAndCount(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-002")

	ctx := failureCtx(dir, 5, "EPIC-5-002", types.TaskTypeFeature, st, ts)

	err := handlers.HandleFailure(ctx, 0)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "EPIC-5-002") {
		t.Errorf("error message should contain task ID %q, got: %q", "EPIC-5-002", msg)
	}
	if !strings.Contains(msg, "5") {
		t.Errorf("error message should contain retry count 5, got: %q", msg)
	}
}

func TestHandleFailure_AtMaxRetries_MissingActiveFail_ArchiveSkippedNonFatal(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	// .doug/ACTIVE_FAILURE.md does not exist
	ctx := failureCtx(dir, 5, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	// Should not panic or return an error solely because the archive file is missing
	err := handlers.HandleFailure(ctx, 0)

	// Still returns an error (max retries reached), but the cause is the retry limit
	// not the missing archive file
	if err == nil {
		t.Fatal("expected non-nil error at max_retries")
	}
	// Archive destination should NOT be created
	archiveDir := filepath.Join(dir, ".doug", "logs", "failures", "EPIC-5")
	if _, statErr := os.Stat(archiveDir); statErr == nil {
		t.Error("archive directory should not be created when ACTIVE_FAILURE.md is missing")
	}
}

func TestHandleFailure_AtMaxRetries_ArchivesReportToCorrectPath(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-003")

	// Write a failure report to .doug/ACTIVE_FAILURE.md
	dougDir := filepath.Join(dir, ".doug")
	testutil.WriteFile(t, filepath.Join(dougDir, "ACTIVE_FAILURE.md"), "# Failure\n\nDetailed failure report.")

	ctx := failureCtx(dir, 5, "EPIC-5-003", types.TaskTypeFeature, st, ts)

	err := handlers.HandleFailure(ctx, 0)

	if err == nil {
		t.Fatal("expected non-nil error at max_retries")
	}

	// Check archive destination: .doug/logs/failures/{epic}/failure-{taskID}.md
	expectedArchive := filepath.Join(dougDir, "logs", "failures", "EPIC-5", "failure-EPIC-5-003.md")
	data, readErr := os.ReadFile(expectedArchive)
	if readErr != nil {
		t.Fatalf("archived file not found at %s: %v", expectedArchive, readErr)
	}
	if !strings.Contains(string(data), "Detailed failure report.") {
		t.Errorf("archived content does not match source: %q", string(data))
	}
}

func TestHandleFailure_AtOrAboveMaxRetries_Blocks(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
	}{
		{"at max retries", 5},
		{"above max retries", 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeFeatureState()
			ts := makeInProgressTasks("EPIC-5-001")

			ctx := failureCtx(dir, tc.attempts, "EPIC-5-001", types.TaskTypeFeature, st, ts)

			err := handlers.HandleFailure(ctx, 0)

			if err == nil {
				t.Fatalf("expected non-nil error at/above max_retries (attempts=%d)", tc.attempts)
			}
			// Task must be marked BLOCKED in memory.
			var found bool
			for _, task := range ts.Epic.Tasks {
				if task.ID == "EPIC-5-001" {
					found = true
					if task.Status != types.StatusBlocked {
						t.Errorf("task status: got %q, want %q", task.Status, types.StatusBlocked)
					}
				}
			}
			if !found {
				t.Error("task EPIC-5-001 not found in tasks list")
			}
			// Active task must be set to manual_review.
			if st.ActiveTask.Type != types.TaskTypeManualReview {
				t.Errorf("ActiveTask.Type: got %q, want %q", st.ActiveTask.Type, types.TaskTypeManualReview)
			}
			if st.ActiveTask.ID != "EPIC-5-001" {
				t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "EPIC-5-001")
			}
		})
	}
}

func TestHandleFailure_MetricsRecorded(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	initialCount := len(st.Metrics.Tasks)

	// Use below-max-retries to keep it simple
	ctx := failureCtx(dir, 1, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	_ = handlers.HandleFailure(ctx, 0)

	if len(st.Metrics.Tasks) != initialCount+1 {
		t.Errorf("metrics: got %d tasks, want %d", len(st.Metrics.Tasks), initialCount+1)
	}
	last := st.Metrics.Tasks[len(st.Metrics.Tasks)-1]
	if last.TaskID != "EPIC-5-001" {
		t.Errorf("metric task_id: got %q, want %q", last.TaskID, "EPIC-5-001")
	}
	if last.Outcome != "FAILURE" {
		t.Errorf("metric outcome: got %q, want %q", last.Outcome, "FAILURE")
	}
}

func TestHandleFailure_RetryPath_PersistsMetricsToDisk(t *testing.T) {
	// Simulate a process restart between a failed attempt and the next iteration:
	// the failure metric must survive by being written to project-state.yaml on
	// the retry path (attempts < max_retries).
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := failureCtx(dir, 1, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleFailure(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error on retry path: %v", err)
	}

	// Reload state from disk to simulate a process restart.
	loaded, loadErr := state.LoadProjectState(ctx.StatePath)
	if loadErr != nil {
		t.Fatalf("could not reload project-state.yaml: %v", loadErr)
	}
	if len(loaded.Metrics.Tasks) == 0 {
		t.Fatal("metrics.tasks is empty after reload — failure metric was not persisted")
	}
	last := loaded.Metrics.Tasks[len(loaded.Metrics.Tasks)-1]
	if last.TaskID != "EPIC-5-001" {
		t.Errorf("persisted metric task_id: got %q, want %q", last.TaskID, "EPIC-5-001")
	}
	if last.Outcome != "FAILURE" {
		t.Errorf("persisted metric outcome: got %q, want %q", last.Outcome, "FAILURE")
	}
}

func TestHandleFailure_SyntheticTask_DoesNotMarkBlocked(t *testing.T) {
	// Bugfix tasks (synthetic) are not in tasks.yaml; blocking is skipped.
	dir := setupGitRepo(t)
	st := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:        "EPIC-5",
			StartedAt: "2026-02-24T00:00:00Z",
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeBugfix,
			ID:       "BUG-EPIC-5-001",
			Attempts: 5,
		},
	}
	// Tasks list does NOT contain the bug task (it's synthetic)
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := failureCtx(dir, 5, "BUG-EPIC-5-001", types.TaskTypeBugfix, st, ts)

	err := handlers.HandleFailure(ctx, 0)

	// Should still return a fatal error (max retries) but not panic/error on missing task
	if err == nil {
		t.Fatal("expected non-nil error at max_retries for synthetic task")
	}
	// User-defined task status should be unchanged
	for _, task := range ts.Epic.Tasks {
		if task.Status == types.StatusBlocked {
			t.Errorf("user-defined task %q should not be blocked when synthetic task fails", task.ID)
		}
	}
}
