package cmd

import (
	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/orchestrator"
)

// runFlags holds CLI flag values that override doug.yaml config settings.
// Only flags explicitly changed by the user are applied (checked via cmd.Flags().Changed).
var runFlags struct {
	agentCommand          string
	buildSystem           string
	maxRetries            int
	maxIterations         int
	kbEnabled             bool
	agentHeartbeatSeconds int
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the orchestration loop",
	Long:  "Run the orchestration loop, executing tasks defined in .doug/tasks.yaml.",
	RunE:  runOrchestrate,
}

func init() {
	runCmd.Flags().StringVar(&runFlags.agentCommand, "agent", "", "override agent_command from doug.yaml")
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
	orch, err := orchestrator.New(cfg, paths)
	if err != nil {
		return err
	}
	return orch.Run(cmd.Context())
}
