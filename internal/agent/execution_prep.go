package agent

import (
	"fmt"

	"github.com/robertgumeny/doug/internal/config"
)

// ExecutionPrep holds the fully resolved execution inputs for one agent
// invocation. Produced by PrepareExecution before the RunRequest is assembled.
type ExecutionPrep struct {
	SkillName     string
	InitialPrompt string
	Exec          config.ResolvedExecution
}

// PrepareExecution resolves the skill name, applies policy overrides, resolves
// the full execution policy, and builds the initial Pi prompt from built-in
// phase constants. The prompt is not taken from config — Doug's interaction
// model is authoritative in code, not in operator-supplied templates.
func PrepareExecution(phase, taskType, taskID string, policy config.PolicyConfig) (ExecutionPrep, error) {
	skillFallback, ok := DefaultSkillName(taskType)
	if !ok {
		return ExecutionPrep{}, fmt.Errorf("unknown task type %q: no skill mapping found", taskType)
	}
	skillName := policy.ResolveSkill(taskType, skillFallback)
	exec := policy.ResolveExecution(phase, taskType)
	if exec.InteractionMode == "" {
		return ExecutionPrep{}, fmt.Errorf("unknown Doug workflow phase %q: no source-owned Pi routing is defined", phase)
	}
	if err := config.ValidateInteractionMode(exec.InteractionMode); err != nil {
		return ExecutionPrep{}, fmt.Errorf("invalid source-owned execution routing for phase %q: %w", phase, err)
	}
	return ExecutionPrep{
		SkillName:     skillName,
		InitialPrompt: config.BuildInitialPrompt(phase, taskID, skillName),
		Exec:          exec,
	}, nil
}
