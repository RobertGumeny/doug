package agent

import (
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

func TestPrepareExecution(t *testing.T) {
	t.Run("resolves skill from hardcoded defaults", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd {{skill_name}} {{task_id}}", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "implement-feature" {
			t.Errorf("expected implement-feature, got %q", prep.SkillName)
		}
	})

	t.Run("policy overrides skill", func(t *testing.T) {
		policy := config.PolicyConfig{
			Tasks: map[string]config.TaskPolicy{
				"feature": {Skill: "custom-skill"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd {{skill_name}}", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "custom-skill" {
			t.Errorf("expected custom-skill, got %q", prep.SkillName)
		}
	})

	t.Run("substitutes placeholders in command template", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "MY-TASK", `run "{{skill_name}} {{task_id}}"`, config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `run "implement-feature MY-TASK"`
		if prep.ResolvedCommand != want {
			t.Errorf("expected %q, got %q", want, prep.ResolvedCommand)
		}
	})

	t.Run("resolves execution policy from config", func(t *testing.T) {
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {ExecutionMode: "subprocess", RoutingProfile: "standard"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.ExecutionMode != "subprocess" {
			t.Errorf("expected subprocess, got %q", prep.Exec.ExecutionMode)
		}
		if prep.Exec.RoutingProfile != "standard" {
			t.Errorf("expected standard, got %q", prep.Exec.RoutingProfile)
		}
	})

	t.Run("resolves rpc execution mode from task-level policy", func(t *testing.T) {
		policy := config.PolicyConfig{
			Tasks: map[string]config.TaskPolicy{
				"feature": {ExecutionMode: "rpc"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.ExecutionMode != "rpc" {
			t.Errorf("expected rpc, got %q", prep.Exec.ExecutionMode)
		}
	})

	t.Run("task-level execution mode overrides phase-level", func(t *testing.T) {
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {ExecutionMode: "subprocess"},
			},
			Tasks: map[string]config.TaskPolicy{
				"feature": {ExecutionMode: "rpc"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.ExecutionMode != "rpc" {
			t.Errorf("expected task-level rpc to override phase subprocess, got %q", prep.Exec.ExecutionMode)
		}
	})

	t.Run("empty execution mode when no policy configured", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.ExecutionMode != "" {
			t.Errorf("expected empty execution mode, got %q", prep.Exec.ExecutionMode)
		}
	})

	t.Run("returns error for unknown task type", func(t *testing.T) {
		_, err := PrepareExecution("runtime", "unknown_type", "T-1", "cmd", config.PolicyConfig{})
		if err == nil {
			t.Fatal("expected error for unknown task type, got nil")
		}
	})
}
