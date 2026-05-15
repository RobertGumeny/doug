package agent

import (
	"fmt"

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
// the full execution policy, and builds the agent invocation command from
// built-in phase constants. The command is not taken from config — Doug's
// execution model is authoritative in code, not in operator-supplied templates.
func PrepareExecution(phase, taskType, taskID string, policy config.PolicyConfig) (ExecutionPrep, error) {
	skillFallback, ok := DefaultSkillName(taskType)
	if !ok {
		return ExecutionPrep{}, fmt.Errorf("unknown task type %q: no skill mapping found", taskType)
	}
	skillName := policy.ResolveSkill(taskType, skillFallback)
	exec := policy.ResolveExecution(phase, taskType)
	if err := config.ValidateExecutionMode(exec.ExecutionMode); err != nil {
		return ExecutionPrep{}, fmt.Errorf("invalid execution policy for task type %q: %w", taskType, err)
	}
	return ExecutionPrep{
		SkillName:       skillName,
		ResolvedCommand: config.BuildCommand(phase, taskID, skillName),
		Exec:            exec,
	}, nil
}
