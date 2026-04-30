package config

// PhasePolicy describes execution policy for a specific Doug workflow phase.
// Valid phase keys match the RunPhase constants in internal/agent/backend.go:
// "runtime", "planning", "scaffold", "post_epic_kb".
type PhasePolicy struct {
	ExecutionMode  string `yaml:"execution_mode,omitempty"`
	RoutingProfile string `yaml:"routing_profile,omitempty"`
}

// TaskPolicy describes execution policy for a specific task type.
// Task-level settings take precedence over phase-level settings when both are present.
// Valid task type keys: "feature", "bugfix", "documentation", "manual_review", "scaffold", "plan".
type TaskPolicy struct {
	Skill             string `yaml:"skill,omitempty"`
	ExecutionMode     string `yaml:"execution_mode,omitempty"`
	RoutingProfile    string `yaml:"routing_profile,omitempty"`
	RestrictionPolicy string `yaml:"restriction_policy,omitempty"`
}

// PolicyConfig is the execution-policy block in .doug/doug.yaml. It is the
// canonical source for skill selection, execution mode, routing profile, and
// restriction policy. Phase settings apply to all tasks in that phase; task
// settings override phase settings for matching task types.
//
// Example doug.yaml fragment:
//
//	policy:
//	  phases:
//	    runtime:
//	      execution_mode: subprocess
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

// ResolveSkill returns the skill name for the given task type. If the policy
// defines a non-empty skill for taskType, it takes precedence over fallback.
// Use the result of GetSkillForTaskType as the fallback to preserve the
// skills-config.yaml → hardcoded-default resolution chain.
func (p PolicyConfig) ResolveSkill(taskType, fallback string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.Skill != "" {
		return tp.Skill
	}
	return fallback
}

// ResolveExecutionMode returns the execution mode for a given phase and task
// type. Task-level setting overrides phase-level setting. Returns empty string
// when neither is set (caller applies its own default).
func (p PolicyConfig) ResolveExecutionMode(phase, taskType string) string {
	if tp, ok := p.Tasks[taskType]; ok && tp.ExecutionMode != "" {
		return tp.ExecutionMode
	}
	if pp, ok := p.Phases[phase]; ok && pp.ExecutionMode != "" {
		return pp.ExecutionMode
	}
	return ""
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
