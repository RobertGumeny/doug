package cmd

type agentInfo struct {
	command string
}

var agentRegistry = map[string]agentInfo{
	"claude": {
		command: `claude -p "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."`,
	},
	"codex": {
		command: `codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."`,
	},
	"gemini": {
		command: `gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."`,
	},
}
