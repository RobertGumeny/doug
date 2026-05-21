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
// ResolveInteractionMode tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveInteractionMode(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		phase    string
		taskType string
		want     string
	}{
		{
			name:     "no policy runtime — returns default rpc",
			policy:   config.PolicyConfig{},
			phase:    "runtime",
			taskType: "feature",
			want:     "rpc",
		},
		{
			name:     "no policy planning — returns default interactive",
			policy:   config.PolicyConfig{},
			phase:    "planning",
			taskType: "plan",
			want:     "interactive",
		},
		{
			name: "phase-level setting applies when no task override",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {InteractionMode: "interactive"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "interactive",
		},
		{
			name: "task-level setting overrides phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {InteractionMode: "interactive"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {InteractionMode: "rpc"},
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
					"runtime": {InteractionMode: "interactive"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {InteractionMode: "rpc"},
				},
			},
			phase:    "runtime",
			taskType: "bugfix",
			want:     "interactive",
		},
		{
			name: "empty task interaction mode — falls through to phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {InteractionMode: "interactive"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {InteractionMode: ""},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "interactive",
		},
		{
			name: "missing known phase — returns built-in default",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"planning": {InteractionMode: "interactive"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "rpc",
		},
		{
			name:     "unknown phase — returns empty string",
			policy:   config.PolicyConfig{},
			phase:    "unknown",
			taskType: "feature",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveInteractionMode(tt.phase, tt.taskType)
			if got != tt.want {
				t.Errorf("ResolveInteractionMode(%q, %q) = %q, want %q", tt.phase, tt.taskType, got, tt.want)
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

// ---------------------------------------------------------------------------
// ResolveToolPolicy tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveToolPolicy(t *testing.T) {
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
			name: "phase-level tool policy applies when no task override",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ToolPolicy: "restricted"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "restricted",
		},
		{
			name: "task-level tool policy overrides phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ToolPolicy: "restricted"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {ToolPolicy: "permissive"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "permissive",
		},
		{
			name: "empty task tool policy falls through to phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ToolPolicy: "restricted"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {ToolPolicy: ""},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "restricted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveToolPolicy(tt.phase, tt.taskType)
			if got != tt.want {
				t.Errorf("ResolveToolPolicy(%q, %q) = %q, want %q", tt.phase, tt.taskType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveWriteScopes tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveWriteScopes(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		phase    string
		taskType string
		want     []string
	}{
		{
			name:     "no policy — returns nil",
			policy:   config.PolicyConfig{},
			phase:    "runtime",
			taskType: "feature",
			want:     nil,
		},
		{
			name: "phase-level scopes only",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {WriteScopes: []string{"/tmp/a", "/tmp/b"}},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     []string{"/tmp/a", "/tmp/b"},
		},
		{
			name: "task-level scopes appended after phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {WriteScopes: []string{"/tmp/phase"}},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {WriteScopes: []string{"/tmp/task"}},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     []string{"/tmp/phase", "/tmp/task"},
		},
		{
			name: "task scopes only — no phase scopes",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {WriteScopes: []string{"/tmp/task"}},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     []string{"/tmp/task"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveWriteScopes(tt.phase, tt.taskType)
			if len(got) != len(tt.want) {
				t.Fatalf("ResolveWriteScopes(%q, %q) = %v, want %v", tt.phase, tt.taskType, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ResolveWriteScopes[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveReadPathAdditions tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveReadPathAdditions(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		phase    string
		taskType string
		want     []string
	}{
		{
			name:     "no policy — returns nil",
			policy:   config.PolicyConfig{},
			phase:    "runtime",
			taskType: "feature",
			want:     nil,
		},
		{
			name: "phase and task additions merged in order",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {ReadPathAdditions: []string{"/docs"}},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {ReadPathAdditions: []string{"/specs"}},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     []string{"/docs", "/specs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveReadPathAdditions(tt.phase, tt.taskType)
			if len(got) != len(tt.want) {
				t.Fatalf("ResolveReadPathAdditions(%q, %q) = %v, want %v", tt.phase, tt.taskType, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ResolveReadPathAdditions[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveSessionDefaults tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveSessionDefaults(t *testing.T) {
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
			name: "phase-level session defaults applies when no task override",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {SessionDefaults: "compact"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "compact",
		},
		{
			name: "task-level session defaults overrides phase",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {SessionDefaults: "compact"},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {SessionDefaults: "verbose"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want:     "verbose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveSessionDefaults(tt.phase, tt.taskType)
			if got != tt.want {
				t.Errorf("ResolveSessionDefaults(%q, %q) = %q, want %q", tt.phase, tt.taskType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveExecution tests
// ---------------------------------------------------------------------------

func TestPolicyConfig_ResolveExecution(t *testing.T) {
	tests := []struct {
		name     string
		policy   config.PolicyConfig
		phase    string
		taskType string
		want     config.ResolvedExecution
	}{
		{
			name:     "empty policy — uses phase interaction default",
			policy:   config.PolicyConfig{},
			phase:    "runtime",
			taskType: "feature",
			want:     config.ResolvedExecution{InteractionMode: "rpc"},
		},
		{
			name: "task overrides all single-value phase fields",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {
						InteractionMode: "interactive",
						RoutingProfile:  "standard",
						ToolPolicy:      "phase-tool",
						SessionDefaults: "compact",
					},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {
						InteractionMode: "rpc",
						RoutingProfile:  "fast",
						ToolPolicy:      "task-tool",
						SessionDefaults: "verbose",
					},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want: config.ResolvedExecution{
				InteractionMode: "rpc",
				RoutingProfile:  "fast",
				ToolPolicy:      "task-tool",
				SessionDefaults: "verbose",
			},
		},
		{
			name: "list fields merged from phase and task",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {
						WriteScopes:       []string{"/phase-write"},
						ReadPathAdditions: []string{"/phase-read"},
					},
				},
				Tasks: map[string]config.TaskPolicy{
					"feature": {
						WriteScopes:       []string{"/task-write"},
						ReadPathAdditions: []string{"/task-read"},
					},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want: config.ResolvedExecution{
				InteractionMode:   "rpc",
				WriteScopes:       []string{"/phase-write", "/task-write"},
				ReadPathAdditions: []string{"/phase-read", "/task-read"},
			},
		},
		{
			name: "restriction policy comes from task level only",
			policy: config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {RestrictionPolicy: "strict"},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want: config.ResolvedExecution{
				InteractionMode:   "rpc",
				RestrictionPolicy: "strict",
			},
		},
		{
			name: "phase-level values apply when no task entry exists",
			policy: config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {
						InteractionMode: "interactive",
						RoutingProfile:  "standard",
					},
				},
			},
			phase:    "runtime",
			taskType: "feature",
			want: config.ResolvedExecution{
				InteractionMode: "interactive",
				RoutingProfile:  "standard",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.ResolveExecution(tt.phase, tt.taskType)

			if got.InteractionMode != tt.want.InteractionMode {
				t.Errorf("InteractionMode = %q, want %q", got.InteractionMode, tt.want.InteractionMode)
			}
			if got.RoutingProfile != tt.want.RoutingProfile {
				t.Errorf("RoutingProfile = %q, want %q", got.RoutingProfile, tt.want.RoutingProfile)
			}
			if got.ToolPolicy != tt.want.ToolPolicy {
				t.Errorf("ToolPolicy = %q, want %q", got.ToolPolicy, tt.want.ToolPolicy)
			}
			if got.SessionDefaults != tt.want.SessionDefaults {
				t.Errorf("SessionDefaults = %q, want %q", got.SessionDefaults, tt.want.SessionDefaults)
			}
			if got.RestrictionPolicy != tt.want.RestrictionPolicy {
				t.Errorf("RestrictionPolicy = %q, want %q", got.RestrictionPolicy, tt.want.RestrictionPolicy)
			}
			if len(got.WriteScopes) != len(tt.want.WriteScopes) {
				t.Fatalf("WriteScopes = %v, want %v", got.WriteScopes, tt.want.WriteScopes)
			}
			for i := range got.WriteScopes {
				if got.WriteScopes[i] != tt.want.WriteScopes[i] {
					t.Errorf("WriteScopes[%d] = %q, want %q", i, got.WriteScopes[i], tt.want.WriteScopes[i])
				}
			}
			if len(got.ReadPathAdditions) != len(tt.want.ReadPathAdditions) {
				t.Fatalf("ReadPathAdditions = %v, want %v", got.ReadPathAdditions, tt.want.ReadPathAdditions)
			}
			for i := range got.ReadPathAdditions {
				if got.ReadPathAdditions[i] != tt.want.ReadPathAdditions[i] {
					t.Errorf("ReadPathAdditions[%d] = %q, want %q", i, got.ReadPathAdditions[i], tt.want.ReadPathAdditions[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateInteractionMode tests
// ---------------------------------------------------------------------------

func TestValidateInteractionMode(t *testing.T) {
	valid := []string{"", config.InteractionModeInteractive, config.InteractionModeRPC}
	for _, mode := range valid {
		t.Run("accepts "+modeRepr(mode), func(t *testing.T) {
			if err := config.ValidateInteractionMode(mode); err != nil {
				t.Fatalf("unexpected error for mode %q: %v", mode, err)
			}
		})
	}

	invalid := []string{"docker", "grpc", "subprocess", "SUBPROCESS", "RPC", " rpc"}
	for _, mode := range invalid {
		t.Run("rejects "+modeRepr(mode), func(t *testing.T) {
			if err := config.ValidateInteractionMode(mode); err == nil {
				t.Fatalf("expected error for mode %q, got nil", mode)
			}
		})
	}
}

func modeRepr(s string) string {
	if s == "" {
		return `empty`
	}
	return s
}
