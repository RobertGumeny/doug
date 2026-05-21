package agent

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

func TestPrepareExecution(t *testing.T) {
	t.Run("resolves skill from hardcoded defaults", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1", config.PolicyConfig{})
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
		prep, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "custom-skill" {
			t.Errorf("expected custom-skill, got %q", prep.SkillName)
		}
	})

	t.Run("command is built from phase and task context", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "MY-TASK", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.ResolvedCommand, "MY-TASK") {
			t.Errorf("expected task ID in command, got %q", prep.ResolvedCommand)
		}
		if !strings.Contains(prep.ResolvedCommand, "implement-feature") {
			t.Errorf("expected skill name in command, got %q", prep.ResolvedCommand)
		}
	})

	t.Run("planning phase uses plan prompt", func(t *testing.T) {
		prep, err := PrepareExecution("planning", "plan", "PLAN-1", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.ResolvedCommand, config.PlanPrompt) {
			t.Errorf("expected PlanPrompt in planning command, got %q", prep.ResolvedCommand)
		}
	})

	t.Run("research phase uses research prompt", func(t *testing.T) {
		prep, err := PrepareExecution("research", "research", "RES-1", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.ResolvedCommand, config.ResearchPrompt) {
			t.Errorf("expected ResearchPrompt in research command, got %q", prep.ResolvedCommand)
		}
	})

	t.Run("resolves execution policy from config", func(t *testing.T) {
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: "subprocess", RoutingProfile: "standard"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "subprocess" {
			t.Errorf("expected subprocess, got %q", prep.Exec.InteractionMode)
		}
		if prep.Exec.RoutingProfile != "standard" {
			t.Errorf("expected standard, got %q", prep.Exec.RoutingProfile)
		}
	})

	t.Run("resolves rpc interaction mode from task-level policy", func(t *testing.T) {
		policy := config.PolicyConfig{
			Tasks: map[string]config.TaskPolicy{
				"feature": {InteractionMode: "rpc"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "rpc" {
			t.Errorf("expected rpc, got %q", prep.Exec.InteractionMode)
		}
	})

	t.Run("task-level interaction mode overrides phase-level", func(t *testing.T) {
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: "subprocess"},
			},
			Tasks: map[string]config.TaskPolicy{
				"feature": {InteractionMode: "rpc"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "rpc" {
			t.Errorf("expected task-level rpc to override phase subprocess, got %q", prep.Exec.InteractionMode)
		}
	})

	t.Run("default interaction mode when no policy configured", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "rpc" {
			t.Errorf("expected default rpc interaction mode, got %q", prep.Exec.InteractionMode)
		}
	})

	t.Run("returns error for unknown task type", func(t *testing.T) {
		_, err := PrepareExecution("runtime", "unknown_type", "T-1", config.PolicyConfig{})
		if err == nil {
			t.Fatal("expected error for unknown task type, got nil")
		}
	})

	t.Run("returns error for unknown interaction mode", func(t *testing.T) {
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: "docker"},
			},
		}
		_, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err == nil {
			t.Fatal("expected error for unknown interaction mode, got nil")
		}
	})
}
