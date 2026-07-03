package handlers_test

import (
	"errors"
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

func TestHandleFailure_AtMaxRetries_DoesNotArchiveSeparateFailureHandoff(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	ctx := failureCtx(dir, 5, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleFailure(ctx, 0)

	if err == nil {
		t.Fatal("expected non-nil error at max_retries")
	}
	archiveDir := filepath.Join(dir, ".doug", "logs", "failures", "EPIC-5")
	if _, statErr := os.Stat(archiveDir); statErr == nil {
		t.Error("separate failure handoff archive directory should not be created")
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
			// Active task must remain on the blocked backlog task.
			if st.ActiveTask.Type != types.TaskTypeFeature {
				t.Errorf("ActiveTask.Type: got %q, want %q", st.ActiveTask.Type, types.TaskTypeFeature)
			}
			if st.ActiveTask.ID != "EPIC-5-001" {
				t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "EPIC-5-001")
			}
			if st.ActiveTask.Attempts != 0 || st.ActiveTask.ConsecutiveTestFailures != 0 || st.ActiveTask.TestFailureOutput != "" {
				t.Errorf("blocked active task should clear transient fields, got %+v", st.ActiveTask)
			}
			if st.NextTask.ID != "" {
				t.Errorf("NextTask should be cleared when blocking active task, got %+v", st.NextTask)
			}
		})
	}
}

func TestHandleFailure_RetryPathRollsBackWorkspaceAndDoesNotAdvanceLifecycle(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := failureCtx(dir, 1, "EPIC-5-001", types.TaskTypeFeature, st, ts)
	if err := state.SaveProjectState(ctx.StatePath, st); err != nil {
		t.Fatalf("SaveProjectState: %v", err)
	}
	if err := state.SaveTasks(ctx.TasksPath, ts); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	original, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("ReadFile original CHANGELOG.md: %v", err)
	}
	if err := os.WriteFile(changelogPath, []byte("agent change that should roll back\n"), 0o644); err != nil {
		t.Fatalf("WriteFile CHANGELOG.md: %v", err)
	}

	if err := handlers.HandleFailure(ctx, 0); err != nil {
		t.Fatalf("HandleFailure: %v", err)
	}

	rolledBack, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("ReadFile rolled-back CHANGELOG.md: %v", err)
	}
	if string(rolledBack) != string(original) {
		t.Fatalf("CHANGELOG.md was not rolled back; got %q want %q", string(rolledBack), string(original))
	}
	persistedTasks, err := state.LoadTasks(ctx.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if persistedTasks.Epic.Tasks[0].Status != types.StatusInProgress || persistedTasks.Epic.Tasks[1].Status != types.StatusTODO {
		t.Fatalf("task statuses advanced after failure: %+v", persistedTasks.Epic.Tasks)
	}
	persistedState, err := state.LoadProjectState(ctx.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if persistedState.ActiveTask.ID != "EPIC-5-001" || persistedState.NextTask.ID != "EPIC-5-002" {
		t.Fatalf("state advanced after failure: active=%+v next=%+v", persistedState.ActiveTask, persistedState.NextTask)
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
	activeTaskPath := writeLiveActiveTask(t, dir, "# Active Task\n")
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
	if _, err := os.Stat(activeTaskPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ACTIVE_TASK.md to be cleaned up, stat err=%v", err)
	}
}

func TestHandleFailure_SaveProjectStateFails_RetryPath_StillReturnsNil(t *testing.T) {
	// On the retry path (attempts < MaxRetries) a SaveProjectState failure is
	// non-fatal: it is logged as a warning and HandleFailure still returns nil
	// so the orchestrator loop retries the task.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := failureCtx(dir, 1, "EPIC-5-001", types.TaskTypeFeature, st, ts)
	// Point StatePath to a non-existent directory so SaveProjectState fails.
	ctx.StatePath = filepath.Join(dir, "nonexistent", "project-state.yaml")

	err := handlers.HandleFailure(ctx, 0)

	if err != nil {
		t.Errorf("expected nil on retry path even when SaveProjectState fails, got: %v", err)
	}
}

func TestHandleFailure_HandlerInjectedBugfix_BlocksInterruptedBacklogTask(t *testing.T) {
	dir := setupGitRepo(t)
	st := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:        "EPIC-5",
			StartedAt: "2026-02-24T00:00:00Z",
		},
		ActiveTask: types.TaskPointer{
			Type:                    types.TaskTypeBugfix,
			ID:                      "BUG-EPIC-5-001",
			Attempts:                5,
			ConsecutiveTestFailures: 2,
			TestFailureOutput:       "boom",
		},
		NextTask: types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-5-001"},
	}
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := failureCtx(dir, 5, "BUG-EPIC-5-001", types.TaskTypeBugfix, st, ts)

	err := handlers.HandleFailure(ctx, 0)
	if err == nil {
		t.Fatal("expected non-nil error at max_retries for handler-injected bugfix task")
	}
	if ts.Epic.Tasks[0].Status != types.StatusBlocked {
		t.Fatalf("interrupted backlog task status: got %q, want BLOCKED", ts.Epic.Tasks[0].Status)
	}
	if st.ActiveTask.Type != types.TaskTypeFeature || st.ActiveTask.ID != "EPIC-5-001" {
		t.Fatalf("active task should fold back to interrupted backlog task, got %+v", st.ActiveTask)
	}
	if st.ActiveTask.Attempts != 0 || st.ActiveTask.ConsecutiveTestFailures != 0 || st.ActiveTask.TestFailureOutput != "" {
		t.Fatalf("blocked active task should clear transient fields, got %+v", st.ActiveTask)
	}
	if st.NextTask.ID != "" {
		t.Fatalf("NextTask should be cleared when blocking interrupted backlog task, got %+v", st.NextTask)
	}
}
