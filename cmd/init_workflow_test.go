package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/interactive"
)

// ---------------------------------------------------------------------------
// runInitWorkflow – non-interactive paths
// ---------------------------------------------------------------------------

func TestRunInitWorkflow_NonInteractive_DefaultsToGo(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runInitWorkflow(&out, strings.NewReader(""), false, dir, initWorkflowOptions{
		noGitInit: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .doug/doug.yaml was created with expected defaults.
	cfg := loadDougConfig(t, dir)
	if cfg.BuildSystem != "go" {
		t.Errorf("expected BuildSystem=go (default); got %q", cfg.BuildSystem)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3 (default); got %d", cfg.MaxRetries)
	}
	if cfg.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10 (default); got %d", cfg.MaxIterations)
	}
	if !cfg.KBEnabled {
		t.Error("expected KBEnabled=true (default)")
	}
}

func TestRunInitWorkflow_AgentsFlag_Parsed(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runInitWorkflow(&out, strings.NewReader(""), false, dir, initWorkflowOptions{
		agents:    "claude,codex",
		noGitInit: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both agent directories should have skills.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".claude skills not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".codex skills not created: %v", err)
	}
}

func TestRunInitWorkflow_BuildSystemFlag_Respected(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runInitWorkflow(&out, strings.NewReader(""), false, dir, initWorkflowOptions{
		buildSystem: "npm",
		agents:      "claude",
		noGitInit:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if cfg.BuildSystem != "npm" {
		t.Errorf("expected BuildSystem=npm; got %q", cfg.BuildSystem)
	}
}

func TestRunInitWorkflow_InvalidBuildSystemFlag_Errors(t *testing.T) {
	dir := t.TempDir()
	// doInitProject validates the build system.
	err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		buildSystem: "rust",
		agents:      "claude",
		noGitInit:   true,
	})
	if err == nil {
		t.Fatal("expected error for unsupported build system")
	}
	if !strings.Contains(err.Error(), "rust") {
		t.Errorf("error should mention the invalid value; got: %v", err)
	}
}

func TestRunInitWorkflow_ForceFlag_Overwrites(t *testing.T) {
	dir := t.TempDir()
	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		agents: "claude", noGitInit: true,
	}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		force: true, agents: "claude", noGitInit: true,
	}); err != nil {
		t.Fatalf("second run with --force: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runInitWorkflow – interactive TTY paths (injected reader)
// ---------------------------------------------------------------------------

func TestRunInitWorkflow_Interactive_AgentAndBuildSystemPrompts(t *testing.T) {
	dir := t.TempDir()

	// Inject a non-TTY Prompter so agent and build-system selection both return
	// defaults (claude and go) without consuming any input. Config prompts also
	// use the shared Prompter and return defaults in non-interactive mode.
	var out bytes.Buffer
	p := interactive.NewWithIO(&out, strings.NewReader(""), false)

	err := runInitWorkflow(&out, strings.NewReader(""), true, dir, initWorkflowOptions{
		noGitInit: true,
		prompter:  p,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if cfg.BuildSystem != "go" {
		t.Errorf("expected BuildSystem=go; got %q", cfg.BuildSystem)
	}
	// Config prompts are called (isTTY=true) but the non-TTY prompter returns
	// defaults, so all three config values must equal the hardcoded defaults.
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3 (default); got %d", cfg.MaxRetries)
	}
	if cfg.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10 (default); got %d", cfg.MaxIterations)
	}
	if !cfg.KBEnabled {
		t.Error("expected KBEnabled=true (default)")
	}
}

func TestRunInitWorkflow_Interactive_ConfigPrompts(t *testing.T) {
	dir := t.TempDir()

	// Inject a stub Prompter that returns defaults for agent/build-system selection
	// (SelectOne) and specific values for the three config prompts (Text, Confirm).
	p := &configStubPrompter{
		textValues: []string{"5", "20"},
		boolValues: []bool{false},
	}

	err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), true, dir, initWorkflowOptions{
		agents:    "claude", // bypass interactive agent selection to isolate config prompts
		noGitInit: true,
		prompter:  p,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5; got %d", cfg.MaxRetries)
	}
	if cfg.MaxIterations != 20 {
		t.Errorf("expected MaxIterations=20; got %d", cfg.MaxIterations)
	}
	if cfg.KBEnabled {
		t.Error("expected KBEnabled=false")
	}
}

// ---------------------------------------------------------------------------
// selectAgentsInteractive
// ---------------------------------------------------------------------------

// All tests use NewWithIO with isTTY=false (the fallbackPrompter path), which
// returns defaults without reading from the reader — consistent with the
// internal/interactive test convention.

func TestSelectAgentsInteractive_DefaultsToClaudeOnNonTTY(t *testing.T) {
	p := interactive.NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got := selectAgentsInteractive(p)
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("want [claude]; got %v", got)
	}
}

func TestSelectAgentsInteractive_NoAdditionalAgentsWhenConfirmDefaultIsFalse(t *testing.T) {
	// Non-TTY Confirm always returns false (the default), so only the primary
	// agent (claude) is included even though additional agents exist.
	p := interactive.NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got := selectAgentsInteractive(p)
	if len(got) != 1 {
		t.Errorf("want single agent; got %v", got)
	}
}

func TestSelectAgentsInteractive_SelectsNonDefaultPrimaryAgent(t *testing.T) {
	// Index 2 in the options list is "gemini".
	p := &configStubPrompter{selectIdxValues: []int{2}}
	got := selectAgentsInteractive(p)
	if len(got) != 1 || got[0] != "gemini" {
		t.Errorf("want [gemini]; got %v", got)
	}
}

func TestSelectAgentsInteractive_SelectsMultipleAgents(t *testing.T) {
	// Select claude (index 0) as primary, confirm codex, decline gemini.
	p := &configStubPrompter{
		selectIdxValues: []int{0},
		boolValues:      []bool{true, false},
	}
	got := selectAgentsInteractive(p)
	if len(got) != 2 || got[0] != "claude" || got[1] != "codex" {
		t.Errorf("want [claude codex]; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// selectBuildSystemInteractive
// ---------------------------------------------------------------------------

// All tests use NewWithIO with isTTY=false (the fallbackPrompter path), which
// returns defaults without reading from the reader.

func TestSelectBuildSystemInteractive_DefaultsToGoWhenNoDetected(t *testing.T) {
	p := interactive.NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got := selectBuildSystemInteractive(p, "")
	if got != "go" {
		t.Errorf("want go (fallback default); got %q", got)
	}
}

func TestSelectBuildSystemInteractive_UsesDetectedAsDefault(t *testing.T) {
	p := interactive.NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got := selectBuildSystemInteractive(p, "pnpm")
	if got != "pnpm" {
		t.Errorf("want pnpm (detected default); got %q", got)
	}
}

func TestSelectBuildSystemInteractive_UnknownDetectedFallsBackToGo(t *testing.T) {
	// A detected value not in the options list falls back to index 0 ("go").
	p := interactive.NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got := selectBuildSystemInteractive(p, "rust")
	if got != "go" {
		t.Errorf("want go (index-0 fallback); got %q", got)
	}
}

func TestSelectBuildSystemInteractive_NpmDetectedReturnsNpm(t *testing.T) {
	p := interactive.NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got := selectBuildSystemInteractive(p, "npm")
	if got != "npm" {
		t.Errorf("want npm; got %q", got)
	}
}

func TestSelectBuildSystemInteractive_SelectsNonDefaultSystem(t *testing.T) {
	// Index 1 in the options list is "npm"; defaultIdx is 0 ("go") when nothing
	// is detected. The stub overrides the selection to index 1.
	p := &configStubPrompter{selectIdxValues: []int{1}}
	got := selectBuildSystemInteractive(p, "")
	if got != "npm" {
		t.Errorf("want npm; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// configStubPrompter — test stub for Prompter
// ---------------------------------------------------------------------------

// configStubPrompter returns pre-set values for SelectOne, Text, and Confirm.
// selectIdxValues controls which index SelectOne returns for each call (in
// order); when exhausted, the default index is returned. textValues and
// boolValues control Text and Confirm responses the same way.
type configStubPrompter struct {
	selectIdxValues []int
	selectIdx       int
	textValues      []string
	textIdx         int
	boolValues      []bool
	boolIdx         int
}

func (s *configStubPrompter) SelectOne(_ string, options []string, defaultIdx int) (int, string, error) {
	if len(options) == 0 {
		return 0, "", fmt.Errorf("empty options")
	}
	idx := defaultIdx
	if s.selectIdx < len(s.selectIdxValues) {
		idx = s.selectIdxValues[s.selectIdx]
		s.selectIdx++
	}
	if idx < 0 || idx >= len(options) {
		idx = defaultIdx
	}
	return idx, options[idx], nil
}

func (s *configStubPrompter) Confirm(_ string, defaultYes bool) (bool, error) {
	if s.boolIdx >= len(s.boolValues) {
		return defaultYes, nil
	}
	v := s.boolValues[s.boolIdx]
	s.boolIdx++
	return v, nil
}

func (s *configStubPrompter) Text(_, defaultVal string) (string, error) {
	if s.textIdx >= len(s.textValues) {
		return defaultVal, nil
	}
	v := s.textValues[s.textIdx]
	s.textIdx++
	return v, nil
}

func (s *configStubPrompter) Compose(_, defaultVal string) (string, error) {
	return defaultVal, nil
}

// ---------------------------------------------------------------------------
// promptConfigInt
// ---------------------------------------------------------------------------

func TestPromptConfigInt_ValidInput(t *testing.T) {
	p := &configStubPrompter{textValues: []string{"7"}}
	got := promptConfigInt(p, "max_retries", 3)
	if got != 7 {
		t.Errorf("want 7; got %d", got)
	}
}

func TestPromptConfigInt_EmptyInputReturnsDefault(t *testing.T) {
	p := &configStubPrompter{textValues: []string{""}}
	got := promptConfigInt(p, "label", 5)
	if got != 5 {
		t.Errorf("want 5 (default); got %d", got)
	}
}

func TestPromptConfigInt_NegativeReturnsDefault(t *testing.T) {
	p := &configStubPrompter{textValues: []string{"-1"}}
	got := promptConfigInt(p, "label", 3)
	if got != 3 {
		t.Errorf("want 3 (default); got %d", got)
	}
}

func TestPromptConfigInt_NonNumericReturnsDefault(t *testing.T) {
	p := &configStubPrompter{textValues: []string{"abc"}}
	got := promptConfigInt(p, "label", 3)
	if got != 3 {
		t.Errorf("want 3 (default); got %d", got)
	}
}

func TestPromptConfigInt_NoInputReturnsDefault(t *testing.T) {
	p := &configStubPrompter{}
	got := promptConfigInt(p, "label", 5)
	if got != 5 {
		t.Errorf("want 5 (default); got %d", got)
	}
}

// ---------------------------------------------------------------------------
// runInitWorkflow — Pi RPC policy is agent-dependent
// ---------------------------------------------------------------------------

// TestRunInitWorkflow_PiPrimaryWritesRPCPolicy verifies that selecting "pi" as
// the primary agent causes runInitWorkflow to write execution_mode: rpc for all
// phases into .doug/doug.yaml. This is the documented Pi activation path.
func TestRunInitWorkflow_PiPrimaryWritesRPCPolicy(t *testing.T) {
	dir := t.TempDir()
	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		agents:    "pi",
		noGitInit: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	for _, phase := range []string{"runtime", "planning", "scaffold", "research", "post_epic_kb"} {
		if cfg.Policy.Phases[phase].ExecutionMode != "rpc" {
			t.Errorf("pi primary: policy.phases.%s.execution_mode = %q; want rpc", phase, cfg.Policy.Phases[phase].ExecutionMode)
		}
	}
}

// TestRunInitWorkflow_NonPiPrimaryNoRPCPolicy verifies that non-Pi primary agents
// do NOT write execution_mode: rpc into .doug/doug.yaml. Those projects use the
// default subprocess backend.
func TestRunInitWorkflow_NonPiPrimaryNoRPCPolicy(t *testing.T) {
	for _, agent := range []string{"claude", "codex", "gemini"} {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
				agents:    agent,
				noGitInit: true,
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			cfg := loadDougConfig(t, dir)
			for _, phase := range []string{"runtime", "planning", "scaffold", "research", "post_epic_kb"} {
				if cfg.Policy.Phases[phase].ExecutionMode == "rpc" {
					t.Errorf("agent=%q: policy.phases.%s.execution_mode = rpc; must not be set for non-Pi primary", agent, phase)
				}
			}
		})
	}
}
