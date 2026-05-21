package config

import "fmt"

// InteractionModeInteractive is the Pi-mediated interactive mode. Doug uses
// this for planning phases where an operator needs to remain in the session.
const InteractionModeInteractive = "interactive"

// InteractionModeRPC is the Pi-mediated RPC one-shot mode. Doug uses this for
// runtime, scaffold, research, and post-epic KB phases.
const InteractionModeRPC = "rpc"

// ValidateInteractionMode reports an error if mode is not a recognised
// interaction mode. Accepted values: "" (unset — resolved through phase
// defaults), InteractionModeInteractive ("interactive"), and InteractionModeRPC
// ("rpc"). Any other string is rejected so stale transport configs are caught.
func ValidateInteractionMode(mode string) error {
	switch mode {
	case "", InteractionModeInteractive, InteractionModeRPC:
		return nil
	default:
		return fmt.Errorf("unknown interaction_mode %q: valid values are %q and %q", mode, InteractionModeInteractive, InteractionModeRPC)
	}
}

// ValidatePhaseInteractionMode reports an actionable phase-scoped validation
// error. It is used when parsing phase names so operators can find the exact
// stale or unsupported entry.
func ValidatePhaseInteractionMode(phase, mode string) error {
	if err := ValidateInteractionMode(mode); err != nil {
		return fmt.Errorf("unsupported policy.phases.%s.interaction_mode %q; accepted implemented modes are %q and %q", phase, mode, InteractionModeInteractive, InteractionModeRPC)
	}
	return nil
}

// DefaultInteractionModeForPhase returns Doug's built-in interaction-mode
// default for known workflow phases when no override is configured.
func DefaultInteractionModeForPhase(phase string) string {
	switch phase {
	case "planning":
		return InteractionModeInteractive
	case "runtime", "scaffold", "research", "post_epic_kb":
		return InteractionModeRPC
	default:
		return ""
	}
}

func rejectStaleExecutionMode(executionMode *string) error {
	if executionMode == nil {
		return nil
	}
	return fmt.Errorf("stale config field execution_mode is no longer supported; use interaction_mode instead")
}
