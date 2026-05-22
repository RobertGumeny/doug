package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Shared types used only by revert tests.
// ---------------------------------------------------------------------------

type revertMetric struct {
	taskID          string
	commitSHA       string // empty → field omitted (simulates missing CommitSHA)
	durationSeconds int
	outcome         string
}

type revertTask struct {
	id     string
	status string
}

type revertPointer struct {
	typeName                string
	id                      string
	attempts                int
	consecutiveTestFailures int
	testFailureOutput       string
}

type revertStateSpec struct {
	status      string
	completedAt string
	active      revertPointer
	next        revertPointer
	metrics     []revertMetric
}

// ---------------------------------------------------------------------------
// YAML builders
// ---------------------------------------------------------------------------

func buildRevertTasksYAML(epicID string, tasks []revertTask) string {
	var sb strings.Builder
	sb.WriteString("epic:\n")
	sb.WriteString("  id: " + epicID + "\n")
	sb.WriteString("  name: Test Epic\n")
	sb.WriteString("  tasks:\n")
	for _, t := range tasks {
		sb.WriteString("  - id: " + t.id + "\n")
		sb.WriteString("    type: feature\n")
		sb.WriteString("    status: " + t.status + "\n")
		sb.WriteString("    description: task " + t.id + "\n")
	}
	return sb.String()
}

func buildRevertStateYAML(epicID string, spec revertStateSpec) string {
	var sb strings.Builder
	sb.WriteString("current_epic:\n")
	sb.WriteString("  id: " + epicID + "\n")
	sb.WriteString("  name: Test Epic\n")
	sb.WriteString("  branch_name: main\n")
	sb.WriteString("  started_at: \"2024-01-01T00:00:00Z\"\n")
	if spec.completedAt != "" {
		sb.WriteString("  completed_at: \"" + spec.completedAt + "\"\n")
	}
	if spec.status != "" {
		sb.WriteString("status: " + spec.status + "\n")
	}
	if spec.active.id != "" || spec.active.typeName != "" {
		sb.WriteString("active_task:\n")
		sb.WriteString("  type: " + spec.active.typeName + "\n")
		sb.WriteString("  id: " + spec.active.id + "\n")
		if spec.active.attempts > 0 {
			fmt.Fprintf(&sb, "  attempts: %d\n", spec.active.attempts)
		}
		if spec.active.consecutiveTestFailures > 0 {
			fmt.Fprintf(&sb, "  consecutive_test_failures: %d\n", spec.active.consecutiveTestFailures)
		}
		if spec.active.testFailureOutput != "" {
			sb.WriteString("  test_failure_output: |\n")
			for _, line := range strings.Split(strings.TrimRight(spec.active.testFailureOutput, "\n"), "\n") {
				sb.WriteString("    " + line + "\n")
			}
		}
	}
	if spec.next.id != "" || spec.next.typeName != "" {
		sb.WriteString("next_task:\n")
		sb.WriteString("  type: " + spec.next.typeName + "\n")
		sb.WriteString("  id: " + spec.next.id + "\n")
	}
	totalDuration := 0
	for _, m := range spec.metrics {
		totalDuration += m.durationSeconds
	}
	sb.WriteString("metrics:\n")
	fmt.Fprintf(&sb, "  total_tasks_completed: %d\n", len(spec.metrics))
	fmt.Fprintf(&sb, "  total_duration_seconds: %d\n", totalDuration)
	sb.WriteString("  tasks:\n")
	for _, m := range spec.metrics {
		outcome := m.outcome
		if outcome == "" {
			outcome = "SUCCESS"
		}
		sb.WriteString("  - task_id: " + m.taskID + "\n")
		sb.WriteString("    outcome: " + outcome + "\n")
		fmt.Fprintf(&sb, "    duration_seconds: %d\n", m.durationSeconds)
		sb.WriteString("    completed_at: \"2024-01-01T00:00:00Z\"\n")
		if m.commitSHA != "" {
			sb.WriteString("    commit_sha: " + m.commitSHA + "\n")
		}
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Test environment setup
// ---------------------------------------------------------------------------

const revertEpicID = "EPIC-1"

// revertEnv holds paths and SHAs produced by setupRevertEnv.
type revertEnv struct {
	dir      string // repo root
	sha1     string // HEAD after "feat: EPIC-1-001" commit
	sha2     string // HEAD after "feat: EPIC-1-002" commit
	sessions string // .doug/logs/sessions/EPIC-1/
}

func setupRevertEnv(t *testing.T) *revertEnv {
	t.Helper()
	return setupRevertEnvWithDougTracking(t, true)
}

func setupRevertEnvWithIgnoredDoug(t *testing.T) *revertEnv {
	t.Helper()
	return setupRevertEnvWithDougTracking(t, false)
}

// setupRevertEnvWithDougTracking creates a git repository with two committed
// tasks (EPIC-1-001 and EPIC-1-002) and one pending task (EPIC-1-003).
// When trackDoug is false, .doug/ is ignored and remains local-only.
func setupRevertEnvWithDougTracking(t *testing.T, trackDoug bool) *revertEnv {
	t.Helper()

	dir := t.TempDir()

	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitSHA := func() string {
		t.Helper()
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git rev-parse HEAD: %v", err)
		}
		return strings.TrimSpace(string(out))
	}

	writeFile := func(relPath, content string) {
		t.Helper()
		path := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", relPath, err)
		}
	}

	gitRun("init")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "Test Agent")

	writeFile("README.md", "# test repo\n")
	if !trackDoug {
		writeFile(".gitignore", ".doug/\n")
	}
	gitRun("add", ".")
	gitRun("commit", "-m", "initial commit")

	sessionsDir := filepath.Join(dir, ".doug", "logs", "sessions", revertEpicID)
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	writeFile(".doug/tasks.yaml", buildRevertTasksYAML(revertEpicID, []revertTask{
		{id: "EPIC-1-001", status: "DONE"},
		{id: "EPIC-1-002", status: "DONE"},
		{id: "EPIC-1-003", status: "TODO"},
	}))
	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, revertStateSpec{}))
	writeFile("src/task001.go", "// task 001\n")

	gitRun("add", ".")
	gitRun("commit", "-m", "feat: EPIC-1-001")
	sha1 := gitSHA()

	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, revertStateSpec{
		metrics: []revertMetric{{taskID: "EPIC-1-001", commitSHA: sha1, durationSeconds: 11}},
	}))

	writeFile("src/task002.go", "// task 002\n")

	gitRun("add", ".")
	gitRun("commit", "-m", "feat: EPIC-1-002")
	sha2 := gitSHA()

	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, revertStateSpec{
		metrics: []revertMetric{
			{taskID: "EPIC-1-001", commitSHA: sha1, durationSeconds: 11},
			{taskID: "EPIC-1-002", commitSHA: sha2, durationSeconds: 22},
		},
	}))

	return &revertEnv{
		dir:      dir,
		sha1:     sha1,
		sha2:     sha2,
		sessions: sessionsDir,
	}
}

func writeRevertFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func loadRevertTasks(t *testing.T, dir string) *types.Tasks {
	t.Helper()
	tasks, err := state.LoadTasks(filepath.Join(dir, ".doug", "tasks.yaml"))
	if err != nil {
		t.Fatalf("load tasks.yaml: %v", err)
	}
	return tasks
}

func loadRevertState(t *testing.T, dir string) *types.ProjectState {
	t.Helper()
	projectState, err := state.LoadProjectState(filepath.Join(dir, ".doug", "project-state.yaml"))
	if err != nil {
		t.Fatalf("load project-state.yaml: %v", err)
	}
	return projectState
}

// createSessionLog writes a minimal session log file and returns its path.
func createSessionLog(t *testing.T, sessionsDir, taskID string) string {
	t.Helper()
	name := fmt.Sprintf("session-%s_attempt-1.md", taskID)
	path := filepath.Join(sessionsDir, name)
	if err := os.WriteFile(path, []byte("# session "+taskID+"\n"), 0644); err != nil {
		t.Fatalf("write session log %s: %v", name, err)
	}
	return path
}

// headSHA returns the current HEAD SHA of the repo at dir.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

func TestDoRevert_ValidDoneTask_RewritesDougStateAndWipesSessionLogs(t *testing.T) {
	env := setupRevertEnv(t)

	writeRevertFile(t, env.dir, ".doug/tasks.yaml", buildRevertTasksYAML(revertEpicID, []revertTask{
		{id: "EPIC-1-001", status: "DONE"},
		{id: "EPIC-1-002", status: "BLOCKED"},
		{id: "EPIC-1-003", status: "IN_PROGRESS"},
	}))
	writeRevertFile(t, env.dir, ".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, revertStateSpec{
		status:      string(types.ProjectStatusPaused),
		completedAt: "2024-01-02T00:00:00Z",
		active: revertPointer{
			typeName:                "feature",
			id:                      "EPIC-1-003",
			attempts:                4,
			consecutiveTestFailures: 2,
			testFailureOutput:       "boom\ntrace",
		},
		metrics: []revertMetric{
			{taskID: "EPIC-1-001", commitSHA: env.sha1, durationSeconds: 11},
			{taskID: "EPIC-1-002", commitSHA: env.sha2, durationSeconds: 22, outcome: string(types.OutcomeBuildFailure)},
		},
	}))

	log001 := createSessionLog(t, env.sessions, "EPIC-1-001")
	log002 := createSessionLog(t, env.sessions, "EPIC-1-002")
	log003 := createSessionLog(t, env.sessions, "EPIC-1-003")

	if err := doRevert(env.dir, "EPIC-1-001", true); err != nil {
		t.Fatalf("doRevert: %v", err)
	}

	if got := headSHA(t, env.dir); got != env.sha1 {
		t.Errorf("expected HEAD %s, got %s", env.sha1, got)
	}

	tasks := loadRevertTasks(t, env.dir)
	gotStatuses := []types.Status{
		tasks.Epic.Tasks[0].Status,
		tasks.Epic.Tasks[1].Status,
		tasks.Epic.Tasks[2].Status,
	}
	wantStatuses := []types.Status{types.StatusDone, types.StatusTODO, types.StatusTODO}
	for i := range wantStatuses {
		if gotStatuses[i] != wantStatuses[i] {
			t.Fatalf("task %d status: got %q, want %q", i, gotStatuses[i], wantStatuses[i])
		}
	}

	projectState := loadRevertState(t, env.dir)
	if projectState.Status != "" {
		t.Fatalf("status should be cleared, got %q", projectState.Status)
	}
	if projectState.CurrentEpic.CompletedAt != nil {
		t.Fatalf("current_epic.completed_at should be cleared, got %v", *projectState.CurrentEpic.CompletedAt)
	}
	if projectState.Metrics.TotalTasksCompleted != 1 {
		t.Fatalf("TotalTasksCompleted: got %d, want 1", projectState.Metrics.TotalTasksCompleted)
	}
	if projectState.Metrics.TotalDurationSeconds != 11 {
		t.Fatalf("TotalDurationSeconds: got %d, want 11", projectState.Metrics.TotalDurationSeconds)
	}
	if len(projectState.Metrics.Tasks) != 1 || projectState.Metrics.Tasks[0].TaskID != "EPIC-1-001" {
		t.Fatalf("metrics should be trimmed to EPIC-1-001, got %+v", projectState.Metrics.Tasks)
	}
	if projectState.ActiveTask.ID != "EPIC-1-002" || projectState.ActiveTask.Type != types.TaskTypeFeature {
		t.Fatalf("active_task not rebuilt correctly: %+v", projectState.ActiveTask)
	}
	if projectState.ActiveTask.Attempts != 0 || projectState.ActiveTask.ConsecutiveTestFailures != 0 || projectState.ActiveTask.TestFailureOutput != "" {
		t.Fatalf("active_task transient fields should be cleared: %+v", projectState.ActiveTask)
	}
	if projectState.NextTask.ID != "EPIC-1-003" || projectState.NextTask.Type != types.TaskTypeFeature {
		t.Fatalf("next_task not rebuilt correctly: %+v", projectState.NextTask)
	}

	if _, err := os.Stat(log001); err != nil {
		t.Errorf("expected session log for EPIC-1-001 to survive: %v", err)
	}
	for _, path := range []string{log002, log003} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected session log %s to be deleted", filepath.Base(path))
		}
	}
}

func TestDoRevert_IgnoredUntrackedDoug_SucceedsAndRewritesLocalState(t *testing.T) {
	env := setupRevertEnvWithIgnoredDoug(t)

	log001 := createSessionLog(t, env.sessions, "EPIC-1-001")
	log002 := createSessionLog(t, env.sessions, "EPIC-1-002")

	if err := doRevert(env.dir, "EPIC-1-001", true); err != nil {
		t.Fatalf("doRevert with ignored local .doug/: %v", err)
	}

	if got := headSHA(t, env.dir); got != env.sha1 {
		t.Fatalf("expected HEAD %s, got %s", env.sha1, got)
	}

	tasks := loadRevertTasks(t, env.dir)
	if tasks.Epic.Tasks[1].Status != types.StatusTODO || tasks.Epic.Tasks[2].Status != types.StatusTODO {
		t.Fatalf("tasks after revert point should reset to TODO, got %+v", tasks.Epic.Tasks)
	}

	projectState := loadRevertState(t, env.dir)
	if projectState.ActiveTask.ID != "EPIC-1-002" || projectState.NextTask.ID != "EPIC-1-003" {
		t.Fatalf("task pointers should be rebuilt from local state, got active=%+v next=%+v", projectState.ActiveTask, projectState.NextTask)
	}
	if len(projectState.Metrics.Tasks) != 1 || projectState.Metrics.Tasks[0].TaskID != "EPIC-1-001" {
		t.Fatalf("metrics should be trimmed after revert, got %+v", projectState.Metrics.Tasks)
	}

	if _, err := os.Stat(log001); err != nil {
		t.Fatalf("expected session log for EPIC-1-001 to survive: %v", err)
	}
	if _, err := os.Stat(log002); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected session log for EPIC-1-002 to be deleted")
	}
}

func TestDoRevert_LastTask_ClearsTaskPointers(t *testing.T) {
	env := setupRevertEnv(t)

	writeRevertFile(t, env.dir, ".doug/tasks.yaml", buildRevertTasksYAML(revertEpicID, []revertTask{
		{id: "EPIC-1-001", status: "DONE"},
		{id: "EPIC-1-002", status: "DONE"},
		{id: "EPIC-1-003", status: "DONE"},
	}))
	writeRevertFile(t, env.dir, ".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, revertStateSpec{
		completedAt: "2024-01-02T00:00:00Z",
		metrics: []revertMetric{
			{taskID: "EPIC-1-001", commitSHA: env.sha1, durationSeconds: 11},
			{taskID: "EPIC-1-002", commitSHA: env.sha2, durationSeconds: 22},
			{taskID: "EPIC-1-003", commitSHA: env.sha2, durationSeconds: 33},
		},
	}))

	if err := doRevert(env.dir, "EPIC-1-003", true); err != nil {
		t.Fatalf("doRevert: %v", err)
	}

	projectState := loadRevertState(t, env.dir)
	if projectState.ActiveTask.ID != "" || projectState.NextTask.ID != "" {
		t.Fatalf("task pointers should be empty after reverting to last task: active=%+v next=%+v", projectState.ActiveTask, projectState.NextTask)
	}
}

func TestDoRevert_NonExistentTaskID_ReturnsError(t *testing.T) {
	env := setupRevertEnv(t)

	err := doRevert(env.dir, "EPIC-1-999", true)
	if err == nil {
		t.Fatal("expected error for non-existent task ID, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestDoRevert_NonDoneTask_ReturnsError(t *testing.T) {
	env := setupRevertEnv(t)

	err := doRevert(env.dir, "EPIC-1-003", true)
	if err == nil {
		t.Fatal("expected error for non-DONE task, got nil")
	}
	if !strings.Contains(err.Error(), "TODO") {
		t.Errorf("error should mention task status, got: %v", err)
	}
}

func TestDoRevert_DirtyWorkingTree_ErrorsWithoutForce(t *testing.T) {
	env := setupRevertEnv(t)

	if err := os.WriteFile(filepath.Join(env.dir, "README.md"), []byte("# dirty\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	err := doRevert(env.dir, "EPIC-1-001", false)
	if err == nil {
		t.Fatal("expected error for dirty tree without --force, got nil")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error should mention uncommitted changes, got: %v", err)
	}
	if got := headSHA(t, env.dir); got != env.sha2 {
		t.Errorf("HEAD should remain at sha2 %s after failed revert, got %s", env.sha2, got)
	}
}

func TestDoRevert_DirtyWorkingTree_SucceedsWithForce(t *testing.T) {
	env := setupRevertEnv(t)

	if err := os.WriteFile(filepath.Join(env.dir, "README.md"), []byte("# dirty\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	if err := doRevert(env.dir, "EPIC-1-001", true); err != nil {
		t.Fatalf("doRevert with --force on dirty tree: %v", err)
	}
	if got := headSHA(t, env.dir); got != env.sha1 {
		t.Errorf("expected HEAD %s after forced revert, got %s", env.sha1, got)
	}
}

func TestDoRevert_SessionLogsBeforeRevertPointSurvive(t *testing.T) {
	env := setupRevertEnv(t)

	log001 := createSessionLog(t, env.sessions, "EPIC-1-001")
	log002 := createSessionLog(t, env.sessions, "EPIC-1-002")
	log003 := createSessionLog(t, env.sessions, "EPIC-1-003")

	if err := doRevert(env.dir, "EPIC-1-002", true); err != nil {
		t.Fatalf("doRevert: %v", err)
	}

	for _, path := range []string{log001, log002} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected session log %s to survive: %v", filepath.Base(path), err)
		}
	}
	if _, err := os.Stat(log003); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected session log for EPIC-1-003 to be deleted")
	}
}

func TestDoRevert_MissingCommitSHA_FallsBackToGrep(t *testing.T) {
	dir := t.TempDir()

	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	writeFile := func(relPath, content string) {
		t.Helper()
		path := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", relPath, err)
		}
	}

	gitRun("init")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "Test Agent")

	writeFile("README.md", "# test\n")
	gitRun("add", ".")
	gitRun("commit", "-m", "initial commit")

	sessionsDir := filepath.Join(dir, ".doug", "logs", "sessions", revertEpicID)
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	writeFile(".doug/tasks.yaml", buildRevertTasksYAML(revertEpicID, []revertTask{{id: "EPIC-1-001", status: "DONE"}}))
	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, revertStateSpec{
		metrics: []revertMetric{{taskID: "EPIC-1-001", commitSHA: ""}},
	}))
	writeFile("src/task001.go", "// task 001\n")

	gitRun("add", ".")
	gitRun("commit", "-m", "feat: EPIC-1-001")

	if err := doRevert(dir, "EPIC-1-001", true); err != nil {
		t.Fatalf("expected grep fallback to succeed, got error: %v", err)
	}
}
