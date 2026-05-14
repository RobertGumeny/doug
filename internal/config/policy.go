package config

// PhasePolicy describes execution policy for a specific Doug workflow phase.
// Valid phase keys match the RunPhase constants in internal/agent/backend.go:
// "runtime", "planning", "scaffold", "post_epic_kb".
type PhasePolicy struct {
	ExecutionMode     string   `yaml:"execution_mode,omitempty"`
	RoutingProfile    string   `yaml:"routing_profile,omitempty"`
	ToolPolicy        string   `yaml:"tool_policy,omitempty"`
	WriteScopes       []string `yaml:"write_scopes,omitempty"`
	ReadPathAdditions []string `yaml:"read_path_additions,omitempty"`
	SessionDefaults   string   `yaml:"session_defaults,omitempty"`
}

// TaskPolicy describes execution policy for a specific task type.
// Task-level settings take precedence over phase-level settings when both are present.
// Valid task type keys: "feature", "bugfix", "documentation", "scaffold", "plan".
type TaskPolicy struct {
	Skill             string   `yaml:"skill,omitempty"`
	ExecutionMode     string   `yaml:"execution_mode,omitempty"`
	RoutingProfile    string   `yaml:"routing_profile,omitempty"`
	RestrictionPolicy string   `yaml:"restriction_policy,omitempty"`
	ToolPolicy        string   `yaml:"tool_policy,omitempty"`
	WriteScopes       []string `yaml:"write_scopes,omitempty"`
	ReadPathAdditions []string `yaml:"read_path_additions,omitempty"`
	SessionDefaults   string   `yaml:"session_defaults,omitempty"`
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
// Pass the result of DefaultSkillName as the fallback.
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
	// ExecutionMode is resolved from phase and task; task overrides phase.
	// Empty string means use the backend default.
	ExecutionMode string
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
		ExecutionMode:     p.ResolveExecutionMode(phase, taskType),
		RoutingProfile:    p.ResolveRoutingProfile(phase, taskType),
		ToolPolicy:        p.ResolveToolPolicy(phase, taskType),
		WriteScopes:       p.ResolveWriteScopes(phase, taskType),
		ReadPathAdditions: p.ResolveReadPathAdditions(phase, taskType),
		SessionDefaults:   p.ResolveSessionDefaults(phase, taskType),
		RestrictionPolicy: p.ResolveRestrictionPolicy(taskType),
	}
}
