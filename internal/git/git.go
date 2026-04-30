// Package git provides Git operations for the orchestrator.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// ErrNothingToCommit is returned by Commit when there are no changes to commit.
// Callers should treat this as non-fatal.
var ErrNothingToCommit = errors.New("nothing to commit")

// ErrGuardedPath would be committed when a generated dependency or build
// directory from the guarded set is present in the pending git changes.
var ErrGuardedPath = errors.New("guarded generated directory would be committed")

// DefaultProtectedPaths are the orchestrator state files that must be
// preserved across a git rollback so the orchestrator does not lose its place
// after a bad agent run. This is the single source of truth for protected
// paths; handlers should reference this var rather than defining their own.
var DefaultProtectedPaths = []string{
	".doug/project-state.yaml",
	".doug/tasks.yaml",
}

// defaultCleanExcludes are the patterns passed to git clean --exclude during
// a rollback. Keeping them here alongside DefaultProtectedPaths ensures both
// sets of .doug/ path literals live in one place.
var defaultCleanExcludes = []string{
	"--exclude=.doug/",
	"--exclude=docs/kb/",
	"--exclude=.env",
	"--exclude=*.backup",
}

// guardedGeneratedDirNames is the deterministic set of common generated
// directories Doug refuses to commit when ignore hygiene is missing.
var guardedGeneratedDirNames = []string{
	".next",
	".nuxt",
	".svelte-kit",
	"build",
	"coverage",
	"dist",
	"node_modules",
}

// EnsureEpicBranch ensures the working tree is on branchName.
//   - If already on branchName: no-op.
//   - If branchName exists locally: git checkout branchName.
//   - If branchName does not exist: git checkout -b branchName.
func EnsureEpicBranch(branchName, projectRoot string) error {
	current, err := currentBranch(projectRoot)
	if err != nil {
		return fmt.Errorf("EnsureEpicBranch: get current branch: %w", err)
	}
	if current == branchName {
		return nil
	}

	exists, err := branchExists(branchName, projectRoot)
	if err != nil {
		return fmt.Errorf("EnsureEpicBranch: check branch existence: %w", err)
	}

	if exists {
		cmd := exec.Command("git", "checkout", branchName)
		cmd.Dir = projectRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("EnsureEpicBranch: checkout %q: %w\n%s", branchName, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("EnsureEpicBranch: create branch %q: %w\n%s", branchName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(projectRoot string) (string, error) {
	return currentBranch(projectRoot)
}

// currentBranch returns the name of the currently checked-out branch.
func currentBranch(projectRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// HasUncommittedChanges reports whether the working tree has any staged or
// unstaged changes to tracked files. Untracked files are not considered.
func HasUncommittedChanges(projectRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("HasUncommittedChanges: git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// PendingPaths returns the sorted set of tracked or untracked paths currently
// reported by git status. Rename entries return the destination path.
func PendingPaths(projectRoot string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("PendingPaths: git status: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	seen := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pathField := fields[len(fields)-1]
		if len(fields) >= 4 && fields[len(fields)-2] == "->" {
			pathField = fields[len(fields)-1]
		}
		if pathField == "" {
			continue
		}
		seen[pathField] = struct{}{}
	}

	if len(seen) == 0 {
		return nil, nil
	}

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths, nil
}

// LookupCommitByGrep searches git log for the most recent commit whose message
// contains grep, returning its full SHA. Returns an empty string (no error) if
// no matching commit is found.
func LookupCommitByGrep(grep, projectRoot string) (string, error) {
	cmd := exec.Command("git", "log", "--grep="+grep, "--format=%H", "-1")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("LookupCommitByGrep: git log: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SHAExists reports whether sha refers to an object that exists in the repository.
func SHAExists(sha, projectRoot string) (bool, error) {
	cmd := exec.Command("git", "cat-file", "-e", sha)
	cmd.Dir = projectRoot
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, fmt.Errorf("SHAExists: git cat-file: %w", err)
	}
	return true, nil
}

// IsFileTracked reports whether relPath is tracked by git (i.e., committed).
// relPath must be relative to projectRoot.
func IsFileTracked(relPath, projectRoot string) (bool, error) {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", relPath)
	cmd.Dir = projectRoot
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, fmt.Errorf("IsFileTracked: git ls-files: %w", err)
	}
	return true, nil
}

// branchExists reports whether branchName exists as a local branch.
func branchExists(branchName, projectRoot string) (bool, error) {
	cmd := exec.Command("git", "branch", "--list", branchName)
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git branch --list: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// fileBackup holds the contents of a protected file for later restoration.
type fileBackup struct {
	relPath string
	data    []byte
}

// RollbackChanges performs a safe rollback of the git working tree while
// preserving files listed in protectedPaths across the reset.
//
// Steps:
//  1. Read each file in protectedPaths into memory (skip missing files).
//  2. Run git reset --hard HEAD to revert all tracked changes.
//  3. Run git clean -fd with --exclude for logs/, docs/kb/, .env, and *.backup.
//  4. Write the backed-up files back to their original locations.
//
// The clean (step 3) runs before the restore (step 4) so that protected files
// survive even when they are untracked — git clean would otherwise delete them
// if they were restored before the clean ran.
func RollbackChanges(projectRoot string, protectedPaths []string) error {
	// Step 1: read protected files into memory.
	backups := make([]fileBackup, 0, len(protectedPaths))
	for _, rel := range protectedPaths {
		data, err := os.ReadFile(filepath.Join(projectRoot, rel))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("RollbackChanges: backup %q: %w", rel, err)
		}
		backups = append(backups, fileBackup{relPath: rel, data: data})
	}

	// Step 2: git reset --hard HEAD.
	resetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	resetCmd.Dir = projectRoot
	if out, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("RollbackChanges: git reset --hard HEAD: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	// Step 3: git clean -fd with excludes.
	cleanArgs := append([]string{"clean", "-fd"}, defaultCleanExcludes...)
	cleanCmd := exec.Command("git", cleanArgs...)
	cleanCmd.Dir = projectRoot
	if out, err := cleanCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("RollbackChanges: git clean: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	// Step 4: restore protected files.
	for _, b := range backups {
		dst := filepath.Join(projectRoot, b.relPath)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("RollbackChanges: mkdir for %q: %w", b.relPath, err)
		}
		if err := os.WriteFile(dst, b.data, 0644); err != nil {
			return fmt.Errorf("RollbackChanges: restore %q: %w", b.relPath, err)
		}
	}

	return nil
}

// ResetHard rewinds the repository to the given SHA via git reset --hard <sha>.
// This is a deliberate history rewind and is intentionally separate from
// RollbackChanges (which always resets to HEAD and preserves protected files).
func ResetHard(sha, projectRoot string) error {
	cmd := exec.Command("git", "reset", "--hard", sha)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ResetHard: git reset --hard %s: %w\n%s", sha, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// HasRemoteTrackingBranch reports whether branchName has an upstream remote
// tracking branch configured (i.e. git branch --list --format has an upstream).
// Returns false (no error) if there is no upstream.
func HasRemoteTrackingBranch(branchName, projectRoot string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", branchName+"@{upstream}")
	cmd.Dir = projectRoot
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, fmt.Errorf("HasRemoteTrackingBranch: git rev-parse: %w", err)
	}
	return true, nil
}

// CurrentSHA returns the full SHA of the current HEAD commit, trimmed of whitespace.
func CurrentSHA(projectRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("CurrentSHA: git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Commit stages all changes with git add -A and creates a commit with message.
// Returns ErrNothingToCommit (non-fatal) if there is nothing to commit.
// All other errors are fatal.
func Commit(message, projectRoot string) error {
	guardedDirs, err := detectGuardedGeneratedDirs(projectRoot)
	if err != nil {
		return err
	}
	if len(guardedDirs) > 0 {
		quotedDirs := make([]string, 0, len(guardedDirs))
		for _, dir := range guardedDirs {
			quotedDirs = append(quotedDirs, fmt.Sprintf("%q", dir+"/"))
		}
		return fmt.Errorf(
			"%w: refusing to commit %s. Add the path to .gitignore or remove it from git tracking, then retry",
			ErrGuardedPath,
			strings.Join(quotedDirs, ", "),
		)
	}

	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = projectRoot
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Commit: git add -A: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = projectRoot
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "nothing added to commit") {
			return fmt.Errorf("%w", ErrNothingToCommit)
		}
		return fmt.Errorf("Commit: git commit: %w\n%s", err, strings.TrimSpace(outStr))
	}
	return nil
}

func detectGuardedGeneratedDirs(projectRoot string) ([]string, error) {
	paths, err := PendingPaths(projectRoot)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, pathField := range paths {
		if guardedDir := guardedDirForPath(pathField); guardedDir != "" {
			seen[guardedDir] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil, nil
	}

	guardedDirs := make([]string, 0, len(seen))
	for dir := range seen {
		guardedDirs = append(guardedDirs, dir)
	}
	slices.Sort(guardedDirs)
	return guardedDirs, nil
}

func guardedDirForPath(path string) string {
	if path == "" {
		return ""
	}

	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if slices.Contains(guardedGeneratedDirNames, part) {
			return strings.Join(parts[:i+1], "/")
		}
	}
	return ""
}
