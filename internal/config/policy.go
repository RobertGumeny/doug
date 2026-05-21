package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// InteractionModeInteractive is the Pi-mediated interactive mode. It selects the
// PiAdapter and asks Pi to keep the run interaction open for follow-up input.
const InteractionModeInteractive = "interactive"

// InteractionModeRPC is the Pi-mediated RPC mode. Set interaction_mode: rpc in
// doug.yaml for Pi-configured projects. PiAdapter is selected when this mode is
// resolved; Pi owns model selection, tool enforcement, and agent lifecycle.
const InteractionModeRPC = "rpc"

// ValidateInteractionMode reports an error if mode is not a recognised
// interaction mode. Accepted values: "" (unset — resolved through phase
// defaults), InteractionModeInteractive ("interactive"), and InteractionModeRPC
// ("rpc"). Any other string is rejected so stale direct-subprocess configs are
// caught before backend execution.
func ValidateInteractionMode(mode string) error {
	switch mode {
	case "", InteractionModeInteractive, InteractionModeRPC:
		return nil
	default:
		return fmt.Errorf("unknown interaction_mode %q: valid values are %q and %q", mode, InteractionModeInteractive, InteractionModeRPC)
	}
}

// ValidatePhaseInteractionMode reports an actionable phase-scoped validation
// error. It is used when parsing policy.phases so operators can find the exact
// stale or unsupported phase entry in doug.yaml.
func ValidatePhaseInteractionMode(phase, mode string) error {
	if err := ValidateInteractionMode(mode); err != nil {
		return fmt.Errorf("unsupported policy.phases.%s.interaction_mode %q; accepted implemented modes are %q and %q", phase, mode, InteractionModeInteractive, InteractionModeRPC)
	}
	return nil
}

// DefaultInteractionModeForPhase returns Doug's built-in interaction-mode
// default for known workflow phases when neither task nor phase policy sets one.
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

// PhasePolicy describes execution policy for a specific Doug workflow phase.
// Valid phase keys match the RunPhase constants in internal/agent/backend.go:
// "runtime", "planning", "scaffold", "research", "post_epic_kb".
type PhasePolicy struct {
	InteractionMode   string   `yaml:"interaction_mode,omitempty"`
	RoutingProfile    string   `yaml:"routing_profile,omitempty"`
	ToolPolicy        string   `yaml:"tool_policy,omitempty"`
	WriteScopes       []string `yaml:"write_scopes,omitempty"`
	ReadPathAdditions []string `yaml:"read_path_additions,omitempty"`
	SessionDefaults   string   `yaml:"session_defaults,omitempty"`
}

func (p *PhasePolicy) UnmarshalYAML(value *yaml.Node) error {
	type phasePolicy PhasePolicy
	var raw struct {
		phasePolicy   `yaml:",inline"`
		ExecutionMode *string `yaml:"execution_mode"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	if err := rejectStaleExecutionMode(raw.ExecutionMode); err != nil {
		return err
	}
	*p = PhasePolicy(raw.phasePolicy)
	return ValidateInteractionMode(p.InteractionMode)
}

// TaskPolicy describes execution policy for a specific task type.
// Task-level settings take precedence over phase-level settings when both are present.
// Valid task type keys: "feature", "bugfix", "documentation", "scaffold", "plan", "research".
type TaskPolicy struct {
	Skill             string   `yaml:"skill,omitempty"`
	InteractionMode   string   `yaml:"interaction_mode,omitempty"`
	RoutingProfile    string   `yaml:"routing_profile,omitempty"`
	RestrictionPolicy string   `yaml:"restriction_policy,omitempty"`
	ToolPolicy        string   `yaml:"tool_policy,omitempty"`
	WriteScopes       []string `yaml:"write_scopes,omitempty"`
	ReadPathAdditions []string `yaml:"read_path_additions,omitempty"`
	SessionDefaults   string   `yaml:"session_defaults,omitempty"`
}

func (p *TaskPolicy) UnmarshalYAML(value *yaml.Node) error {
	type taskPolicy TaskPolicy
	var raw struct {
		taskPolicy    `yaml:",inline"`
		ExecutionMode *string `yaml:"execution_mode"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	if err := rejectStaleExecutionMode(raw.ExecutionMode); err != nil {
		return err
	}
	*p = TaskPolicy(raw.taskPolicy)
	return ValidateInteractionMode(p.InteractionMode)
}

// PolicyConfig is the execution-policy block in .doug/doug.yaml. It is the
// canonical source for skill selection, interaction mode, routing profile, and
// restriction policy. Phase settings apply to all tasks in that phase; task
// settings override phase settings for matching task types.
//
// Example doug.yaml fragment:
//
//	policy:
//	  phases:
//	    runtime:
//	      interaction_mode: rpc
//	      routing_profile: standard
//	  tasks:
//	    feature:
//	      skill: implement-feature
//	    bugfix:
//	      skill: implement-bugfix
type PolicyConfig struct {
	Phases map[string]PhasePolicy `yaml:"phases,omitempty"`
	Tasks  map[string]TaskPolicy  `yaml:"tasks,omitempty"`
}

func (p *PolicyConfig) UnmarshalYAML(value *yaml.Node) error {
	if err := validatePolicyNode(value); err != nil {
		return err
	}
	type policyConfig PolicyConfig
	var raw policyConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*p = PolicyConfig(raw)
	return nil
}

func validatePolicyNode(value *yaml.Node) error {
	phases := mappingNodeValue(value, "phases")
	if phases != nil {
		for i := 0; i+1 < len(phases.Content); i += 2 {
			phase := phases.Content[i].Value
			phaseNode := phases.Content[i+1]
			if executionMode := scalarValue(phaseNode, "execution_mode"); executionMode != nil {
				if phase == "planning" && *executionMode == InteractionModeRPC {
					return fmt.Errorf("stale config field policy.phases.planning.execution_mode is no longer supported; migrate to policy.phases.planning.interaction_mode: %s", InteractionModeInteractive)
				}
				return fmt.Errorf("stale config field policy.phases.%s.execution_mode is no longer supported; use policy.phases.%s.interaction_mode instead", phase, phase)
			}
			if interactionMode := scalarValue(phaseNode, "interaction_mode"); interactionMode != nil {
				if err := ValidatePhaseInteractionMode(phase, *interactionMode); err != nil {
					return err
				}
			}
		}
	}

	tasks := mappingNodeValue(value, "tasks")
	if tasks != nil {
		for i := 0; i+1 < len(tasks.Content); i += 2 {
			taskType := tasks.Content[i].Value
			taskNode := tasks.Content[i+1]
			if scalarValue(taskNode, "execution_mode") != nil {
				return fmt.Errorf("stale config field policy.tasks.%s.execution_mode is no longer supported; use policy.tasks.%s.interaction_mode instead", taskType, taskType)
			}
		}
	}
	return nil
}

func mappingNodeValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func scalarValue(node *yaml.Node, key string) *string {
	value := mappingNodeValue(node, key)
	if value == nil {
		return nil
	}
	mode := value.Value
	return &mode
}

// RequiresPi reports whether any configured phase or task uses a Pi-backed
// interaction mode. Used by preflight dependency checks to determine whether the
// pi binary must be present on PATH for CLI-backed Pi modes.
func (p PolicyConfig) RequiresPi() bool {
	for _, phase := range p.Phases {
		if isPiInteractionMode(phase.InteractionMode) {
			return true
		}
	}
	for _, task := range p.Tasks {
		if isPiInteractionMode(task.InteractionMode) {
			return true
		}
	}
	return false
}

// RequiresRPC reports whether any configured phase or task specifically uses
// rpc interaction mode. Keep this predicate for checks that require Pi's RPC
// transport, not for generic Pi CLI availability.
func (p PolicyConfig) RequiresRPC() bool {
	for _, phase := range p.Phases {
		if phase.InteractionMode == InteractionModeRPC {
			return true
		}
	}
	for _, task := range p.Tasks {
		if task.InteractionMode == InteractionModeRPC {
			return true
		}
	}
	return false
}

func isPiInteractionMode(mode string) bool {
	return mode == InteractionModeInteractive || mode == InteractionModeRPC
}

// ResolveSkill returns the skill name for the given task type. If the policy
// defines a non-empty skill for taskType, it takes precedence over fallback.
// Pass the result of DefaultSkillName as the fallback.
func (p PolicyConfig) ResolveSkill(taskType, fallback string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.Skill != "" {
		return tp.Skill
	}
	return fallback
}

// ResolveInteractionMode returns the interaction mode for a given phase and task
// type. Task-level setting overrides phase-level setting. When neither is set,
// Doug applies the built-in default for known workflow phases.
func (p PolicyConfig) ResolveInteractionMode(phase, taskType string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.InteractionMode != "" {
		return tp.InteractionMode
	}
	if pp, ok := p.Phases[phase]; ok && pp.InteractionMode != "" {
		return pp.InteractionMode
	}
	return DefaultInteractionModeForPhase(phase)
}

// ResolveRoutingProfile returns the routing profile for a given phase and task
// type. Task-level setting overrides phase-level setting. Returns empty string
// when neither is set.
func (p PolicyConfig) ResolveRoutingProfile(phase, taskType string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.RoutingProfile != "" {
		return tp.RoutingProfile
	}
	if pp, ok := p.Phases[phase]; ok && pp.RoutingProfile != "" {
		return pp.RoutingProfile
	}
	return ""
}

// ResolveRestrictionPolicy returns the restriction policy for a given task
// type. Returns empty string when no policy is configured.
func (p PolicyConfig) ResolveRestrictionPolicy(taskType string) string {
	if tp, ok := p.Tasks[taskType]; ok {
		return tp.RestrictionPolicy
	}
	return ""
}

// ResolveToolPolicy returns the tool-access policy for a given phase and task
// type. Task-level setting overrides phase-level setting. Returns empty string
// when neither is set.
func (p PolicyConfig) ResolveToolPolicy(phase, taskType string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.ToolPolicy != "" {
		return tp.ToolPolicy
	}
	if pp, ok := p.Phases[phase]; ok && pp.ToolPolicy != "" {
		return pp.ToolPolicy
	}
	return ""
}

// ResolveWriteScopes returns the merged write-scope path additions for a given
// phase and task type. Phase-level paths come first; task-level paths are
// appended. Returns nil when neither level defines any scopes.
func (p PolicyConfig) ResolveWriteScopes(phase, taskType string) []string {
	var scopes []string
	if pp, ok := p.Phases[phase]; ok {
		scopes = append(scopes, pp.WriteScopes...)
	}
	if tp, ok := p.Tasks[taskType]; ok {
		scopes = append(scopes, tp.WriteScopes...)
	}
	return scopes
}

// ResolveReadPathAdditions returns the merged read-path additions for a given
// phase and task type. Phase-level paths come first; task-level paths are
// appended. Returns nil when neither level defines any additions.
func (p PolicyConfig) ResolveReadPathAdditions(phase, taskType string) []string {
	var paths []string
	if pp, ok := p.Phases[phase]; ok {
		paths = append(paths, pp.ReadPathAdditions...)
	}
	if tp, ok := p.Tasks[taskType]; ok {
		paths = append(paths, tp.ReadPathAdditions...)
	}
	return paths
}

// ResolveSessionDefaults returns the session defaults identifier for a given
// phase and task type. Task-level setting overrides phase-level setting.
// Returns empty string when neither is set.
func (p PolicyConfig) ResolveSessionDefaults(phase, taskType string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.SessionDefaults != "" {
		return tp.SessionDefaults
	}
	if pp, ok := p.Phases[phase]; ok && pp.SessionDefaults != "" {
		return pp.SessionDefaults
	}
	return ""
}

// ResolvedExecution is the fully-resolved execution contract for one agent
// invocation. Produced by PolicyConfig.ResolveExecution before the RunRequest
// is assembled, so all policy inputs are determined by Doug rather than
// invented by the backend.
//
// Inheritance rules applied by ResolveExecution:
//   - Single-value fields: task setting overrides phase; empty string falls through.
//   - List fields (WriteScopes, ReadPathAdditions): merged additively (phase first).
type ResolvedExecution struct {
	// InteractionMode is resolved from task, phase, or built-in phase default.
	InteractionMode string
	// RoutingProfile is the resolved session routing profile; task overrides phase.
	RoutingProfile string
	// ToolPolicy is the resolved tool-access policy identifier; task overrides phase.
	ToolPolicy string
	// WriteScopes merges phase-level and task-level additional write paths.
	WriteScopes []string
	// ReadPathAdditions merges phase-level and task-level additional read paths.
	ReadPathAdditions []string
	// SessionDefaults is the resolved session defaults identifier; task overrides phase.
	SessionDefaults string
	// RestrictionPolicy is the resolved restriction policy (task-level only).
	RestrictionPolicy string
}

// ResolveExecution produces a ResolvedExecution for the given phase and task
// type by applying all inheritance and override rules in one call. Use the
// result to populate RunRequest fields before invoking the backend.
func (p PolicyConfig) ResolveExecution(phase, taskType string) ResolvedExecution {
	return ResolvedExecution{
		InteractionMode:   p.ResolveInteractionMode(phase, taskType),
		RoutingProfile:    p.ResolveRoutingProfile(phase, taskType),
		ToolPolicy:        p.ResolveToolPolicy(phase, taskType),
		WriteScopes:       p.ResolveWriteScopes(phase, taskType),
		ReadPathAdditions: p.ResolveReadPathAdditions(phase, taskType),
		SessionDefaults:   p.ResolveSessionDefaults(phase, taskType),
		RestrictionPolicy: p.ResolveRestrictionPolicy(taskType),
	}
}
