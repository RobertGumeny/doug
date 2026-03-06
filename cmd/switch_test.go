package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/config"
)

// setupSwitchProject initialises a doug project in a temp dir and returns the dir.
func setupSwitchProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	return dir
}

func TestSwitchAgent_Claude(t *testing.T) {
	dir := setupSwitchProject(t)
	if err := switchAgent(dir, "claude"); err != nil {
		t.Fatalf("switchAgent(claude): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.OrchestratorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("resulting doug.yaml is not valid YAML: %v\ncontent:\n%s", err, data)
	}
	if !strings.Contains(cfg.AgentCommand, "claude") {
		t.Errorf("agent_command does not reference claude; got: %q", cfg.AgentCommand)
	}
	if cfg.SkillsDir != agentRegistry["claude"].skillsDir {
		t.Errorf("skills_dir mismatch; want %q, got %q", agentRegistry["claude"].skillsDir, cfg.SkillsDir)
	}
}

func TestSwitchAgent_Codex(t *testing.T) {
	dir := setupSwitchProject(t)
	if err := switchAgent(dir, "codex"); err != nil {
		t.Fatalf("switchAgent(codex): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.OrchestratorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("resulting doug.yaml is not valid YAML: %v\ncontent:\n%s", err, data)
	}
	if !strings.Contains(cfg.AgentCommand, "codex") {
		t.Errorf("agent_command does not reference codex; got: %q", cfg.AgentCommand)
	}
	if cfg.SkillsDir != agentRegistry["codex"].skillsDir {
		t.Errorf("skills_dir mismatch; want %q, got %q", agentRegistry["codex"].skillsDir, cfg.SkillsDir)
	}
}

func TestSwitchAgent_Gemini(t *testing.T) {
	dir := setupSwitchProject(t)
	if err := switchAgent(dir, "gemini"); err != nil {
		t.Fatalf("switchAgent(gemini): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.OrchestratorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("resulting doug.yaml is not valid YAML: %v\ncontent:\n%s", err, data)
	}
	if !strings.Contains(cfg.AgentCommand, "gemini") {
		t.Errorf("agent_command does not reference gemini; got: %q", cfg.AgentCommand)
	}
	if cfg.SkillsDir != agentRegistry["gemini"].skillsDir {
		t.Errorf("skills_dir mismatch; want %q, got %q", agentRegistry["gemini"].skillsDir, cfg.SkillsDir)
	}
}

// TestSwitchAgent_SubsequentSwitch verifies that a file rewritten by doug switch
// can be read and rewritten again without YAML parse errors.
func TestSwitchAgent_SubsequentSwitch(t *testing.T) {
	dir := setupSwitchProject(t)

	if err := switchAgent(dir, "codex"); err != nil {
		t.Fatalf("first switch to codex: %v", err)
	}
	if err := switchAgent(dir, "gemini"); err != nil {
		t.Fatalf("second switch to gemini: %v", err)
	}
	if err := switchAgent(dir, "claude"); err != nil {
		t.Fatalf("third switch to claude: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.OrchestratorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("doug.yaml invalid after three switches: %v\ncontent:\n%s", err, data)
	}
	if !strings.Contains(cfg.AgentCommand, "claude") {
		t.Errorf("expected final agent_command to reference claude; got: %q", cfg.AgentCommand)
	}
}

// TestSwitchAgent_PreservesOtherFields checks that fields not touched by switch
// (build_system, max_retries, etc.) survive the read-modify-write cycle.
func TestSwitchAgent_PreservesOtherFields(t *testing.T) {
	dir := setupSwitchProject(t)

	// Read original values.
	origData, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var origCfg config.OrchestratorConfig
	if err := yaml.Unmarshal(origData, &origCfg); err != nil {
		t.Fatal(err)
	}

	if err := switchAgent(dir, "codex"); err != nil {
		t.Fatalf("switchAgent: %v", err)
	}

	newData, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var newCfg config.OrchestratorConfig
	if err := yaml.Unmarshal(newData, &newCfg); err != nil {
		t.Fatal(err)
	}

	if newCfg.BuildSystem != origCfg.BuildSystem {
		t.Errorf("build_system changed: want %q, got %q", origCfg.BuildSystem, newCfg.BuildSystem)
	}
	if newCfg.MaxRetries != origCfg.MaxRetries {
		t.Errorf("max_retries changed: want %d, got %d", origCfg.MaxRetries, newCfg.MaxRetries)
	}
	if newCfg.MaxIterations != origCfg.MaxIterations {
		t.Errorf("max_iterations changed: want %d, got %d", origCfg.MaxIterations, newCfg.MaxIterations)
	}
	if newCfg.KBEnabled != origCfg.KBEnabled {
		t.Errorf("kb_enabled changed: want %v, got %v", origCfg.KBEnabled, newCfg.KBEnabled)
	}
}

func TestSwitchAgent_UnknownAgent(t *testing.T) {
	dir := setupSwitchProject(t)
	err := switchAgent(dir, "unknownbot")
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error should mention 'unknown agent'; got: %v", err)
	}
}

func TestSwitchAgent_MissingConfig(t *testing.T) {
	dir := t.TempDir() // no doug init
	err := switchAgent(dir, "claude")
	if err == nil {
		t.Fatal("expected error when doug.yaml missing, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found'; got: %v", err)
	}
}

// TestDougYAMLContent_IsValidYAML ensures that dougYAMLContent produces YAML that
// gopkg.in/yaml.v3 can parse without error — i.e., agent_command and other values
// containing special characters are correctly quoted in the template.
func TestDougYAMLContent_IsValidYAML(t *testing.T) {
	for _, bs := range []string{"go", "npm"} {
		for _, sd := range []string{".claude/skills", ".codex/skills", ".agents/skills"} {
			content := dougYAMLContent(bs, sd)
			var raw map[string]interface{}
			if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
				t.Errorf("dougYAMLContent(%q, %q) produced invalid YAML: %v\ncontent:\n%s", bs, sd, err, content)
				continue
			}
			if _, ok := raw["agent_command"]; !ok {
				t.Errorf("dougYAMLContent(%q, %q): parsed YAML missing agent_command key", bs, sd)
			}
		}
	}
}
