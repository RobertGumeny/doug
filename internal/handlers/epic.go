package handlers

import (
	"errors"
	"fmt"

	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/metrics"
	"github.com/robertgumeny/doug/internal/orchestrator"
)

// HandleEpicComplete processes the EPIC_COMPLETE outcome after the KB synthesis
// documentation task succeeds (or when kb_enabled is false and all feature tasks
// are DONE).
//
// Sequence:
//  1. Print epic summary (metrics table).
//  2. git add -A, then commit with the epic finalization message.
//     ErrNothingToCommit is treated as success — all changes were already
//     committed by prior task handlers.
//     Any other commit failure is a Tier 3 exit: the error is returned
//     explicitly so the caller surfaces it as a non-zero exit code (CI-6 fix).
//  3. Print the completion banner.
func HandleEpicComplete(ctx *orchestrator.LoopContext) error {
	// 1. Print the metrics summary for the completed epic.
	metrics.PrintEpicSummary(ctx.State)

	// 2. Commit any remaining changes with the finalization message.
	epicID := ctx.State.CurrentEpic.ID
	commitMsg := fmt.Sprintf("chore: finalize %s", epicID)
	if err := git.Commit(commitMsg, ctx.ProjectRoot); err != nil {
		if !errors.Is(err, git.ErrNothingToCommit) {
			// Tier 3: return an explicit error — callers must check this and
			// exit with code 1, never swallow it silently (CI-6 fix).
			return fmt.Errorf("HandleEpicComplete: git commit failed for %s: %w", epicID, err)
		}
		// Nothing to commit is non-fatal: all changes already committed by
		// the documentation task handler.
		log.Info(fmt.Sprintf("no new changes to commit for %s finalization", epicID))
	}

	// 3. Print the completion banner.
	log.Section(fmt.Sprintf("EPIC %s COMPLETE", epicID))
	log.Success(fmt.Sprintf("epic %s (%s) completed successfully",
		epicID, ctx.State.CurrentEpic.Name))

	return nil
}
