package config_test

import (
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

// ---------------------------------------------------------------------------
// ResolveSkill tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveSkill(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		taskType string
		fallback string
		want     string
	}{
		{
			name:     "no policy — returns fallback",
			policy:   config.PolicyConfig{},
			taskType: "feature",
			fallback: "implement-feature",
			want:     "implement-feature",
		},
		{
			name: "task policy overrides fallback",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {Skill: "custom-feature-skill"},
				},
			},
			taskType: "feature",
			fallback: "implement-feature",
			want:     "custom-feature-skill",
		},
		{
			name: "task policy for different type — returns fallback",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"bugfix": {Skill: "custom-bugfix-skill"},
				},
			},
			taskType: "feature",
			fallback: "implement-feature",
			want:     "implement-feature",
		},
		{
			name: "empty skill in task policy — returns fallback",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {Skill: ""},
				},
			},
			taskType: "feature",
			fallback: "implement-feature",
			want:     "implement-feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveSkill(tt.taskType, tt.fallback)
			if got != tt.want {
				t.Errorf("ResolveSkill(%q, %q) = %q, want %q", tt.taskType, tt.fallback, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveExecutionMode tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		phase    string
		taskType string
		want     string
	}{
		{
			name:     "no policy — returns empty string",
			policy:   config.PolicyConfig{},
			phase:    "runtime",
			taskType: "feature",
			want:     "",
		},
		{
			name: "phase-level setting applies when no task override",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ExecutionMode: "subprocess"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "subprocess",
		},
		{
			name: "task-level setting overrides phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ExecutionMode: "subprocess"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {ExecutionMode: "rpc"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "rpc",
		},
		{
			name: "task override does not affect other task types",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ExecutionMode: "subprocess"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {ExecutionMode: "rpc"},
				},
			},
			phase:    "runtime",
			taskType: "bugfix",
			want:     "subprocess",
		},
		{
			name: "empty task execution mode — falls through to phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ExecutionMode: "subprocess"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {ExecutionMode: ""},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "subprocess",
		},
		{
			name: "unknown phase — returns empty string",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"planning": {ExecutionMode: "subprocess"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveExecutionMode(tt.phase, tt.taskType)
			if got != tt.want {
				t.Errorf("ResolveExecutionMode(%q, %q) = %q, want %q", tt.phase, tt.taskType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveRoutingProfile tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveRoutingProfile(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		phase    string
		taskType string
		want     string
	}{
		{
			name:     "no policy — returns empty string",
			policy:   config.PolicyConfig{},
			phase:    "runtime",
			taskType: "feature",
			want:     "",
		},
		{
			name: "phase-level routing profile applies when no task override",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {RoutingProfile: "standard"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "standard",
		},
		{
			name: "task-level routing profile overrides phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {RoutingProfile: "standard"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {RoutingProfile: "fast"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "fast",
		},
		{
			name: "empty task routing profile falls through to phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {RoutingProfile: "standard"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {RoutingProfile: ""},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveRoutingProfile(tt.phase, tt.taskType)
			if got != tt.want {
				t.Errorf("ResolveRoutingProfile(%q, %q) = %q, want %q", tt.phase, tt.taskType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveRestrictionPolicy tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveRestrictionPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		taskType string
		want     string
	}{
		{
			name:     "no policy — returns empty string",
			policy:   config.PolicyConfig{},
			taskType: "feature",
			want:     "",
		},
		{
			name: "task restriction policy returned",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {RestrictionPolicy: "strict"},
				},
			},
			taskType: "feature",
			want:     "strict",
		},
		{
			name: "unknown task type — returns empty string",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {RestrictionPolicy: "strict"},
				},
			},
			taskType: "bugfix",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveRestrictionPolicy(tt.taskType)
			if got != tt.want {
				t.Errorf("ResolveRestrictionPolicy(%q) = %q, want %q", tt.taskType, got, tt.want)
			}
		})
	}
}
