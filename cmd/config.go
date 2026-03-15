package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/orchestrator"
)

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
