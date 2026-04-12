package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

var revertFlags struct {
	force bool
}

var revertCmd = &cobra.Command{
	Use:   "revert <task_id>",
	Short: "Revert the repository to the commit boundary of a completed task",
	Long:  "Validate and rewind the repository to the commit SHA recorded for a DONE task.",
	Args:  cobra.ExactArgs(1),
	RunE:  runRevert,
}

func init() {
	revertCmd.Flags().BoolVar(&revertFlags.force, "force", false, "Skip uncommitted-changes check and confirmation prompt")
}

func runRevert(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return doRevert(projectRoot, strings.TrimSpace(args[0]), revertFlags.force)
}

// doRevert is the testable core of the revert subcommand.
// projectRoot is the repository root; force skips both the dirty-tree check
// and the interactive confirmation prompt.
func doRevert(projectRoot, taskID string, force bool) error {
	// Step 1: Verify .doug/ is initialized.
	dougDir := filepath.Join(projectRoot, ".doug")
	if _, err := os.Stat(dougDir); os.IsNotExist(err) {
		return fmt.Errorf(".doug/ not found — run doug init first")
	}

	statePath := filepath.Join(dougDir, "project-state.yaml")
	tasksPath := filepath.Join(dougDir, "tasks.yaml")

	// Step 2: Load project-state.yaml and tasks.yaml.
	projectState, err := state.LoadProjectState(statePath)
	if err != nil {
		return fmt.Errorf("load project-state.yaml: %w", err)
	}

	tasks, err := state.LoadTasks(tasksPath)
	if err != nil {
		return fmt.Errorf("load tasks.yaml: %w", err)
	}

	// Step 3: Verify task_id exists in tasks.
	var targetTask *types.Task
	for i := range tasks.Epic.Tasks {
		if tasks.Epic.Tasks[i].ID == taskID {
			targetTask = &tasks.Epic.Tasks[i]
			break
		}
	}
	if targetTask == nil {
		return fmt.Errorf("task %q not found in tasks.yaml — revert only applies to user-defined tasks", taskID)
	}

	// Step 4: Verify task status is DONE.
	if targetTask.Status != types.StatusDone {
		return fmt.Errorf("task %q has status %q — revert only applies to DONE tasks", taskID, targetTask.Status)
	}

	// Step 5: Look up CommitSHA from metrics, fall back to git log --grep with warning.
	var sha string
	for _, m := range projectState.Metrics.Tasks {
		if m.TaskID == taskID && m.CommitSHA != "" {
			sha = m.CommitSHA
			break
		}
	}
	if sha == "" {
		log.Warning(fmt.Sprintf("no CommitSHA recorded in metrics for task %s — falling back to git log --grep", taskID))
		sha, err = git.LookupCommitByGrep(taskID, projectRoot)
		if err != nil {
			return fmt.Errorf("git log --grep for task %s: %w", taskID, err)
		}
		if sha == "" {
			return fmt.Errorf("no commit found for task %s in git log — cannot revert without a known SHA", taskID)
		}
	}

	// Step 6: Verify SHA exists via git cat-file.
	exists, err := git.SHAExists(sha, projectRoot)
	if err != nil {
		return fmt.Errorf("verify commit SHA %s: %w", sha, err)
	}
	if !exists {
		return fmt.Errorf("commit %s does not exist in this repository — cannot revert", sha)
	}

	// Step 7: Verify project-state.yaml is git-tracked.
	tracked, err := git.IsFileTracked(filepath.Join(".doug", "project-state.yaml"), projectRoot)
	if err != nil {
		return fmt.Errorf("check git tracking for project-state.yaml: %w", err)
	}
	if !tracked {
		return fmt.Errorf(".doug/project-state.yaml is not tracked by git — commit it before reverting")
	}

	// Step 8: Check for uncommitted changes (error unless --force).
	dirty, err := git.HasUncommittedChanges(projectRoot)
	if err != nil {
		return fmt.Errorf("check for uncommitted changes: %w", err)
	}
	if dirty && !force {
		return fmt.Errorf("working tree has uncommitted changes — commit or stash them first, or use --force to skip this check")
	}

	// Step 9: Warn if current branch differs from state.CurrentEpic.BranchName.
	currentBranch, err := git.CurrentBranch(projectRoot)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	if currentBranch != projectState.CurrentEpic.BranchName {
		log.Warning(fmt.Sprintf("current branch %q differs from epic branch %q", currentBranch, projectState.CurrentEpic.BranchName))
	}

	// Step 10: Confirmation prompt unless force.
	if !force {
		writef(os.Stdout, "This will reset the repository to commit %s (task %s).\nAll commits after this point will be lost from the branch. Type 'yes' to confirm: ", sha, taskID)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if strings.TrimSpace(scanner.Text()) != "yes" {
			return fmt.Errorf("revert cancelled")
		}
	}

	// Step 11: Collect task IDs after the revert point from in-memory tasks.yaml
	// before git.ResetHard overwrites tasks.yaml on disk.
	var afterIDs []string
	revertPointPassed := false
	for _, t := range tasks.Epic.Tasks {
		if revertPointPassed {
			afterIDs = append(afterIDs, t.ID)
		}
		if t.ID == taskID {
			revertPointPassed = true
		}
	}

	// Execute: git reset --hard <sha>.
	if err := git.ResetHard(sha, projectRoot); err != nil {
		return fmt.Errorf("revert failed: %w", err)
	}

	// Delete session logs for all tasks after the revert point.
	epicID := tasks.Epic.ID
	sessionsDir := filepath.Join(dougDir, "logs", "sessions", epicID)
	for _, id := range afterIDs {
		pattern := filepath.Join(sessionsDir, "session-"+id+"_attempt-*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("glob session logs for %s: %w", id, err)
		}
		for _, match := range matches {
			if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete session log %s: %w", match, err)
			}
		}
	}

	// Print success message with short SHA.
	shortSHA := sha
	if len(sha) >= 7 {
		shortSHA = sha[:7]
	}
	log.Success(fmt.Sprintf("reverted to %s (task %s)", shortSHA, taskID))

	// Print next-steps guidance.
	writef(os.Stdout, "\nNext steps:\n  Run 'doug run' to continue from the next task after %s.\n", taskID)

	// Print force-push warning if a remote tracking branch exists.
	hasRemote, err := git.HasRemoteTrackingBranch(currentBranch, projectRoot)
	if err != nil {
		log.Warning(fmt.Sprintf("could not check remote tracking branch: %v", err))
	} else if hasRemote {
		log.Warning(fmt.Sprintf("branch %q has a remote tracking branch — history was rewritten, you must force-push:\n  git push --force-with-lease origin %s", currentBranch, currentBranch))
	}

	return nil
}
