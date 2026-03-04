package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"gopkg.in/yaml.v3"
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

	info, ok := agentRegistry[agentName]
	if !ok {
		names := make([]string, 0, len(agentRegistry))
		for k := range agentRegistry {
			names = append(names, k)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown agent %q; supported agents: %s", agentName, strings.Join(names, ", "))
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	configPath := filepath.Join(projectRoot, ".doug", "doug.yaml")

	// Load existing config as raw YAML to preserve comments and unknown fields.
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".doug/doug.yaml not found — run doug init first")
		}
		return fmt.Errorf("read .doug/doug.yaml: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse .doug/doug.yaml: %w", err)
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}

	raw["agent_command"] = info.command
	raw["skills_dir"] = info.skillsDir

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal .doug/doug.yaml: %w", err)
	}

	if err := state.AtomicWrite(configPath, out); err != nil {
		return fmt.Errorf("write .doug/doug.yaml: %w", err)
	}

	log.Success(fmt.Sprintf("switched to agent %q — agent_command and skills_dir updated in .doug/doug.yaml", agentName))
	return nil
}
