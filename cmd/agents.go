package cmd

import "github.com/robertgumeny/doug/internal/config"

type agentInfo struct {
	runCommand      string
	planCommand     string
	scaffoldCommand string
}

var agentRegistry = map[string]agentInfo{
	"claude": {
		runCommand:      config.AgentCommandSets["claude"].Run,
		planCommand:     config.AgentCommandSets["claude"].Plan,
		scaffoldCommand: config.AgentCommandSets["claude"].Scaffold,
	},
	"codex": {
		runCommand:      config.AgentCommandSets["codex"].Run,
		planCommand:     config.AgentCommandSets["codex"].Plan,
		scaffoldCommand: config.AgentCommandSets["codex"].Scaffold,
	},
	"gemini": {
		runCommand:      config.AgentCommandSets["gemini"].Run,
		planCommand:     config.AgentCommandSets["gemini"].Plan,
		scaffoldCommand: config.AgentCommandSets["gemini"].Scaffold,
	},
}
