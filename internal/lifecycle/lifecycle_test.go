package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func writeLifecycleFixtures(t *testing.T, root string, projectState string, tasks string) Paths {
	t.Helper()
	paths := DefaultPaths(root)
	testutil.WriteFile(t, paths.StatePath, projectState)
	testutil.WriteFile(t, paths.TasksPath, tasks)
	return paths
}

func baseProjectState() string {
	return `current_epic:
    id: EPIC-1
    name: Test Epic
    branch_name: epic/test
    started_at: "2026-01-01T00:00:00Z"
    completed_at: null
active_task:
    type: feature
    id: TASK-1
    attempts: 1
next_task:
    type: feature
    id: TASK-2
metrics:
    total_tasks_completed: 0
    total_duration_seconds: 0
    tasks: []
`
}

func baseTasks(firstStatus, secondStatus types.Status) string {
	return `epic:
    id: EPIC-1
    name: Test Epic
    tasks:
        - id: TASK-1
          type: feature
          status: ` + string(firstStatus) + `
          description: First task
          acceptance_criteria:
            - First task works
        - id: TASK-2
          type: documentation
          status: ` + string(secondStatus) + `
          description: Second task
          acceptance_criteria:
            - Second task works
`
}

func TestDiscoverStatusNoActiveTask(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, strings.Replace(baseProjectState(), "    id: TASK-1", "    id: \"\"", 1), baseTasks(types.StatusTODO, types.StatusTODO))

	status, err := DiscoverStatus(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiscoverStatus returned error: %v", err)
	}
	if status.Kind != StatusNoActiveTask {
		t.Fatalf("Kind = %q, want %q", status.Kind, StatusNoActiveTask)
	}
	if status.AllTasksDone {
		t.Fatal("AllTasksDone = true, want false")
	}
}

func TestDiscoverStatusActiveTask(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "# active\n")

	status, err := DiscoverStatus(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiscoverStatus returned error: %v", err)
	}
	if status.Kind != StatusActiveTask {
		t.Fatalf("Kind = %q, want %q", status.Kind, StatusActiveTask)
	}
	if status.ActiveTask.ID != "TASK-1" {
		t.Fatalf("ActiveTask.ID = %q, want TASK-1", status.ActiveTask.ID)
	}
}

func TestDiscoverStatusAllUserTasksComplete(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusDone, types.StatusDone))

	status, err := DiscoverStatus(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiscoverStatus returned error: %v", err)
	}
	if status.Kind != StatusComplete {
		t.Fatalf("Kind = %q, want %q", status.Kind, StatusComplete)
	}
	if !status.AllTasksDone {
		t.Fatal("AllTasksDone = false, want true")
	}
}

func TestClaimNextWritesActiveTaskAndPersistsAttemptWithoutInProgress(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "    id: TASK-1", "    id: \"\"", 1)
	projectState = strings.Replace(projectState, "    attempts: 1\n", "", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusTODO, types.StatusTODO))

	claim, err := ClaimNext(Options{Paths: paths, MaxRetries: 3, BuildSystem: "go"})
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if !claim.Claimed || claim.AlreadyActive {
		t.Fatalf("Claimed=%v AlreadyActive=%v, want claimed new assignment", claim.Claimed, claim.AlreadyActive)
	}
	if claim.ActiveTask.ID != "TASK-1" || claim.Attempt != 1 {
		t.Fatalf("claimed task=%q attempt=%d, want TASK-1 attempt 1", claim.ActiveTask.ID, claim.Attempt)
	}
	if _, err := os.Stat(filepath.Join(paths.DougDir, "ACTIVE_TASK.md")); err != nil {
		t.Fatalf("ACTIVE_TASK.md not written: %v", err)
	}

	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.ActiveTask.ID != "TASK-1" || loadedState.ActiveTask.Attempts != 1 {
		t.Fatalf("persisted active task = %#v, want TASK-1 attempt 1", loadedState.ActiveTask)
	}
	loadedTasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if got := loadedTasks.Epic.Tasks[0].Status; got != types.StatusTODO {
		t.Fatalf("task status = %q, want TODO; claim must not write IN_PROGRESS", got)
	}
}

func TestClaimNextAlreadyActiveDoesNotAdvancePointer(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, strings.Replace(baseProjectState(), "attempts: 1", "", 1), baseTasks(types.StatusTODO, types.StatusTODO))
	// Simulate an existing live assignment. The second claim must not advance to TASK-2.
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "# existing active task\n")

	before, err := os.ReadFile(paths.StatePath)
	if err != nil {
		t.Fatalf("read state before claim: %v", err)
	}
	claim, err := ClaimNext(Options{Paths: paths, MaxRetries: 3, BuildSystem: "go"})
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if !claim.AlreadyActive || claim.Claimed {
		t.Fatalf("AlreadyActive=%v Claimed=%v, want already-active non-mutating response", claim.AlreadyActive, claim.Claimed)
	}
	if claim.ActiveTask.ID != "TASK-1" {
		t.Fatalf("ActiveTask.ID = %q, want TASK-1", claim.ActiveTask.ID)
	}
	after, err := os.ReadFile(paths.StatePath)
	if err != nil {
		t.Fatalf("read state after claim: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("state mutated during already-active claim")
	}
}

func TestCompleteVerifiedTaskPersistsDoneAndPointerAdvanceTogether(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))

	result, err := CompleteVerifiedTask(Options{Paths: paths}, "TASK-1")
	if err != nil {
		t.Fatalf("CompleteVerifiedTask returned error: %v", err)
	}
	if !result.Advanced || result.Terminal {
		t.Fatalf("Advanced=%v Terminal=%v, want non-terminal pointer advance", result.Advanced, result.Terminal)
	}

	loadedTasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if got := loadedTasks.Epic.Tasks[0].Status; got != types.StatusDone {
		t.Fatalf("TASK-1 status = %q, want DONE", got)
	}
	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.ActiveTask.ID != "TASK-2" || loadedState.ActiveTask.Attempts != 0 {
		t.Fatalf("active task = %#v, want TASK-2 with reset attempts", loadedState.ActiveTask)
	}
	if loadedState.NextTask.ID != "" {
		t.Fatalf("next task = %#v, want cleared", loadedState.NextTask)
	}
}

func TestTerminalCompletionAndFinalizationStampEpicMetadataOnlyThroughLifecycle(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "status: TODO", "status: DONE", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusDone, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "PRD.md"), "# PRD\n")
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "# active task\n")
	metadataPath := filepath.Join(paths.DougDir, "plan", "epics", "EPIC-1", "metadata.yaml")
	testutil.WriteFile(t, metadataPath, "epic_id: EPIC-1\nstatus: ACTIVE\ncreated_at: \"2026-01-01T00:00:00Z\"\nsource_plan_path: .doug/PLAN.md\nactivated_at: \"2026-01-01T00:00:00Z\"\n")

	beforeMetadata, err := plan.LoadEpicMetadata(metadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata before transition: %v", err)
	}
	if beforeMetadata.CompletedAt != nil || beforeMetadata.Status != types.EpicStatusActive {
		t.Fatalf("metadata was finalized before lifecycle transition: %#v", beforeMetadata)
	}

	completion, err := CompleteVerifiedTask(Options{Paths: paths}, "TASK-2")
	if err != nil {
		t.Fatalf("CompleteVerifiedTask returned error: %v", err)
	}
	if !completion.Terminal || completion.Advanced {
		t.Fatalf("Terminal=%v Advanced=%v, want terminal completion", completion.Terminal, completion.Advanced)
	}
	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.CurrentEpic.CompletedAt == nil || *loadedState.CurrentEpic.CompletedAt == "" {
		t.Fatal("completed_at was not stamped by terminal completion transition")
	}

	finalized, err := FinalizeEpic(Options{Paths: paths})
	if err != nil {
		t.Fatalf("FinalizeEpic returned error: %v", err)
	}
	if finalized.ArchiveDir == "" {
		t.Fatal("ArchiveDir is empty")
	}
	metadata, err := plan.LoadEpicMetadata(metadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata after finalization: %v", err)
	}
	if metadata.Status != types.EpicStatusCompleted {
		t.Fatalf("metadata status = %q, want COMPLETED", metadata.Status)
	}
	if metadata.CompletedAt == nil || *metadata.CompletedAt != *loadedState.CurrentEpic.CompletedAt {
		t.Fatalf("metadata completed_at = %v, want %q", metadata.CompletedAt, *loadedState.CurrentEpic.CompletedAt)
	}
	finalState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState finalized: %v", err)
	}
	if finalState.ActiveTask.ID != "" || finalState.NextTask.ID != "" {
		t.Fatalf("runtime pointers not cleared by finalization: active=%#v next=%#v", finalState.ActiveTask, finalState.NextTask)
	}
}

func TestRecordTaskFailurePreservesRetryStateWithoutDone(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "attempts: 1", "attempts: 1\n    consecutive_test_failures: 2\n    test_failure_output: failing tests", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusTODO, types.StatusTODO))

	result, err := RecordTaskFailure(Options{Paths: paths, MaxRetries: 3}, "TASK-1")
	if err != nil {
		t.Fatalf("RecordTaskFailure returned error: %v", err)
	}
	if !result.Retryable || result.Blocked {
		t.Fatalf("Retryable=%v Blocked=%v, want retryable failure", result.Retryable, result.Blocked)
	}
	loadedTasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if got := loadedTasks.Epic.Tasks[0].Status; got != types.StatusTODO {
		t.Fatalf("TASK-1 status = %q, want TODO (not DONE)", got)
	}
	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.ActiveTask.Attempts != 1 || loadedState.ActiveTask.ConsecutiveTestFailures != 2 || loadedState.ActiveTask.TestFailureOutput != "failing tests" {
		t.Fatalf("retry diagnostics not preserved: %#v", loadedState.ActiveTask)
	}
}

func TestRecordTaskFailureBlocksAtMaxRetriesWithoutMarkingDone(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "attempts: 1", "attempts: 3\n    consecutive_test_failures: 3\n    test_failure_output: still failing", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusTODO, types.StatusTODO))

	result, err := RecordTaskFailure(Options{Paths: paths, MaxRetries: 3}, "TASK-1")
	if err != nil {
		t.Fatalf("RecordTaskFailure returned error: %v", err)
	}
	if !result.Blocked || result.Retryable {
		t.Fatalf("Retryable=%v Blocked=%v, want blocked failure", result.Retryable, result.Blocked)
	}
	loadedTasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if got := loadedTasks.Epic.Tasks[0].Status; got != types.StatusBlocked {
		t.Fatalf("TASK-1 status = %q, want BLOCKED", got)
	}
	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.ActiveTask.ID != "TASK-1" || loadedState.ActiveTask.Attempts != 3 || loadedState.ActiveTask.ConsecutiveTestFailures != 3 || loadedState.ActiveTask.TestFailureOutput != "still failing" {
		t.Fatalf("manual-review state not preserved: %#v", loadedState.ActiveTask)
	}
	if loadedState.NextTask.ID != "" {
		t.Fatalf("NextTask.ID = %q, want cleared for manual review", loadedState.NextTask.ID)
	}
}
