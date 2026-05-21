package agent

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

func TestPrepareExecution(t *testing.T) {
	t.Run("resolves skill from hardcoded defaults", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.SkillName != "implement-feature" {
			t.Errorf("expected implement-feature, got %q", prep.SkillName)
		}
	})

	t.Run("initial prompt is built from phase and task context", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "MY-TASK")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.InitialPrompt, "MY-TASK") {
			t.Errorf("expected task ID in prompt, got %q", prep.InitialPrompt)
		}
		if !strings.Contains(prep.InitialPrompt, "implement-feature") {
			t.Errorf("expected skill name in prompt, got %q", prep.InitialPrompt)
		}
	})

	t.Run("planning phase uses plan prompt", func(t *testing.T) {
		prep, err := PrepareExecution("planning", "plan", "PLAN-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.InitialPrompt, config.PlanPrompt) {
			t.Errorf("expected PlanPrompt in planning prompt, got %q", prep.InitialPrompt)
		}
	})

	t.Run("research phase uses research prompt", func(t *testing.T) {
		prep, err := PrepareExecution("research", "research", "RES-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prep.InitialPrompt, config.ResearchPrompt) {
			t.Errorf("expected ResearchPrompt in research prompt, got %q", prep.InitialPrompt)
		}
	})

	t.Run("runtime phase uses source-owned rpc interaction mode", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.InteractionMode != "rpc" {
			t.Errorf("expected source-owned runtime mode rpc, got %q", prep.InteractionMode)
		}
	})

	t.Run("planning phase uses source-owned interactive interaction mode", func(t *testing.T) {
		prep, err := PrepareExecution("planning", "plan", "PLAN-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.InteractionMode != "interactive" {
			t.Errorf("expected source-owned planning mode interactive, got %q", prep.InteractionMode)
		}
	})

	t.Run("default interaction mode for runtime is rpc", func(t *testing.T) {
		prep, err := PrepareExecution("runtime", "feature", "T-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prep.InteractionMode != "rpc" {
			t.Errorf("expected default rpc interaction mode, got %q", prep.InteractionMode)
		}
	})

	t.Run("returns error for unknown task type", func(t *testing.T) {
		_, err := PrepareExecution("runtime", "unknown_type", "T-1")
		if err == nil {
			t.Fatal("expected error for unknown task type, got nil")
		}
	})

	t.Run("returns error for unknown workflow phase", func(t *testing.T) {
		_, err := PrepareExecution("unknown_phase", "feature", "T-1")
		if err == nil {
			t.Fatal("expected error for unknown workflow phase, got nil")
		}
		if !strings.Contains(err.Error(), "unknown Doug workflow phase") {
			t.Fatalf("expected clear unknown phase error, got %v", err)
		}
	})
}
