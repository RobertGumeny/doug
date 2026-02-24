package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/git"
)

// initGitRepo creates a temporary directory, initialises a git repository,
// configures a local user identity, and creates an initial commit.
// Returns the path to the repository root.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test Agent")

	// An initial commit is required so HEAD is valid before any reset/branch ops.
	writeTestFile(t, dir, "README.md", "# test repo\n")
	run("add", ".")
	run("commit", "-m", "initial commit")

	return dir
}

// writeTestFile writes contents to name inside dir.
func writeTestFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// readTestFile reads and returns the contents of name inside dir.
func readTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// currentBranchOf returns the name of the current branch in dir.
func currentBranchOf(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse --abbrev-ref HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// --- EnsureEpicBranch ---

func TestEnsureEpicBranch_AlreadyOnBranch_IsNoOp(t *testing.T) {
	dir := initGitRepo(t)
	current := currentBranchOf(t, dir)

	// Calling with the branch we're already on must succeed without error.
	if err := git.EnsureEpicBranch(current, dir); err != nil {
		t.Errorf("EnsureEpicBranch with current branch: %v", err)
	}
	// Branch must not have changed.
	if got := currentBranchOf(t, dir); got != current {
		t.Errorf("expected branch %q to be unchanged, got %q", current, got)
	}
}

func TestEnsureEpicBranch_ExistingBranch_ChecksOut(t *testing.T) {
	dir := initGitRepo(t)

	// Pre-create the branch without switching to it.
	run := exec.Command("git", "branch", "feature/existing")
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git branch feature/existing: %v\n%s", err, out)
	}

	if err := git.EnsureEpicBranch("feature/existing", dir); err != nil {
		t.Fatalf("EnsureEpicBranch: %v", err)
	}
	if got := currentBranchOf(t, dir); got != "feature/existing" {
		t.Errorf("expected branch %q, got %q", "feature/existing", got)
	}
}

func TestEnsureEpicBranch_NewBranch_CreatesAndChecksOut(t *testing.T) {
	dir := initGitRepo(t)

	if err := git.EnsureEpicBranch("feature/brand-new", dir); err != nil {
		t.Fatalf("EnsureEpicBranch: %v", err)
	}
	if got := currentBranchOf(t, dir); got != "feature/brand-new" {
		t.Errorf("expected branch %q, got %q", "feature/brand-new", got)
	}
}

// --- RollbackChanges ---

func TestRollbackChanges_ProtectedFilePreserved(t *testing.T) {
	dir := initGitRepo(t)

	// Commit an initial version of the protected file.
	writeTestFile(t, dir, "project-state.yaml", "version: committed\n")
	gitAddCommit(t, dir, "add project-state.yaml")

	// Modify the protected file (simulates agent writing state).
	writeTestFile(t, dir, "project-state.yaml", "version: modified\n")

	if err := git.RollbackChanges(dir, []string{"project-state.yaml"}); err != nil {
		t.Fatalf("RollbackChanges: %v", err)
	}

	// The modified content should be preserved — not reset to "version: committed".
	got := readTestFile(t, dir, "project-state.yaml")
	if got != "version: modified\n" {
		t.Errorf("expected protected file to retain modified content, got: %q", got)
	}
}

func TestRollbackChanges_UnprotectedTrackedFileReverted(t *testing.T) {
	dir := initGitRepo(t)

	// Commit an original version of a regular tracked file.
	writeTestFile(t, dir, "tracked.txt", "original\n")
	gitAddCommit(t, dir, "add tracked.txt")

	// Modify the tracked file without protecting it.
	writeTestFile(t, dir, "tracked.txt", "modified\n")

	if err := git.RollbackChanges(dir, []string{}); err != nil {
		t.Fatalf("RollbackChanges: %v", err)
	}

	// The file should be reverted to its committed state.
	// Normalise CRLF → LF to handle Windows git autocrlf behaviour.
	got := strings.ReplaceAll(readTestFile(t, dir, "tracked.txt"), "\r\n", "\n")
	if got != "original\n" {
		t.Errorf("expected tracked.txt to be reverted to original, got: %q", got)
	}
}

func TestRollbackChanges_UntrackedFileRemovedByClean(t *testing.T) {
	dir := initGitRepo(t)

	// Create an untracked file that is not in an excluded directory.
	writeTestFile(t, dir, "untracked.txt", "should be removed\n")

	if err := git.RollbackChanges(dir, []string{}); err != nil {
		t.Fatalf("RollbackChanges: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected untracked.txt to be removed by git clean, but it still exists")
	}
}

func TestRollbackChanges_MissingProtectedFileIsSkipped(t *testing.T) {
	dir := initGitRepo(t)

	// Protected path references a file that does not exist — must not error.
	if err := git.RollbackChanges(dir, []string{"nonexistent.yaml"}); err != nil {
		t.Errorf("RollbackChanges with missing protected file should not error: %v", err)
	}
}

func TestRollbackChanges_MultipleProtectedFilesAllPreserved(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "project-state.yaml", "state: committed\n")
	writeTestFile(t, dir, "tasks.yaml", "tasks: committed\n")
	gitAddCommit(t, dir, "add state files")

	writeTestFile(t, dir, "project-state.yaml", "state: modified\n")
	writeTestFile(t, dir, "tasks.yaml", "tasks: modified\n")

	if err := git.RollbackChanges(dir, []string{"project-state.yaml", "tasks.yaml"}); err != nil {
		t.Fatalf("RollbackChanges: %v", err)
	}

	if got := readTestFile(t, dir, "project-state.yaml"); got != "state: modified\n" {
		t.Errorf("project-state.yaml: expected modified content, got %q", got)
	}
	if got := readTestFile(t, dir, "tasks.yaml"); got != "tasks: modified\n" {
		t.Errorf("tasks.yaml: expected modified content, got %q", got)
	}
}

// --- Commit ---

func TestCommit_NothingToCommit_ReturnsErrNothingToCommit(t *testing.T) {
	dir := initGitRepo(t)

	// No changes since the initial commit.
	err := git.Commit("should fail gracefully", dir)
	if !errors.Is(err, git.ErrNothingToCommit) {
		t.Errorf("expected ErrNothingToCommit, got: %v", err)
	}
}

func TestCommit_WithChanges_CreatesCommit(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "new-file.txt", "hello\n")

	if err := git.Commit("add new-file.txt", dir); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify the commit appears in the log.
	logCmd := exec.Command("git", "log", "--oneline", "-1")
	logCmd.Dir = dir
	out, err := logCmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "add new-file.txt") {
		t.Errorf("expected commit message in log, got: %s", strings.TrimSpace(string(out)))
	}
}

func TestCommit_StagesAllChanges(t *testing.T) {
	dir := initGitRepo(t)

	// Add two files — neither is staged yet.
	writeTestFile(t, dir, "a.txt", "a\n")
	writeTestFile(t, dir, "b.txt", "b\n")

	if err := git.Commit("add a and b", dir); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Both files should be in the latest commit.
	showCmd := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	showCmd.Dir = dir
	out, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	files := string(out)
	if !strings.Contains(files, "a.txt") {
		t.Errorf("expected a.txt in commit, got: %s", files)
	}
	if !strings.Contains(files, "b.txt") {
		t.Errorf("expected b.txt in commit, got: %s", files)
	}
}

// gitAddCommit is a test helper that stages all files and creates a commit.
func gitAddCommit(t *testing.T, dir, message string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
