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
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Bug handler helpers
// ---------------------------------------------------------------------------

// bugCtx builds a LoopContext suitable for HandleBug tests.
func bugCtx(dir string, taskID string, taskType types.TaskType, st *types.ProjectState, ts *types.Tasks) *orchestrator.LoopContext {
	dougDir := filepath.Join(dir, ".doug")
	return &orchestrator.LoopContext{
		TaskID:        taskID,
		TaskType:      taskType,
		Attempts:      1,
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
	}
}

// ---------------------------------------------------------------------------
// Tests: nested bug detection
// ---------------------------------------------------------------------------

func TestHandleBug_NestedBug(t *testing.T) {
	tests := []struct {
		name         string
		taskID       string
		checkMessage bool
	}{
		{name: "returns fatal error", taskID: "BUG-EPIC-5-001"},
		{name: "error contains diagnostic info", taskID: "BUG-EPIC-5-002", checkMessage: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeBugfixState()
			ts := makeInProgressTasks(tc.taskID)
			ctx := bugCtx(dir, tc.taskID, types.TaskTypeBugfix, st, ts)

			err := handlers.HandleBug(ctx, 0)

			if err == nil {
				t.Fatal("expected non-nil error for nested bug, got nil")
			}
			if tc.checkMessage {
				msg := err.Error()
				if !strings.Contains(msg, tc.taskID) {
					t.Errorf("error should contain task ID %q, got: %q", tc.taskID, msg)
				}
				if !strings.Contains(msg, "nested") {
					t.Errorf("error should contain %q, got: %q", "nested", msg)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: bug ID generation and state mutation
// ---------------------------------------------------------------------------

func TestHandleBug_SchedulesBugFixTask(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.ActiveTask.ID != "BUG-EPIC-5-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "BUG-EPIC-5-001")
	}
	if st.ActiveTask.Type != types.TaskTypeBugfix {
		t.Errorf("ActiveTask.Type: got %q, want %q", st.ActiveTask.Type, types.TaskTypeBugfix)
	}
}

func TestHandleBug_NextTaskIsInterruptedTask(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	// tasks.yaml has EPIC-5-003 with type "feature"
	ts := makeInProgressTasks("EPIC-5-003")

	ctx := bugCtx(dir, "EPIC-5-003", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.NextTask.ID != "EPIC-5-003" {
		t.Errorf("NextTask.ID: got %q, want %q", st.NextTask.ID, "EPIC-5-003")
	}
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

	err := handlers.HandleBug(ctx, 0)

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

func TestHandleBug_MissingActiveBug(t *testing.T) {
	// If ACTIVE_BUG.md is absent, the bug must still be scheduled (archive is
	// skipped) and no archive directory created.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	// .doug/ACTIVE_BUG.md is NOT created

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx, 0)

	if err != nil {
		t.Fatalf("expected nil error when ACTIVE_BUG.md is missing, got: %v", err)
	}
	if st.ActiveTask.Type != types.TaskTypeBugfix {
		t.Errorf("ActiveTask.Type: got %q, want bugfix", st.ActiveTask.Type)
	}
	if st.ActiveTask.ID != "BUG-EPIC-5-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "BUG-EPIC-5-001")
	}
	archiveDir := filepath.Join(dir, ".doug", "logs", "bugs", "EPIC-5")
	if _, statErr := os.Stat(archiveDir); statErr == nil {
		t.Error("archive directory should not be created when ACTIVE_BUG.md is missing")
	}
}

func TestHandleBug_ArchivesBugReport(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		content     string
		wantContent string
	}{
		{
			name:        "archives to correct path",
			taskID:      "EPIC-5-003",
			content:     "# Bug\n\nDetailed bug report content.",
			wantContent: "Detailed bug report content.",
		},
		{
			// Must read from .doug/ACTIVE_BUG.md, NOT .doug/logs/ACTIVE_BUG.md.
			name:        "reads from .doug/ not .doug/logs/",
			taskID:      "EPIC-5-001",
			content:     "# Bug\n\nCorrect doug dir path.",
			wantContent: "Correct doug dir path.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeFeatureState()
			ts := makeInProgressTasks(tc.taskID)

			dougDir := filepath.Join(dir, ".doug")
			testutil.WriteFile(t, filepath.Join(dougDir, "ACTIVE_BUG.md"), tc.content)

			ctx := bugCtx(dir, tc.taskID, types.TaskTypeFeature, st, ts)

			err := handlers.HandleBug(ctx, 0)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// .doug/logs/bugs/{epic}/bug-{taskID}.md
			archivePath := filepath.Join(dougDir, "logs", "bugs", "EPIC-5", "bug-"+tc.taskID+".md")
			data, readErr := os.ReadFile(archivePath)
			if readErr != nil {
				t.Fatalf("archived file not found at %s: %v", archivePath, readErr)
			}
			if !strings.Contains(string(data), tc.wantContent) {
				t.Errorf("archived content does not match source: %q", string(data))
			}
		})
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

	_ = handlers.HandleBug(ctx, 0)

	if len(st.Metrics.Tasks) != initialCount+1 {
		t.Errorf("metrics: got %d tasks, want %d", len(st.Metrics.Tasks), initialCount+1)
	}
	last := st.Metrics.Tasks[len(st.Metrics.Tasks)-1]
	if last.TaskID != "EPIC-5-001" {
		t.Errorf("metric task_id: got %q, want %q", last.TaskID, "EPIC-5-001")
	}
	if last.Outcome != "BUG" {
		t.Errorf("metric outcome: got %q, want %q", last.Outcome, "BUG")
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

	err := handlers.HandleBug(ctx, 0)

	if err != nil {
		t.Errorf("expected nil error for normal bug scheduling, got: %v", err)
	}
}
