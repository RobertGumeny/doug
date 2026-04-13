package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// Simulate: select agent "2" (codex), build system "1" (go), defaults for int/bool.
	input := strings.NewReader("2\n1\n\n\n\n")
	var out bytes.Buffer

	err := runInitWorkflow(&out, input, true, dir, initWorkflowOptions{noGitInit: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if !strings.Contains(cfg.RunAgentCommand, "codex") {
		t.Errorf("expected codex in RunAgentCommand; got %q", cfg.RunAgentCommand)
	}
	if cfg.BuildSystem != "go" {
		t.Errorf("expected BuildSystem=go; got %q", cfg.BuildSystem)
	}
	// Output should contain the agent prompt.
	if !strings.Contains(out.String(), "Which agent") {
		t.Errorf("expected agent prompt in output; got: %s", out.String())
	}
}

func TestRunInitWorkflow_Interactive_ConfigPrompts(t *testing.T) {
	dir := t.TempDir()

	// Simulate: default agent (Enter), default build system (Enter),
	// maxRetries=5, maxIterations=20, kbEnabled=false.
	input := strings.NewReader("\n\n5\n20\nfalse\n")
	var out bytes.Buffer

	err := runInitWorkflow(&out, input, true, dir, initWorkflowOptions{noGitInit: true})
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

func TestSelectAgentsInteractive_SingleSelection(t *testing.T) {
	var out bytes.Buffer
	got := selectAgentsInteractive(&out, strings.NewReader("2\n"))
	if len(got) != 1 || got[0] != "codex" {
		t.Errorf("want [codex]; got %v", got)
	}
	if !strings.Contains(out.String(), "Which agent") {
		t.Error("expected prompt text in output")
	}
}

func TestSelectAgentsInteractive_MultipleSelections(t *testing.T) {
	got := selectAgentsInteractive(&bytes.Buffer{}, strings.NewReader("1,3\n"))
	if len(got) != 2 || got[0] != "claude" || got[1] != "gemini" {
		t.Errorf("want [claude gemini]; got %v", got)
	}
}

func TestSelectAgentsInteractive_EmptyInputDefaultsClaude(t *testing.T) {
	got := selectAgentsInteractive(&bytes.Buffer{}, strings.NewReader("\n"))
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("want [claude]; got %v", got)
	}
}

func TestSelectAgentsInteractive_InvalidInputDefaultsClaude(t *testing.T) {
	got := selectAgentsInteractive(&bytes.Buffer{}, strings.NewReader("99\n"))
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("want [claude]; got %v", got)
	}
}

func TestSelectAgentsInteractive_EOFDefaultsClaude(t *testing.T) {
	got := selectAgentsInteractive(&bytes.Buffer{}, strings.NewReader(""))
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("want [claude]; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// promptBuildSystemSelection
// ---------------------------------------------------------------------------

func TestPromptBuildSystemSelection_ValidChoice(t *testing.T) {
	var out bytes.Buffer
	got := promptBuildSystemSelection(&out, strings.NewReader("2\n"), "go")
	if got != "npm" {
		t.Errorf("want npm; got %q", got)
	}
	if !strings.Contains(out.String(), "Build system") {
		t.Error("expected prompt text in output")
	}
}

func TestPromptBuildSystemSelection_EmptyInputReturnsDetected(t *testing.T) {
	got := promptBuildSystemSelection(&bytes.Buffer{}, strings.NewReader("\n"), "pnpm")
	if got != "pnpm" {
		t.Errorf("want pnpm (detected default); got %q", got)
	}
}

func TestPromptBuildSystemSelection_EmptyDetectedDefaultsGo(t *testing.T) {
	got := promptBuildSystemSelection(&bytes.Buffer{}, strings.NewReader("\n"), "")
	if got != "go" {
		t.Errorf("want go (hardcoded default); got %q", got)
	}
}

func TestPromptBuildSystemSelection_OutOfRangeReturnsDefault(t *testing.T) {
	got := promptBuildSystemSelection(&bytes.Buffer{}, strings.NewReader("99\n"), "go")
	if got != "go" {
		t.Errorf("want go (default); got %q", got)
	}
}

// ---------------------------------------------------------------------------
// promptIntValue
// ---------------------------------------------------------------------------

func TestPromptIntValue_ValidInput(t *testing.T) {
	var out bytes.Buffer
	got := promptIntValue(&out, strings.NewReader("7\n"), "max_retries", 3)
	if got != 7 {
		t.Errorf("want 7; got %d", got)
	}
	if !strings.Contains(out.String(), "max_retries") {
		t.Error("expected label in output")
	}
}

func TestPromptIntValue_EmptyInputReturnsDefault(t *testing.T) {
	got := promptIntValue(&bytes.Buffer{}, strings.NewReader("\n"), "label", 5)
	if got != 5 {
		t.Errorf("want 5 (default); got %d", got)
	}
}

func TestPromptIntValue_NegativeReturnsDefault(t *testing.T) {
	got := promptIntValue(&bytes.Buffer{}, strings.NewReader("-1\n"), "label", 3)
	if got != 3 {
		t.Errorf("want 3 (default); got %d", got)
	}
}

func TestPromptIntValue_NonNumericReturnsDefault(t *testing.T) {
	got := promptIntValue(&bytes.Buffer{}, strings.NewReader("abc\n"), "label", 3)
	if got != 3 {
		t.Errorf("want 3 (default); got %d", got)
	}
}

// ---------------------------------------------------------------------------
// promptBoolValue
// ---------------------------------------------------------------------------

func TestPromptBoolValue_TrueInputs(t *testing.T) {
	for _, input := range []string{"true\n", "yes\n", "y\n", "1\n", "TRUE\n", "YES\n"} {
		got := promptBoolValue(&bytes.Buffer{}, strings.NewReader(input), "kb_enabled", false)
		if !got {
			t.Errorf("input %q: expected true", input)
		}
	}
}

func TestPromptBoolValue_FalseInputs(t *testing.T) {
	for _, input := range []string{"false\n", "no\n", "n\n", "0\n", "FALSE\n", "NO\n"} {
		got := promptBoolValue(&bytes.Buffer{}, strings.NewReader(input), "kb_enabled", true)
		if got {
			t.Errorf("input %q: expected false", input)
		}
	}
}

func TestPromptBoolValue_EmptyInputReturnsDefault(t *testing.T) {
	got := promptBoolValue(&bytes.Buffer{}, strings.NewReader("\n"), "kb_enabled", true)
	if !got {
		t.Error("expected true (default)")
	}
}

func TestPromptBoolValue_LabelShownInOutput(t *testing.T) {
	var out bytes.Buffer
	promptBoolValue(&out, strings.NewReader("\n"), "kb_enabled", true)
	if !strings.Contains(out.String(), "kb_enabled") {
		t.Errorf("expected label in output; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "[true]") {
		t.Errorf("expected default value in output; got: %s", out.String())
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
