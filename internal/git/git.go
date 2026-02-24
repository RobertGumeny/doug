// Package git provides Git operations for the orchestrator.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNothingToCommit is returned by Commit when there are no changes to commit.
// Callers should treat this as non-fatal.
var ErrNothingToCommit = errors.New("nothing to commit")

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
//  3. Write the backed-up files back to their original locations.
//  4. Run git clean -fd with --exclude for logs/, docs/kb/, .env, and *.backup.
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

	// Step 3: restore protected files.
	for _, b := range backups {
		dst := filepath.Join(projectRoot, b.relPath)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("RollbackChanges: mkdir for %q: %w", b.relPath, err)
		}
		if err := os.WriteFile(dst, b.data, 0644); err != nil {
			return fmt.Errorf("RollbackChanges: restore %q: %w", b.relPath, err)
		}
	}

	// Step 4: git clean -fd with excludes.
	cleanCmd := exec.Command("git", "clean", "-fd",
		"--exclude=logs/",
		"--exclude=docs/kb/",
		"--exclude=.env",
		"--exclude=*.backup",
	)
	cleanCmd.Dir = projectRoot
	if out, err := cleanCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("RollbackChanges: git clean: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// Commit stages all changes with git add -A and creates a commit with message.
// Returns ErrNothingToCommit (non-fatal) if there is nothing to commit.
// All other errors are fatal.
func Commit(message, projectRoot string) error {
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
