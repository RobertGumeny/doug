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

func requireFinding(t *testing.T, findings []DiagnosticFinding, code string) DiagnosticFinding {
	t.Helper()
	for _, finding := range findings {
		if finding.Code == code {
			return finding
		}
	}
	t.Fatalf("missing finding %q in %#v", code, findings)
	return DiagnosticFinding{}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func containsChangedFile(files []ChangedFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func containsChangedField(fields []ChangedField, fieldName string) bool {
	for _, field := range fields {
		if field.Field == fieldName {
			return true
		}
	}
	return false
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

func TestDiagnoseLifecycleCleanState(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	if len(diagnostics.Findings) != 0 {
		t.Fatalf("Findings = %#v, want none", diagnostics.Findings)
	}
	if !containsString(diagnostics.AllowedNextActions, DiagnosticActionReportTaskComplete) || !containsString(diagnostics.AllowedNextActions, DiagnosticActionReportTaskBlocked) {
		t.Fatalf("AllowedNextActions = %#v, want report actions", diagnostics.AllowedNextActions)
	}
}

func TestDiagnoseLifecycleActiveBriefMissing(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	finding := requireFinding(t, diagnostics.Findings, "ACTIVE_BRIEF_MISSING")
	if !finding.RequiresManualReview {
		t.Fatalf("ACTIVE_BRIEF_MISSING RequiresManualReview = false, want true")
	}
}

func TestDiagnoseLifecycleActivePointerStatusMismatch(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusBlocked, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	finding := requireFinding(t, diagnostics.Findings, "ACTIVE_POINTER_STATUS_MISMATCH")
	if !finding.RequiresManualReview {
		t.Fatalf("ACTIVE_POINTER_STATUS_MISMATCH RequiresManualReview = false, want true")
	}
}

func TestDiagnoseLifecycleCompletedTaskPointerDrift(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusDone, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	requireFinding(t, diagnostics.Findings, "COMPLETED_TASK_POINTER_DRIFT")
}

func TestDiagnoseLifecycleStaleActiveBriefAfterPointerAdvanced(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "    id: TASK-1", "    id: TASK-2", 1)
	projectState = strings.Replace(projectState, "next_task:\n    type: feature\n    id: TASK-2", "next_task:\n    type: \"\"\n    id: \"\"", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusDone, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	requireFinding(t, diagnostics.Findings, "STALE_ACTIVE_BRIEF")
}

func TestDiagnoseLifecycleReadOnlyReturnsFindingsAndActions(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusDone, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")
	stateBefore := readFileString(t, paths.StatePath)
	tasksBefore := readFileString(t, paths.TasksPath)
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	briefBefore := readFileString(t, briefPath)

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	if len(diagnostics.Findings) == 0 {
		t.Fatal("Findings empty, want drift finding")
	}
	if !containsString(diagnostics.AllowedNextActions, DiagnosticActionManualReview) {
		t.Fatalf("AllowedNextActions = %#v, want manual review", diagnostics.AllowedNextActions)
	}
	if got := readFileString(t, paths.StatePath); got != stateBefore {
		t.Fatal("DiagnoseLifecycle modified project-state.yaml")
	}
	if got := readFileString(t, paths.TasksPath); got != tasksBefore {
		t.Fatal("DiagnoseLifecycle modified tasks.yaml")
	}
	if got := readFileString(t, briefPath); got != briefBefore {
		t.Fatal("DiagnoseLifecycle modified ACTIVE_TASK.md")
	}
}

func TestDiagnoseLifecycleAmbiguousDriftRequiresManualReview(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-2\n")

	diagnostics, err := DiagnoseLifecycle(Options{Paths: paths})
	if err != nil {
		t.Fatalf("DiagnoseLifecycle returned error: %v", err)
	}
	finding := requireFinding(t, diagnostics.Findings, "AMBIGUOUS_ACTIVE_BRIEF_DRIFT")
	if !finding.RequiresManualReview {
		t.Fatal("ambiguous drift should require manual review")
	}
	if !containsString(diagnostics.AllowedNextActions, DiagnosticActionManualReview) {
		t.Fatalf("AllowedNextActions = %#v, want manual review", diagnostics.AllowedNextActions)
	}
}

func TestReconcileLifecycleRepairRewritesMissingActiveBrief(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))

	result, err := ReconcileLifecycle(Options{Paths: paths, MaxRetries: 3, BuildSystem: "go"}, ReconcileModeRepair)
	if err != nil {
		t.Fatalf("ReconcileLifecycle returned error: %v", err)
	}
	if !result.Repaired || result.ManualReview {
		t.Fatalf("Repaired=%v ManualReview=%v, want repaired without manual review", result.Repaired, result.ManualReview)
	}
	brief := readFileString(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"))
	if !strings.Contains(brief, "**Task ID**: TASK-1") {
		t.Fatalf("repaired brief does not reference active task TASK-1:\n%s", brief)
	}
	if !containsChangedFile(result.ChangedFiles, filepath.Join(paths.DougDir, "ACTIVE_TASK.md")) {
		t.Fatalf("ChangedFiles = %#v, want ACTIVE_TASK.md", result.ChangedFiles)
	}
	if !containsChangedField(result.ChangedFields, "active_brief.task_id") {
		t.Fatalf("ChangedFields = %#v, want active_brief.task_id", result.ChangedFields)
	}
}

func TestReconcileLifecycleRepairRewritesStaleActiveBriefAndReportsChanges(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "    id: TASK-1", "    id: TASK-2", 1)
	projectState = strings.Replace(projectState, "next_task:\n    type: feature\n    id: TASK-2", "next_task:\n    type: \"\"\n    id: \"\"", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusDone, types.StatusTODO))
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	testutil.WriteFile(t, briefPath, "**Task ID**: TASK-1\n")

	result, err := ReconcileLifecycle(Options{Paths: paths, MaxRetries: 3, BuildSystem: "go"}, ReconcileModeRepair)
	if err != nil {
		t.Fatalf("ReconcileLifecycle returned error: %v", err)
	}
	if !result.Repaired || len(result.Findings) != 0 {
		t.Fatalf("Repaired=%v Findings=%#v, want successful clean repair", result.Repaired, result.Findings)
	}
	brief := readFileString(t, briefPath)
	if !strings.Contains(brief, "**Task ID**: TASK-2") || strings.Contains(brief, "**Task ID**: TASK-1") {
		t.Fatalf("stale brief was not rewritten for active pointer TASK-2:\n%s", brief)
	}
	if !containsChangedFile(result.ChangedFiles, briefPath) {
		t.Fatalf("ChangedFiles = %#v, want brief path", result.ChangedFiles)
	}
	if !containsChangedField(result.ChangedFields, "active_brief.task_id") {
		t.Fatalf("ChangedFields = %#v, want active brief field", result.ChangedFields)
	}
}

func TestReconcileLifecycleRepairFixesNextPointerMismatchAndReportsLifecycleField(t *testing.T) {
	root := t.TempDir()
	projectState := strings.Replace(baseProjectState(), "    id: TASK-2", "    id: \"\"", 1)
	paths := writeLifecycleFixtures(t, root, projectState, baseTasks(types.StatusTODO, types.StatusTODO))
	testutil.WriteFile(t, filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), "**Task ID**: TASK-1\n")

	result, err := ReconcileLifecycle(Options{Paths: paths}, ReconcileModeRepair)
	if err != nil {
		t.Fatalf("ReconcileLifecycle returned error: %v", err)
	}
	if !result.Repaired {
		t.Fatalf("Repaired=false, result=%#v", result)
	}
	loadedState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		t.Fatalf("LoadProjectState: %v", err)
	}
	if loadedState.NextTask.ID != "TASK-2" {
		t.Fatalf("next_task.ID = %q, want TASK-2", loadedState.NextTask.ID)
	}
	if !containsChangedFile(result.ChangedFiles, paths.StatePath) {
		t.Fatalf("ChangedFiles = %#v, want project-state.yaml", result.ChangedFiles)
	}
	if !containsChangedField(result.ChangedFields, "next_task") {
		t.Fatalf("ChangedFields = %#v, want next_task", result.ChangedFields)
	}
}

func TestReconcileLifecycleRepairUnsupportedAmbiguousDriftDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	paths := writeLifecycleFixtures(t, root, baseProjectState(), baseTasks(types.StatusTODO, types.StatusTODO))
	briefPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	testutil.WriteFile(t, briefPath, "**Task ID**: TASK-2\n")
	stateBefore := readFileString(t, paths.StatePath)
	tasksBefore := readFileString(t, paths.TasksPath)
	briefBefore := readFileString(t, briefPath)

	result, err := ReconcileLifecycle(Options{Paths: paths}, ReconcileModeRepair)
	if err != nil {
		t.Fatalf("ReconcileLifecycle returned error: %v", err)
	}
	if !result.ManualReview || result.Repaired {
		t.Fatalf("ManualReview=%v Repaired=%v, want manual-review no-op", result.ManualReview, result.Repaired)
	}
	if got := readFileString(t, paths.StatePath); got != stateBefore {
		t.Fatal("project-state.yaml mutated for ambiguous drift")
	}
	if got := readFileString(t, paths.TasksPath); got != tasksBefore {
		t.Fatal("tasks.yaml mutated for ambiguous drift")
	}
	if got := readFileString(t, briefPath); got != briefBefore {
		t.Fatal("ACTIVE_TASK.md mutated for ambiguous drift")
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
