package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/plan"
)

// runFlags holds CLI flag values that override doug.yaml config settings.
// Only flags explicitly changed by the user are applied (checked via cmd.Flags().Changed).
var runFlags struct {
	buildSystem           string
	maxRetries            int
	maxIterations         int
	kbEnabled             bool
	agentHeartbeatSeconds int
}

type runExecutor interface {
	Run(ctx context.Context) error
}

var (
	runNow         = time.Now
	runPromoteEpic = plan.PromoteEpic
	newRunExecutor = func(cfg *config.OrchestratorConfig, paths orchestrator.Paths) (runExecutor, error) {
		return orchestrator.New(cfg, paths)
	}
)

var runCmd = &cobra.Command{
	Use:   "run [EPIC-ID]",
	Short: "Run implementation work with deterministic validation",
	Long:  "Run Doug's implementation loop for the current project. Doug prepares the task brief, runs the agent in the repo, validates the result, and records the outcome. When EPIC-ID is provided, Doug first loads that planned epic into the active workspace.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOrchestrate,
}

func init() {
	runCmd.Flags().StringVar(&runFlags.buildSystem, "build-system", "", "override build_system from doug.yaml (go|npm|pnpm)")
	runCmd.Flags().IntVar(&runFlags.maxRetries, "max-retries", 0, "override max_retries from doug.yaml")
	runCmd.Flags().IntVar(&runFlags.maxIterations, "max-iterations", 0, "override max_iterations from doug.yaml")
	runCmd.Flags().BoolVar(&runFlags.kbEnabled, "kb-enabled", false, "override kb_enabled from doug.yaml")
	runCmd.Flags().IntVar(&runFlags.agentHeartbeatSeconds, "agent-heartbeat-seconds", 0, "override agent_heartbeat_seconds from doug.yaml (0 disables heartbeat)")
}

func runOrchestrate(cmd *cobra.Command, args []string) error {
	cfg, paths, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		if err := runPromoteEpic(paths.ProjectRoot, args[0], runNow()); err != nil {
			return err
		}
	}

	orch, err := newRunExecutor(cfg, paths)
	if err != nil {
		return err
	}
	return orch.Run(cmd.Context())
}
