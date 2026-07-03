package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
