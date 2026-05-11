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

func TestRunInitWorkflow_NonInteractive_DefaultsToClaudeAndGo(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runInitWorkflow(&out, strings.NewReader(""), false, dir, initWorkflowOptions{
		noGitInit: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .doug/doug.yaml was created with defaults.
	cfg := loadDougConfig(t, dir)
	if !strings.Contains(cfg.RunAgentCommand, "claude") {
		t.Errorf("expected claude in RunAgentCommand; got %q", cfg.RunAgentCommand)
	}
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
	if !strings.Contains(cfg.RunAgentCommand, "claude") {
		t.Errorf("expected claude in RunAgentCommand; got %q", cfg.RunAgentCommand)
	}
	if cfg.BuildSystem != "go" {
		t.Errorf("expected BuildSystem=go; got %q", cfg.BuildSystem)
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

// ---------------------------------------------------------------------------
// configStubPrompter — test stub for Prompter
// ---------------------------------------------------------------------------

// configStubPrompter returns pre-set values for Text and Confirm (config prompts)
// and returns index-0 defaults for SelectOne (agent/build-system prompts).
type configStubPrompter struct {
	textValues []string
	textIdx    int
	boolValues []bool
	boolIdx    int
}

func (s *configStubPrompter) SelectOne(_ string, options []string, defaultIdx int) (int, string, error) {
	if len(options) == 0 {
		return 0, "", fmt.Errorf("empty options")
	}
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	return defaultIdx, options[defaultIdx], nil
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
// Cobra command entry path
// ---------------------------------------------------------------------------

// TestRunInit_CobraEntryPath exercises the real runInit cobra handler end-to-end by
// changing the working directory to a temp dir and calling runInit directly.
// This verifies that the cobra wiring, flag resolution, and workflow delegation
// all integrate correctly.
func TestRunInit_CobraEntryPath(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Set flags to non-interactive values so the test doesn't block on stdin.
	initFlags.force = false
	initFlags.buildSystem = "go"
	initFlags.agents = "claude"
	initFlags.noGitInit = true

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Spot-check that the init workflow ran and produced the expected artifacts.
	if _, err := os.Stat(filepath.Join(dir, ".doug", "doug.yaml")); err != nil {
		t.Errorf(".doug/doug.yaml not created by cobra entry path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".claude skills not created by cobra entry path: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runInitWorkflow — per-provider command routing (EPIC-18 regression)
// ---------------------------------------------------------------------------

// TestRunInitWorkflow_CodexAgent_CommandsInDougYAML verifies that selecting
// codex as the agent results in codex-specific commands in .doug/doug.yaml,
// not claude defaults.
func TestRunInitWorkflow_CodexAgent_CommandsInDougYAML(t *testing.T) {
	dir := t.TempDir()
	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		agents:    "codex",
		noGitInit: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if !strings.Contains(cfg.RunAgentCommand, "codex") {
		t.Errorf("expected codex in RunAgentCommand; got %q", cfg.RunAgentCommand)
	}
	// Verify the plan and scaffold commands are also populated.
	if cfg.PlanAgentCommand == "" {
		t.Errorf("expected non-empty PlanAgentCommand for codex agent")
	}
	if cfg.ScaffoldAgentCommand == "" {
		t.Errorf("expected non-empty ScaffoldAgentCommand for codex agent")
	}
}

// TestRunInitWorkflow_GeminiAgent_CommandsInDougYAML verifies that selecting
// gemini results in gemini-specific commands in .doug/doug.yaml.
func TestRunInitWorkflow_GeminiAgent_CommandsInDougYAML(t *testing.T) {
	dir := t.TempDir()
	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		agents:    "gemini",
		noGitInit: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if !strings.Contains(cfg.RunAgentCommand, "gemini") {
		t.Errorf("expected gemini in RunAgentCommand; got %q", cfg.RunAgentCommand)
	}
	if cfg.PlanAgentCommand == "" {
		t.Errorf("expected non-empty PlanAgentCommand for gemini agent")
	}
	if cfg.ScaffoldAgentCommand == "" {
		t.Errorf("expected non-empty ScaffoldAgentCommand for gemini agent")
	}
}
