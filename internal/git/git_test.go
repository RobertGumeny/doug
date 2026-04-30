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

func TestRollbackChanges_UntrackedProtectedFilePreserved(t *testing.T) {
	dir := initGitRepo(t)

	// Write the protected file without committing — it remains untracked.
	// This simulates tasks.yaml/project-state.yaml written during a run before
	// they have ever been committed. git clean -fd would delete untracked files,
	// so the restore step must happen AFTER the clean, not before.
	writeTestFile(t, dir, "tasks.yaml", "status: todo\n")

	if err := git.RollbackChanges(dir, []string{"tasks.yaml"}); err != nil {
		t.Fatalf("RollbackChanges: %v", err)
	}

	// The untracked protected file must survive the rollback.
	got := readTestFile(t, dir, "tasks.yaml")
	if got != "status: todo\n" {
		t.Errorf("expected untracked protected file to be preserved, got: %q", got)
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

func TestCommit_WithGuardedGeneratedDir_ReturnsActionableError(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "main.go", "package main\n")
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "left-pad"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	writeTestFile(t, dir, "node_modules/left-pad/index.js", "module.exports = 1\n")

	err := git.Commit("feat: add project files", dir)
	if err == nil {
		t.Fatal("expected guarded generated directory error, got nil")
	}
	if !errors.Is(err, git.ErrGuardedPath) {
		t.Fatalf("expected ErrGuardedPath, got: %v", err)
	}
	for _, want := range []string{"node_modules/", ".gitignore", "remove it from git tracking"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to mention %q, got: %v", want, err)
		}
	}

	statusCmd := exec.Command("git", "status", "--short")
	statusCmd.Dir = dir
	out, statusErr := statusCmd.Output()
	if statusErr != nil {
		t.Fatalf("git status --short: %v", statusErr)
	}
	if strings.Contains(string(out), "A  node_modules/left-pad/index.js") {
		t.Fatalf("guard should fail before staging node_modules; status was:\n%s", out)
	}
}

func TestCommit_IgnoresGuardedDirWhenGitignoreIsCorrect(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, ".gitignore", "node_modules/\n")
	writeTestFile(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	writeTestFile(t, dir, "node_modules/pkg/index.js", "module.exports = true\n")

	if err := git.Commit("add package metadata", dir); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	showCmd := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	showCmd.Dir = dir
	out, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	files := string(out)
	if strings.Contains(files, "node_modules/") {
		t.Fatalf("expected ignored node_modules to stay out of commit, got:\n%s", files)
	}
	if !strings.Contains(files, ".gitignore") || !strings.Contains(files, "package.json") {
		t.Fatalf("expected legit project files in commit, got:\n%s", files)
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

func TestPendingPaths_ReturnsSortedChangedPaths(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "z.txt", "new\n")
	writeTestFile(t, dir, "a.txt", "new\n")
	writeTestFile(t, dir, "README.md", "updated\n")

	paths, err := git.PendingPaths(dir)
	if err != nil {
		t.Fatalf("PendingPaths: %v", err)
	}

	want := []string{"README.md", "a.txt", "z.txt"}
	if len(paths) != len(want) {
		t.Fatalf("PendingPaths len = %d, want %d (%v)", len(paths), len(want), paths)
	}
	for i, got := range paths {
		if got != want[i] {
			t.Fatalf("PendingPaths[%d] = %q, want %q (all=%v)", i, got, want[i], paths)
		}
	}
}

// --- CurrentSHA ---

func TestCurrentSHA_ReturnsHEADSHA(t *testing.T) {
	dir := initGitRepo(t)

	sha, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA: %v", err)
	}

	// SHA must be a non-empty hex string (40 characters for full SHA).
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len=%d)", sha, len(sha))
	}
	for _, c := range sha {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("SHA contains unexpected character %q: %s", c, sha)
			break
		}
	}
}

func TestCurrentSHA_UpdatesAfterNewCommit(t *testing.T) {
	dir := initGitRepo(t)

	sha1, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA before second commit: %v", err)
	}

	writeTestFile(t, dir, "extra.txt", "content\n")
	if err := git.Commit("second commit", dir); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	sha2, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA after second commit: %v", err)
	}

	if sha1 == sha2 {
		t.Errorf("expected SHA to change after new commit, but both are %q", sha1)
	}
}

func TestCurrentSHA_NotGitRepo_ReturnsError(t *testing.T) {
	dir := t.TempDir() // plain directory, not a git repo

	_, err := git.CurrentSHA(dir)
	if err == nil {
		t.Error("expected error for non-git directory, got nil")
	}
}

// --- ResetHard ---

func TestResetHard_RevertsToGivenSHA(t *testing.T) {
	dir := initGitRepo(t)

	// Capture the SHA of the initial commit.
	sha1, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA: %v", err)
	}

	// Make a second commit so HEAD advances.
	writeTestFile(t, dir, "extra.txt", "content\n")
	gitAddCommit(t, dir, "second commit")

	sha2, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA after second commit: %v", err)
	}
	if sha1 == sha2 {
		t.Fatal("SHAs should differ after second commit")
	}

	// Reset back to the initial commit.
	if err := git.ResetHard(sha1, dir); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}

	got, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA after ResetHard: %v", err)
	}
	if got != sha1 {
		t.Errorf("expected HEAD to be %s after ResetHard, got %s", sha1, got)
	}
}

func TestResetHard_InvalidSHA_ReturnsError(t *testing.T) {
	dir := initGitRepo(t)

	err := git.ResetHard("0000000000000000000000000000000000000000", dir)
	if err == nil {
		t.Error("expected error for invalid SHA, got nil")
	}
	if !strings.Contains(err.Error(), "ResetHard") {
		t.Errorf("error should mention ResetHard, got: %v", err)
	}
}

func TestResetHard_DoesNotChangeRollbackChanges(t *testing.T) {
	// Verify that RollbackChanges still resets to HEAD (not affected by ResetHard).
	dir := initGitRepo(t)

	writeTestFile(t, dir, "file.txt", "committed\n")
	gitAddCommit(t, dir, "add file.txt")

	writeTestFile(t, dir, "file.txt", "modified\n")

	if err := git.RollbackChanges(dir, []string{}); err != nil {
		t.Fatalf("RollbackChanges: %v", err)
	}

	got := strings.ReplaceAll(readTestFile(t, dir, "file.txt"), "\r\n", "\n")
	if got != "committed\n" {
		t.Errorf("RollbackChanges should still revert to HEAD, got %q", got)
	}
}

// --- CurrentBranch ---

func TestCurrentBranch_ReturnsCurrentBranch(t *testing.T) {
	dir := initGitRepo(t)

	got, err := git.CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty branch name")
	}
	// Must match what git itself reports.
	if want := currentBranchOf(t, dir); got != want {
		t.Errorf("CurrentBranch = %q, want %q", got, want)
	}
}

// --- HasUncommittedChanges ---

func TestHasUncommittedChanges_CleanTree_ReturnsFalse(t *testing.T) {
	dir := initGitRepo(t)

	dirty, err := git.HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if dirty {
		t.Error("expected clean working tree to report no uncommitted changes")
	}
}

func TestHasUncommittedChanges_ModifiedTrackedFile_ReturnsTrue(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "README.md", "# modified\n")

	dirty, err := git.HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !dirty {
		t.Error("expected modified tracked file to report uncommitted changes")
	}
}

func TestHasUncommittedChanges_UntrackedFile_ReturnsTrue(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "newfile.txt", "content\n")

	dirty, err := git.HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !dirty {
		t.Error("expected untracked file to report uncommitted changes")
	}
}

// --- LookupCommitByGrep ---

func TestLookupCommitByGrep_MatchingCommit_ReturnsSHA(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "task.txt", "done\n")
	gitAddCommit(t, dir, "feat: EPIC-1-001")

	sha, err := git.LookupCommitByGrep("EPIC-1-001", dir)
	if err != nil {
		t.Fatalf("LookupCommitByGrep: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q", sha)
	}
}

func TestLookupCommitByGrep_NoMatchingCommit_ReturnsEmpty(t *testing.T) {
	dir := initGitRepo(t)

	sha, err := git.LookupCommitByGrep("EPIC-99-999", dir)
	if err != nil {
		t.Fatalf("LookupCommitByGrep: %v", err)
	}
	if sha != "" {
		t.Errorf("expected empty SHA for non-matching grep, got %q", sha)
	}
}

func TestLookupCommitByGrep_ReturnsLatestMatch(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "a.txt", "a\n")
	gitAddCommit(t, dir, "feat: EPIC-1-001 first attempt")

	writeTestFile(t, dir, "b.txt", "b\n")
	gitAddCommit(t, dir, "feat: EPIC-1-001 second attempt")

	sha2, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA: %v", err)
	}

	got, err := git.LookupCommitByGrep("EPIC-1-001", dir)
	if err != nil {
		t.Fatalf("LookupCommitByGrep: %v", err)
	}
	if got != sha2 {
		t.Errorf("expected latest matching commit %s, got %s", sha2, got)
	}
}

// --- SHAExists ---

func TestSHAExists_ValidSHA_ReturnsTrue(t *testing.T) {
	dir := initGitRepo(t)

	sha, err := git.CurrentSHA(dir)
	if err != nil {
		t.Fatalf("CurrentSHA: %v", err)
	}

	exists, err := git.SHAExists(sha, dir)
	if err != nil {
		t.Fatalf("SHAExists: %v", err)
	}
	if !exists {
		t.Errorf("expected existing commit %s to be found", sha)
	}
}

func TestSHAExists_InvalidSHA_ReturnsFalse(t *testing.T) {
	dir := initGitRepo(t)

	exists, err := git.SHAExists("0000000000000000000000000000000000000000", dir)
	if err != nil {
		t.Fatalf("SHAExists: %v", err)
	}
	if exists {
		t.Error("expected non-existent SHA to return false")
	}
}

// --- IsFileTracked ---

func TestIsFileTracked_CommittedFile_ReturnsTrue(t *testing.T) {
	dir := initGitRepo(t)

	tracked, err := git.IsFileTracked("README.md", dir)
	if err != nil {
		t.Fatalf("IsFileTracked: %v", err)
	}
	if !tracked {
		t.Error("expected committed README.md to be tracked")
	}
}

func TestIsFileTracked_UntrackedFile_ReturnsFalse(t *testing.T) {
	dir := initGitRepo(t)

	writeTestFile(t, dir, "untracked.txt", "content\n")

	tracked, err := git.IsFileTracked("untracked.txt", dir)
	if err != nil {
		t.Fatalf("IsFileTracked: %v", err)
	}
	if tracked {
		t.Error("expected untracked file to return false")
	}
}

func TestIsFileTracked_NonexistentFile_ReturnsFalse(t *testing.T) {
	dir := initGitRepo(t)

	tracked, err := git.IsFileTracked("does-not-exist.txt", dir)
	if err != nil {
		t.Fatalf("IsFileTracked: %v", err)
	}
	if tracked {
		t.Error("expected non-existent file to return false")
	}
}

// --- HasRemoteTrackingBranch ---

func TestHasRemoteTrackingBranch_NoUpstream_ReturnsFalse(t *testing.T) {
	dir := initGitRepo(t)

	// A fresh local repo with no remotes has no upstream.
	has, err := git.HasRemoteTrackingBranch(currentBranchOf(t, dir), dir)
	if err != nil {
		t.Fatalf("HasRemoteTrackingBranch: %v", err)
	}
	if has {
		t.Error("expected no remote tracking branch for local-only repo")
	}
}

func TestHasRemoteTrackingBranch_WithUpstream_ReturnsTrue(t *testing.T) {
	// Create an "origin" bare repo and clone it so the clone has a tracking branch.
	origin := t.TempDir()
	clone := t.TempDir()

	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}

	// Initialise bare origin with an initial commit.
	runIn(origin, "init", "--bare")
	tmp := t.TempDir()
	runIn(tmp, "init")
	runIn(tmp, "config", "user.email", "test@example.com")
	runIn(tmp, "config", "user.name", "Test Agent")
	writeTestFile(t, tmp, "README.md", "# origin\n")
	runIn(tmp, "add", ".")
	runIn(tmp, "commit", "-m", "initial")
	runIn(tmp, "remote", "add", "origin", origin)
	runIn(tmp, "push", "origin", "HEAD:main")

	// Clone to get a proper tracking branch.
	runIn(clone, "init")
	runIn(clone, "config", "user.email", "test@example.com")
	runIn(clone, "config", "user.name", "Test Agent")
	runIn(clone, "remote", "add", "origin", origin)
	runIn(clone, "fetch", "origin")
	runIn(clone, "checkout", "-b", "main", "--track", "origin/main")

	has, err := git.HasRemoteTrackingBranch("main", clone)
	if err != nil {
		t.Fatalf("HasRemoteTrackingBranch: %v", err)
	}
	if !has {
		t.Error("expected remote tracking branch to be detected after clone")
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
