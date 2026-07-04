package handlers_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
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

// blockingBugResult returns a SessionResult with exactly one blocking bug entry.
func blockingBugResult(body string) *types.SessionResult {
	return &types.SessionResult{
		Outcome: types.OutcomeBug,
		Bugs: []types.SessionBug{
			{Severity: types.SessionBugSeverityBlocking, Body: body},
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

			err := handlers.HandleBug(ctx, blockingBugResult("body"), 0)

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
// Tests: blocking bug validation
// ---------------------------------------------------------------------------

func TestHandleBug_NoBlockingBugInResult(t *testing.T) {
	// A BUG outcome with no blocking bug payload in the result is rejected
	// before scheduling a synthetic bugfix task or mutating state.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	st.NextTask = types.TaskPointer{}
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	// Result with no bugs at all.
	result := &types.SessionResult{Outcome: types.OutcomeBug}
	err := handlers.HandleBug(ctx, result, 0)

	if err == nil {
		t.Fatal("expected error when result has no blocking bug, got nil")
	}
	if !strings.Contains(err.Error(), "no blocking bug") {
		t.Errorf("expected error mentioning 'no blocking bug', got: %v", err)
	}
	// State must not have been mutated.
	if st.ActiveTask.Type != types.TaskTypeFeature {
		t.Errorf("ActiveTask.Type: got %q, want feature to remain unchanged", st.ActiveTask.Type)
	}
	if st.ActiveTask.ID != "EPIC-5-001" {
		t.Errorf("ActiveTask.ID: got %q, want original task ID to remain unchanged", st.ActiveTask.ID)
	}
	if st.NextTask.ID != "" || st.NextTask.Type != "" {
		t.Errorf("NextTask should remain empty, got %+v", st.NextTask)
	}
	// No archive directory should be created.
	archiveDir := filepath.Join(dir, ".doug", "intake", "bugs", "EPIC-5")
	if _, statErr := os.Stat(archiveDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("archive directory should not be created when blocking bug is absent, stat err=%v", statErr)
	}
}

func TestHandleBug_NonBlockingOnlyResult_Rejected(t *testing.T) {
	// A BUG outcome with only non-blocking bugs is rejected.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	result := &types.SessionResult{
		Outcome: types.OutcomeBug,
		Bugs:    []types.SessionBug{{Severity: types.SessionBugSeverityNonBlocking, Body: "minor issue"}},
	}
	err := handlers.HandleBug(ctx, result, 0)

	if err == nil {
		t.Fatal("expected error when result has only non-blocking bugs, got nil")
	}
	if !strings.Contains(err.Error(), "no blocking bug") {
		t.Errorf("expected error mentioning 'no blocking bug', got: %v", err)
	}
}

func TestHandleBug_MultipleBlockingBugsRejected(t *testing.T) {
	// A BUG outcome with more than one blocking bug is rejected.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	result := &types.SessionResult{
		Outcome: types.OutcomeBug,
		Bugs: []types.SessionBug{
			{Severity: types.SessionBugSeverityBlocking, Body: "first blocking"},
			{Severity: types.SessionBugSeverityBlocking, Body: "second blocking"},
		},
	}
	err := handlers.HandleBug(ctx, result, 0)

	if err == nil {
		t.Fatal("expected error when result has multiple blocking bugs, got nil")
	}
	if !strings.Contains(err.Error(), "2 blocking bug") {
		t.Errorf("expected error mentioning count of blocking bugs, got: %v", err)
	}
	// State must not have been mutated.
	if st.ActiveTask.Type != types.TaskTypeFeature {
		t.Errorf("ActiveTask.Type should remain feature, got %q", st.ActiveTask.Type)
	}
}

// ---------------------------------------------------------------------------
// Tests: bug ID generation and state mutation
// ---------------------------------------------------------------------------

func TestHandleBug_SchedulesBugFixTask(t *testing.T) {
	dir := setupGitRepo(t)
	activeTaskPath := writeLiveActiveTask(t, dir, "# Active Task\n")
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx, blockingBugResult("blocking bug"), 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.ActiveTask.ID != "BUG-EPIC-5-001" {
		t.Errorf("ActiveTask.ID: got %q, want %q", st.ActiveTask.ID, "BUG-EPIC-5-001")
	}
	if st.ActiveTask.Type != types.TaskTypeBugfix {
		t.Errorf("ActiveTask.Type: got %q, want %q", st.ActiveTask.Type, types.TaskTypeBugfix)
	}
	if _, err := os.Stat(activeTaskPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ACTIVE_TASK.md to be cleaned up, stat err=%v", err)
	}
}

func TestHandleBug_NextTaskIsInterruptedTask(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	// tasks.yaml has EPIC-5-003 with type "feature"
	ts := makeInProgressTasks("EPIC-5-003")

	ctx := bugCtx(dir, "EPIC-5-003", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx, blockingBugResult("blocking bug"), 0)

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

func TestHandleBug_NonBacklogTask_NextTaskTypeFromCtx(t *testing.T) {
	// Tasks with IDs not in tasks.yaml (e.g., handler-injected documentation
	// tasks) fall back to ctx.TaskType for next_task type resolution.
	dir := setupGitRepo(t)
	st := makeDocsState()
	ts := makeSingleTaskDone()

	ctx := bugCtx(dir, "KB_UPDATE", types.TaskTypeDocumentation, st, ts)

	err := handlers.HandleBug(ctx, blockingBugResult("blocking bug"), 0)

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
// Tests: blocking bug archive
// ---------------------------------------------------------------------------

func TestHandleBug_ArchivesBugReport(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		bugBody     string
		wantContent string
	}{
		{
			name:        "archives body to correct path",
			taskID:      "EPIC-5-003",
			bugBody:     "## Summary\n\nDetailed bug report content.",
			wantContent: "Detailed bug report content.",
		},
		{
			name:        "bug body preserved in archive",
			taskID:      "EPIC-5-001",
			bugBody:     "## Summary\n\nCorrect bug payload body.",
			wantContent: "Correct bug payload body.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepo(t)
			st := makeFeatureState()
			ts := makeInProgressTasks(tc.taskID)

			ctx := bugCtx(dir, tc.taskID, types.TaskTypeFeature, st, ts)

			err := handlers.HandleBug(ctx, blockingBugResult(tc.bugBody), 0)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// .doug/intake/bugs/{epic}/bug-{taskID}.md
			dougDir := filepath.Join(dir, ".doug")
			archivePath := filepath.Join(dougDir, "intake", "bugs", "EPIC-5", "bug-"+tc.taskID+".md")
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

func TestHandleBug_RepeatedBugReportsUseVersionedArchives(t *testing.T) {
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	dougDir := filepath.Join(dir, ".doug")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)
	if err := handlers.HandleBug(ctx, blockingBugResult("first report"), 0); err != nil {
		t.Fatalf("first HandleBug returned error: %v", err)
	}

	st.ActiveTask = types.TaskPointer{Type: types.TaskTypeFeature, ID: "EPIC-5-001", Attempts: 1}
	st.NextTask = types.TaskPointer{}
	ctx = bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)
	if err := handlers.HandleBug(ctx, blockingBugResult("second report"), 0); err != nil {
		t.Fatalf("second HandleBug returned error: %v", err)
	}

	firstPath := filepath.Join(dougDir, "intake", "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	secondPath := filepath.Join(dougDir, "intake", "bugs", "EPIC-5", "bug-EPIC-5-001-v2.md")

	firstData, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first bug archive: %v", err)
	}
	secondData, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second bug archive: %v", err)
	}
	if !strings.Contains(string(firstData), "first report") {
		t.Errorf("first archive missing original content: %q", string(firstData))
	}
	if !strings.Contains(string(secondData), "second report") {
		t.Errorf("second archive missing updated content: %q", string(secondData))
	}
}

func TestHandleBug_ArchiveContainsFrontmatter(t *testing.T) {
	// The archive file written by HandleBug should have YAML frontmatter
	// with the expected fields stamped by WriteBugArchive.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	if err := handlers.HandleBug(ctx, blockingBugResult("bug details"), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	archivePath := filepath.Join(dir, ".doug", "intake", "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive not found: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"bug_id: BUG-EPIC-5-001",
		"discovered_by_task: EPIC-5-001",
		"severity: high",
		"status: open",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("frontmatter missing %q\ncontent:\n%s", want, content)
		}
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

	_ = handlers.HandleBug(ctx, blockingBugResult("blocking bug"), 0)

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

	err := handlers.HandleBug(ctx, blockingBugResult("blocking bug"), 0)

	if err != nil {
		t.Errorf("expected nil error for normal bug scheduling, got: %v", err)
	}
}

func TestHandleBug_SaveProjectStateFails_ReturnsError(t *testing.T) {
	// When SaveProjectState fails (step 10), HandleBug must return the error
	// rather than swallow it — the state machine depends on this being fatal.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)
	// Point StatePath to a non-existent directory so SaveProjectState fails.
	ctx.StatePath = filepath.Join(dir, "nonexistent", "project-state.yaml")

	err := handlers.HandleBug(ctx, blockingBugResult("blocking bug"), 0)

	if err == nil {
		t.Error("expected non-nil error when SaveProjectState fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: bug payload persisted on TaskPointer and rendered in bugfix brief
// ---------------------------------------------------------------------------

func TestHandleBug_BugPayloadPersistedOnActiveTaskPointer(t *testing.T) {
	// After HandleBug, the active_task TaskPointer must carry the full bug
	// payload (bug_id, bug_severity, bug_source_task, bug_body, bug_archive_path)
	// so that the bugfix brief can be rendered without any separate ACTIVE_BUG.md.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	bugBody := "## Summary\n\nNull pointer dereference in handler.\n\n## Steps\n1. Call handler with nil input."

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	err := handlers.HandleBug(ctx, blockingBugResult(bugBody), 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ptr := st.ActiveTask
	if ptr.BugID != "BUG-EPIC-5-001" {
		t.Errorf("BugID: got %q, want %q", ptr.BugID, "BUG-EPIC-5-001")
	}
	if ptr.BugSeverity == "" {
		t.Error("BugSeverity must not be empty")
	}
	if ptr.BugSourceTask != "EPIC-5-001" {
		t.Errorf("BugSourceTask: got %q, want %q", ptr.BugSourceTask, "EPIC-5-001")
	}
	if !strings.Contains(ptr.BugBody, "Null pointer dereference") {
		t.Errorf("BugBody does not contain expected content; got: %q", ptr.BugBody)
	}
	if ptr.BugArchivePath == "" {
		t.Error("BugArchivePath must not be empty")
	}
}

func TestHandleBug_BugfixBriefRenderedFromPayloadWithoutActiveBugFile(t *testing.T) {
	// A scheduled BUG-<taskID> task must have enough rendered context in
	// ACTIVE_TASK.md for an implement-bugfix agent without any separate
	// ACTIVE_BUG.md file. This test proves the payload carried on the
	// TaskPointer is sufficient to populate the brief.
	dir := setupGitRepo(t)
	st := makeFeatureState()
	ts := makeInProgressTasks("EPIC-5-001")
	bugBody := "## Summary\n\nPanic on nil input at line 42.\n\n## Reproduction\n1. Run the command.\n2. Observe panic."

	ctx := bugCtx(dir, "EPIC-5-001", types.TaskTypeFeature, st, ts)

	// Run the bug handler (schedules BUG-EPIC-5-001 on st.ActiveTask)
	if err := handlers.HandleBug(ctx, blockingBugResult(bugBody), 0); err != nil {
		t.Fatalf("HandleBug returned error: %v", err)
	}

	// Ensure no ACTIVE_BUG.md exists — the brief must not depend on it.
	dougDir := filepath.Join(dir, ".doug")
	activeBugPath := filepath.Join(dougDir, "ACTIVE_BUG.md")
	if _, err := os.Stat(activeBugPath); !errors.Is(err, os.ErrNotExist) {
		// Verify we're testing the right thing: if ACTIVE_BUG.md somehow exists
		// remove it to make the test meaningful.
		_ = os.Remove(activeBugPath)
	}

	// Write ACTIVE_TASK.md for the scheduled bugfix task using the payload
	// from the active_task pointer (simulating the next run-loop iteration).
	ptr := st.ActiveTask
	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:         ptr.ID,
		TaskType:       ptr.Type,
		DougDir:        dougDir,
		Attempts:       1,
		MaxRetries:     5,
		BugID:          ptr.BugID,
		BugSeverity:    ptr.BugSeverity,
		BugSourceTask:  ptr.BugSourceTask,
		BugBody:        ptr.BugBody,
		BugArchivePath: ptr.BugArchivePath,
	}, log.Discard()); err != nil {
		t.Fatalf("WriteActiveTask returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
	if err != nil {
		t.Fatalf("ACTIVE_TASK.md not found: %v", err)
	}
	content := string(data)

	// The brief must include all required bug context fields.
	for _, want := range []string{
		"BUG-EPIC-5-001",                // bug ID
		string(types.BugSeverityHigh),   // severity
		"EPIC-5-001",                    // source task
		"Panic on nil input at line 42", // bug body summary
		"Reproduction",                  // bug body details
	} {
		if !strings.Contains(content, want) {
			t.Errorf("bugfix brief missing %q; got:\n%s", want, content)
		}
	}

	// The brief must NOT reference ACTIVE_BUG.md.
	if strings.Contains(content, "ACTIVE_BUG.md") {
		t.Errorf("bugfix brief must not reference ACTIVE_BUG.md; got:\n%s", content)
	}

	// The brief must include the archive path for durable reference.
	if !strings.Contains(content, "Archive") {
		t.Errorf("bugfix brief must reference the durable archive; got:\n%s", content)
	}
}
