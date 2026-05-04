package config

import "strings"

// AgentCommandSet defines the launch command template for each Doug workflow.
type AgentCommandSet struct {
	Run      string
	Plan     string
	Scaffold string
}

const (
	RuntimePrompt = "This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there. When filling `## Agent Result.outcome`, use only `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`."
	PlanPrompt    = "This is a doug-orchestrated planning run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Read it first, then update .doug/plan/PLAN.md as the planning workbook described there. Treat PLAN.md and any handoff outputs as working artifacts, not competing canonical briefs. If the repository is empty or near-empty and the user has explicit day-0 or bootstrap intent, prefer scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic."
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
