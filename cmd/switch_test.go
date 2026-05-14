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
	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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
	if !strings.Contains(cfg.RunAgentCommand, "claude") {
		t.Errorf("run_agent_command does not reference claude; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.RunAgentCommand, " -p ") {
		t.Errorf("run_agent_command should use headless claude -p mode; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.PlanAgentCommand, "claude") {
		t.Errorf("plan_agent_command does not reference claude; got: %q", cfg.PlanAgentCommand)
	}
	if strings.Contains(cfg.PlanAgentCommand, " -p ") {
		t.Errorf("plan_agent_command should be interactive, got: %q", cfg.PlanAgentCommand)
	}
	if !strings.Contains(cfg.ScaffoldAgentCommand, "claude") {
		t.Errorf("scaffold_agent_command does not reference claude; got: %q", cfg.ScaffoldAgentCommand)
	}
	if !strings.Contains(cfg.RunAgentCommand, "{{task_id}}") {
		t.Errorf("run_agent_command should include task_id placeholder; got: %q", cfg.RunAgentCommand)
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
	if !strings.Contains(cfg.RunAgentCommand, "codex") {
		t.Errorf("run_agent_command does not reference codex; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.PlanAgentCommand, "codex") {
		t.Errorf("plan_agent_command does not reference codex; got: %q", cfg.PlanAgentCommand)
	}
	if strings.Contains(cfg.PlanAgentCommand, "exec") {
		t.Errorf("plan_agent_command should not use codex exec; got: %q", cfg.PlanAgentCommand)
	}
	if !strings.Contains(cfg.RunAgentCommand, "`SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`") {
		t.Errorf("run_agent_command should state the allowed outcome values; got: %q", cfg.RunAgentCommand)
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
	if !strings.Contains(cfg.RunAgentCommand, "gemini") {
		t.Errorf("run_agent_command does not reference gemini; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.PlanAgentCommand, "gemini") {
		t.Errorf("plan_agent_command does not reference gemini; got: %q", cfg.PlanAgentCommand)
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
	if !strings.Contains(cfg.RunAgentCommand, "claude") {
		t.Errorf("expected final run_agent_command to reference claude; got: %q", cfg.RunAgentCommand)
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
	if newCfg.AgentHeartbeatSeconds != origCfg.AgentHeartbeatSeconds {
		t.Errorf("agent_heartbeat_seconds changed: want %d, got %d", origCfg.AgentHeartbeatSeconds, newCfg.AgentHeartbeatSeconds)
	}
}

func TestSwitchAgent_Pi(t *testing.T) {
	dir := setupSwitchProject(t)
	if err := switchAgent(dir, "pi"); err != nil {
		t.Fatalf("switchAgent(pi): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.OrchestratorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("resulting doug.yaml is not valid YAML: %v\ncontent:\n%s", err, data)
	}
	// Pi commands are prompt-only: no CLI binary prefix.
	if strings.Contains(cfg.RunAgentCommand, "pi ") {
		t.Errorf("pi run_agent_command should not contain a cli binary prefix; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.RunAgentCommand, "{{task_id}}") {
		t.Errorf("pi run_agent_command missing {{task_id}} placeholder; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.RunAgentCommand, "{{skill_name}}") {
		t.Errorf("pi run_agent_command missing {{skill_name}} placeholder; got: %q", cfg.RunAgentCommand)
	}
	if !strings.Contains(cfg.RunAgentCommand, "doug-orchestrated run") {
		t.Errorf("pi run_agent_command should contain doug-orchestrated run marker; got: %q", cfg.RunAgentCommand)
	}
}

func TestDougYAMLContent_Pi_HasRPCPolicy(t *testing.T) {
	content := dougYAMLContent("go", "pi", 3, 10, true)
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		t.Fatalf("dougYAMLContent(pi) produced invalid YAML: %v\ncontent:\n%s", err, content)
	}
	policy, ok := raw["policy"].(map[string]interface{})
	if !ok {
		t.Fatalf("dougYAMLContent(pi) missing policy block; content:\n%s", content)
	}
	phases, ok := policy["phases"].(map[string]interface{})
	if !ok {
		t.Fatalf("dougYAMLContent(pi) policy missing phases; content:\n%s", content)
	}
	for _, phase := range []string{"runtime", "planning", "scaffold", "research", "post_epic_kb"} {
		ph, ok := phases[phase].(map[string]interface{})
		if !ok {
			t.Errorf("dougYAMLContent(pi) policy.phases missing %q phase", phase)
			continue
		}
		if ph["execution_mode"] != "rpc" {
			t.Errorf("dougYAMLContent(pi) policy.phases.%s.execution_mode = %v; want rpc", phase, ph["execution_mode"])
		}
	}
}

func TestDougYAMLContent_NonPi_HasNoPolicy(t *testing.T) {
	for _, agent := range []string{"claude", "codex", "gemini"} {
		content := dougYAMLContent("go", agent, 3, 10, true)
		var raw map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
			t.Fatalf("dougYAMLContent(%q) produced invalid YAML: %v\ncontent:\n%s", agent, err, content)
		}
		if _, ok := raw["policy"]; ok {
			t.Errorf("dougYAMLContent(%q) should not contain a policy block; content:\n%s", agent, content)
		}
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

// TestAgentCommandSets_AllCommandsContainPlaceholders verifies that every entry in
// config.AgentCommandSets includes both {{task_id}} and {{skill_name}} template placeholders.
// config.AgentCommandSets is the canonical source for all agent command templates; this
// test documents its post-cutover contract: each registered agent must supply correctly
// wired run/plan/scaffold commands that Doug can dispatch without further transformation.
func TestAgentCommandSets_AllCommandsContainPlaceholders(t *testing.T) {
	for name, set := range config.AgentCommandSets {
		for _, command := range []string{set.Run, set.Plan, set.Scaffold} {
			if !strings.Contains(command, "{{task_id}}") {
				t.Errorf("agent %q command missing {{task_id}} placeholder: %q", name, command)
			}
			if !strings.Contains(command, "{{skill_name}}") {
				t.Errorf("agent %q command missing {{skill_name}} placeholder: %q", name, command)
			}
		}
		if !strings.Contains(set.Run, "doug-orchestrated run") {
			t.Errorf("agent %q run command should mark the run as doug-orchestrated: %q", name, set.Run)
		}
		if !strings.Contains(set.Run, ".doug/ACTIVE_TASK.md as the task brief") {
			t.Errorf("agent %q run command should explicitly route doug runs through ACTIVE_TASK.md: %q", name, set.Run)
		}
		if !strings.Contains(set.Run, "`SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`") {
			t.Errorf("agent %q run command should constrain allowed outcome values: %q", name, set.Run)
		}
		if !strings.Contains(set.Plan, ".doug/ACTIVE_TASK.md as the canonical brief for this run") {
			t.Errorf("agent %q plan command should explicitly route planning through ACTIVE_TASK.md: %q", name, set.Plan)
		}
		if !strings.Contains(set.Plan, "update .doug/plan/PLAN.md as the planning workbook described there") {
			t.Errorf("agent %q plan command should explicitly route planning work into PLAN.md: %q", name, set.Plan)
		}
	}
}

// TestDougYAMLContent_IsValidYAML ensures that dougYAMLContent produces YAML that
// gopkg.in/yaml.v3 can parse without error — i.e., mode-specific agent commands and other values
// containing special characters are correctly quoted in the template.
func TestDougYAMLContent_IsValidYAML(t *testing.T) {
	for _, bs := range []string{"go", "npm"} {
		content := dougYAMLContent(bs, "claude", 3, 10, true)
		var raw map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
			t.Errorf("dougYAMLContent(%q) produced invalid YAML: %v\ncontent:\n%s", bs, err, content)
			continue
		}
		if _, ok := raw["run_agent_command"]; !ok {
			t.Errorf("dougYAMLContent(%q): parsed YAML missing run_agent_command key", bs)
		}
		if _, ok := raw["plan_agent_command"]; !ok {
			t.Errorf("dougYAMLContent(%q): parsed YAML missing plan_agent_command key", bs)
		}
		if _, ok := raw["scaffold_agent_command"]; !ok {
			t.Errorf("dougYAMLContent(%q): parsed YAML missing scaffold_agent_command key", bs)
		}
		if _, ok := raw["research_agent_command"]; !ok {
			t.Errorf("dougYAMLContent(%q): parsed YAML missing research_agent_command key", bs)
		}
	}
}
