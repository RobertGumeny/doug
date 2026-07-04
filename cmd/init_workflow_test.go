package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

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

func TestRunInitWorkflow_BuildSystemFlag_Respected(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runInitWorkflow(&out, strings.NewReader(""), false, dir, initWorkflowOptions{
		buildSystem: "npm",
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
	err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		buildSystem: "rust",
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
		noGitInit: true,
	}); err != nil {
		t.Fatalf("first run: %v", err)
	}

	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		force:     true,
		noGitInit: true,
	}); err != nil {
		t.Fatalf("second run with --force: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runInitWorkflow – interactive TTY paths (injected prompter)
// ---------------------------------------------------------------------------

func TestRunInitWorkflow_Interactive_BuildSystemPrompt(t *testing.T) {
	dir := t.TempDir()

	// Inject a non-TTY Prompter so build-system selection returns the default
	// (go) without consuming any input. Config prompts also use the shared
	// Prompter and return defaults in non-interactive mode.
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

	// Inject a stub Prompter that returns specific values for the config prompts.
	p := &configStubPrompter{
		textValues: []string{"5", "20"},
		boolValues: []bool{false},
	}

	err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), true, dir, initWorkflowOptions{
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

	for _, want := range []string{
		"max_retries — max FAILURE outcomes before a task is BLOCKED",
		"max_iterations — max orchestrator loop iterations before Doug stops",
		"kb_enabled — synthesize knowledge-base updates after feature work",
	} {
		if !containsString(append(p.textQuestions, p.confirmQuestions...), want) {
			t.Errorf("expected prompt question %q; text=%v confirm=%v", want, p.textQuestions, p.confirmQuestions)
		}
	}
}

// ---------------------------------------------------------------------------
// runInitWorkflow — no policy block emitted
// ---------------------------------------------------------------------------

// TestRunInitWorkflow_NoPolicyBlockWritten verifies that runInitWorkflow does
// not write a policy: block — execution routing is source-owned by Doug.
func TestRunInitWorkflow_NoPolicyBlockWritten(t *testing.T) {
	dir := t.TempDir()
	if err := runInitWorkflow(&bytes.Buffer{}, strings.NewReader(""), false, dir, initWorkflowOptions{
		noGitInit: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatalf("read doug.yaml: %v", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse doug.yaml: %v\ncontent:\n%s", err, data)
	}
	if _, ok := raw["policy"]; ok {
		t.Fatalf("runInitWorkflow must not write policy block; execution routing is source-owned\ncontent:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// selectBuildSystemInteractive
// ---------------------------------------------------------------------------

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
	p := &configStubPrompter{selectIdxValues: []int{1}}
	got := selectBuildSystemInteractive(p, "")
	if got != "npm" {
		t.Errorf("want npm; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// configStubPrompter — test stub for Prompter
// ---------------------------------------------------------------------------

type configStubPrompter struct {
	selectIdxValues  []int
	selectIdx        int
	selectQuestions  []string
	textValues       []string
	textIdx          int
	textQuestions    []string
	boolValues       []bool
	boolIdx          int
	confirmQuestions []string
}

func (s *configStubPrompter) SelectOne(question string, options []string, defaultIdx int) (int, string, error) {
	s.selectQuestions = append(s.selectQuestions, question)
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

func (s *configStubPrompter) Confirm(question string, defaultYes bool) (bool, error) {
	s.confirmQuestions = append(s.confirmQuestions, question)
	if s.boolIdx >= len(s.boolValues) {
		return defaultYes, nil
	}
	v := s.boolValues[s.boolIdx]
	s.boolIdx++
	return v, nil
}

func (s *configStubPrompter) Text(question, defaultVal string) (string, error) {
	s.textQuestions = append(s.textQuestions, question)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
