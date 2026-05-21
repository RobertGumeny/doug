package agent

import (
	"fmt"

	"github.com/robertgumeny/doug/internal/config"
)

// ExecutionPrep holds the fully resolved execution inputs for one agent
// invocation. Produced by PrepareExecution before the RunRequest is assembled.
type ExecutionPrep struct {
	SkillName       string
	InitialPrompt   string
	InteractionMode string // source-owned by workflow phase
}

// PrepareExecution resolves the skill name, determines the source-owned
// interaction mode for the phase, and builds the initial Pi prompt from
// built-in phase constants. The prompt is not taken from config — Doug's
// interaction model is authoritative in code, not in operator-supplied templates.
//
// Skill names are resolved from the hardcoded DefaultSkillName map. There is
// no config-level skill override; Doug source owns the task-to-skill mapping.
func PrepareExecution(phase, taskType, taskID string) (ExecutionPrep, error) {
	skillName, ok := DefaultSkillName(taskType)
	if !ok {
		return ExecutionPrep{}, fmt.Errorf("unknown task type %q: no skill mapping found", taskType)
	}
	interactionMode := config.DefaultInteractionModeForPhase(phase)
	if interactionMode == "" {
		return ExecutionPrep{}, fmt.Errorf("unknown Doug workflow phase %q: no source-owned Pi routing is defined", phase)
	}
	return ExecutionPrep{
		SkillName:       skillName,
		InitialPrompt:   config.BuildInitialPrompt(phase, taskID, skillName),
		InteractionMode: interactionMode,
	}, nil
}
