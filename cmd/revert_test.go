package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Shared types used only by revert tests.
// ---------------------------------------------------------------------------

type revertMetric struct {
	taskID    string
	commitSHA string // empty → field omitted (simulates missing CommitSHA)
}

type revertTask struct {
	id     string
	status string
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

func buildRevertStateYAML(epicID string, metrics []revertMetric) string {
	var sb strings.Builder
	sb.WriteString("current_epic:\n")
	sb.WriteString("  id: " + epicID + "\n")
	sb.WriteString("  name: Test Epic\n")
	sb.WriteString("  branch_name: main\n")
	sb.WriteString("  started_at: \"2024-01-01T00:00:00Z\"\n")
	sb.WriteString("metrics:\n")
	fmt.Fprintf(&sb, "  total_tasks_completed: %d\n", len(metrics))
	sb.WriteString("  total_duration_seconds: 0\n")
	sb.WriteString("  tasks:\n")
	for _, m := range metrics {
		sb.WriteString("  - task_id: " + m.taskID + "\n")
		sb.WriteString("    outcome: SUCCESS\n")
		sb.WriteString("    duration_seconds: 0\n")
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

// setupRevertEnv creates a git repository with two committed tasks (EPIC-1-001
// and EPIC-1-002) and one pending task (EPIC-1-003).  The on-disk
// project-state.yaml (uncommitted) carries CommitSHAs for both DONE tasks so
// doRevert can look them up without falling back to git log --grep.
func setupRevertEnv(t *testing.T) *revertEnv {
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

	// Initialise git repo with known identity.
	gitRun("init")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "Test Agent")

	// Initial commit (README).
	writeFile("README.md", "# test repo\n")
	gitRun("add", ".")
	gitRun("commit", "-m", "initial commit")

	// Pre-create the sessions directory (empty; git ignores empty dirs).
	sessionsDir := filepath.Join(dir, ".doug", "logs", "sessions", revertEpicID)
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	// ------------------------------------------------------------------
	// Commit 1 — task EPIC-1-001 completes.
	// Write tasks.yaml (001 DONE, 002 DONE, 003 TODO) and an initial
	// project-state.yaml (no metrics yet) plus the task source file.
	// ------------------------------------------------------------------
	writeFile(".doug/tasks.yaml", buildRevertTasksYAML(revertEpicID, []revertTask{
		{id: "EPIC-1-001", status: "DONE"},
		{id: "EPIC-1-002", status: "DONE"},
		{id: "EPIC-1-003", status: "TODO"},
	}))
	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, nil))
	writeFile("src/task001.go", "// task 001\n")

	gitRun("add", ".")
	gitRun("commit", "-m", "feat: EPIC-1-001")
	sha1 := gitSHA()

	// Update project-state.yaml on disk (not committed yet) so that sha1
	// is recorded as the CommitSHA for EPIC-1-001.
	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, []revertMetric{
		{taskID: "EPIC-1-001", commitSHA: sha1},
	}))

	// ------------------------------------------------------------------
	// Commit 2 — task EPIC-1-002 completes.
	// Stage the updated project-state.yaml (which now has sha1 for 001)
	// together with the task002 source file.
	// ------------------------------------------------------------------
	writeFile("src/task002.go", "// task 002\n")

	gitRun("add", ".")
	gitRun("commit", "-m", "feat: EPIC-1-002")
	sha2 := gitSHA()

	// Update project-state.yaml on disk (not committed) so that sha2 is
	// also recorded.  doRevert reads from disk, so both SHAs are visible.
	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, []revertMetric{
		{taskID: "EPIC-1-001", commitSHA: sha1},
		{taskID: "EPIC-1-002", commitSHA: sha2},
	}))

	return &revertEnv{
		dir:      dir,
		sha1:     sha1,
		sha2:     sha2,
		sessions: sessionsDir,
	}
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

// TestDoRevert_ValidDoneTask_RestoresStateAndWipesSessionLogs is the happy
// path: revert to EPIC-1-001 resets HEAD to sha1, deletes the session log for
// EPIC-1-002, and leaves the session log for EPIC-1-001 intact.
func TestDoRevert_ValidDoneTask_RestoresStateAndWipesSessionLogs(t *testing.T) {
	env := setupRevertEnv(t)

	log001 := createSessionLog(t, env.sessions, "EPIC-1-001")
	log002 := createSessionLog(t, env.sessions, "EPIC-1-002")

	if err := doRevert(env.dir, "EPIC-1-001", true); err != nil {
		t.Fatalf("doRevert: %v", err)
	}

	// HEAD must be at sha1.
	if got := headSHA(t, env.dir); got != env.sha1 {
		t.Errorf("expected HEAD %s, got %s", env.sha1, got)
	}

	// Session log for 002 (after the revert point) must be wiped.
	if _, err := os.Stat(log002); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected session log for EPIC-1-002 to be deleted")
	}

	// Session log for 001 (at the revert point) must survive.
	if _, err := os.Stat(log001); err != nil {
		t.Errorf("expected session log for EPIC-1-001 to survive: %v", err)
	}
}

// TestDoRevert_NonExistentTaskID_ReturnsError verifies that a task ID that
// does not appear in tasks.yaml produces a clear error.
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

// TestDoRevert_NonDoneTask_ReturnsError verifies that reverting to a task that
// is not in DONE status produces a clear error.
func TestDoRevert_NonDoneTask_ReturnsError(t *testing.T) {
	env := setupRevertEnv(t)

	err := doRevert(env.dir, "EPIC-1-003", true) // 003 is TODO
	if err == nil {
		t.Fatal("expected error for non-DONE task, got nil")
	}
	if !strings.Contains(err.Error(), "TODO") {
		t.Errorf("error should mention task status, got: %v", err)
	}
}

// TestDoRevert_DirtyWorkingTree_ErrorsWithoutForce verifies that a dirty
// working tree blocks the revert when --force is not passed.
func TestDoRevert_DirtyWorkingTree_ErrorsWithoutForce(t *testing.T) {
	env := setupRevertEnv(t)

	// Dirty the working tree by modifying a tracked file.
	if err := os.WriteFile(filepath.Join(env.dir, "README.md"), []byte("# dirty\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	err := doRevert(env.dir, "EPIC-1-001", false /* no force */)
	if err == nil {
		t.Fatal("expected error for dirty tree without --force, got nil")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error should mention uncommitted changes, got: %v", err)
	}

	// HEAD must NOT have moved.
	if got := headSHA(t, env.dir); got != env.sha2 {
		t.Errorf("HEAD should remain at sha2 %s after failed revert, got %s", env.sha2, got)
	}
}

// TestDoRevert_DirtyWorkingTree_SucceedsWithForce verifies that --force allows
// the revert to proceed even when the working tree is dirty.
func TestDoRevert_DirtyWorkingTree_SucceedsWithForce(t *testing.T) {
	env := setupRevertEnv(t)

	// Dirty the working tree.
	if err := os.WriteFile(filepath.Join(env.dir, "README.md"), []byte("# dirty\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	if err := doRevert(env.dir, "EPIC-1-001", true /* force */); err != nil {
		t.Fatalf("doRevert with --force on dirty tree: %v", err)
	}

	// HEAD must be at sha1.
	if got := headSHA(t, env.dir); got != env.sha1 {
		t.Errorf("expected HEAD %s after forced revert, got %s", env.sha1, got)
	}
}

// TestDoRevert_SessionLogsBeforeRevertPointSurvive verifies that session logs
// for tasks that precede the revert point are not deleted.
// Reverting to EPIC-1-002 must delete the log for EPIC-1-003 but keep 001 and 002.
func TestDoRevert_SessionLogsBeforeRevertPointSurvive(t *testing.T) {
	env := setupRevertEnv(t)

	log001 := createSessionLog(t, env.sessions, "EPIC-1-001")
	log002 := createSessionLog(t, env.sessions, "EPIC-1-002")
	log003 := createSessionLog(t, env.sessions, "EPIC-1-003")

	if err := doRevert(env.dir, "EPIC-1-002", true); err != nil {
		t.Fatalf("doRevert: %v", err)
	}

	// Logs at or before the revert point must survive.
	for _, path := range []string{log001, log002} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected session log %s to survive: %v", filepath.Base(path), err)
		}
	}

	// Log after the revert point must be wiped.
	if _, err := os.Stat(log003); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected session log for EPIC-1-003 to be deleted")
	}
}

// TestDoRevert_MissingCommitSHA_FallsBackToGrep verifies that when
// CommitSHA is absent from metrics, doRevert falls back to git log --grep
// and succeeds (no hard error).  The task ID appears in the commit message,
// so the fallback finds the correct SHA.
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

	// Initialise git repo.
	gitRun("init")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "Test Agent")

	// Initial commit.
	writeFile("README.md", "# test\n")
	gitRun("add", ".")
	gitRun("commit", "-m", "initial commit")

	// Ensure sessions directory exists.
	sessionsDir := filepath.Join(dir, ".doug", "logs", "sessions", revertEpicID)
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	// Write tasks.yaml with EPIC-1-001 DONE.
	writeFile(".doug/tasks.yaml", buildRevertTasksYAML(revertEpicID, []revertTask{
		{id: "EPIC-1-001", status: "DONE"},
	}))

	// Write project-state.yaml with EMPTY CommitSHA (simulates missing SHA).
	writeFile(".doug/project-state.yaml", buildRevertStateYAML(revertEpicID, []revertMetric{
		{taskID: "EPIC-1-001", commitSHA: ""}, // empty → grep fallback triggered
	}))

	writeFile("src/task001.go", "// task 001\n")

	// Commit message includes task ID so the grep fallback can find it.
	gitRun("add", ".")
	gitRun("commit", "-m", "feat: EPIC-1-001")

	// doRevert must succeed (not return an error) when CommitSHA is missing
	// but the commit can be found via git log --grep.
	if err := doRevert(dir, "EPIC-1-001", true); err != nil {
		t.Fatalf("expected grep fallback to succeed, got error: %v", err)
	}
}
