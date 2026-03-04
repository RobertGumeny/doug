package cmd

type agentInfo struct {
	command   string
	skillsDir string
}

var agentRegistry = map[string]agentInfo{
	"claude": {
		command:   `claude -p "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
		skillsDir: ".claude/skills",
	},
	"codex": {
		command:   `codex --ask-for-approval never --sandbox workspace-write "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
		skillsDir: ".codex/skills",
	},
	"gemini": {
		command:   `gemini --approval-mode auto_edit --sandbox "Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"`,
		skillsDir: ".gemini/skills",
	},
}
