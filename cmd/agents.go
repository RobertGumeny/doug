package cmd

type agentInfo struct {
	command string
}

var agentRegistry = map[string]agentInfo{
	"claude": {
		command: `claude -p "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	},
	"codex": {
		command: `codex --ask-for-approval never --sandbox workspace-write "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	},
	"gemini": {
		command: `gemini --approval-mode auto_edit --sandbox "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
	},
}
