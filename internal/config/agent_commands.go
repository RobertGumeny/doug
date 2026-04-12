package config

import "strings"

// AgentCommandSet defines the launch command template for each Doug workflow.
type AgentCommandSet struct {
	Run      string
	Plan     string
	Scaffold string
}

const (
	RuntimePrompt = "This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."
	PlanPrompt    = "This is a doug-orchestrated planning run: use .doug/plan/PLAN.md as the planning workbook. Read the Doug-owned briefing at the top of PLAN.md, then help the user refine the plan and complete the workbook there."
)

var AgentCommandSets = map[string]AgentCommandSet{
	"claude": {
		Run:      `claude -p "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt + `"`,
		Plan:     `claude "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + PlanPrompt + `"`,
		Scaffold: `claude "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt + `"`,
	},
	"codex": {
		Run:      `codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt + `"`,
		Plan:     `codex "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + PlanPrompt + `"`,
		Scaffold: `codex "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt + `"`,
	},
	"gemini": {
		Run:      `gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt + `"`,
		Plan:     `gemini "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + PlanPrompt + `"`,
		Scaffold: `gemini "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt + `"`,
	},
}

func DefaultCommandSet() AgentCommandSet {
	return AgentCommandSets["claude"]
}

func CommandSetForAgent(agent string) (AgentCommandSet, bool) {
	set, ok := AgentCommandSets[strings.ToLower(strings.TrimSpace(agent))]
	return set, ok
}

func InferCommandSetFromLegacyCommand(command string) (AgentCommandSet, bool) {
	trimmed := strings.TrimSpace(command)
	for _, set := range AgentCommandSets {
		if trimmed == set.Run {
			return set, true
		}
	}
	return AgentCommandSet{}, false
}
