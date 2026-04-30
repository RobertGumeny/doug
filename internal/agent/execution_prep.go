package agent

import (
	"strings"

	"github.com/robertgumeny/doug/internal/config"
)

// ExecutionPrep holds the fully resolved execution inputs for one agent
// invocation. Produced by PrepareExecution before the RunRequest is assembled.
type ExecutionPrep struct {
	SkillName       string
	ResolvedCommand string
	Exec            config.ResolvedExecution
}

// PrepareExecution resolves the skill name, applies policy overrides, resolves
// the full execution policy, and substitutes {{skill_name}} and {{task_id}}
// placeholders in commandTemplate. All policy inputs are determined here so
// the backend does not need to invent policy.
func PrepareExecution(phase, taskType, taskID, commandTemplate, skillsConfigPath string, policy config.PolicyConfig) (ExecutionPrep, error) {
	skillFallback, err := GetSkillForTaskType(taskType, skillsConfigPath)
	if err != nil {
		return ExecutionPrep{}, err
	}
	skillName := policy.ResolveSkill(taskType, skillFallback)
	exec := policy.ResolveExecution(phase, taskType)
	resolvedCmd := strings.ReplaceAll(commandTemplate, "{{skill_name}}", skillName)
	resolvedCmd = strings.ReplaceAll(resolvedCmd, "{{task_id}}", taskID)
	return ExecutionPrep{
		SkillName:       skillName,
		ResolvedCommand: resolvedCmd,
		Exec:            exec,
	}, nil
}
