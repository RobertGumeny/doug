package cmd

type agentInfo struct {
	command string
}

var agentRegistry = map[string]agentInfo{
	"claude": {
		command: `claude -p "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	},
	"codex": {
		command: `codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	},
	"gemini": {
		command: `gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	},
}
