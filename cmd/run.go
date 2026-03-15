package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
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

// loadConfig resolves the project root, derives all paths, loads doug.yaml,
// and applies any CLI flag overrides. It returns the merged config and the
// resolved path layout.
func loadConfig(cmd *cobra.Command) (*config.OrchestratorConfig, orchestrator.Paths, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, orchestrator.Paths{}, fmt.Errorf("get working directory: %w", err)
	}

	paths := orchestrator.NewPaths(projectRoot)

	cfg, err := config.LoadConfig(paths.ConfigPath)
	if err != nil {
		return nil, orchestrator.Paths{}, fmt.Errorf("load config: %w", err)
	}

	// Apply CLI flag overrides — only when the user explicitly set the flag.
	if cmd.Flags().Changed("agent") {
		cfg.AgentCommand = runFlags.agentCommand
	}
	if cmd.Flags().Changed("build-system") {
		cfg.BuildSystem = runFlags.buildSystem
	}
	if cmd.Flags().Changed("max-retries") {
		cfg.MaxRetries = runFlags.maxRetries
	}
	if cmd.Flags().Changed("max-iterations") {
		cfg.MaxIterations = runFlags.maxIterations
	}
	if cmd.Flags().Changed("kb-enabled") {
		cfg.KBEnabled = runFlags.kbEnabled
	}
	if cmd.Flags().Changed("agent-heartbeat-seconds") {
		cfg.AgentHeartbeatSeconds = runFlags.agentHeartbeatSeconds
	}

	return cfg, paths, nil
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
