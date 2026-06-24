package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/orchestrator"
)

type reviewExecutor interface {
	ReviewCompletedEpic(ctx context.Context, epicID string) (string, error)
}

var newReviewExecutor = func(cfg *config.OrchestratorConfig, paths orchestrator.Paths) (reviewExecutor, error) {
	return orchestrator.New(cfg, paths)
}

var reviewCmd = &cobra.Command{
	Use:   "review <EPIC-ID>",
	Short: "Rerun the advisory review for a completed epic archive",
	Args:  cobra.ExactArgs(1),
	RunE:  runReview,
}

func runReview(cmd *cobra.Command, args []string) error {
	cfg, paths, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	reviewer, err := newReviewExecutor(cfg, paths)
	if err != nil {
		return err
	}
	artifactPath, err := reviewer.ReviewCompletedEpic(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	writef(cmd.OutOrStdout(), "review artifact written: %s\n", artifactPath)
	return nil
}
