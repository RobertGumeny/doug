package handlers_test

import (
	"fmt"
	"testing"

	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// HandleResume tests
// ---------------------------------------------------------------------------

func TestHandleResume_VerificationFails_ReturnsBuildFailure(t *testing.T) {
	tests := []struct {
		name string
		bs   *mockBuildSystem
	}{
		{
			name: "build fails",
			bs:   &mockBuildSystem{initialized: true, buildErr: fmt.Errorf("compile error")},
		},
		{
			name: "tests fail",
			bs:   &mockBuildSystem{initialized: true, testErr: fmt.Errorf("test failure")},
		},
		{
			name: "uninitialized build system install fails",
			bs:   &mockBuildSystem{initialized: false, installErr: fmt.Errorf("npm install failed")},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeFeatureState()
			st.Status = types.ProjectStatusPaused
			ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
			ctx := baseCtx(dir, tc.bs, st, ts)

			result, err := handlers.HandleResume(ctx)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Kind != handlers.BuildFailure {
				t.Errorf("expected BuildFailure, got %v", result.Kind)
			}
			if st.Status != types.ProjectStatusPaused {
				t.Errorf("expected project status PAUSED, got %q", st.Status)
			}
		})
	}
}

func TestHandleResume_BuildPassesMarksTaskDoneAndAdvances(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{initialized: true}
	st := makeFeatureState()
	st.Status = types.ProjectStatusPaused
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)

	result, err := handlers.HandleResume(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.Continue {
		t.Errorf("expected Continue, got %v", result.Kind)
	}
	// Status must be cleared.
	if st.Status != "" {
		t.Errorf("expected empty status, got %q", st.Status)
	}
	// First task must be DONE.
	if ts.Epic.Tasks[0].Status != types.StatusDone {
		t.Errorf("expected task DONE, got %q", ts.Epic.Tasks[0].Status)
	}
	// Active task must have advanced to the next task.
	if st.ActiveTask.ID != "EPIC-5-002" {
		t.Errorf("expected active task EPIC-5-002, got %q", st.ActiveTask.ID)
	}
}

func TestHandleResume_BuildPassesDoesNotIncrementAttempts(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{initialized: true}
	st := makeFeatureState()
	st.Status = types.ProjectStatusPaused
	st.ActiveTask.Attempts = 0 // simulates state after pause decremented from 1 → 0
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	ctx.Attempts = 0

	_, err := handlers.HandleResume(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After advance, active task is the next one; its attempts must be 0.
	if st.ActiveTask.Attempts != 0 {
		t.Errorf("expected attempts 0 after advance, got %d", st.ActiveTask.Attempts)
	}
}

func TestHandleResume_BuildFails_DoesNotDecrementBelowZero(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{initialized: true, buildErr: fmt.Errorf("still broken")}
	st := makeFeatureState()
	st.Status = types.ProjectStatusPaused
	st.ActiveTask.Attempts = 0 // already at zero after previous pause
	ts := makeTwoTaskTasks(types.StatusInProgress, types.StatusTODO)
	ctx := baseCtx(dir, bs, st, ts)
	ctx.Attempts = 0

	result, err := handlers.HandleResume(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.BuildFailure {
		t.Errorf("expected BuildFailure, got %v", result.Kind)
	}
	// Attempts must not go negative.
	if st.ActiveTask.Attempts < 0 {
		t.Errorf("attempts went negative: %d", st.ActiveTask.Attempts)
	}
}

func TestHandleResume_LastTaskWithoutKBEnabled_ReturnsEpicComplete(t *testing.T) {
	dir := setupGitRepo(t)
	bs := &mockBuildSystem{initialized: true}
	st := makeFeatureState()
	st.Status = types.ProjectStatusPaused
	st.NextTask = types.TaskPointer{}
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

	result, err := handlers.HandleResume(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Kind != handlers.EpicComplete {
		t.Fatalf("result kind = %v, want %v", result.Kind, handlers.EpicComplete)
	}
	if ts.Epic.Tasks[0].Status != types.StatusDone {
		t.Fatalf("task status = %q, want %q", ts.Epic.Tasks[0].Status, types.StatusDone)
	}
	if st.CurrentEpic.CompletedAt == nil || *st.CurrentEpic.CompletedAt == "" {
		t.Fatal("expected completed_at to be set on terminal resume completion path")
	}
}
