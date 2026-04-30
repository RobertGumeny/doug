package agent

import (
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

func TestPrepareExecution(t *testing.T) {
	t.Run("resolves skill from hardcoded defaults", func(t *testing.T) {
		dir := t.TempDir()
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd {{skill_name}} {{task_id}}", filepath.Join(dir, "missing.yaml"), config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "implement-feature" {
			t.Errorf("expected implement-feature, got %q", prep.SkillName)
		}
	})

	t.Run("policy overrides skill", func(t *testing.T) {
		dir := t.TempDir()
		policy := config.PolicyConfig{
			Tasks: map[string]config.TaskPolicy{
				"feature": {Skill: "custom-skill"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd {{skill_name}}", filepath.Join(dir, "missing.yaml"), policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "custom-skill" {
			t.Errorf("expected custom-skill, got %q", prep.SkillName)
		}
	})

	t.Run("substitutes placeholders in command template", func(t *testing.T) {
		dir := t.TempDir()
		prep, err := PrepareExecution("runtime", "feature", "MY-TASK", `run "{{skill_name}} {{task_id}}"`, filepath.Join(dir, "missing.yaml"), config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `run "implement-feature MY-TASK"`
		if prep.ResolvedCommand != want {
			t.Errorf("expected %q, got %q", want, prep.ResolvedCommand)
		}
	})

	t.Run("resolves execution policy from config", func(t *testing.T) {
		dir := t.TempDir()
		policy := config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {ExecutionMode: "subprocess", RoutingProfile: "standard"},
			},
		}
		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd", filepath.Join(dir, "missing.yaml"), policy)
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

	t.Run("returns error for unknown task type", func(t *testing.T) {
		dir := t.TempDir()
		_, err := PrepareExecution("runtime", "unknown_type", "T-1", "cmd", filepath.Join(dir, "missing.yaml"), config.PolicyConfig{})
		if err == nil {
			t.Fatal("expected error for unknown task type, got nil")
		}
	})

	t.Run("reads skill from skills config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		makeSkillsConfig(t, configPath, map[string]string{"feature": "repo-feature-skill"})

		prep, err := PrepareExecution("runtime", "feature", "T-1", "cmd {{skill_name}}", configPath, config.PolicyConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "repo-feature-skill" {
			t.Errorf("expected repo-feature-skill, got %q", prep.SkillName)
		}
		if prep.ResolvedCommand != "cmd repo-feature-skill" {
			t.Errorf("expected %q, got %q", "cmd repo-feature-skill", prep.ResolvedCommand)
		}
	})
}
