package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/log"
	"gopkg.in/yaml.v3"
)

// agentDefaults maps agent names to their default agent_command strings.
var agentDefaults = map[string]string{
	"claude": `claude -p "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	"codex":  `codex --ask-for-approval never --sandbox workspace-write "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	"gemini": `gemini --approval-mode auto_edit --sandbox "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
}

// agentSkillDirs maps agent names to their skills_dir value.
var agentSkillDirs = map[string]string{
	"claude": ".claude/skills",
	"codex":  ".codex/skills",
	"gemini": ".gemini/skills",
}

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
		names := make([]string, 0, len(agentDefaults))
		for k := range agentDefaults {
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

	newCmd, ok := agentDefaults[agentName]
	if !ok {
		names := make([]string, 0, len(agentDefaults))
		for k := range agentDefaults {
			names = append(names, k)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown agent %q; supported agents: %s", agentName, strings.Join(names, ", "))
	}
	newSkillsDir := agentSkillDirs[agentName]

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

	raw["agent_command"] = newCmd
	raw["skills_dir"] = newSkillsDir

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal .doug/doug.yaml: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("write .doug/doug.yaml: %w", err)
	}

	log.Success(fmt.Sprintf("switched to agent %q — agent_command and skills_dir updated in .doug/doug.yaml", agentName))
	return nil
}
