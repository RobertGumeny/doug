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
		if !strings.Contains(prep.ResolvedPrompt, "MY-TASK") {
			t.Errorf("expected task ID in prompt, got %q", prep.ResolvedPrompt)
		}
		if !strings.Contains(prep.ResolvedPrompt, "implement-feature") {
			t.Errorf("expected skill name in prompt, got %q", prep.ResolvedPrompt)
		}
	})

	t.Run("planning phase uses plan prompt", func(t *testing.T) {
		prep, err := PrepareExecution("planning", "plan", "PLAN-1", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.ResolvedPrompt, config.PlanPrompt) {
			t.Errorf("expected PlanPrompt in planning prompt, got %q", prep.ResolvedPrompt)
		}
	})

	t.Run("research phase uses research prompt", func(t *testing.T) {
		prep, err := PrepareExecution("research", "research", "RES-1", config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.ResolvedPrompt, config.ResearchPrompt) {
			t.Errorf("expected ResearchPrompt in research prompt, got %q", prep.ResolvedPrompt)
		}
	})

	t.Run("resolves non-mode execution policy from config", func(t *testing.T) {
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: "interactive", RoutingProfile: "standard"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "rpc" {
			t.Errorf("expected source-owned runtime mode rpc, got %q", prep.Exec.InteractionMode)
		}
		if prep.Exec.RoutingProfile != "standard" {
			t.Errorf("expected standard, got %q", prep.Exec.RoutingProfile)
		}
	})

	t.Run("task-level interaction mode cannot change source-owned planning mode", func(t *testing.T) {
		policy := config.PolicyConfig{
			Tasks: map[string]config.TaskPolicy{
				"plan": {InteractionMode: "rpc"},
			},
		}
		prep, err := PrepareExecution("planning", "plan", "PLAN-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "interactive" {
			t.Errorf("expected source-owned planning mode interactive, got %q", prep.Exec.InteractionMode)
		}
	})

	t.Run("task-level interaction mode cannot change source-owned runtime mode", func(t *testing.T) {
		policy := config.PolicyConfig{
			Tasks: map[string]config.TaskPolicy{
				"feature": {InteractionMode: "interactive"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.Exec.InteractionMode != "rpc" {
			t.Errorf("expected source-owned runtime mode rpc, got %q", prep.Exec.InteractionMode)
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

	t.Run("returns error for unknown workflow phase", func(t *testing.T) {
		_, err := PrepareExecution("unknown_phase", "feature", "T-1", config.PolicyConfig{})
		if err == nil {
			t.Fatal("expected error for unknown workflow phase, got nil")
		}
		if !strings.Contains(err.Error(), "unknown Doug workflow phase") {
			t.Fatalf("expected clear unknown phase error, got %v", err)
		}
	})
}
