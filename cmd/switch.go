package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
)

var switchFlags struct {
	list bool
}

var switchCmd = &cobra.Command{
	Use:   "switch [agent]",
	Short: "Switch to a different agent",
	Long:  "Update .doug/doug.yaml to use the specified agent's command and skills directory.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSwitch,
}

func init() {
	switchCmd.Flags().BoolVar(&switchFlags.list, "list", false, "List supported agents")
}

func runSwitch(cmd *cobra.Command, args []string) error {
	if switchFlags.list {
		names := make([]string, 0, len(agentRegistry))
		for k := range agentRegistry {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Println("Supported agents:")
		for _, name := range names {
			fmt.Printf("  %s\n", name)
		}
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("agent name required; use --list to see supported agents")
	}
	agentName := strings.ToLower(strings.TrimSpace(args[0]))

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return switchAgent(projectRoot, agentName)
}

// switchAgent updates .doug/doug.yaml in projectRoot to use the specified agent.
// It reads the existing config into a typed struct (preserving all fields), updates
// agent_command and skills_dir, then marshals back to YAML with correct quoting.
func switchAgent(projectRoot, agentName string) error {
	info, ok := agentRegistry[agentName]
	if !ok {
		names := make([]string, 0, len(agentRegistry))
		for k := range agentRegistry {
			names = append(names, k)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown agent %q; supported agents: %s", agentName, strings.Join(names, ", "))
	}

	configPath := filepath.Join(projectRoot, ".doug", "doug.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".doug/doug.yaml not found — run doug init first")
		}
		return fmt.Errorf("read .doug/doug.yaml: %w", err)
	}

	var cfg config.OrchestratorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse .doug/doug.yaml: %w", err)
	}

	cfg.AgentCommand = info.command
	cfg.SkillsDir = info.skillsDir

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal .doug/doug.yaml: %w", err)
	}

	if err := state.AtomicWrite(configPath, out); err != nil {
		return fmt.Errorf("write .doug/doug.yaml: %w", err)
	}

	log.Success(fmt.Sprintf("switched to agent %q — agent_command and skills_dir updated in .doug/doug.yaml", agentName))
	return nil
}
